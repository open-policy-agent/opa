package topdown

import (
	"fmt"
	"testing"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		note     string
		object   string
		key      interface{}
		fallback interface{}
		expected interface{}
	}{
		{
			note:     "basic case . found",
			object:   `{"a": "b"}`,
			key:      `"a"`,
			fallback: `"c"`,
			expected: `"b"`,
		},
		{
			note:     "basic case . not found",
			object:   `{"a": "b"}`,
			key:      `"c"`,
			fallback: `"c"`,
			expected: `"c"`,
		},
		{

			note:     "integer key . found",
			object:   "{1: 2}",
			key:      "1",
			fallback: "3",
			expected: "2",
		},
		{
			note:     "integer key . not found",
			object:   "{1: 2}",
			key:      "2",
			fallback: "3",
			expected: "3",
		},
		{
			note:     "complex value . found",
			object:   `{"a": {"b": "c"}}`,
			key:      `"a"`,
			fallback: "true",
			expected: `{"b": "c"}`,
		},
		{
			note:     "complex value . not found",
			object:   `{"a": {"b": "c"}}`,
			key:      `"b"`,
			fallback: "true",
			expected: "true",
		},
	}

	for _, tc := range cases {
		rules := []string{
			fmt.Sprintf("p = x { x := lookup(%s, %s, %s) }", tc.object, tc.key, tc.fallback),
		}
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, rules, tc.expected)
	}
}
