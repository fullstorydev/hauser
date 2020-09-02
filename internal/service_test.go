package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	hausertest "github.com/fullstorydev/hauser/testing"
	"github.com/fullstorydev/hauser/testing/testutils"
	"github.com/fullstorydev/hauser/warehouse"
	"github.com/pkg/errors"
)

var update = flag.Bool("update", false, "update upload files")

func Ok(t *testing.T, err error, format string, a ...interface{}) {
	if err != nil {
		format += ": unexpected error: %s"
		a = append(a, err)
		t.Errorf(format, a...)
	}
}

func TestHauser(t *testing.T) {

	testCases := []struct {
		name            string
		testdata        string
		outputDir       string
		freqSetting     int32
		expectedBundles int
		config          *config.Config
	}{
		{
			name:            "base case",
			testdata:        "../testing/testdata/raw.json",
			outputDir:       "../testing/testdata/default",
			freqSetting:     48,
			expectedBundles: 5,
			config:          &config.Config{},
		},
		{
			name:            "group by day case",
			testdata:        "../testing/testdata/raw.json",
			outputDir:       "../testing/testdata/groupByDay",
			freqSetting:     48,
			expectedBundles: 5,
			config: &config.Config{
				GroupFilesByDay: true,
			},
		},
		{
			name:            "storage only",
			testdata:        "../testing/testdata/raw.json",
			outputDir:       "../testing/testdata/json",
			freqSetting:     48,
			expectedBundles: 5,
			config: &config.Config{
				SaveAsJson:  true,
				StorageOnly: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fsClient := hausertest.NewMockDataExportClient(tc.freqSetting, tc.testdata)
			storage := hausertest.NewMockStorage()

			var db *hausertest.MockDatabase
			if !tc.config.StorageOnly {
				db = hausertest.NewMockDatabase()
			}

			hauser := NewHauser(tc.config, fsClient, storage, db)
			err := hauser.Init(ctx)

			Ok(t, err, "failed to init")
			if db != nil {
				testutils.Assert(t, db.Initialized, "expected warehouse to be initialized")
			}

			numBundles := 0
			for {
				newBundles, err := hauser.ProcessNext(ctx)
				Ok(t, err, "failed to process next bundles")
				if newBundles == 0 {
					break
				}
				numBundles += newBundles
			}
			testutils.Equals(t, tc.expectedBundles, numBundles, "wrong number of bundles processed")
			testutils.Equals(t, tc.expectedBundles, len(storage.UploadedFiles), "unexpected number of upload files")
			if db != nil {
				// Files should only be deleted from storage if they were successfully loaded into the database
				testutils.Equals(t, tc.expectedBundles, len(storage.DeletedFiles), "unexpected number of deleted files")
				testutils.Equals(t, tc.expectedBundles, len(db.LoadedFiles), "unexpected number of loaded files")
				for i, loaded := range db.LoadedFiles {
					testutils.Equals(t, loaded, fmt.Sprintf("mock://%s", storage.DeletedFiles[i]), "unexpected loaded file")
				}
			}

			for name, data := range storage.UploadedFiles {
				fname := path.Join(tc.outputDir, name)
				if *update {
					_ = os.MkdirAll(tc.outputDir, os.ModePerm)
					Ok(t, ioutil.WriteFile(fname, data, os.ModePerm), "failed to write test file")
				} else {
					expected, err := ioutil.ReadFile(fname)
					Ok(t, err, "failed to read expected output file")
					testutils.Assert(t, bytes.Equal(expected, data), "uploaded file doesn't match expected")
				}
			}
		})
	}
}

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
			err:           client.StatusError{StatusCode: http.StatusTooManyRequests, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           client.StatusError{StatusCode: http.StatusInternalServerError, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           client.StatusError{StatusCode: http.StatusServiceUnavailable, RetryAfter: 3 * time.Second},
			expDoRetry:    true,
			expRetryAfter: 3 * time.Second,
		},
		{
			err:           client.StatusError{StatusCode: http.StatusNotFound, RetryAfter: 3 * time.Second},
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
		result, err := TransformExportJSONRecord(warehouse.ValueToString, tc.tableColumns, tc.rec)
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
