package topdown

import "testing"

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"glob match with . delimiter", []string{`p[x] { glob.match("*.github.com", ["."], "api.github.com", x) }`}, "[true]"},
		{"super glob match with . delimiter", []string{`p[x] { glob.match("api.**.com", ["."], "api.github.com", x) }`}, "[true]"},
		{"super glob match with . delimiter", []string{`p[x] { glob.match("api.**.com", ["."], "api.cdn.github.com", x) }`}, "[true]"},
		{"glob match with : delimiter", []string{`p[x] { glob.match("*:github:com", [":"], "api:github:com", x) }`}, "[true]"},
		{"glob no match with . delimiter", []string{`p[x] { glob.match("*.github.com", ["."], "api.not-github.com", x) }`}, "[false]"},
		{"glob match with character-list matchers", []string{`p[x] { glob.match("[abc]at", [], "cat", x) }`}, "[true]"},
		{"glob no match with character-list matchers", []string{`p[x] { glob.match("[abc]at", [], "fat", x) }`}, "[false]"},
		{"glob match with negated character-list matchers", []string{`p[x] { glob.match("[!abc]at", [], "fat", x) }`}, "[true]"},
		{"glob no match with negated character-list matchers", []string{`p[x] { glob.match("[!abc]at", [], "cat", x) }`}, "[false]"},
		{"glob match with character-range matchers", []string{`p[x] { glob.match("[a-c]at", [], "bat", x) }`}, "[true]"},
		{"glob no match with character-range matchers", []string{`p[x] { glob.match("[a-c]at", [], "fat", x) }`}, "[false]"},
		{"glob no match with character-range matchers", []string{`p[x] { glob.match("[!a-c]at", [], "bat", x) }`}, "[false]"},
		{"glob match with character-range matchers", []string{`p[x] { glob.match("[!a-c]at", [], "fat", x) }`}, "[true]"},
		{"glob match with single wild-card", []string{`p[x] { glob.match("?at", [], "fat", x) }`}, "[true]"},
		{"glob no match with single wild-card", []string{`p[x] { glob.match("?at", [], "at", x) }`}, "[false]"},
		{"glob match with single wild-card and delimiter", []string{`p[x] { glob.match("?at", ["f"], "bat", x) }`}, "[true]"},
		{"glob no match with single wild-card and delimiter", []string{`p[x] { glob.match("?at", ["f"], "fat", x) }`}, "[false]"},
		{"glob match with pattern-alternatives list (cat)", []string{`p[x] { glob.match("{cat,bat,[fr]at}", [], "cat", x) }`}, "[true]"},
		{"glob match with pattern-alternatives list (bat)", []string{`p[x] { glob.match("{cat,bat,[fr]at}", [], "bat", x) }`}, "[true]"},
		{"glob match with pattern-alternatives list (fat)", []string{`p[x] { glob.match("{cat,bat,[fr]at}", [], "fat", x) }`}, "[true]"},
		{"glob match with pattern-alternatives list (rat)", []string{`p[x] { glob.match("{cat,bat,[fr]at}", [], "rat", x) }`}, "[true]"},
		{"glob no match with pattern-alternatives list", []string{`p[x] { glob.match("{cat,bat,[fr]at}", [], "at", x) }`}, "[false]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestGlobQuoteMeta(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"glob quote meta", []string{`p[x] { glob.quote_meta("*.github.com", x) }`}, `["\\*.github.com"]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
