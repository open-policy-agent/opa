package labeledfailures

import (
	"reflect"
	"testing"
)

func TestLabeledFailures(t *testing.T) {
	cases := []struct {
		input    string
		captures []string
		errors   []string
	}{
		// Test cases from reference implementation peglabel:
		// https://github.com/sqmedeiros/lpeglabel/blob/976b38458e0bba58ca748e96b53afd9ee74a1d1d/README.md#relabel-syntax
		// https://github.com/sqmedeiros/lpeglabel/blame/976b38458e0bba58ca748e96b53afd9ee74a1d1d/README.md#L418-L440
		{
			input:    "one,two",
			captures: []string{"one", "two"},
		},
		{
			input:    "one two three",
			captures: []string{"one", "two", "three"},
			errors: []string{
				"1:4 (3): rule ErrComma: expecting ','",
				"1:8 (7): rule ErrComma: expecting ','",
			},
		},
		{
			input:    "1,\n two, \n3,",
			captures: []string{"NONE", "two", "NONE", "NONE"},
			errors: []string{
				"1:1 (0): rule ErrID: expecting an identifier",
				"2:6 (8): rule ErrID: expecting an identifier",
				// is line 3, col 2 in peglabel, pigeon increments the position behind the last character of the input if !. is matched
				"3:3 (12): rule ErrID: expecting an identifier",
			},
		},
		{
			input:    "one\n two123, \nthree,",
			captures: []string{"one", "two", "three", "NONE"},
			errors: []string{
				// is line 2, col 1 in peglabel, in pigeon, if a \n causes an error, this is at col 0
				"2:0 (3): rule ErrComma: expecting ','",
				"2:5 (8): rule ErrComma: expecting ','",
				// is line 3, col 6 in peglabel, pigeon increments the position behind the last character of the input if !. is matched
				"3:7 (20): rule ErrID: expecting an identifier",
			},
		},
		// Additional test cases
		{
			input:    "",
			captures: []string{"NONE"},
			errors: []string{
				"1:1 (0): rule ErrID: expecting an identifier",
			},
		},
		{
			input:    "1",
			captures: []string{"NONE"},
			errors:   []string{"1:1 (0): rule ErrID: expecting an identifier"},
		},
		{
			input:    "1,2",
			captures: []string{"NONE", "NONE"},
			errors: []string{
				"1:1 (0): rule ErrID: expecting an identifier",
				"1:3 (2): rule ErrID: expecting an identifier",
			},
		},
	}
	for _, test := range cases {
		got, err := Parse("", []byte(test.input))
		if test.errors == nil && err != nil {
			t.Fatalf("for input %q got error: %s, but expect to parse without errors", test.input, err)
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
