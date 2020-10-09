package internal

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/warehouse"
)

var (
	currentBackoffStep = uint(0)
)

const (
	// Default duration Hauser will wait before retrying a 429 or 5xx response. If Retry-After is specified, uses that instead. Default arbitrarily set to 10s.
	defaultRetryAfterDuration time.Duration = time.Duration(10) * time.Second
)

var (
	// Provided as global variable for mocking
	getNow               = time.Now
	progressPollDuration = 5 * time.Second
)

// Record represents a single export row in the export file
type Record map[string]interface{}

type HauserService struct {
	config   *config.Config
	fsClient client.DataExportClient
	storage  warehouse.Storage
	database warehouse.Database
	schema   warehouse.Schema
	// cached map of the schema for translating json records
	schemaMap map[string]bool
}

func NewHauserService(config *config.Config, fsClient client.DataExportClient, storage warehouse.Storage, db warehouse.Database) *HauserService {
	return &HauserService{
		config:   config,
		fsClient: fsClient,
		storage:  storage,
		database: db,
		schema:   warehouse.MakeSchema(warehouse.BaseExportFields{}),
	}
}

// TransformExportJSONRecord transforms the record map (extracted from the API response json) to a
// slice of strings. The slice of strings contains values in the same order as the existing export table.
// For existing export table fields that do not exist in the json record, an empty string is populated.
func (h *HauserService) transformExportJSONRecord(convert warehouse.ValueToStringFn, rec map[string]interface{}) ([]string, error) {
	var line []string
	lowerRec := make(map[string]interface{})
	customVarsMap := make(map[string]interface{})

	if len(h.schemaMap) == 0 {
		h.schemaMap = make(map[string]bool, len(h.schema))
		for _, field := range h.schema {
			if field.FullStoryFieldName != "" {
				h.schemaMap[strings.ToLower(field.FullStoryFieldName)] = true
			}
		}
	}

	// Do a single pass over the data record to:
	// a) extract all custom variables
	// b) change standard/non-custom field names to lowercase for case-insensitive column name matching
	for key, val := range rec {
		lowerKey := strings.ToLower(key)
		if _, ok := h.schemaMap[lowerKey]; !ok {
			customVarsMap[key] = val
		} else {
			lowerRec[lowerKey] = val
		}
	}

	for _, field := range h.schema {
		if field.FullStoryFieldName == "" {
			// This is a column in the export table that doesn't come from the export
			line = append(line, "")
			continue
		}
		if field.DBName == "CustomVars" {
			customVars, err := json.Marshal(customVarsMap)
			if err != nil {
				return nil, err
			}
			line = append(line, string(customVars))
		} else {
			if val, valExists := lowerRec[strings.ToLower(field.FullStoryFieldName)]; valExists {
				line = append(line, convert(val, field.FieldType == reflect.TypeOf(time.Time{})))
			} else {
				line = append(line, "")
			}
		}
	}
	return line, nil
}

func (h *HauserService) LoadBundles(ctx context.Context, filename string, startTime, endTime time.Time) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, objName := path.Split(filename)
	objRef, err := h.storage.SaveFile(ctx, objName, f)
	if err != nil {
		return fmt.Errorf("failed to save file: %s", err)
	}

	if h.config.StorageOnly {
		return h.storage.SaveSyncPoint(ctx, endTime)
	}

	defer h.storage.DeleteFile(ctx, objName)

	if err := h.database.LoadToWarehouse(objRef, startTime); err != nil {
		log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
		return err
	}

	// If we've already copied in the data but fail to save the sync point, we're
	// still okay - the next call to LastSyncPoint() will see that there are export
	// records beyond the sync point and remove them - ie, we will reprocess the
	// current export file
	if err := h.database.SaveSyncPoint(ctx, endTime); err != nil {
		log.Printf("Failed to save sync point for %s: %s", endTime, err)
		return err
	}
	return nil
}

func getRetryInfo(err error) (bool, time.Duration) {
	if statusError, ok := err.(client.StatusError); ok {
		// If the status code is NOT 429 and the code is below 500 we will not attempt to retry
		if statusError.StatusCode != http.StatusTooManyRequests && statusError.StatusCode < 500 {
			return false, defaultRetryAfterDuration
		}

		if statusError.RetryAfter > 0 {
			return true, statusError.RetryAfter
		}
	}

	return true, defaultRetryAfterDuration
}

// WriteBundleToCSV writes the bundle corresponding to the given bundleID to the csv Writer
func (h *HauserService) WriteBundleToCSV(stream io.Reader, csvOut *csv.Writer) (numRecords int, err error) {
	headers := make([]string, len(h.schema))
	for i, field := range h.schema {
		headers[i] = field.DBName
	}
	if err := csvOut.Write(headers); err != nil {
		return 0, err
	}

	decoder := json.NewDecoder(stream)
	decoder.UseNumber()

	// skip array open delimiter
	if _, err := decoder.Token(); err != nil {
		log.Printf("Failed json decode of array open token: %s", err)
		return 0, err
	}

	var recordCount int
	for decoder.More() {
		var r Record
		if err := decoder.Decode(&r); err != nil {
			log.Printf("failed json decode of record: %s", err)
			return recordCount, err
		}
		line, err := h.transformExportJSONRecord(h.getValueConverter(), r)
		if err != nil {
			log.Printf("Failed object transform, skipping record. %s", err)
			continue
		}
		csvOut.Write(line)
		recordCount++
	}

	if _, err := decoder.Token(); err != nil {
		log.Printf("Failed json decode of array close token: %s", err)
		return recordCount, err
	}

	csvOut.Flush()
	return recordCount, nil
}

