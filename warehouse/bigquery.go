package warehouse

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
)

var (
	BigQueryTypeMap = FieldTypeMapper{
		"int64":     "BIGINT",
		"string":    "VARCHAR(max)",
		"time.Time": "TIMESTAMP",
	}
)

type BigQuery struct {
	conf     *config.Config
	ctx      context.Context
	bqClient *bigquery.Client
}

var _ Warehouse = &BigQuery{}

func NewBigQuery(c *config.Config) *BigQuery {
	if c.GCS.GCSOnly {
		log.Printf("Config flag GCSOnly is on, data will not be loaded to BigQuery")
	}

	return &BigQuery{
		conf: c,
	}
}

// GetExportTableColumns returns a slice of the columns in the existing export table
func (bq *BigQuery) GetExportTableColumns() []string {
	if err := bq.connectToBQ(); err != nil {
		log.Fatal(err)
	}
	defer bq.bqClient.Close()

	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(bq.conf.BigQuery.ExportTable)
	md, err := table.Metadata(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var columns []string
	for _, f := range md.Schema {
		columns = append(columns, strings.ToLower(f.Name))
	}
	return columns
}

func (bq *BigQuery) LastSyncPoint() (time.Time, error) {
	t := beginningOfTime

	if err := bq.connectToBQ(); err != nil {
		return t, err
	}
	defer bq.bqClient.Close()

	if !bq.doesTableExist(bq.conf.BigQuery.SyncTable) {
		err := bq.createSyncTable()
		if err != nil {
			log.Printf("Could not create sync table: %s", err)
		}
		return t, err
	}

	q := fmt.Sprintf("SELECT max(BundleEndTime) FROM %s.%s;", bq.conf.BigQuery.Dataset, bq.conf.BigQuery.SyncTable)
	t, err := bq.fetchTimeVal(q)
	if err != nil {
		log.Printf("Couldn't get max(BundleEndTime): %s", err)
		return t, err
	}

	if !bq.conf.GCS.GCSOnly {
		// Find the time of the latest export record...if it's after
		// the time in the sync table, then there must have been a failure
		// after some records have been loaded, but before the sync record
		// was written. Use this as the latest sync time, and don't load
		// any records before this point to prevent duplication
		exportTime, err := bq.latestEventStart()
		if err != nil {
			log.Printf("Couldn't get max(EventStart): %s", err)
			return t, err
		}

		if !exportTime.IsZero() && exportTime.After(t) {
			// Partitioned tables cannot be dropped, so loading must restart with the first bundle of the day on
			// which leftover records were found.  The last sync point should be backtracked to the first instant of the day.
			// Data "cleanup" will occur on load, as the first bundle of the day always uses WRITE_TRUNCATE
			log.Printf("Export record timestamp after sync time (%s vs %s); starting from beginning of the day", exportTime, t)
			t = t.Truncate(24 * time.Hour)
			if err := bq.removeSyncPointsAfter(t); err != nil {
				return t, err
			}
		}
	}

	return t, nil
}

func (bq *BigQuery) SaveSyncPoints(bundles ...client.ExportMeta) error {
	if err := bq.connectToBQ(); err != nil {
		return err
	}
	defer bq.bqClient.Close()

	values := make([]string, len(bundles))
	for i, p := range bundles {
		values[i] = fmt.Sprintf("(%d, TIMESTAMP(\"%s\"), TIMESTAMP(\"%s\"))", p.ID, time.Now().UTC().Format(time.RFC3339), p.Stop.UTC().Format(time.RFC3339))
	}

	// BQ supports inserting multiple records at once
	q := fmt.Sprintf("INSERT INTO %s.%s (ID, Processed, BundleEndtime) VALUES %s;", bq.conf.BigQuery.Dataset, bq.conf.BigQuery.SyncTable, strings.Join(values, ","))
	log.Printf("Save SQL: %s", q)
	query := bq.bqClient.Query(q)
	query.QueryConfig.UseStandardSQL = true

	job, err := query.Run(bq.ctx)
	if err != nil {
		log.Printf("Failed to start job to save sync point %s", err)
		return err
	}

	return bq.waitForJob(job)
}

func (bq *BigQuery) LoadToWarehouse(objName string, bundles ...client.ExportMeta) error {
	if err := bq.connectToBQ(); err != nil {
		return err
	}
	defer bq.bqClient.Close()

	// create loader to load from file into export table
	gcsURI := fmt.Sprintf("gs://%s/%s", bq.conf.GCS.Bucket, objName)
	gcsRef := bigquery.NewGCSReference(gcsURI) // defaults to CSV
	gcsRef.FileConfig.IgnoreUnknownValues = true
	gcsRef.AllowJaggedRows = true
	start := bundles[0].Start.UTC()
	partitionTable := bq.conf.BigQuery.ExportTable + "$" + start.Format("20060102")
	log.Printf("Loading GCS file: %s into table %s", gcsURI, partitionTable)

	loader := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(partitionTable).LoaderFrom(gcsRef)
	loader.CreateDisposition = bigquery.CreateNever
	if start.Equal(start.Truncate(24 * time.Hour)) {
		// this is the first file of the partition, truncate the partition in case there is leftover data from previous failed loads
		log.Printf("Detected first bundle of the day (start: %s), using WriteTruncate to replace any existing data in partition", start)
		loader.WriteDisposition = bigquery.WriteTruncate
	}

	// start and wait on loading job
	job, err := loader.Run(bq.ctx)
	if err != nil {
		log.Printf("Could not start BQ load job for file %s", objName)
		return err
	}

	return bq.waitForJob(job)
}

// EnsureCompatibleExportTable ensures that the all the fields present in the hauser schema are present in the BigQuery table schema
// If the table exists, it compares the schema of the export table in BigQuery to the schema in hauser and adds any missing fields
// if the table doesn't exist, it creates a new export table with all the fields specified in hauser
func (bq *BigQuery) EnsureCompatibleExportTable() error {
	// Get Hauser Schema
	// this is required if we create a new table or if we have to compare to the existing table schema
	hauserSchema, err := bigquery.InferSchema(BundleEvent{})
	if err != nil {
		return err
	}

	if err := bq.connectToBQ(); err != nil {
		log.Fatal(err)
	}
	defer bq.bqClient.Close()

	if !bq.doesTableExist(bq.conf.BigQuery.ExportTable) {
		// Table does not exist, create new table
		log.Printf("Export table does not exist, creating one.")
		if err := bq.createExportTable(hauserSchema); err != nil {
			return err
		}
		return nil
	}

	log.Printf("Export table exists, making sure the schema in BigQuery is compatible with the schema specified in Hauser")

	// get current table schema in BigQuery
	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(bq.conf.BigQuery.ExportTable)
	md, err := table.Metadata(context.Background())
	if err != nil {
		return err
	}

	needsUpdate := false
	// Find the fields from the hauser schema that are missing from the BiqQuery table
	missingFields := bq.GetMissingFields(hauserSchema, md.Schema)
	// If fields are missing, we add them to the table schema
	update := bigquery.TableMetadataToUpdate{}
	if len(missingFields) > 0 {
		// Append missing fields to export table schema
		update.Schema = append(md.Schema, missingFields...)
		needsUpdate = true
	}

	if md.TimePartitioning.Expiration != bq.conf.BigQuery.PartitionExpiration.Duration {
		update.TimePartitioning = &bigquery.TimePartitioning{
			Expiration: bq.conf.BigQuery.PartitionExpiration.Duration,
			Field:      md.TimePartitioning.Field,
		}
		needsUpdate = true
	}

	if needsUpdate {
		if _, err := table.Update(bq.ctx, update, md.ETag); err != nil {
			return nil
		}
	}
	return nil
}

// GetMissingFields returns all fields that are present in the hauserSchema, but not in the bqSchema
func (bq *BigQuery) GetMissingFields(hauserSchema, bqSchema bigquery.Schema) []*bigquery.FieldSchema {
	bqSchemaMap := makeSchemaMap(bqSchema)
	var missingFields []*bigquery.FieldSchema
	for _, f := range hauserSchema {
		if _, ok := bqSchemaMap[strings.ToLower(f.Name)]; !ok {
			f.Required = false
			missingFields = append(missingFields, f)
		}
	}

	return missingFields
}

func makeSchemaMap(schema bigquery.Schema) map[string]struct{} {
	schemaMap := make(map[string]struct{})
	for _, f := range schema {
		schemaMap[strings.ToLower(f.Name)] = struct{}{}
	}
	return schemaMap
}

func (bq *BigQuery) ValueToString(val interface{}, isTime bool) string {
	return valueToString(val, isTime)
}

func (bq *BigQuery) connectToBQ() error {
	var err error
	bq.ctx = context.Background()
	bq.bqClient, err = bigquery.NewClient(bq.ctx, bq.conf.BigQuery.Project)
	if err != nil {
		log.Printf("Could not connect to BigQuery: %s", err)
		return err
	}
	return nil
}

func (bq *BigQuery) doesTableExist(name string) bool {
	log.Printf("Checking if table %s exists", name)
	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(name)
	if _, err := table.Metadata(bq.ctx); err != nil {
		return false
	}

	return true
}

func (bq *BigQuery) createSyncTable() error {
	log.Printf("Creating table %s", bq.conf.BigQuery.SyncTable)

	schema, err := bigquery.InferSchema(syncTable{})
	if err != nil {
		return err
	}

	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(bq.conf.BigQuery.SyncTable)
	tableMetaData := bigquery.TableMetadata{
		Schema: schema,
	}

	if err := table.Create(bq.ctx, &tableMetaData); err != nil {
		return err
	}

	return nil
}

func (bq *BigQuery) createExportTable(hauserSchema bigquery.Schema) error {
	log.Printf("Creating table %s", bq.conf.BigQuery.ExportTable)

	// only EventStart and EventType should be required
	for i := range hauserSchema {
		if hauserSchema[i].Name != "EventStart" && hauserSchema[i].Name != "EventType" {
			hauserSchema[i].Required = false
		}
	}

	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(bq.conf.BigQuery.ExportTable)
	tableMetaData := bigquery.TableMetadata{
		Schema: hauserSchema,
		TimePartitioning: &bigquery.TimePartitioning{
			Expiration: bq.conf.BigQuery.PartitionExpiration.Duration,
		},
	}

	// create export table as date partitioned, with no expiration date (it can be set later)
	if err := table.Create(bq.ctx, &tableMetaData); err != nil {
		return err
	}

	return nil
}

func (bq *BigQuery) fetchTimeVal(q string) (time.Time, error) {
	query := bq.bqClient.Query(q)
	query.QueryConfig.UseStandardSQL = true

	iter, err := query.Read(bq.ctx)
	if err != nil {
		log.Printf("Could not run query %q because: %s", q, err)
		return time.Time{}, err
	}

	var row []bigquery.Value
	err = iter.Next(&row)
	if err != nil {
		log.Printf("Could not fetch result for query %q because: %s", q, err)
		return time.Time{}, err
	}

	if row[0] == nil {
		return time.Time{}, nil
	}

	return row[0].(time.Time), nil
}

func (bq *BigQuery) latestEventStart() (time.Time, error) {
	if !bq.doesTableExist(bq.conf.BigQuery.ExportTable) {
		return time.Time{}, nil
	}

	// export table exists, get latest EventStart from it
	q := fmt.Sprintf("SELECT max(EventStart) FROM %s.%s;", bq.conf.BigQuery.Dataset, bq.conf.BigQuery.ExportTable)
	return bq.fetchTimeVal(q)
}

func (bq *BigQuery) UploadFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gcsClient, err := storage.NewClient(bq.ctx)
	if err != nil {
		return "", err
	}
	defer gcsClient.Close()

	_, objName := filepath.Split(filename)
	w := gcsClient.Bucket(bq.conf.GCS.Bucket).Object(objName).NewWriter(bq.ctx)
	if _, err = io.Copy(w, f); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	return objName, nil
}

