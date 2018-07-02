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

func TestGlobIntersect(t *testing.T) {

}
