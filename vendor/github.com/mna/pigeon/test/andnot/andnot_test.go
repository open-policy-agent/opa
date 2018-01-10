package andnot

import "testing"

// ABs must end in Bs, CDs must end in Ds
var cases = map[string]string{
	"":             `1:1 (0): no match found, expected: [ \t\n\r], [ab] or [cd]`,
	"a":            `1:2 (1): no match found, expected: [ab]`,
	"b":            "",
	"ab":           "",
	"ba":           `1:3 (2): no match found, expected: [ab]`,
	"aab":          "",
	"bba":          `1:4 (3): no match found, expected: [ab]`,
	"aabbaba":      `1:8 (7): no match found, expected: [ab]`,
	"bbaabaaabbbb": "",
	"abc":          `1:3 (2): no match found, expected: [ \t\n\r], [ab] or EOF`,
	"c":            `1:2 (1): no match found, expected: [cd]`,
	"d":            "",
	"cd":           "",
	"dc":           `1:3 (2): no match found, expected: [cd]`,
	"dcddcc":       `1:7 (6): no match found, expected: [cd]`,
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