// WriteBundleToJson writes the bundle corresponding to the given bundleID to a Json file
func (h *HauserService) WriteBundleToJson(stream io.Reader, filename string) (bytesWritten int64, err error) {
	outfile, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create json file: %s", err)
		return 0, err
	}
	defer outfile.Close()

	written, err := io.Copy(outfile, stream)
	if err != nil {
		log.Printf("Failed to copy input stream to file: %s", err)
		return 0, err
	}

	return written, nil
}

func (h *HauserService) getValueConverter() warehouse.ValueToStringFn {
	if h.config.StorageOnly {
		return warehouse.ValueToString
	}
	return h.database.ValueToString
}

func (h *HauserService) lastSyncPoint(ctx context.Context) (time.Time, error) {
	if h.config.StorageOnly {
		return h.storage.LastSyncPoint(ctx)
	}
	return h.database.LastSyncPoint(ctx)
}

func (h *HauserService) BackoffOnError(err error) bool {
	if err != nil {
		log.Printf("failed to process exports: %s", err)
		if currentBackoffStep == uint(h.config.BackoffStepsMax) {
			log.Fatalf("Reached max retries; exiting")
		}
		dur := h.config.Backoff.Duration * (1 << currentBackoffStep)
		log.Printf("Pausing; will retry operation in %s", dur)
		time.Sleep(dur)
		currentBackoffStep++
		return true
	}
	currentBackoffStep = 0
	return false
}

func (h *HauserService) Init(ctx context.Context) error {
	// Initialize the warehouse's schema
	if !h.config.StorageOnly {
		return h.InitDatabase(ctx)
	}
	return nil
}

func (h *HauserService) InitDatabase(_ context.Context) error {
	if created, err := h.database.InitExportTable(h.schema); err != nil {
		return err
	} else if !created {
		existingCols := h.database.GetExportTableColumns()
		newSchema := h.schema.ReconcileWithExisting(existingCols)
		if err := h.database.ApplyExportSchema(newSchema); err != nil {
			return err
		}
		h.schema = newSchema
	}
	return nil
}

// ProcessNext will return the number of
func (h *HauserService) ProcessNext(ctx context.Context) (time.Duration, error) {
	lastSyncedRecord, err := h.lastSyncPoint(ctx)
	if err != nil {
		return 0, err
	}

	if lastSyncedRecord.IsZero() {
		// If starting fresh, use the start time provided in the config
		lastSyncedRecord = h.config.StartTime
	}

	// We need to ensure that the end time for the export is aligned with the export duration
	nextEndTime := lastSyncedRecord.Add(h.config.ExportDuration.Duration).Truncate(h.config.ExportDuration.Duration)
	for {
		lastAvailableEndTime := getNow().Add(-1 * h.config.ExportDelay.Duration)
		if nextEndTime.After(lastAvailableEndTime) {
			waitUntil := nextEndTime.Add(h.config.ExportDelay.Duration)
			return waitUntil.Sub(getNow()), nil
		} else {
			break
		}
	}

	log.Printf("Creating export for %s to %s", lastSyncedRecord, nextEndTime)
	id, err := h.fsClient.CreateExport(lastSyncedRecord, nextEndTime, h.schema.GetFullStoryFields())
	if err != nil {
		return 0, err
	}

	var exportId string
	var prog int
	for {
		prog, exportId, err = h.fsClient.GetExportProgress(id)
		if err != nil {
			return 0, err
		}
		log.Printf("Export progress: %d%%", prog)
		if exportId != "" {
			break
		}
		time.Sleep(progressPollDuration)
	}

	log.Printf("Fetching export for operation id %s", id)
	body, err := h.fsClient.GetExport(exportId)
	if err != nil {
		return 0, err
	}
	defer body.Close()

	unzipped, err := gzip.NewReader(body)
	if err != nil {
		return 0, err
	}

	if h.config.SaveAsJson {
		// Short circuit since we don't support loading json into the database
		if _, err := h.storage.SaveFile(ctx, fmt.Sprintf("%d.json", lastSyncedRecord.Unix()), unzipped); err != nil {
			return 0, err
		}
		return 0, h.storage.SaveSyncPoint(ctx, nextEndTime)
	}

	filename := filepath.Join(h.config.TmpDir, fmt.Sprintf("%d.csv", lastSyncedRecord.Unix()))
	outfile, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create tmp csv file: %s", err)
		return 0, err
	}
	defer os.Remove(filename)
	defer outfile.Close()

	csvOut := csv.NewWriter(outfile)
	_, err = h.WriteBundleToCSV(unzipped, csvOut)
	if err != nil {
		return 0, err
	}
	err = h.LoadBundles(ctx, filename, lastSyncedRecord, nextEndTime)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (h *HauserService) Run(ctx context.Context) {
	if err := h.Init(ctx); err != nil {
		log.Fatal(err)
	}
	for {
		timeToWait, err := h.ProcessNext(ctx)
		if h.BackoffOnError(err) {
			continue
		}

		if timeToWait == 0 {
			continue
		}
		log.Printf("Waiting until %s to start next export\n", time.Now().Add(timeToWait))
		time.Sleep(timeToWait)
	}
}
