package internal

import (
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
	"strings"
	"time"

	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/warehouse"
	"github.com/pkg/errors"
)

var (
	currentBackoffStep = uint(0)
	bundleFieldsMap    = warehouse.BundleFields()
)

const (
	// Maximum number of times Hauser will attempt to retry each request made to FullStory
	maxAttempts int = 3

	// Default duration Hauser will wait before retrying a 429 or 5xx response. If Retry-After is specified, uses that instead. Default arbitrarily set to 10s.
	defaultRetryAfterDuration time.Duration = time.Duration(10) * time.Second
)

// Record represents a single export row in the export file
type Record map[string]interface{}

type HauserService struct {
	config       *config.Config
	fsClient     client.DataExportClient
	storage      warehouse.Storage
	database     warehouse.Database
	tableColumns []string
}

func NewHauser(config *config.Config, fsClient client.DataExportClient, storage warehouse.Storage, db warehouse.Database) *HauserService {
	return &HauserService{
		config:   config,
		fsClient: fsClient,
		storage:  storage,
		database: db,
	}
}

// TransformExportJSONRecord transforms the record map (extracted from the API response json) to a
// slice of strings. The slice of strings contains values in the same order as the existing export table.
// For existing export table fields that do not exist in the json record, an empty string is populated.
func TransformExportJSONRecord(convert warehouse.ValueToStringFn, tableColumns []string, rec map[string]interface{}) ([]string, error) {
	var line []string
	lowerRec := make(map[string]interface{})
	customVarsMap := make(map[string]interface{})

	// Do a single pass over the data record to:
	// a) extract all custom variables
	// b) change standard/non-custom field names to lowercase for case-insensitive column name matching
	for key, val := range rec {
		lowerKey := strings.ToLower(key)
		if _, ok := bundleFieldsMap[lowerKey]; !ok {
			customVarsMap[key] = val
		} else {
			lowerRec[lowerKey] = val
		}
	}

	// Fetch the table columns so can build the csv with a column order that matches the export table
	for _, col := range tableColumns {
		field, isPartOfExportBundle := bundleFieldsMap[col]

		// These are columns in the export table that we are not going to populate
		if !isPartOfExportBundle {
			line = append(line, "")
			continue
		}

		if field.IsCustomVar {
			customVars, err := json.Marshal(customVarsMap)
			if err != nil {
				return nil, err
			}
			line = append(line, string(customVars))
		} else {
			if val, valExists := lowerRec[col]; valExists {
				line = append(line, convert(val, field.IsTime))
			} else {
				line = append(line, "")
			}
		}
	}
	return line, nil
}

func (h *HauserService) ProcessExportsSince(ctx context.Context, since time.Time) (int, error) {
	log.Printf("Checking for new export files since %s", since)

	exports, err := h.fsClient.ExportList(since)
	if err != nil {
		log.Printf("Failed to fetch export list: %s", err)
		return 0, err
	}

	if h.config.GroupFilesByDay {
		return h.ProcessFilesByDay(ctx, exports)
	}
	return h.ProcessFilesIndividually(ctx, exports)
}

// ProcessFilesIndividually iterates over the list of available export files and processes them one by one, until an error
// occurs, or until they are all processed.
func (h *HauserService) ProcessFilesIndividually(ctx context.Context, exports []client.ExportMeta) (int, error) {
	for _, e := range exports {
		err := func() error {
			log.Printf("Processing bundle %d (start: %s, end: %s)", e.ID, e.Start.UTC(), e.Stop.UTC())
			mark := time.Now()
			var filename string
			var statusMessage string
			if h.config.SaveAsJson {
				filename = filepath.Join(h.config.TmpDir, fmt.Sprintf("%d.json", e.ID))
				defer os.Remove(filename)
				writtenCount, err := h.WriteBundleToJson(e.ID, filename)
				if err != nil {
					log.Printf("Failed to create tmp json file: %s", err)
					return err
				}
				statusMessage = fmt.Sprintf("Processing of bundle %d (%d bytes)", e.ID, writtenCount)
			} else {
				filename = filepath.Join(h.config.TmpDir, fmt.Sprintf("%d.csv", e.ID))
				outfile, err := os.Create(filename)
				if err != nil {
					log.Printf("Failed to create tmp csv file: %s", err)
					return err
				}
				defer os.Remove(filename)
				defer outfile.Close()
				csvOut := csv.NewWriter(outfile)

				recordCount, err := h.WriteBundleToCSV(e.ID, h.tableColumns, csvOut)
				if err != nil {
					return err
				}
				statusMessage = fmt.Sprintf("Processing of bundle %d (%d records)", e.ID, recordCount)
			}

			if err := h.LoadBundles(ctx, filename, e); err != nil {
				return err
			}

			log.Printf("%s took %s", statusMessage, time.Since(mark))
			return nil
		}()
		if err != nil {
			return 0, errors.Errorf("Failed to process bundle: %s", err)
		}
	}

	// return how many files were processed
	return len(exports), nil
}

