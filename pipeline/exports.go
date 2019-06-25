package pipeline

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fullstorydev/hauser/config"
	"github.com/nishanths/fullstory"
)

const (
	// Maximum number of times Hauser will attempt to retry each request made to FullStory
	maxAttempts int = 3

	// Default duration Hauser will wait before retrying a 429 or 5xx response. If Retry-After is specified, uses that instead. Default arbitrarily set to 10s.
	defaultRetryAfterDuration = time.Duration(10) * time.Second
)

// Record represents a single event in the downloaded event export
type Record map[string]interface{}

// RecordGroup represents a group of downloaded exports that may contain one or more "bundles" downloaded from FullStory.
type RecordGroup struct {
	bundles []fullstory.ExportMeta
	records []Record
}

// ExportData contains metadata about a certain export as well as a reader for the raw data.
type ExportData struct {
	meta fullstory.ExportMeta
	src  io.Reader
}

// SavedExport
type SavedExport struct {
	Meta     []fullstory.ExportMeta
	Filename string
}

// GetRecords translates a raw data export download into an array of Records. This includes unzipping and decoding the
// raw JSON.
func (d *ExportData) GetRecords() ([]Record, error) {
	stream, err := gzip.NewReader(d.src)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	var recs []Record

	decoder := json.NewDecoder(stream)
	decoder.UseNumber()

	// skip array open delimiter
	if _, err := decoder.Token(); err != nil {
		log.Printf("Failed json decode of array open token: %s", err)
		return nil, err
	}

	for decoder.More() {
		var r Record
		decoder.Decode(&r)
		recs = append(recs, r)
	}

	if _, err := decoder.Token(); err != nil {
		log.Fatalf("Failed json decode of array close token: %s", err)
	}

	return recs, nil
}

func getFSClient(conf *config.Config) *fullstory.Client {
	fs := fullstory.NewClient(conf.FsApiToken)
	if conf.ExportURL != "" {
		fs.Config.BaseURL = conf.ExportURL
	}
	return fs
}

func getDataWithRetry(fs *fullstory.Client, meta fullstory.ExportMeta) (ExportData, error) {
	log.Printf("Getting Export Data for bundle %d\n", meta.ID)
	var fsErr error
	for r := 1; r <= maxAttempts; r++ {
		stream, err := fs.ExportData(meta.ID, withAcceptEncoding())
		if err != nil {
			log.Printf("Failed to fetch export data for Bundle %d: %s", meta.ID, err)

			fsErr = err
			doRetry, retryAfterDuration := getRetryInfo(err)
			if !doRetry {
				return ExportData{}, err
			}

			log.Printf("Attempt #%d failed. Retrying after %s\n", r, retryAfterDuration)
			time.Sleep(retryAfterDuration)
			continue
		}

		return ExportData{src: stream, meta: meta}, nil
	}
	return ExportData{}, fsErr
}

func withAcceptEncoding() func(r *http.Request) {
	return func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "*")
	}
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
