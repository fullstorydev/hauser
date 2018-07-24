package warehouse

import (
	"testing"

	"cloud.google.com/go/bigquery"
)

var _ Warehouse = &BigQuery{}

func TestGetMissingFields(t *testing.T) {
	wh := &BigQuery{}

	requiredField1 := bigquery.FieldSchema {
		Name: "RequiredField1",
	}
	requiredField2 := bigquery.FieldSchema {
		Name: "RequiredField2",
	}
	dummyField := bigquery.FieldSchema {
		Name: "DummyField",
	}

	hauserSchema := bigquery.Schema([]*bigquery.FieldSchema {
		&requiredField1,
		&requiredField2,
	})

	var testCases = []struct {
		hauserSchema	bigquery.Schema
		tableSchema		bigquery.Schema
		missingFields	int
	} {
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema {
			}),
			2,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema {
				&dummyField,
			}),
			2,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema {
				&requiredField2,
				&dummyField,
			}),
			1,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema {
				&requiredField1,
				&requiredField2,
			}),
			0,
		},
	}

	for _, testCase := range testCases {
		if got := wh.GetMissingFields(testCase.hauserSchema, testCase.tableSchema); len(got) != testCase.missingFields {
			t.Errorf("Expected %d missing fields, got %d", testCase.missingFields, len(got))
		}
	}
}

// The purpose of this test is to make sure we mark Required to false for all the fields we add
func TestAppendToSchema(t *testing.T) {
	wh := &BigQuery{}

	requiredField1 := bigquery.FieldSchema {
		Name: "RequiredField1",
		Required: true,
	}
	requiredField2 := bigquery.FieldSchema {
		Name: "RequiredField2",
		Required: false,
	}

	emptySchema := bigquery.Schema([]*bigquery.FieldSchema {})

	var testCases = []struct {
		hauserSchema	bigquery.Schema
		requiredFields	[]*bigquery.FieldSchema
	} {
		{
			emptySchema,
			[]*bigquery.FieldSchema {},
		},
		{
			emptySchema,
			[]*bigquery.FieldSchema {
				&requiredField1,
			},
		},
		{
			emptySchema,
			[]*bigquery.FieldSchema {
				&requiredField1,
				&requiredField2,
			},
		},
	}

	for _, testCase := range testCases {
		schema := wh.AppendToSchema(testCase.hauserSchema, testCase.requiredFields)
		if len(schema) != len(testCase.requiredFields) {
			t.Errorf("Expected %d fields to be appended, got %d", len(testCase.requiredFields), len(schema))
		}
		for _, f := range schema {
			if f.Required {
				t.Errorf("Expected all fields to have 'Required'=false got true for field %s", f.Name)
			}
		}
	}
}
