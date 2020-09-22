package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/fullstorydev/hauser/config"
)

var _ error = StatusError{}

// StatusError is returned when the HTTP roundtrip succeeds, but the response status
// does not equal http.StatusOK.
type StatusError struct {
	Status     string
	StatusCode int
	RetryAfter time.Duration
	Body       io.Reader
}

func (e StatusError) Error() string {
	return fmt.Sprintf("fullstory: response error: Status:%s, StatusCode:%d, RetryAfter:%v", e.Status, e.StatusCode, e.RetryAfter)
}

// DataExportClient represents an interface for interacting with the FullStory Data Export API
type DataExportClient interface {
	// ExportList returns a list of exports that contain data from the provided start time.
	// This list can then be used to request the actual data from `ExportData()` below.
	// DEPRECATED
	ExportList(start time.Time) ([]ExportMeta, error)
	// ExportData retrieves the data for a corresponding export ID and returns a reader for
	// the data.
	// DEPRECATED
	ExportData(id int, modifyReq ...func(r *http.Request)) (ExportData, error)

	// CreateExport starts an asynchronous export of the "Everyone" segment for the specified time range.
	// The time bounds for start and stop are inclusive and exclusive, respectively.
	// If successful, returns the id for the created export which can be used to check the progress.
	CreateExport(start time.Time, end time.Time) (string, error)

	// GetExportProgress returns the estimated progress of the export for the provided and whether
	// the export is ready for download. The progress value is an integer between 1 and 100
	// and represents and estimated completion percentage.
	GetExportProgress(operationId string) (progress int, ready bool, err error)

	// GetExport returns a stream for the provided export ID. If the export is not ready, this
	// will fail with ErrExportNotReady.
	GetExport(operationId string) (io.ReadCloser, error)
}

// Client represents a HTTP client for making requests to the FullStory API.
type Client struct {
	HTTPClient *http.Client
	Config     *config.Config
}

var _ DataExportClient = (*Client)(nil)

// NewClient returns a Client initialized with http.DefaultClient and the
// supplied apiToken.
func NewClient(config *config.Config) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Transport: &APIKeyRoundTripper{
				Key:               config.FsApiToken,
				AdditionalHeaders: config.AdditionalHttpHeader,
			},
		},
		Config: config,
	}
}

// doReq performs the supplied HTTP request and returns the data in the response.
// Necessary authentication headers are added before performing the request.
//
// If the error is nil, the caller is responsible for closing the returned data.
func (c *Client) doReq(req *http.Request) (io.ReadCloser, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		retryAfter := getRetryAfter(resp)
		b := &bytes.Buffer{}
		io.Copy(b, resp.Body) // Ignore error.
		return nil, StatusError{
			Body:       b,
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			RetryAfter: time.Duration(retryAfter) * time.Second,
		}
	}

	return resp.Body, nil
}

// getRetryAfter returns the value of the "Retry-After" header as an integer.
// When applicable, FullStory APIs set this header to the number of seconds
// to wait before retrying. If the header isn't present or if there is an
// error parsing the value, it returns 0.
func getRetryAfter(resp *http.Response) int {
	header := resp.Header.Get("Retry-After")
	if header != "" {
		if result, err := strconv.Atoi(header); err == nil {
			return result
		}
	}

	return 0
}
