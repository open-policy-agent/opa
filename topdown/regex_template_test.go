package topdown

import (
	"fmt"
	"regexp"
	"testing"
)

func TestRegexCompiler(t *testing.T) {
	for _, tc := range []struct {
		template       string
		delimiterStart byte
		delimiterEnd   byte
		failCompile    bool
		matchAgainst   string
		failMatch      bool
	}{
		{"urn:foo:{.*}", '{', '}', false, "urn:foo:bar:baz", false},
		{"urn:foo.bar.com:{.*}", '{', '}', false, "urn:foo.bar.com:bar:baz", false},
		{"urn:foo.bar.com:{.*}", '{', '}', false, "urn:foo.com:bar:baz", true},
		{"urn:foo.bar.com:{.*}", '{', '}', false, "foobar", true},
		{"urn:foo.bar.com:{.{1,2}}", '{', '}', false, "urn:foo.bar.com:aa", false},
		{"urn:foo.bar.com:{.*{}", '{', '}', true, "", true},
		{"urn:foo:<.*>", '<', '>', false, "urn:foo:bar:baz", false},
	} {
		t.Run(fmt.Sprintf("template=%s", tc.template), func(t *testing.T) {
			result, err := compileRegexTemplate(tc.template, tc.delimiterStart, tc.delimiterEnd)
			if tc.failCompile != (err != nil) {
				t.Fatalf("failed regex template compilation: %t != %t", tc.failCompile, err != nil)
			}

			if tc.failCompile || err != nil {
				return
			}

			ok, err := regexp.MatchString(result.String(), tc.matchAgainst)
			if err != nil {
				t.Fatalf("unexpected error while matching string: %s", err)
			}

			if !tc.failMatch != ok {
				t.Logf("match result %t is not expected value %t", ok, !tc.failMatch)
			}
		})
	}
}
