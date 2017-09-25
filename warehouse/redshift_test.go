package warehouse

import (
	"reflect"
	"testing"
	"time"
)

var _ Field = &dataField{}

func TestToColumnDef(t *testing.T) {
	var testCases = []struct {
		input    Field
		expected string
	}{
		{&dataField{name: "StringField", dataType: reflect.TypeOf(""), isTime: false}, "StringField varchar(max)"},
		{&dataField{name: "BigintField", dataType: reflect.TypeOf(int64(0)), isTime: false}, "BigintField BIGINT"},
		{&dataField{name: "TimestampField", dataType: reflect.TypeOf(time.Now()), isTime: false}, "TimestampField TIMESTAMP"},
	}

	for _, testCase := range testCases {
		if got := toColumnDef(testCase.input); got != testCase.expected {
			t.Errorf("Expected column definition %q from field %v, got %q", testCase.expected, testCase.input, got)
		}
	}
}
