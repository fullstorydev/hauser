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
	"github.com/nishanths/fullstory"

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

func (bq *BigQuery) SaveSyncPoints(bundles ...fullstory.ExportMeta) error {
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

func (bq *BigQuery) LoadToWarehouse(filename string, bundles ...fullstory.ExportMeta) error {
	if err := bq.connectToBQ(); err != nil {
		return err
	}
	defer bq.bqClient.Close()

	log.Printf("Uploading file: %s", filename)
	objName, err := bq.uploadToGCS(filename)
	if err != nil {
		log.Printf("Failed to upload file %s to s3: %s", filename, err)
		return err
	}

	if bq.conf.GCS.GCSOnly {
		return nil
	}

	defer bq.deleteFromGCS(objName)

	if !bq.doesTableExist(bq.conf.BigQuery.ExportTable) {
		if err := bq.createExportTable(); err != nil {
			return err
		}
	}

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
		log.Printf("Could not start BQ load job for file %s", filename)
		return err
	}

	return bq.waitForJob(job)
}

func (bq *BigQuery) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(time.RFC3339)
	}

	s = strings.Replace(s, "\n", " ", -1)
	s = strings.Replace(s, "\r", " ", -1)
	s = strings.Replace(s, "\x00", "", -1)

	return s
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
	if err := table.Create(bq.ctx, schema); err != nil {
		return err
	}

	return nil
}

func (bq *BigQuery) createExportTable() error {
	log.Printf("Creating table %s", bq.conf.BigQuery.ExportTable)

	schema, err := bigquery.InferSchema(bundleEvent{})
	if err != nil {
		return err
	}

	// only EventStart and EventType should be required
	for i := range schema {
		if schema[i].Name != "EventStart" && schema[i].Name != "EventType" {
			schema[i].Required = false
		}
	}

	table := bq.bqClient.Dataset(bq.conf.BigQuery.Dataset).Table(bq.conf.BigQuery.ExportTable)
	// create export table as date partitioned, with no expiration date (it can be set later)
	if err := table.Create(bq.ctx, schema, &bigquery.TimePartitioning{}); err != nil {
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

func (bq *BigQuery) uploadToGCS(filename string) (string, error) {
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

func (bq *BigQuery) deleteFromGCS(objName string) error {
	gcsClient, err := storage.NewClient(bq.ctx)
	if err != nil {
		return err
	}
	defer gcsClient.Close()

	if err := gcsClient.Bucket(bq.conf.GCS.Bucket).Object(objName).Delete(bq.ctx); err != nil {
		log.Printf("Could not remove %s from bucket %s", objName, bq.conf.GCS.Bucket)
		return err
	}

	return nil
}

func (bq *BigQuery) removeSyncPointsAfter(t time.Time) error {
	q := fmt.Sprintf("DELETE FROM %s.%s WHERE BundleEndTime > TIMESTAMP(\"%s\")", bq.conf.BigQuery.Dataset, bq.conf.BigQuery.SyncTable, t.UTC().Format(time.RFC3339))
	log.Printf(q)
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
