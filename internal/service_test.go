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

	getNow = func() time.Time {
		return time.Date(2020, 9, 2, 0, 0, 0, 0, time.UTC)
	}
	progressPollDuration = time.Millisecond
	testCases := []struct {
		name            string
		testdata        string
		outputDir       string
		initialColumns  []string
		expectedBundles int
		config          *config.Config
	}{
		{
			name:      "base case",
			testdata:  "../testing/testdata/raw.json",
			outputDir: "../testing/testdata/default",
			initialColumns: []string{
				"EventCustomName",
				"EventStart",
				"EventType",
				"EventTargetText",
				"EventTargetSelectorTok",
				"EventModFrustrated",
				"EventModDead",
				"EventModError",
				"EventModSuspicious",
				"IndvId",
				"PageClusterId",
				"PageUrl",
				"PageDuration",
				"PageActiveDuration",
				"PageRefererUrl",
				"PageLatLong",
				"PageAgent",
				"PageIp",
				"PageBrowser",
				"PageDevice",
				"PageOperatingSystem",
				"PageNumInfos",
				"PageNumWarnings",
				"PageNumErrors",
				"SessionId",
				"PageId",
				"UserAppKey",
				"UserEmail",
				"UserDisplayName",
				"UserId",
				"CustomVars",
				"LoadDomContentTime",
				"LoadFirstPaintTime",
				"LoadEventTime",
			},
			expectedBundles: 6,
			config: &config.Config{
				Provider:       "gcp",
				ExportDuration: config.Duration{Duration: 24 * time.Hour},
				StartTime:      time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:            "group by day case",
			testdata:        "../testing/testdata/raw.json",
			outputDir:       "../testing/testdata/groupByDay",
			expectedBundles: 6,
			config: &config.Config{
				Provider:        "gcp",
				GroupFilesByDay: true,
				StartTime:       time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:            "storage only",
			testdata:        "../testing/testdata/raw.json",
			outputDir:       "../testing/testdata/json",
			expectedBundles: 6,
			config: &config.Config{
				Provider:       "local",
				SaveAsJson:     true,
				StorageOnly:    true,
				ExportDuration: config.Duration{Duration: 24 * time.Hour},
				StartTime:      time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "with new columns",
			testdata:  "../testing/testdata/raw.json",
			outputDir: "../testing/testdata/existing",
			initialColumns: []string{
				"EventStart",
				"PageAgent",
				"EventTargetSelectorTok",
				"CustomColumn",
			},
			expectedBundles: 6,
			config: &config.Config{
				Provider:       "gcp",
				ExportDuration: config.Duration{Duration: 24 * time.Hour},
				StartTime:      time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fsClient := hausertest.NewMockDataExportClient(tc.testdata)
			storage := hausertest.NewMockStorage()

			var db *hausertest.MockDatabase
			if !tc.config.StorageOnly {
				db = hausertest.NewMockDatabase(tc.initialColumns)
			}

			Ok(t, config.Validate(tc.config, getNow), "invalid config")

			hauser := NewHauserService(tc.config, fsClient, storage, db)
			err := hauser.Init(ctx)

			Ok(t, err, "failed to init")
			if db != nil {
				testutils.Assert(t, db.Initialized, "expected warehouse to be initialized")
			}

			numBundles := 0
			for {
				timeToWait, err := hauser.ProcessNext(ctx)
				Ok(t, err, "failed to process next bundles")
				if timeToWait > 0 {
					break
				}
				numBundles++
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
	defaultSchema := warehouse.MakeSchema(struct {
		EventTargetText string
		PageDuration    int64
		CustomVars      string
	}{})
	testCases := []struct {
		schema    warehouse.Schema
		rec       map[string]interface{}
		expResult []string
	}{
		// no custom vars
		{
			schema: defaultSchema,
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
				"PageDuration":    42,
			},
			expResult: []string{"Heyo!", "42", `{}`},
		},
		// two custom vars
		{
			schema: defaultSchema,
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
			schema: defaultSchema,
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
			},
			expResult: []string{"Heyo!", "", `{}`},
		},
		// additional columns in target table that are not in the export
		{
			schema: append(defaultSchema, warehouse.WarehouseField{
				DBName:             "RandomColumnNotInExport",
				FullStoryFieldName: "",
			}),
			rec: map[string]interface{}{
				"EventTargetText": "Heyo!",
				"PageDuration":    42,
			},
			expResult: []string{"Heyo!", "42", `{}`, ""},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {

			h := &HauserService{schema: tc.schema}
			result, err := h.transformExportJSONRecord(warehouse.ValueToString, tc.rec)
			if err != nil {
				t.Errorf("Unexpected err %s on test case %d", err, i)
			}
			if len(result) != len(tc.expResult) {
				t.Errorf("Incorrect length of result; expected %d, got %d on test case %d", len(result), len(tc.expResult), i)
			}
			for j := range tc.expResult {
				if !compareTransformedStrings(t, tc.expResult[j], result[j]) {
					t.Errorf("Result mismatch; expected %s, got %s on test case %d, item %d", tc.expResult[j], result[j], i, j)
				}
			}
		})
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
