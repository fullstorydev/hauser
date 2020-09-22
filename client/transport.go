package client

import (
	"net/http"

	"github.com/fullstorydev/hauser/config"
)

// APIKeyRoundTripper is an HTTP transport which wraps the underlying transport and
// sets the Authorization header
type APIKeyRoundTripper struct {
	Key               string
	AdditionalHeaders []config.Header

	// Transport is the underlying HTTP transport.
	// if nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

func (t *APIKeyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	req.Header.Set("Authorization", "Basic "+t.Key)
	for _, header := range t.AdditionalHeaders {
		req.Header.Set(header.Key, header.Value)
	}
	return rt.RoundTrip(req)
}
