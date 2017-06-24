package main

import "testing"

func TestGlobalStore(t *testing.T) {
	cases := []struct {
		initial  int
		input    string
		expected int
	}{
		{
			input:    "i",
			expected: 1,
		},
		{
			initial:  -1,
			input:    "ii",
			expected: 1,
		},
		{
			input:    "d",
			expected: -1,
		},
		{
			initial:  10,
			input:    "iizddiii",
			expected: 1,
		},
	}
	for _, test := range cases {
		got, err := Parse("", []byte(test.input), GlobalStore("initial", test.initial))
		if err != nil {
			t.Fatalf("for input %q got error: %s", test.input, err)
		}
		goti := got.(int)
		if goti != test.expected {
			t.Errorf("for input %q want %d, got %d", test.input, test.expected, goti)
		}
	}
}
