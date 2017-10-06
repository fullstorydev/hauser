package warehouse

import (
	"testing"

	"../config"
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
		field    Field
		expected string
	}{
		{"short string", Field{IsTime: false}, "short string"},
		{"I'm too long, truncate me", Field{IsTime: false}, "I'm too long, trunc"},
		{"no\nnew\nlines", Field{IsTime: false}, "no new lines"},
		{"no\x00null\x00chars", Field{IsTime: false}, "nonullchars"},
		{5, Field{IsTime: false}, "5"},
		{"2009-11-10T23:00:00.000Z", Field{IsTime: true}, "2009-11-10 23:00:00 +0000 UTC"},
	}

	for _, testCase := range testCases {
		if got := wh.ValueToString(testCase.input, testCase.field); got != testCase.expected {
			t.Errorf("Expected value %q, got %q", testCase.expected, got)
		}
	}
}
