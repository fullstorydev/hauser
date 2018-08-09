package main

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nishanths/fullstory"

	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/warehouse"
)

var (
	conf               *config.Config
	currentBackoffStep = uint(0)
	bundleFieldsMap    = warehouse.BundleFields()
)

const (
	maxAttempts int = 3

	// Default retry after duration. Arbitrarily set to 10s.
	defaultRetryAfterDuration time.Duration = time.Duration(10) * time.Second
)

// Record represents a single export row in the export file
type Record map[string]interface{}

type ExportProcessor func(warehouse.Warehouse, *fullstory.Client, []fullstory.ExportMeta) (int, error)

// TransformExportJSONRecord transforms the record map (extracted from the API response json) to a
// slice of strings. The slice of strings contains values in the same order as the existing export table.
// For existing export table fields that do not exist in the json record, an empty string is populated.
func TransformExportJSONRecord(wh warehouse.Warehouse, rec map[string]interface{}) ([]string, error) {
	var line []string
	// Change all record keys to lower case. We do this because columns are case insensitive for most warehouse solutions.
	rec = getRecordWithLowerCaseKeys(rec)

	// Map of CustomVars
	customVarsMap := make(map[string]interface{})
	for key, val := range rec {
		if field, ok := bundleFieldsMap[key]; !ok {
			customVarsMap[field.Name] = val
		}
	}

	// Fetch the table columns so can build the csv with a column order that matches the export table
	tableColumns := wh.GetExportTableColumns()
	for _, col := range tableColumns {
		field, isPartOfExportBundle := bundleFieldsMap[col]

		// These are columns in the export table that we are not going to populate
		if !isPartOfExportBundle {
			line = append(line, "")
			continue;
		}

		if field.IsCustomVar {
			customVars, err := json.Marshal(customVarsMap)
			if err != nil {
				return nil, err
			}
			line = append(line, string(customVars))
		} else {
			if val, valExists := rec[col]; valExists {
				line = append(line, wh.ValueToString(val, field.IsTime))
			} else {
				line = append(line, "")
			}
		}
	}
	return line, nil
}

func ProcessExportsSince(wh warehouse.Warehouse, since time.Time, exportProcessor ExportProcessor) (int, error) {
	log.Printf("Checking for new export files since %s", since)

	fs := fullstory.NewClient(conf.FsApiToken)
	if conf.ExportURL != "" {
		fs.Config.BaseURL = conf.ExportURL
	}
	exports, err := fs.ExportList(since)
	if err != nil {
		log.Printf("Failed to fetch export list: %s", err)
		return 0, err
	}

	return exportProcessor(wh, fs, exports)
}

// ProcessFilesIndividually iterates over the list of available export files and processes them one by one, until an error
// occurs, or until they are all processed.
func ProcessFilesIndividually(wh warehouse.Warehouse, fs *fullstory.Client, exports []fullstory.ExportMeta) (int, error) {
	for _, e := range exports {
		log.Printf("Processing bundle %d (start: %s, end: %s)", e.ID, e.Start.UTC(), e.Stop.UTC())
		filename := filepath.Join(conf.TmpDir, fmt.Sprintf("%d.csv", e.ID))
		mark := time.Now()
		outfile, err := os.Create(filename)
		if err != nil {
			log.Printf("Failed to create tmp file: %s", err)
			return 0, err
		}
		defer os.Remove(filename)
		defer outfile.Close()
		csvOut := csv.NewWriter(outfile)

		recordCount, err := WriteBundleToCSV(fs, e.ID, csvOut, wh)
		if err != nil {
			return 0, err
		}

		if err := LoadBundles(wh, filename, e); err != nil {
			return 0, err
		}

		log.Printf("Processing of bundle %d (%d records) took %s", e.ID, recordCount,
			time.Since(mark))
	}

	// return how many files were processed
	return len(exports), nil
}

// ProcessFilesByDay creates a single intermediate CSV file for all the export bundles on a given day.  It assumes the
// day to be processed is the day from the first export bundle's Start value.  When all the bundles with that same day
// have been written to the CSV file, it is loaded to the warehouse, and the function quits without attempting to
// process remaining bundles (they'll get picked up on the next call to ProcessExportsSince)
func ProcessFilesByDay(wh warehouse.Warehouse, fs *fullstory.Client, exports []fullstory.ExportMeta) (int, error) {
	if len(exports) == 0 {
		return 0, nil
	}

	log.Printf("Creating group file starting with bundle %d (start: %s)", exports[0].ID, exports[0].Start.UTC())
	filename := filepath.Join(conf.TmpDir, fmt.Sprintf("%d-%s.csv", exports[0].ID, exports[0].Start.UTC().Format("20060102")))
	mark := time.Now()
	outfile, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create tmp file: %s", err)
		return 0, err
	}
	defer os.Remove(filename)
	defer outfile.Close()
	csvOut := csv.NewWriter(outfile)

	var processedBundles []fullstory.ExportMeta
	var totalRecords int
	groupDay := exports[0].Start.UTC().Truncate(24 * time.Hour)
	for _, e := range exports {
		if !groupDay.Equal(e.Start.UTC().Truncate(24 * time.Hour)) {
			break
		}

		recordCount, err := WriteBundleToCSV(fs, e.ID, csvOut, wh)
		if err != nil {
			return 0, err
		}

		log.Printf("Wrote bundle %d (%d records, start: %s, stop: %s)", e.ID, recordCount, e.Start.UTC(), e.Stop.UTC())
		totalRecords += recordCount
		processedBundles = append(processedBundles, e)
	}

	if err := LoadBundles(wh, filename, processedBundles...); err != nil {
		return 0, err
	}

	log.Printf("Processing of %d bundles (%d records) took %s", len(processedBundles), totalRecords,
		time.Since(mark))

	// return how many files were processed
	return len(processedBundles), nil
}

