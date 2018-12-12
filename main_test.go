package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/fullstorydev/hauser/warehouse"
	"github.com/nishanths/fullstory"
	"github.com/pkg/errors"
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

func TestTransformExportJSONRecord(t *testing.T) {
	testCases := []struct {
		tableColumns []string
		rec          map[string]interface{}
		expResult    []string
	}{
		// no custom vars
		{
			tableColumns: []string{"eventtargettext", "pageduration", "customvars"},
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
				"PageDuration":    42,
			},
			expResult: []string{"Heyo!", "42", `{}`},
		},
		// two custom vars
		{
			tableColumns: []string{"eventtargettext", "pageduration", "customvars"},
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
				"PageDuration":    42,
				"myCustom_str":    "Heyo again!",
				"myCustom_num":    42,
			},
			expResult: []string{"Heyo!", "42", `{"myCustom_str":"Heyo again!","myCustom_num":42}`},
		},
		// missing column value for pageduration
		{
			tableColumns: []string{"eventtargettext", "pageduration", "customvars"},
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
			},
			expResult: []string{"Heyo!", "", `{}`},
		},
		// additional columns in target table that are not in the export
		{
			tableColumns: []string{"eventtargettext", "pageduration", "customvars", "randomcolumnnotinexport"},
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
				"PageDuration":    42,
			},
			expResult: []string{"Heyo!", "42", `{}`, ""},
		},
	}

	for i, tc := range testCases {
		wh := StubWarehouse{}
		result, err := TransformExportJSONRecord(&wh, tc.tableColumns, tc.rec)
		if err != nil {
			t.Errorf("Unexpected err %s on test case %d", err, i)
			continue
		}
		if len(result) != len(tc.expResult) {
			t.Errorf("Incorrect length of result; expected %d, got %d on test case %d", len(result), len(tc.expResult), i)
			continue
		}
		for j := range tc.expResult {
			if !compareTransformedStrings(t, tc.expResult[j], result[j]) {
				t.Errorf("Result mismatch; expected %s, got %s on test case %d, item %d", tc.expResult[j], result[j], i, j)
			}
		}
	}
}

func compareTransformedStrings(t *testing.T, str1, str2 string) bool {
	if str1 == str2 {
		return true
	}
	if len(str1) > 0 && str1[0] == '{' {
		return compareJSONStrings(t, str2, str2)
	}
	return false
}

func compareJSONStrings(t *testing.T, str1, str2 string) bool {
	// decode JSON
	var m1, m2 interface{}
	if err := json.Unmarshal([]byte(str1), &m1); err != nil {
		t.Fatalf("Could not unmarshal JSON string: %s", str1)
	}
	if err := json.Unmarshal([]byte(str2), &m2); err != nil {
		t.Fatalf("Could not unmarshal JSON string: %s", str2)
	}
	decoded1 := m1.(map[string]interface{})
	decoded2 := m2.(map[string]interface{})

	// compare decoded maps
	for key1, value1 := range decoded1 {
		value2, ok := decoded2[key1]
		if !ok || value1 != value2 {
			return false
		}
	}
	return true
}

type StubWarehouse struct {
	warehouse.Warehouse
}

func (sw *StubWarehouse) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(time.RFC3339)
	}
	return s
}
