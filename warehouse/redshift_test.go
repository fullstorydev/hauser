package warehouse

import (
	"testing"

	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/testing/testutils"
)

func makeConf(databaseSchema string) *config.RedshiftConfig {
	return &config.RedshiftConfig{
		DatabaseSchema: databaseSchema,
		VarCharMax:     20,
		ExportTable:    "exportTable",
		SyncTable:      "syncTable",
	}
}

func TestRedshiftSchema(t *testing.T) {
	fs := MakeSchema(BaseExportFields{}, MobileFields{})
	for _, field := range fs {
		_, ok := redshiftSchemaMap[field.FieldType]
		testutils.Assert(t, ok, "field type %v not found in redshiftSchemaMap", field.FieldType)
	}
}

func TestRedshiftValueToString(t *testing.T) {
	wh := &Redshift{
		conf: makeConf("some_schema"),
	}

	var testCases = []struct {
		input    interface{}
		isTime   bool
		expected string
	}{
		{"short string", false, "short string"},
		{"I'm too long, truncate me", false, "I'm too long, trunc"},
		{"no\nnew\nlines", false, "no new lines"},
		{"no\x00null\x00chars", false, "nonullchars"},
		{5, false, "5"},
		{"2009-11-10T23:00:00.000Z", true, "2009-11-10 23:00:00 +0000 UTC"},
	}

	for _, testCase := range testCases {
		if got := wh.ValueToString(testCase.input, testCase.isTime); got != testCase.expected {
			t.Errorf("Expected value %q, got %q", testCase.expected, got)
		}
	}
}

func TestGetBucketAndKey(t *testing.T) {
	testCases := []struct {
		s3Config  string
		fileName  string
		expBucket string
		expKey    string
	}{
		{
			s3Config:  "plainbucket",
			fileName:  "data.csv",
			expBucket: "plainbucket",
			expKey:    "data.csv",
		},
		{
			s3Config:  "hasslash/",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "data.csv",
		},
		{
			s3Config:  "hasslash/withpath",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "withpath/data.csv",
		}, {
			s3Config:  "hasslash/withpathwithslash/",
			fileName:  "data.csv",
			expBucket: "hasslash",
			expKey:    "withpathwithslash/data.csv",
		},
	}
	for _, tc := range testCases {
		bucketName, key := getBucketAndKey(tc.s3Config, tc.fileName)
		if bucketName != tc.expBucket {
			t.Errorf("getBucketAndKey(%s, %s) returned %s for bucketName, expected %s", tc.s3Config, tc.fileName, bucketName, tc.expBucket)
		}
		if key != tc.expKey {
			t.Errorf("getBucketAndKey(%s, %s) returned %s for key, expected %s", tc.s3Config, tc.fileName, key, tc.expKey)
		}
	}
}

func TestValidateSchemaConfig(t *testing.T) {

	testCases := []struct {
		conf       *config.RedshiftConfig
		hasError   bool
		errMessage string
	}{
		{
			conf:       makeConf(""),
			hasError:   true,
			errMessage: "DatabaseSchema definition missing from Redshift configuration. More information: https://github.com/fullstorydev/hauser/blob/master/Redshift.md#database-schema-configuration",
		},
		{
			conf:       makeConf("test"),
			hasError:   false,
			errMessage: "",
		},
		{
			conf:       makeConf("search_path"),
			hasError:   false,
			errMessage: "",
		},
	}

	for _, tc := range testCases {
		wh := NewRedshift(tc.conf)
		err := wh.validateSchemaConfig()
		if tc.hasError && err == nil {
			t.Errorf("expected Redshift.validateSchemaConfig() to return an error when config.Config.Redshift.DatabaseSchema is empty")
		}
		if tc.hasError && err.Error() != tc.errMessage {
			t.Errorf("expected Redshift.validateSchemaConfig() to return \n%s \nwhen config.Config.Redshift.DatabaseSchema is empty, returned \n%s \ninstead", tc.errMessage, err)
		}
		if !tc.hasError && err != nil {
			t.Errorf("unexpected error thrown for DatabaseSchema %s: %s", tc.conf.DatabaseSchema, err)
		}
	}
}

func TestGetExportTableName(t *testing.T) {
	testCases := []struct {
		conf     *config.RedshiftConfig
		expected string
	}{
		{
			conf:     makeConf("search_path"),
			expected: "exportTable",
		},
		{
			conf:     makeConf("mySchema"),
			expected: "mySchema.exportTable",
		},
	}

	for _, tc := range testCases {
		wh := NewRedshift(tc.conf)
		if got := wh.qualifiedExportTableName(); got != tc.expected {
			t.Errorf("Expected value %q, got %q", tc.expected, got)
		}
	}
}

func TestGetSyncTableName(t *testing.T) {
	testCases := []struct {
		conf     *config.RedshiftConfig
		expected string
	}{
		{
			conf:     makeConf("search_path"),
			expected: "syncTable",
		},
		{
			conf:     makeConf("mySchema"),
			expected: "mySchema.syncTable",
		},
	}

	for _, tc := range testCases {
		wh := NewRedshift(tc.conf)
		if got := wh.qualifiedSyncTableName(); got != tc.expected {
			t.Errorf("Expected value %q, got %q", tc.expected, got)
		}
	}
}

func TestGetSchemaParameter(t *testing.T) {
	testCases := []struct {
		conf     *config.RedshiftConfig
		expected string
	}{
		{
			conf:     makeConf("search_path"),
			expected: "current_schema()",
		},
		{
			conf:     makeConf("mySchema"),
			expected: "'mySchema'",
		},
	}

	for _, tc := range testCases {
		wh := NewRedshift(tc.conf)
		if got := wh.getSchemaParameter(); got != tc.expected {
			t.Errorf("Expected value %q, got %q", tc.expected, got)
		}
	}
}
