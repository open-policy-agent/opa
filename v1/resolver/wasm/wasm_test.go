// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/rego/opa"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestGetResult(t *testing.T) {
	tests := []struct {
		note    string
		result  string
		want    ast.Value
		wantErr string
	}{
		{
			note:   "result key",
			result: `{{"result": 42}}`,
			want:   ast.Number("42"),
		},
		{
			note:   "empty set is undefined",
			result: `set()`,
		},
		{
			note:    "one key not named result",
			result:  `{{"x": 42}}`,
			wantErr: "illegal result type",
		},
		{
			note:    "more than one key",
			result:  `{{"result": 42, "x": 1}}`,
			wantErr: "illegal result type",
		},
		{
			note:    "more than one element",
			result:  `{{"result": 1}, {"result": 2}}`,
			wantErr: "illegal result type",
		},
		{
			note:    "element is not an object",
			result:  `{42}`,
			wantErr: "illegal result type",
		},
		{
			note:    "not a set",
			result:  `[{"result": 42}]`,
			wantErr: "illegal result type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got, err := getResult(&opa.Result{Result: []byte(tc.result)})
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil || got.Compare(tc.want) != 0 {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
