package exec

import (
	"bytes"
	"testing"
)

func TestNewParams(t *testing.T) {
	testString := "test"
	w := bytes.NewBuffer([]byte{})
	p := NewParams(w)
	if _, err := p.Output.Write([]byte(testString)); err != nil {
		t.Fatalf("unexpected error writing to params.Output: %q", err)
	}
	if w.String() != testString {
		t.Fatalf("expected params.Output bytes to be %q, got %q", testString, w.String())
	}
}

func TestParams_validateParams(t *testing.T) {
	tcs := []struct {
		Name        string
		Params      Params
		ShouldError bool
	}{
		{
			Name:        "should return error if p.Fail and p.FailDefined are true",
			Params:      Params{Fail: true, FailDefined: true},
			ShouldError: true,
		},
		{
			Name:        "should return error if p.FailNonEmpty and p.Fail are true",
			Params:      Params{Fail: true, FailNonEmpty: true},
			ShouldError: true,
		},
		{
			Name:        "should return error if .FailNonEmpty and p.FailDefined are true",
			Params:      Params{FailNonEmpty: true, FailDefined: true},
			ShouldError: true,
		},
		{
			Name:        "should not return an error",
			Params:      Params{Fail: true, FailDefined: false, FailNonEmpty: false},
			ShouldError: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.Params.validateParams()
			if tc.ShouldError && err == nil {
				t.Fatalf("expected error, saw none")
			} else if !tc.ShouldError && err != nil {
				t.Fatalf("unexpected error: %q", err.Error())
			}
		})
	}
}