func (bq *BigQuery) DeleteFile(objName string) {
	gcsClient, err := storage.NewClient(bq.ctx)
	if err != nil {
		log.Printf("Could not remove %s from bucket %s. Failed to obtain a new GCS client", objName, bq.conf.GCS.Bucket)
	}
	defer gcsClient.Close()

	if err := gcsClient.Bucket(bq.conf.GCS.Bucket).Object(objName).Delete(bq.ctx); err != nil {
		log.Printf("Could not remove %s from bucket %s", objName, bq.conf.GCS.Bucket)
	}
}

func (bq *BigQuery) removeSyncPointsAfter(t time.Time) error {
	q := fmt.Sprintf("DELETE FROM %s.%s WHERE BundleEndTime > TIMESTAMP(\"%s\")", bq.conf.BigQuery.Dataset, bq.conf.BigQuery.SyncTable, t.UTC().Format(time.RFC3339))
	log.Printf("%s", q)
	query := bq.bqClient.Query(q)
	query.QueryConfig.UseStandardSQL = true

	job, err := query.Run(bq.ctx)
	if err != nil {
		log.Printf("Could not run query to remove orphaned sync points: %s", err)
		return err
	}

	return bq.waitForJob(job)
}

func (bq *BigQuery) waitForJob(job *bigquery.Job) error {
	status, err := job.Wait(bq.ctx)
	if err != nil {
		log.Printf("Failed to wait for job: %s", err)
		return err
	}

	if status.Err() != nil {
		log.Printf("Job failed: %s", status.Err())
		for _, e := range status.Errors {
			log.Printf("Error detail: %s", e)
		}
		return status.Err()
	}

	return nil
}

func (bq *BigQuery) GetUploadFailedMsg(filename string, err error) string {
	return fmt.Sprintf("Failed to upload file %s to GCS: %s", filename, err)
}

func (bq *BigQuery) IsUploadOnly() bool {
	return bq.conf.GCS.GCSOnly
}
