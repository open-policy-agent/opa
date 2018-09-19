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
