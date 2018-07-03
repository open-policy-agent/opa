package topdown

import (
	"testing"
)

func TestGlobMatch(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"* wildcard match", []string{`p = x { glob.match("*.github.com", "api.github.com", x) }`}, "true"},
		{"* wildcard no match", []string{`p = x { glob.match("*.com", "false.co", x) }`}, "false"},
		{"set of patterns match", []string{`p = x { glob.match("{one,two,three}", "two", x) }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestPatternMatchingBackwardsCompatibility(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"re_match still works", []string{`p = x { re_match(".*", "hello", x) }`}, "true"},
		{"regex.globs_match still works", []string{`p = x { regex.globs_match("a.a.", ".b.b", x) }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
