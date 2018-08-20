package fullstory

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// BaseURL is the base URL for the fullstory.com API.
const BaseURL = "https://export.fullstory.com/api/v1"

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

// Client represents a HTTP client for making requests to the FullStory API.
type Client struct {
	HTTPClient *http.Client
	Config
}

// Config is configuration for Client.
type Config struct {
	APIToken string
	BaseURL  string
}

// NewClient returns a Client initialized with http.DefaultClient and the
// supplied apiToken.
func NewClient(apiToken string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		Config: Config{
			APIToken: apiToken,
			BaseURL:  BaseURL,
		},
	}
}

// doReq performs the supplied HTTP request and returns the data in the response.
// Necessary authentication headers are added before performing the request.
//
// If the error is nil, the caller is responsible for closing the returned data.
func (c *Client) doReq(req *http.Request) (io.ReadCloser, error) {
	req.Header.Set("Authorization", "Basic " + c.APIToken)
	req.Header.Set("Accept-encoding", "*")

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
