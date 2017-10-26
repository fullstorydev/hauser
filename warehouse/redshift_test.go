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
