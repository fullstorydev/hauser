package main

import (
	"testing"
	"net/http"
	"time"
	"github.com/pkg/errors"
	"github.com/nishanths/fullstory"
)

func TestGetRetryInfo(t *testing.T) {
	testCases := []struct {
		err           error
		expDoRetry    bool
		expRetryAfter time.Duration
	}{
		{
			err:           errors.New("random error!"),
			expDoRetry:    true,
			expRetryAfter: defaultRetryAfterDuration,
		},
		{
			err:           fullstory.StatusError{StatusCode: http.StatusTooManyRequests, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           fullstory.StatusError{StatusCode: http.StatusInternalServerError, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           fullstory.StatusError{StatusCode: http.StatusServiceUnavailable, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           fullstory.StatusError{StatusCode: http.StatusNotFound, RetryAfter: 3 * time.Second},
			expDoRetry:    false,
			expRetryAfter: defaultRetryAfterDuration,
		},
	}

	for i, tc := range testCases {
		doRetry, retryAfter := getRetryInfo(tc.err)
		if doRetry != tc.expDoRetry {
			t.Errorf("expected %t, got %t for doRetry on test case %d", tc.expDoRetry, doRetry, i)
		}
		if retryAfter != tc.expRetryAfter {
			t.Errorf("expected %v, got %v for doRetry on test case %d", tc.expRetryAfter, retryAfter, i)
		}
	}
}