// ProcessFilesByDay creates a single intermediate CSV file for all the export bundles on a given day.  It assumes the
// day to be processed is the day from the first export bundle's Start value.  When all the bundles with that same day
// have been written to the CSV file, it is loaded to the warehouse, and the function quits without attempting to
// process remaining bundles (they'll get picked up on the next call to ProcessExportsSince)
func (h *HauserService) ProcessFilesByDay(ctx context.Context, exports []client.ExportMeta) (int, error) {
	if len(exports) == 0 {
		return 0, nil
	}
	if h.config.SaveAsJson {
		log.Fatalf("The option to process files by day is only supported for CSV format.")
	}

	log.Printf("Creating group file starting with bundle %d (start: %s)", exports[0].ID, exports[0].Start.UTC())
	filename := filepath.Join(h.config.TmpDir, fmt.Sprintf("%d-%s.csv", exports[0].ID, exports[0].Start.UTC().Format("20060102")))
	mark := time.Now()
	outfile, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create tmp file: %s", err)
		return 0, err
	}
	defer os.Remove(filename)
	defer outfile.Close()
	csvOut := csv.NewWriter(outfile)

	var processedBundles []client.ExportMeta
	var totalRecords int
	groupDay := exports[0].Start.UTC().Truncate(24 * time.Hour)
	for _, e := range exports {
		if !groupDay.Equal(e.Start.UTC().Truncate(24 * time.Hour)) {
			break
		}

		recordCount, err := h.WriteBundleToCSV(e.ID, h.tableColumns, csvOut)
		if err != nil {
			return 0, err
		}

		log.Printf("Wrote bundle %d (%d records, start: %s, stop: %s)", e.ID, recordCount, e.Start.UTC(), e.Stop.UTC())
		totalRecords += recordCount
		processedBundles = append(processedBundles, e)
	}

	if err := h.LoadBundles(ctx, filename, processedBundles...); err != nil {
		return 0, err
	}

	log.Printf("Processing of %d bundles (%d records) took %s", len(processedBundles), totalRecords,
		time.Since(mark))

	// return how many files were processed
	return len(processedBundles), nil
}

func (h *HauserService) LoadBundles(ctx context.Context, filename string, bundles ...client.ExportMeta) error {
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
		return h.storage.SaveSyncPoints(ctx, bundles...)
	}

	defer h.storage.DeleteFile(ctx, objName)

	if err := h.database.LoadToWarehouse(objRef, bundles...); err != nil {
		log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
		return err
	}

	// If we've already copied in the data but fail to save the sync point, we're
	// still okay - the next call to LastSyncPoint() will see that there are export
	// records beyond the sync point and remove them - ie, we will reprocess the
	// current export file
	if err := h.database.SaveSyncPoints(ctx, bundles...); err != nil {
		log.Printf("Failed to save sync points for bundles ending with %d: %s", bundles[len(bundles)].ID, err)
		return err
	}
	return nil
}

func (h *HauserService) getExportData(bundleID int) (client.ExportData, error) {
	log.Printf("Getting Export Data for bundle %d\n", bundleID)
	var fsErr error
	for r := 1; r <= maxAttempts; r++ {
		stream, err := h.fsClient.ExportData(bundleID)
		if err == nil {
			return stream, nil
		}
		log.Printf("Failed to fetch export data for Bundle %d: %s", bundleID, err)

		fsErr = err
		doRetry, retryAfterDuration := getRetryInfo(err)
		if !doRetry {
			break
		}

		log.Printf("Attempt #%d failed. Retrying after %s\n", r, retryAfterDuration)
		time.Sleep(retryAfterDuration)
	}

	return nil, errors.Wrap(fsErr, fmt.Sprintf("Unable to fetch export data. Tried %d times.", maxAttempts))
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
func (h *HauserService) WriteBundleToCSV(bundleID int, tableColumns []string, csvOut *csv.Writer) (numRecords int, err error) {
	stream, err := h.getExportData(bundleID)
	if err != nil {
		log.Printf("Failed to fetch bundle %d: %s", bundleID, err)
		return 0, err
	}
	defer stream.Close()

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
		line, err := TransformExportJSONRecord(h.getValueConverter(), tableColumns, r)
		if err != nil {
			log.Printf("Failed object transform, bundle %d; skipping record. %s", bundleID, err)
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
func (h *HauserService) WriteBundleToJson(bundleID int, filename string) (bytesWritten int64, err error) {
	stream, err := h.getExportData(bundleID)
	if err != nil {
		log.Printf("Failed to fetch bundle %d: %s", bundleID, err)
		return 0, err
	}
	defer stream.Close()

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

func (h *HauserService) Init(_ context.Context) error {
	// Initialize the warehouse's schema
	if !h.config.StorageOnly {
		if err := h.database.EnsureCompatibleExportTable(); err != nil {
			return err
		}
		// NB: We SHOULD fetch the table columns ONLY after the call to EnsureCompatibleExportTable.
		// The EnsureCompatibleExportTable function potentially alters the schema of the export table in the client warehouse.
		h.tableColumns = h.database.GetExportTableColumns()
		for i, col := range h.tableColumns {
			h.tableColumns[i] = strings.ToLower(col)
		}
	}
	// TODO(cam): enable storage only with CSV format by saving the fields locally
	//  (like we do with the sync file)
	return nil
}

func (h *HauserService) ProcessNext(ctx context.Context) (int, error) {
	lastSyncedRecord, err := h.lastSyncPoint(ctx)
	if err != nil {
		return 0, err
	}
	return h.ProcessExportsSince(ctx, lastSyncedRecord)
}

func (h *HauserService) Run(ctx context.Context) {
	if err := h.Init(ctx); err != nil {
		log.Fatal(err)
	}
	for {
		numBundles, err := h.ProcessNext(ctx)
		if h.BackoffOnError(err) {
			continue
		}

		if numBundles > 0 {
			continue
		}

		log.Printf("No exports pending; sleeping %s", h.config.CheckInterval.Duration)
		time.Sleep(h.config.CheckInterval.Duration)
	}
}
