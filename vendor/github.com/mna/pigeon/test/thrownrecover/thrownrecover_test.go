package thrownrecover

import (
	"reflect"
	"testing"
)

func TestThrowAndRecover(t *testing.T) {
	cases := []struct {
		input    string
		captures interface{}
		errors   []string
	}{
		// Case 01: Recover multiple labels
		{
			input:    "case01:123",
			captures: "123",
		},
		{
			input:    "case01:1a3",
			captures: "1?3",
			errors: []string{
				"1:9 (8): rule ErrNonNumber: expecting a number",
			},
		},
		{
			input:    "case01:11+3",
			captures: "11?3",
			errors: []string{
				"1:10 (9): rule ErrNonNumber: expecting a number",
			},
		},

		// Case 02: Throw a undefined label
		{
			input:    "case02:",
			captures: nil,
			errors: []string{
				"1:8 (7): rule case02: Throwed undefined label",
			},
		},

		// Case 03: Nested Recover
		{
			input:    "case03:123",
			captures: "123",
		},
		{
			input:    "case03:1a3",
			captures: "1<3",
			errors: []string{
				"1:9 (8): rule ErrAlphaInner03: expecting a number, got lower case char",
			},
		},
		{
			input:    "case03:11A3",
			captures: "11>3",
			errors: []string{
				"1:10 (9): rule ErrAlphaOuter03: expecting a number, got upper case char",
			},
		},
		{
			input:    "case03:111+3",
			captures: "111?3",
			errors: []string{
				"1:11 (10): rule ErrOtherOuter03: expecting a number, got a non-char",
			},
		},

		// Case 04: Nested Recover, which fails in inner recover
		{
			input:    "case04:123",
			captures: "123",
		},
		{
			input:    "case04:1a3",
			captures: "1x3",
			errors: []string{
				"1:9 (8): rule ErrAlphaOuter04: expecting a number, got a char",
			},
		},
		{
			input:    "case04:11A3",
			captures: "11x3",
			errors: []string{
				"1:10 (9): rule ErrAlphaOuter04: expecting a number, got a char",
			},
		},
		{
			input:    "case04:111+3",
			captures: "111?3",
			errors: []string{
				"1:11 (10): rule ErrOtherOuter04: expecting a number, got a non-char",
			},
		},
	}
	for _, test := range cases {
		got, err := Parse("", []byte(test.input))
		if test.errors == nil && err != nil {
			t.Fatalf("for input %q got error: %s, but expect to parse without errors", test.input, err)
		}
		if test.errors != nil && err == nil {
			t.Fatalf("for input %q got no error, but expect to parse with errors: %s", test.input, test.errors)
		}
		if !reflect.DeepEqual(got, test.captures) {
			t.Errorf("for input %q want %s, got %s", test.input, test.captures, got)
		}
		if err != nil {
			list := err.(errList)
			if len(list) != len(test.errors) {
				t.Errorf("for input %q want %d error(s), got %d", test.input, len(test.errors), len(list))
				t.Logf("expected errors:\n")
				for _, ee := range test.errors {
					t.Logf("- %s\n", ee)
				}
				t.Logf("got errors:\n")
				for _, ee := range list {
					t.Logf("- %s\n", ee)
				}
				t.FailNow()
			}
			for i, err := range list {
				pe := err.(*parserError)
				if pe.Error() != test.errors[i] {
					t.Errorf("for input %q want %dth error to be %s, got %s", test.input, i+1, test.errors[i], pe)
				}
			}
		}
	}
}
