package warehouse

import (
	"testing"

	"cloud.google.com/go/bigquery"
)

var _ Warehouse = &BigQuery{}

func TestGetMissingFields(t *testing.T) {
	wh := &BigQuery{}

	requiredField1 := bigquery.FieldSchema{
		Name: "RequiredField1",
	}
	requiredField1Lowercase := bigquery.FieldSchema{
		Name: "requiredfield1",
	}
	requiredField2 := bigquery.FieldSchema{
		Name: "RequiredField2",
	}
	dummyField := bigquery.FieldSchema{
		Name: "DummyField",
	}

	hauserSchema := bigquery.Schema([]*bigquery.FieldSchema{
		&requiredField1,
		&requiredField2,
	})

	var testCases = []struct {
		hauserSchema  bigquery.Schema
		tableSchema   bigquery.Schema
		missingFields int
	}{
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema{}),
			2,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema{
				&dummyField,
			}),
			2,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema{
				&requiredField2,
				&dummyField,
			}),
			1,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema{
				&requiredField1,
				&requiredField2,
			}),
			0,
		},
		{
			hauserSchema,
			bigquery.Schema([]*bigquery.FieldSchema{
				&requiredField1Lowercase,
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
