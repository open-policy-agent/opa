package main

import (
	"reflect"
	"testing"

	"github.com/mna/pigeon/examples/json"

	"strings"
)

var invalidCases = []struct {
	input    string
	err      string
	expected []string
}{
	{
		input: `{`,
		err:   `no match found`,
		expected: []string{
			`"\""`, `"}"`, `[ \t\r\n]`,
		},
	},
	{
		input: `[`,
		err:   `no match found`,
		expected: []string{
			`"-"`, `"0"`, `"["`, `"\""`, `"]"`, `"false"`, `"null"`, `"true"`, `"{"`, `[ \t\r\n]`, `[1-9]`,
		},
	},
	{
		input: `{
    "foo": "bar",
    "errfoo: "bar"
}`,
		err: `no match found`,
		expected: []string{
			`":"`, `[ \t\r\n]`,
		},
	},
	{
		input: `{
	"foo": "bar with tab",
	"errfoo: "bar"
}`,
		err: `no match found`,
		expected: []string{
			`":"`, `[ \t\r\n]`,
		},
	},
}

func TestInvalidCases(t *testing.T) {
	for _, test := range invalidCases {
		got, err := json.Parse("", []byte(test.input))
		if err == nil {
			t.Errorf("%q: want error, got none (%v)", test.input, got)
			continue
		}
		el, ok := err.(json.ErrorLister)
		if !ok {
			t.Errorf("%q: want error to implement ErrorLister, got: %T", test.input, err)
			continue
		}
		for _, e := range el.Errors() {
			parserErr, ok := e.(json.ParserError)
			if !ok {
				t.Errorf("%q: want all individual errors to implement ParserError, got: %T", test.input, e)
			}
			if !reflect.DeepEqual(test.expected, parserErr.Expected()) {
				t.Errorf("%q: want: %v, got: %v", test.input, test.expected, parserErr.Expected())
			}
			if !strings.Contains(parserErr.Error(), test.err) {
				t.Errorf("%q: want prefix \n%s\n, got \n%s\n", test.input, test.err, parserErr.Error())
			}
		}
	}
}

var caretCases = []struct {
	input    string
	expected string
}{
	{
		input: `{`,
		expected: `{
^
1:2 (1): no match found, expected: "\"", "}" or [ \t\r\n]
`,
	},
	{
		input: `{
`,
		expected: `{

^
2:1 (2): no match found, expected: "\"", "}" or [ \t\r\n]
`,
	},
	{
		input: `{
	"foo": bar"
}`,
		expected: `	"foo": bar"
               ^
2:9 (10): no match found, expected: "-", "0", "[", "\"", "false", "null", "true", "{", [ \t\r\n] or [1-9]
`,
	},
	{
		input: `{
  "foo": "bar",
}`,
		// TODO: Ideally the error message would point to the "," and not to the following "}"
		expected: `}
^
3:1 (18): no match found, expected: "\"" or [ \t\r\n]
`,
	},
	{
		input: `{
  "foo": "bar
}`,
		expected: `  "foo": "bar

^
3:0 (15): no match found, expected: ![\x00-\x1f"\\], "\"" or "\\"
`,
	},
}

func TestCaretCases(t *testing.T) {
	for _, test := range caretCases {
		got, err := json.Parse("", []byte(test.input))
		if err == nil {
			t.Errorf("%q: want error, got none (%v)", test.input, got)
			continue
		}
		caret := caretError(err, test.input)
		if test.expected != caret {
			t.Errorf("%q: want:\n%s\ngot:\n%s\n", test.input, test.expected, caret)
		}
	}
}
