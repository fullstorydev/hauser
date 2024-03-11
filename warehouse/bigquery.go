package warehouse

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/fullstorydev/hauser/config"
)

var (
	bigQueryTypeMap = map[reflect.Type]bigquery.FieldType{
		reflect.TypeOf(int64(0)):    bigquery.IntegerFieldType,
		reflect.TypeOf(""):          bigquery.StringFieldType,
		reflect.TypeOf(time.Time{}): bigquery.TimestampFieldType,
		reflect.TypeOf(float64(0)):  bigquery.FloatFieldType,
		reflect.TypeOf(int32(0)):    bigquery.IntegerFieldType,
	}
)

type BigQuery struct {
	conf     *config.BigQueryConfig
	ctx      context.Context
	bqClient *bigquery.Client
}

var _ Database = (*BigQuery)(nil)

func NewBigQuery(c *config.BigQueryConfig) *BigQuery {
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

	table := bq.bqClient.Dataset(bq.conf.Dataset).Table(bq.conf.ExportTable)
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

func (bq *BigQuery) LastSyncPoint(_ context.Context) (time.Time, error) {
	t := time.Time{}

	if err := bq.connectToBQ(); err != nil {
		return t, err
	}
	defer bq.bqClient.Close()

	if !bq.doesTableExist(bq.conf.SyncTable) {
		err := bq.createSyncTable()
		if err != nil {
			log.Printf("Could not create sync table: %s", err)
		}
		return t, err
	}

	q := fmt.Sprintf("SELECT max(BundleEndTime) FROM %s.%s;", bq.conf.Dataset, bq.conf.SyncTable)
	t, err := bq.fetchTimeVal(q)
	if err != nil {
		log.Printf("Couldn't get max(BundleEndTime): %s", err)
		return t, err
	}

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

	return t, nil
}

func (bq *BigQuery) SaveSyncPoint(_ context.Context, endTime time.Time) error {
	if err := bq.connectToBQ(); err != nil {
		return err
	}
	defer bq.bqClient.Close()

	value := fmt.Sprintf("(%d, TIMESTAMP(\"%s\"), TIMESTAMP(\"%s\"))", -1, time.Now().UTC().Format(time.RFC3339), endTime.UTC().Format(time.RFC3339))

	// BQ supports inserting multiple records at once
	q := fmt.Sprintf("INSERT INTO %s.%s (ID, Processed, BundleEndtime) VALUES %s;", bq.conf.Dataset, bq.conf.SyncTable, value)
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

func (bq *BigQuery) LoadToWarehouse(storageRef string, startTime time.Time) error {
	if err := bq.connectToBQ(); err != nil {
		return err
	}
	defer bq.bqClient.Close()

	// create loader to load from file into export table
	gcsRef := bigquery.NewGCSReference(storageRef) // defaults to CSV
	gcsRef.FileConfig.IgnoreUnknownValues = true
	gcsRef.AllowJaggedRows = true
	// Ignore the header
	gcsRef.SkipLeadingRows = 1
	partitionTable := bq.conf.ExportTable + "$" + startTime.Format("20060102")
	log.Printf("Loading GCS file: %s into table %s", storageRef, partitionTable)

	loader := bq.bqClient.Dataset(bq.conf.Dataset).Table(partitionTable).LoaderFrom(gcsRef)
	loader.CreateDisposition = bigquery.CreateNever
	if startTime.Equal(startTime.Truncate(24 * time.Hour)) {
		// this is the first file of the partition, truncate the partition in case there is leftover data from previous failed loads
		log.Printf("Detected first bundle of the day (start: %s), using WriteTruncate to replace any existing data in partition", startTime)
		loader.WriteDisposition = bigquery.WriteTruncate
	}

	// start and wait on loading job
	job, err := loader.Run(bq.ctx)
	if err != nil {
		log.Printf("Could not start BQ load job for file %s", storageRef)
		return err
	}

	return bq.waitForJob(job)
}

func convertSchema(s Schema, existing bigquery.Schema) (bigquery.Schema, error) {
	bqs := make([]*bigquery.FieldSchema, len(s))
	for i, field := range s {
		// Not checking ok here because we may not need it
		var bqType bigquery.FieldType
		if field.FullStoryFieldName == "" {
			// We need to pull the type from the existing schema
			bqType = existing[i].Type
		} else {
			var ok bool
			if bqType, ok = bigQueryTypeMap[field.FieldType]; !ok {
				return nil, fmt.Errorf("bigquery field type not found for schema type %s", field.FieldType)
			}
		}

		if i < len(existing) {
			// Make sure the existing schema is compatible with the provided hauser schema
			if !strings.EqualFold(field.DBName, existing[i].Name) {
				return nil, fmt.Errorf(
					"columns names don't match at index %d: expected %s, got %s", i, existing[i].Name, field.DBName)
			} else if bqType != existing[i].Type {
				return nil, fmt.Errorf("field types don't match at index %d: expected %s, got %s", i, existing[i].Type, bqType)
			}
		}

		bqs[i] = &bigquery.FieldSchema{
			Name: field.DBName,
			Type: bqType,
		}
	}
	return bqs, nil
}

func (bq *BigQuery) InitExportTable(s Schema) (bool, error) {
	bqSchema, err := convertSchema(s, bigquery.Schema{})
	if err != nil {
		return false, err
	}

	if err := bq.connectToBQ(); err != nil {
		log.Fatal(err)
	}

	if bq.doesTableExist(bq.conf.ExportTable) {
		// Ensure that the expiration is set
		table := bq.bqClient.Dataset(bq.conf.Dataset).Table(bq.conf.ExportTable)
		md, err := table.Metadata(bq.ctx)
		if err != nil {
			return false, err
		}
		if md.TimePartitioning.Expiration != bq.conf.PartitionExpiration.Duration {
			update := bigquery.TableMetadataToUpdate{
				TimePartitioning: &bigquery.TimePartitioning{
					Expiration: bq.conf.PartitionExpiration.Duration,
					Field:      md.TimePartitioning.Field,
				},
			}
			_, err := table.Update(bq.ctx, update, md.ETag)
			if err != nil {
				return false, nil
			}
			return true, nil
		}
		return false, nil
	}

	err = bq.createExportTable(bqSchema)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (bq *BigQuery) ApplyExportSchema(s Schema) error {
	if err := bq.connectToBQ(); err != nil {
		log.Fatal(err)
	}

	// get current table schema in BigQuery
	table := bq.bqClient.Dataset(bq.conf.Dataset).Table(bq.conf.ExportTable)
	md, err := table.Metadata(context.Background())
	if err != nil {
		return err
	}

	newSchema, err := convertSchema(s, md.Schema)
	if err != nil {
		return fmt.Errorf("failed to convert to bigquery schema: %s", err)
	}

	newColumns := newSchema[len(md.Schema):]
	if len(newColumns) > 0 {
		update := bigquery.TableMetadataToUpdate{
			Schema: append(md.Schema, newColumns...),
		}
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
	return ValueToString(val, isTime)
}

func (bq *BigQuery) connectToBQ() error {
	var err error
	bq.ctx = context.Background()
	bq.bqClient, err = bigquery.NewClient(bq.ctx, bq.conf.Project)
	if err != nil {
		log.Printf("Could not connect to BigQuery: %s", err)
		return err
	}
	return nil
}

func (bq *BigQuery) doesTableExist(name string) bool {
	log.Printf("Checking if table %s exists", name)
	table := bq.bqClient.Dataset(bq.conf.Dataset).Table(name)
	if _, err := table.Metadata(bq.ctx); err != nil {
		return false
	}

	return true
}

func (bq *BigQuery) createSyncTable() error {
	log.Printf("Creating table %s", bq.conf.SyncTable)

	schema, err := bigquery.InferSchema(syncTable{})
	if err != nil {
		return err
	}

	table := bq.bqClient.Dataset(bq.conf.Dataset).Table(bq.conf.SyncTable)
	tableMetaData := bigquery.TableMetadata{
		Schema: schema,
	}

	if err := table.Create(bq.ctx, &tableMetaData); err != nil {
		return err
	}

	return nil
}

func (bq *BigQuery) createExportTable(hauserSchema bigquery.Schema) error {
	log.Printf("Creating table %s", bq.conf.ExportTable)

	// only EventStart and EventType should be required
	for i := range hauserSchema {
		if hauserSchema[i].Name != "EventStart" && hauserSchema[i].Name != "EventType" {
			hauserSchema[i].Required = false
		}
	}

	table := bq.bqClient.Dataset(bq.conf.Dataset).Table(bq.conf.ExportTable)
	tableMetaData := bigquery.TableMetadata{
		Schema: hauserSchema,
		TimePartitioning: &bigquery.TimePartitioning{
			Expiration: bq.conf.PartitionExpiration.Duration,
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
	if !bq.doesTableExist(bq.conf.ExportTable) {
		return time.Time{}, nil
	}

	// export table exists, get latest EventStart from it
	q := fmt.Sprintf("SELECT max(EventStart) FROM %s.%s;", bq.conf.Dataset, bq.conf.ExportTable)
	return bq.fetchTimeVal(q)
}

func (bq *BigQuery) removeSyncPointsAfter(t time.Time) error {
	q := fmt.Sprintf("DELETE FROM %s.%s WHERE BundleEndTime > TIMESTAMP(\"%s\")", bq.conf.Dataset, bq.conf.SyncTable, t.UTC().Format(time.RFC3339))
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
