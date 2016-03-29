package main

import "testing"

// ABs must end in Bs, CDs must end in Ds
var cases = map[string]string{
	"":             "1:1 (0): no match found",
	"a":            "1:1 (0): no match found",
	"b":            "",
	"ab":           "",
	"ba":           "1:1 (0): no match found",
	"aab":          "",
	"bba":          "1:1 (0): no match found",
	"aabbaba":      "1:1 (0): no match found",
	"bbaabaaabbbb": "",
	"abc":          "1:1 (0): no match found",
	"c":            "1:1 (0): no match found",
	"d":            "",
	"cd":           "",
	"dc":           "1:1 (0): no match found",
	"dcddcc":       "1:1 (0): no match found",
	"dcddccdd":     "",
}

func TestAndNot(t *testing.T) {
	for tc, exp := range cases {
		_, err := Parse("", []byte(tc))
		var got string
		if err != nil {
			got = err.Error()
		}
		if got != exp {
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}
	}
}
