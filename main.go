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

	"./config"
	"./warehouse"

	"github.com/nishanths/fullstory"
)

var (
	conf               *config.Config
	currentBackoffStep = uint(0)
)

// Represents a single export row in the export file
type Record map[string]interface{}

func TransformExportJsonRecord(wh warehouse.Warehouse, rec map[string]interface{}) ([]string, error) {
	var line []string
	for _, field := range wh.ExportTableSchema() {
		if field.IsCustomVars {
			continue
		}

		if val, ok := rec[field.Name]; ok {
			line = append(line, wh.ValueToString(val, field))
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

func ProcessExportsSince(wh warehouse.Warehouse, since time.Time) (int, error) {
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

	for _, e := range exports {
		log.Printf("Processing bundle %d (start: %s, end: %s)", e.ID, e.Start, e.Stop)
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

		stream, err := fs.ExportData(e.ID)
		if err != nil {
			log.Printf("Failed to fetch bundle %d: %s", e.ID, err)
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
			log.Printf("Failed json decode of array token: %s", err)
			return 0, err
		}

		var recordCount int
		for decoder.More() {
			var r Record
			if err := decoder.Decode(&r); err != nil {
				log.Printf("failed json decode of record: %s", err)
				return 0, err
			}
			line, err := TransformExportJsonRecord(wh, r)
			if err != nil {
				log.Printf("Failed object transform, bundle %d; skipping record. %s", e.ID, err)
				continue
			}
			csvOut.Write(line)
			recordCount++
		}

		if _, err := decoder.Token(); err != nil {
			log.Printf("Failed json decode of array token: %s", err)
			return 0, err
		}

		csvOut.Flush()

		if err := wh.LoadToWarehouse(filename); err != nil {
			log.Printf("Failed to load file '%s' to warehouse: %s", filename, err)
			return 0, err
		}

		if err := wh.SaveSyncPoint(e.ID, e.Stop); err != nil {
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

	var wh warehouse.Warehouse
	switch conf.Warehouse {
	case "redshift":
		wh = warehouse.NewRedshift(conf)
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

		numBundles, err := ProcessExportsSince(wh, lastSyncedRecord)
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