func LoadBundles (wh warehouse.Warehouse, filename string, bundles ...fullstory.ExportMeta) error {
	var objPath string
	var err error
	if objPath, err = wh.UploadFile(filename); err != nil {
		log.Printf(wh.GetUploadFailedMsg(filename, err))
		return err
	}

	if wh.IsUploadOnly() {
		return nil
	}

	defer wh.DeleteFile(objPath)

	if err := wh.LoadToWarehouse(objPath, bundles...); err != nil {
		log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
		return err
	}

	// If we've already copied in the data but fail to save the sync point, we're
	// still okay - the next call to LastSyncPoint() will see that there are export
	// records beyond the sync point and remove them - ie, we will reprocess the
	// current export file
	if err := wh.SaveSyncPoints(bundles...); err != nil {
		log.Printf("Failed to save sync points for bundles ending with %d: %s", bundles[len(bundles)].ID, err)
		return err
	}
	return nil
}

func getExportData(fs *fullstory.Client, bundleID int) (fullstory.ExportData, error) {
	log.Printf("Getting Export Data for bundle %d\n", bundleID)
	for r := 1; r <= maxAttempts; r++ {
		stream, err := fs.ExportData(bundleID)
		if err != nil {
			log.Printf("Failed to fetch export data for Bundle %d", bundleID)
			retryAfterDuration := defaultRetryAfterDuration
			if statusError, ok := err.(fullstory.StatusError); ok {
				if retryAfter, err := strconv.Atoi(statusError.RetryAfter); err == nil {
					retryAfterDuration = time.Duration(retryAfter) * time.Second
				}
			}
			log.Printf("Attempt #%d failed. Retrying after %s\n", r, retryAfterDuration)
			time.Sleep(retryAfterDuration)
		} else {
			return stream, nil
		}
	}
	return nil, fmt.Errorf("Unable to fetch export data. Tried %d times.", maxAttempts)
}

// WriteBundleToCSV writes the bundle corresponding to the given bundleID to the csv Writer
func WriteBundleToCSV(fs *fullstory.Client, bundleID int, csvOut *csv.Writer, wh warehouse.Warehouse) (numRecords int, err error) {
	stream, err := getExportData(fs, bundleID)
	if err != nil {
		log.Printf("Failed to fetch bundle %d: %s", bundleID, err)
		return 0, err
	}
	defer stream.Close()

	gzstream, err := gzip.NewReader(stream)
	if err != nil {
		log.Printf("Failed gzip reader: %s", err)
		return 0, err
	}

	decoder := json.NewDecoder(gzstream)
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
		line, err := TransformExportJSONRecord(wh, r)
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

func BackoffOnError(err error) bool {
	if err != nil {
		if currentBackoffStep == uint(conf.BackoffStepsMax) {
			log.Fatalf("Reached max retries; exiting")
		}
		dur := conf.Backoff.Duration * (1 << currentBackoffStep)
		log.Printf("Pausing; will retry operation in %s", dur)
		time.Sleep(dur)
		currentBackoffStep++
		return true
	}
	currentBackoffStep = 0
	return false
}

func getRecordWithLowerCaseKeys(rec map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range rec {
		m[strings.ToLower(k)] = v
	}
	return m
}

func main() {
	conffile := flag.String("c", "config.toml", "configuration file")
	flag.Parse()

	var err error
	if conf, err = config.Load(*conffile); err != nil {
		log.Fatal(err)
	}

	exportProcessor := ProcessFilesIndividually
	if conf.GroupFilesByDay {
		exportProcessor = ProcessFilesByDay
	}

	var wh warehouse.Warehouse
	switch conf.Warehouse {
	case "redshift":
		wh = warehouse.NewRedshift(conf)
	case "bigquery":
		wh = warehouse.NewBigQuery(conf)
	default:
		if len(conf.Warehouse) == 0 {
			log.Fatal("Warehouse type must be specified in configuration")
		} else {
			log.Fatalf("Warehouse type '%s' unrecognized", conf.Warehouse)
		}
	}

	if err := wh.EnsureCompatibleExportTable(); err != nil {
		log.Fatal(err)
	}

	for {
		lastSyncedRecord, err := wh.LastSyncPoint()
		if BackoffOnError(err) {
			continue
		}

		numBundles, err := ProcessExportsSince(wh, lastSyncedRecord, exportProcessor)
		if BackoffOnError(err) {
			continue
		}

		// if we processed any bundles, there may be more - check until nothing comes back
		if numBundles > 0 {
			continue
		}

		log.Printf("No exports pending; sleeping %s", conf.CheckInterval.Duration)
		time.Sleep(conf.CheckInterval.Duration)
	}
}
