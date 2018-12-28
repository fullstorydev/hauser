package warehouse

import (
	"testing"

	"github.com/fullstorydev/hauser/config"
)

var _ Warehouse = &Redshift{}

func makeConf(databaseSchema string) *config.Config {
	conf := &config.Config{
		Redshift: config.RedshiftConfig{
			DatabaseSchema: databaseSchema,
			VarCharMax:     20,
			ExportTable:    "exportTable",
			SyncTable:      "syncTable",
		},
	}
	return conf
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

func TestGetMissingFieldsRedshift(t *testing.T) {
	wh := &Redshift{
		exportSchema: ExportTableSchema(RedshiftTypeMap),
	}

	var schemaHeaders []string
	for _, f := range wh.exportSchema {
		schemaHeaders = append(schemaHeaders, f.Name)
	}

	var noHeaders []string
	var testCases = []struct {
		schema   Schema
		columns  []string
		expected int
	}{
		// All columns from the schema are present in the export table columns, so there are 0 missing fields
		{wh.exportSchema, schemaHeaders, 0},
		// Only headers are present, therefore all schema fields are missing
		{wh.exportSchema, noHeaders, len(schemaHeaders)},
		// Only headers that are not part of the schema are present, therefore all schema fields are missing
		{wh.exportSchema, []string{"Dummy"}, len(schemaHeaders)},
		// Only one column common between the table columns, and schema, so there are len(schemaHeaders) - 1 missing field
		{wh.exportSchema, []string{"PageUrl"}, len(schemaHeaders) - 1},
		// Same as above, but with additional columns in the export table that we don't care about so still 1 missing field
		{wh.exportSchema, []string{"Dummy Column1", "PageUrl", "Dummy Column2"}, len(schemaHeaders) - 1},
	}

	for _, testCase := range testCases {
		missingFields := wh.getMissingFields(testCase.schema, testCase.columns)
		if len(missingFields) != testCase.expected {
			t.Errorf("Expected %d missing fields, got %d", testCase.expected, len(missingFields))
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
		conf       *config.Config
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
			t.Errorf("unexpected error thrown for DatabaseSchema %s: %s", tc.conf.Redshift.DatabaseSchema, err)
		}
	}
}

func TestGetExportTableName(t *testing.T) {
	testCases := []struct {
		conf     *config.Config
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
		conf     *config.Config
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
		conf     *config.Config
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
