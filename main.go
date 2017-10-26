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
	"time"

	"github.com/nishanths/fullstory"

	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/warehouse"
)

var (
	conf               *config.Config
	currentBackoffStep = uint(0)
	bundleFields       = warehouse.BundleFields()
)

// Represents a single export row in the export file
type Record map[string]interface{}

type ExportProcessor func(warehouse.Warehouse, *fullstory.Client, []fullstory.ExportMeta) (int, error)

func TransformExportJsonRecord(wh warehouse.Warehouse, rec map[string]interface{}) ([]string, error) {
	var line []string

	// TODO(jess): some configurable way to inject additional transformed fields, for very light/limited ETL

	for _, field := range bundleFields {
		if field.IsCustomVar {
			continue
		}

		if val, ok := rec[field.Name]; ok {
			line = append(line, wh.ValueToString(val, field.IsTime))
			delete(rec, field.Name)
		} else {
			line = append(line, "")
		}
	}

	// custom variables will be whatever is left after all well-known fields are accounted for
	customVars, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	line = append(line, string(customVars))
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

		if err := wh.LoadToWarehouse(filename, e); err != nil {
			log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
			return 0, err
		}

		if err := wh.SaveSyncPoints(e); err != nil {
			// If we've already copied in the data but fail to save the sync point, we're
			// still okay - the next call to LastSyncPoint() will see that there are export
			// records beyond the sync point and remove them - ie, we will reprocess the
			// current export file
			log.Printf("Failed to save sync point for bundle %d: %s", e.ID, err)
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

	if err := wh.LoadToWarehouse(filename, processedBundles...); err != nil {
		log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
		return 0, err
	}

	if err := wh.SaveSyncPoints(processedBundles...); err != nil {
		log.Printf("Failed to save sync points for bundles ending with %d: %s", processedBundles[len(processedBundles)].ID, err)
		return 0, err
	}

	log.Printf("Processing of %d bundles (%d records) took %s", len(processedBundles), totalRecords,
		time.Since(mark))

	// return how many files were processed
	return len(processedBundles), nil
}

func WriteBundleToCSV(fs *fullstory.Client, bundleID int, csvOut *csv.Writer, wh warehouse.Warehouse) (numRecords int, err error) {
	stream, err := fs.ExportData(bundleID)
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
		line, err := TransformExportJsonRecord(wh, r)
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
