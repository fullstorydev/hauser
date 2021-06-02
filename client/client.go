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
	// CreateExport starts an asynchronous export of the "Everyone" segment for the specified time range.
	// The time bounds for start and stop are inclusive and exclusive, respectively.
	// If successful, returns the id for the created export which can be used to check the progress.
	CreateExport(start time.Time, end time.Time, fields []string) (string, error)

	// GetExportProgress returns the estimated progress of the export for the provided operation and the id
	// of the export if ready for download. The progress value is an integer between 1 and 100
	// and represents and estimated completion percentage.
	GetExportProgress(operationId string) (progress int, exportId string, err error)

	// GetExport returns a stream for the provided export ID. If the export is not ready, this
	// will fail with ErrExportNotReady.
	GetExport(exportId string) (io.ReadCloser, error)
}

// Client represents a HTTP client for making requests to the FullStory API.
type Client struct {
	HTTPClient            *http.Client
	Config                *config.Config
	createRequestModifier func(r *http.Request)
}

type Option func(*Client)

// WithCreateExportRequestModifier allows for a modification to the CreateExport API request.
// This can be used to modify the request body to customize any of the request parameters.
func WithCreateExportRequestModifier(rm func(r *http.Request)) Option {
	return func(c *Client) {
		c.createRequestModifier = rm
	}
}

// WithHttpClient replaces the default API key-based http client. This option can be used,
// for example, to customize the underlying transport.
func WithHttpClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

var _ DataExportClient = (*Client)(nil)

// NewClient returns a Client initialized with http.DefaultClient and the
// supplied apiToken.
func NewClient(config *config.Config, opts ...Option) *Client {
	c := &Client{
		HTTPClient: &http.Client{
			Transport: &APIKeyRoundTripper{
				Key:               config.FsApiToken,
				AdditionalHeaders: config.AdditionalHttpHeader,
			},
		},
		Config: config,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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
