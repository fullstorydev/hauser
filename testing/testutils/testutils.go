package testutils

import (
	"reflect"
	"testing"
)

func Assert(t *testing.T, condition bool, format string, a ...interface{}) {
	t.Helper()
	if !condition {
		t.Fatalf(format, a...)
	}
}

func Equals(t *testing.T, expected, actual interface{}, format string, a ...interface{}) {
	t.Helper()
	if expected != actual {
		format += ": want %v (type %v), got %v (type %v)"
		a = append(a, expected, reflect.TypeOf(expected), actual, reflect.TypeOf(actual))
		t.Fatalf(format, a...)
	}
}

func StrSliceEquals(t *testing.T, expected, actual []string, format string, a ...interface{}) {
	t.Helper()
	format += ": want %v, got %v (type %v)"
	a = append(a, expected, reflect.TypeOf(expected), actual, reflect.TypeOf(actual))

	if len(expected) != len(actual) {
		t.Errorf(format, a)
	}
	for i, e := range expected {
		if e != actual[i] {
			t.Fatalf(format, a)
		}
	}
}
