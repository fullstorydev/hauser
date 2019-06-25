package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/pipeline"
	"github.com/fullstorydev/hauser/warehouse"
	"github.com/nishanths/fullstory"
)

var (
	conf               *config.Config
	currentBackoffStep = uint(0)
	bundleFieldsMap    = warehouse.BundleFields()
)

const (
	// Maximum number of times Hauser will attempt to retry each request made to FullStory
	maxAttempts int = 3

	// Default duration Hauser will wait before retrying a 429 or 5xx response. If Retry-After is specified, uses that instead. Default arbitrarily set to 10s.
	defaultRetryAfterDuration = time.Duration(10) * time.Second
)

// Record represents a single export row in the export file
type Record map[string]interface{}

// TransformExportJSONRecord transforms the record map (extracted from the API response json) to a
// slice of strings. The slice of strings contains values in the same order as the existing export table.
// For existing export table fields that do not exist in the json record, an empty string is populated.
func TransformExportJSONRecord(rec Record, tableColumns []string, convert func(val interface{}, isTime bool) string) ([]string, error) {
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

func LoadBundles(wh warehouse.Warehouse, filename string, bundles ...fullstory.ExportMeta) error {
	var objPath string
	var err error
	if objPath, err = wh.UploadFile(filename); err != nil {
		log.Printf("%s", wh.GetUploadFailedMsg(filename, err))
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

func getRetryInfo(err error) (bool, time.Duration) {
	if statusError, ok := err.(fullstory.StatusError); ok {
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
	case "local":
		wh = warehouse.NewLocalDisk(conf)
	case "redshift":
		wh = warehouse.NewRedshift(conf)
		if conf.SaveAsJson {
			if !conf.S3.S3Only {
				log.Fatalf("Hauser doesn't currently support loading JSON into Redshift.  Ensure SaveAsJson = false in .toml file.")
			}
		}
	case "bigquery":
		wh = warehouse.NewBigQuery(conf)
		if conf.SaveAsJson {
			if !conf.GCS.GCSOnly {
				log.Fatalf("Hauser doesn't currently support loading JSON into BigQuery.  Ensure SaveAsJson = false in .toml file.")
			}
		}
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

	// NB: We SHOULD fetch the table columns ONLY after the call to EnsureCompatibleExportTable.
	// The EnsureCompatibleExportTable function potentially alters the schema of the export table in the client warehouse.
	tableColumns := wh.GetExportTableColumns()

	recordTransform := func(rec map[string]interface{}) ([]string, error) {
		return TransformExportJSONRecord(rec, tableColumns, wh.ValueToString)
	}

	startTime, err := wh.LastSyncPoint()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Looking for bundles since %s", startTime.Format(time.RFC3339))

	p := pipeline.NewPipeline(conf, recordTransform)
	savedExports, errs := p.Start(startTime)
	defer p.Stop()

	for {
		select {
		case savedExport := <-savedExports:
			log.Printf("Bundle saved to: %s", savedExport.Filename)

			for {
				err := LoadBundles(wh, savedExport.Filename, savedExport.Meta...)
				if BackoffOnError(err) {
					continue
				}
				break
			}

			err = os.Remove(savedExport.Filename)
			if err != nil {
				log.Printf("Error removing temporary file: %s", err)
			}
		case err := <-errs:
			if BackoffOnError(err) {
				continue
			}
		}
	}
}
