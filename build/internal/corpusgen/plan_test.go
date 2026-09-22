// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package corpusgen

import (
	"fmt"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestEntryPointRefs(t *testing.T) {
	tests := []struct {
		note    string
		modules []string
		want    []string
	}{
		{
			note: "one per document, sorted",
			modules: []string{`package test

r.s.t := 2

p := 1

q contains x if { some x in [1, 2] }
`},
			want: []string{"test/p", "test/q", "test/r/s/t"},
		},
		{
			note: "functions are not documents",
			modules: []string{`package test

f(x) := x

p := f(1)
`},
			want: []string{"test/p"},
		},
		{
			note: "nothing but functions leaves none",
			modules: []string{`package test

f(x) := x
`},
			want: nil,
		},
		{
			note: "a key an entrypoint path cannot spell names the document instead",
			modules: []string{`package test

p[1] := "x"

q := 2
`},
			want: []string{"test/p", "test/q"},
		},
		{
			note: "across modules",
			modules: []string{
				"package a\n\np := 1\n",
				"package b\n\nq := 2\n",
			},
			want: []string{"a/p", "b/q"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			modules := map[string]*ast.Module{}
			for i, src := range tc.modules {
				modules[fmt.Sprintf("test-%d.rego", i)] = ast.MustParseModuleWithOpts(src,
					ast.ParserOptions{RegoVersion: ast.RegoV1})
			}

			got, refs, err := EntryPointRefs(modules)
			if err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("entrypoints: expected %v, got %v", tc.want, got)
			}
			if len(refs) != len(got) {
				t.Errorf("expected a ref per entrypoint, got %d for %d", len(refs), len(got))
			}
		})
	}
}
