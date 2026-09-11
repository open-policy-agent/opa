// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import "testing"

func TestYAMLUnmarshalKeywordKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note     string
		doc      string
		expected any
	}{
		{
			note:     "on/off keys stay strings",
			doc:      "on: push\noff: x\n",
			expected: `{"on": "push", "off": "x"}`,
		},
		{
			note:     "yes/no/y/n values stay strings",
			doc:      "a: yes\nb: no\nc: y\nd: n\n",
			expected: `{"a": "yes", "b": "no", "c": "y", "d": "n"}`,
		},
		{
			note:     "1.2 booleans still resolve",
			doc:      "a: true\nb: FALSE\n",
			expected: `{"a": true, "b": false}`,
		},
		{
			note:     "timestamps stay strings",
			doc:      "a: 2023-01-01\n",
			expected: `{"a": "2023-01-01"}`,
		},
		{
			note:     "non-string keys are stringified",
			doc:      "1: a\ntrue: b\n",
			expected: `{"1": "a", "true": "b"}`,
		},
	}

	for _, tc := range cases {
		rules := []string{
			`p = x { yaml.unmarshal(` + quoteRego(tc.doc) + `, x) }`,
		}
		runTopDownTestCase(t, map[string]any{}, tc.note, rules, tc.expected)
	}
}

// quoteRego renders s as a Rego string literal.
func quoteRego(s string) string {
	out := []byte{'"'}
	for i := range len(s) {
		switch c := s[i]; c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, c)
		}
	}
	return string(append(out, '"'))
}
