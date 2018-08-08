package warehouse

import (
	"testing"

	"github.com/fullstorydev/hauser/config"
)

var _ Warehouse = &Redshift{}

func TestRedshiftValueToString(t *testing.T) {
	wh := &Redshift{
		conf: &config.Config{
			Redshift: config.RedshiftConfig{
				VarCharMax: 20,
			},
		},
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
		schema    Schema
		columns   []string
		expected  int
	} {
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
