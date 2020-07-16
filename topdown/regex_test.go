package topdown

import (
	"encoding/json"
	"testing"
)

func TestRegexIsValid(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{
			note:     "bad operand type",
			rules:    []string{"p = x { regex.is_valid(data.num, x) }"},
			expected: "false",
		},
		{
			note:     "bad pattern",
			rules:    []string{"p = x { regex.is_valid(`++`, x) }"},
			expected: "false",
		},
		{
			note:     "good pattern",
			rules:    []string{"p = x { regex.is_valid(`.+`, x) }"},
			expected: "true",
		},
	}
	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{"num": json.Number("10")}, tc.note, tc.rules, tc.expected)
	}
}

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

func TestRegexFindAllStringSubmatch(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"finds no matches", []string{`p[x] { x = regex.find_all_string_submatch_n("a(x*)b", "-", -1) }`}, `[[]]`},
		{"single match without captures", []string{`p[x] { x = regex.find_all_string_submatch_n("a(x*)b", "-ab-", -1) }`}, `[[["ab", ""]]]`},
		{"single match with a capture", []string{`p[x] { x = regex.find_all_string_submatch_n("a(x*)b", "-axxb-", -1) }`}, `[[["axxb", "xx"]]]`},
		{"multiple matches with captures-1", []string{`p[x] { x = regex.find_all_string_submatch_n("a(x*)b", "-ab-axb-", -1) }`}, `[[["ab", ""], ["axb", "x"]]]`},
		{"multiple matches with captures-2", []string{`p[x] { x = regex.find_all_string_submatch_n("a(x*)b", "-axxb-ab-", -1) }`}, `[[["axxb", "xx"], ["ab", ""]]]`},
		{"multiple patterns, matches, and captures", []string{`p[x] { x = regex.find_all_string_submatch_n("[^aouiye]([aouiye])([^aouiye])?", "somestri", -1) }`}, `[[["som", "o", "m"], ["ri", "i", ""]]]`},
		{"multiple patterns, matches, and captures with specified number of matches", []string{`p[x] { x = regex.find_all_string_submatch_n("[^aouiye]([aouiye])([^aouiye])?", "somestri", 1) }`}, `[[["som", "o", "m"]]]`},
	}

	for _, tc := range tests {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, tc.rules, tc.expected)
	}
}
