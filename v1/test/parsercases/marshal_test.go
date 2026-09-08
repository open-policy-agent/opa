// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parsercases

import (
	"testing"
)

func TestFormatAST(t *testing.T) {
	tests := []struct {
		note string
		in   string
		want string
	}{
		{
			note: "object members are ordered lexically",
			in:   `{"head":{"value":true,"name":"p","ref":[]},"body":[]}`,
			want: `{
  "body": [],
  "head": {
    "name": "p",
    "ref": [],
    "value": true
  }
}
`,
		},
		{
			note: "ordering reaches through arrays",
			in:   `[{"b":1,"a":2},{"d":3,"c":4}]`,
			want: `[
  {
    "a": 2,
    "b": 1
  },
  {
    "c": 4,
    "d": 3
  }
]
`,
		},
		{
			// A Rego number literal is text. Decoding one into a float64 would
			// render 1e6 as 1000000 and lose precision on a long integer.
			note: "number literals pass through verbatim",
			in:   `{"c":1e6,"a":14.2,"b":123456789012345678901234567890,"d":-0.0}`,
			want: `{
  "a": 14.2,
  "b": 123456789012345678901234567890,
  "c": 1e6,
  "d": -0.0
}
`,
		},
		{
			note: "empty composites stay inline",
			in:   `{"b":[],"a":{},"c":null}`,
			want: `{
  "a": {},
  "b": [],
  "c": null
}
`,
		},
		{
			note: "non-ascii is escaped to a surrogate pair, existing escapes survive",
			in:   `{"value":" \"quoted\" 𝄞"}`,
			want: `{
  "value": " \"quoted\" \ud834\udd1e"
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got, err := FormatAST([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("expected:\n%s\ngot:\n%s", tc.want, got)
			}

			// Formatting an already-formatted fixture changes nothing, which is
			// what lets the generator rewrite a file only when it really differs.
			again, err := FormatAST([]byte(got))
			if err != nil {
				t.Fatal(err)
			}
			if again != got {
				t.Errorf("not idempotent:\n%s", again)
			}
		})
	}
}

func TestFormatASTRejectsMalformedJSON(t *testing.T) {
	for _, in := range []string{"", "   ", "{", `{"a":}`, `[1,]`} {
		if _, err := FormatAST([]byte(in)); err == nil {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}
