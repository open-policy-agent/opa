package topdown

import "testing"

func TestRegexMatchTemplate(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"matches wildcard with {}", []string{`p[x] { regex.template_match("urn:foo:{.*}", "urn:foo:bar:baz", "{", "}", x) }`}, "[true]"},
		{"matches wildcard with <>", []string{`p[x] { regex.template_match("urn:foo:<.*>", "urn:foo:bar:baz", "<", ">", x) }`}, "[true]"},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}

func TestRegexFind(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"finds all match values", []string{`p[x] { x = regex.find_n("a.", "paranormal", -1) }`}, `[["ar", "an", "al"]]`},
		{"finds specified number of match values", []string{`p[x] { x = regex.find_n("a.", "paranormal", 2) }`}, `[["ar", "an"]]`},
		{"finds no matching values", []string{`p[x] { x = regex.find_n("bork", "paranormal", -1) }`}, `[[]]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}
