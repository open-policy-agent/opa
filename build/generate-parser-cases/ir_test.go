// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestEntrypointRefs(t *testing.T) {
	module := ast.MustParseModuleWithOpts(`package test

p := 1

q contains x if {
	some x in [1, 2]
}

r.s.t := 2
`, ast.ParserOptions{RegoVersion: ast.RegoV1})

	tests := []struct {
		note     string
		authored []string
		want     []string
		wantRefs []string
	}{
		{
			note:     "derived from the module, sorted",
			want:     []string{"test/p", "test/q", "test/r/s/t"},
			wantRefs: []string{"data.test.p", "data.test.q", "data.test.r.s.t"},
		},
		{
			note:     "authored entrypoints override the derived ones",
			authored: []string{"test/q"},
			want:     []string{"test/q"},
			wantRefs: []string{"data.test.q"},
		},
		{
			note:     "an authored entrypoint is a slash-separated path",
			authored: []string{"test/r/s/t"},
			want:     []string{"test/r/s/t"},
			wantRefs: []string{"data.test.r.s.t"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got, refs, err := entrypointRefs(tc.authored, module)
			if err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("entrypoints: expected %v, got %v", tc.want, got)
			}

			gotRefs := make([]string, len(refs))
			for i, ref := range refs {
				gotRefs[i] = ref.String()
			}

			if !slices.Equal(gotRefs, tc.wantRefs) {
				t.Errorf("refs: expected %v, got %v", tc.wantRefs, gotRefs)
			}
		})
	}
}
