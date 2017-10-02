package warehouse

import (
	"reflect"
	"testing"
	"time"

	"../config"
)

var _ Warehouse = &Redshift{}
var _ Field = &dataField{}

func TestToColumnDef(t *testing.T) {
	var testCases = []struct {
		input    Field
		expected string
	}{
		{&dataField{name: "StringField", dataType: reflect.TypeOf(""), isTime: false}, "StringField VARCHAR(max)"},
		{&dataField{name: "BigintField", dataType: reflect.TypeOf(int64(0)), isTime: false}, "BigintField BIGINT"},
		{&dataField{name: "TimestampField", dataType: reflect.TypeOf(time.Now()), isTime: true}, "TimestampField TIMESTAMP"},
	}

	for _, testCase := range testCases {
		if got := toColumnDef(testCase.input); got != testCase.expected {
			t.Errorf("Expected column definition %q from field %v, got %q", testCase.expected, testCase.input, got)
		}
	}
}

func TestRedshiftValueToString(t *testing.T) {
	wh := &Redshift{
		conf: &config.Config{
			Redshift: config.RedshiftConfig{
				VarCharMax: 20,
			},
		},
	}
	ts := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	var testCases = []struct {
		input    interface{}
		field    Field
		expected string
	}{
		{"short string", &dataField{dataType: reflect.TypeOf("")}, "short string"},
		{"I'm too long, truncate me", &dataField{dataType: reflect.TypeOf("")}, "I'm too long, trunc"},
		{"no\nnew\nlines", &dataField{dataType: reflect.TypeOf("")}, "no new lines"},
		{"no\x00null\x00chars", &dataField{dataType: reflect.TypeOf("")}, "nonullchars"},
		{5, &dataField{dataType: reflect.TypeOf(5)}, "5"},
		{"2009-11-10T23:00:00.000Z", &dataField{dataType: reflect.TypeOf(ts), isTime: true}, "2009-11-10 23:00:00 +0000 UTC"},
	}

	for _, testCase := range testCases {
		if got := wh.ValueToString(testCase.input, testCase.field); got != testCase.expected {
			t.Errorf("Expected value %q, got %q", testCase.expected, got)
		}
	}
}
