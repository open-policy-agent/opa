// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestSource_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note         string
		module       string
		expFormatted string
		expErrs      []string
	}{
		{
			note: "v0", // from default rego-version
			module: `package test

p[x]            {
	x = "a"
}`,
			expFormatted: `package test

p[x] {
	x = "a"
}
`,
		},
		{
			note: "v1",
			module: `package test

p    contains    x    if      {
	x = "a"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			formatted, err := Source("test.rego", []byte(tc.module))
			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("expected errors but got nil")
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%q", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				formattedStr := string(formatted)
				if formattedStr != tc.expFormatted {
					t.Fatalf("expected %q but got %q", tc.expFormatted, formattedStr)
				}
			}
		})
	}
}

func TestSourceWithOpts_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note          string
		toRegoVersion ast.RegoVersion
		module        string
		expFormatted  string
		expErrs       []string
	}{
		{
			note:          "v0 -> v0", // from default rego-version
			toRegoVersion: ast.RegoV0,
			module: `package test

p[x]            {
	x = "a"
}`,
			expFormatted: `package test

p[x] {
	x = "a"
}
`,
		},
		{
			note:          "v0 -> v1", // from default rego-version
			toRegoVersion: ast.RegoV1,
			module: `package test

p[x]            {
	x = "a"
}`,
			expFormatted: `package test

p contains x if {
	x = "a"
}
`,
		},
		{
			note:          "v1 -> v1", // from non-default rego-version
			toRegoVersion: ast.RegoV1,
			module: `package test

p    contains    x    if      {
	x = "a"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			formatted, err := SourceWithOpts("test.rego", []byte(tc.module), Opts{RegoVersion: tc.toRegoVersion})
			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("expected errors but got nil")
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%q", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				formattedStr := string(formatted)
				if formattedStr != tc.expFormatted {
					t.Fatalf("expected %q but got %q", tc.expFormatted, formattedStr)
				}
			}
		})
	}
}
