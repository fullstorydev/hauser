package client

import (
	"net/http"

	"github.com/fullstorydev/hauser/config"
)

// FullStory is an HTTP transport which wraps the underlying transport and
// sets the Authorization header
type FullStory struct {
	Key               string
	AdditionalHeaders []config.Header

	// Transport is the underlying HTTP transport.
	// if nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

func (t *FullStory) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	newReq := *req
	newReq.Header.Set("Authorization", "Basic "+t.Key)
	for _, header := range t.AdditionalHeaders {
		newReq.Header.Set(header.Key, header.Value)
	}
	return rt.RoundTrip(&newReq)
}
