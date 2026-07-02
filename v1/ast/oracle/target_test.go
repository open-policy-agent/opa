// Copyright 2025 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestFindTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		stack     []ast.Node
		wantNil   bool
		wantIsRef bool
		wantIsVar bool
		wantValue string
	}{
		{
			name:    "empty stack returns nil",
			stack:   []ast.Node{},
			wantNil: true,
		},
		{
			name: "top node is var (function argument case)",
			stack: []ast.Node{
				ast.VarTerm("x"),
			},
			wantIsVar: true,
			wantValue: "x",
		},
		{
			name: "stack contains ref",
			stack: []ast.Node{
				&ast.Term{Value: ast.MustParseRef("data.foo.bar")},
			},
			wantIsRef: true,
			wantValue: "data.foo.bar",
		},
		{
			name: "stack with var at top",
			stack: []ast.Node{
				&ast.Expr{},
				ast.VarTerm("myvar"),
			},
			wantIsVar: true,
			wantValue: "myvar",
		},
		{
			name: "ref takes precedence over var",
			stack: []ast.Node{
				ast.VarTerm("x"),
				&ast.Term{Value: ast.MustParseRef("data.test.q")},
			},
			wantIsRef: true,
			wantValue: "data.test.q",
		},
		{
			name: "top var beats deeper ref",
			stack: []ast.Node{
				&ast.Term{Value: ast.MustParseRef("input.foo")},
				ast.VarTerm("x"), // top node is var
			},
			wantIsVar: true,
			wantValue: "x",
		},
		{
			name: "stack with no term nodes",
			stack: []ast.Node{
				&ast.Expr{},
				ast.Body{},
			},
			wantNil: true,
		},
		{
			name:    "term with non-var non-ref value",
			stack:   []ast.Node{ast.InternedTerm(42)},
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := findTarget(tc.stack)

			if tc.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.isRef != tc.wantIsRef {
				t.Errorf("isRef: got %v, want %v", result.isRef, tc.wantIsRef)
			}

			if result.isVar != tc.wantIsVar {
				t.Errorf("isVar: got %v, want %v", result.isVar, tc.wantIsVar)
			}

			// Verify the actual value
			if tc.wantIsRef {
				if ref, ok := result.term.Value.(ast.Ref); ok {
					if ref.String() != tc.wantValue {
						t.Errorf("Ref value: got %s, want %s", ref.String(), tc.wantValue)
					}
				} else {
					t.Error("Expected Ref value")
				}
			} else if tc.wantIsVar {
				if v, ok := result.term.Value.(ast.Var); ok {
					if string(v) != tc.wantValue {
						t.Errorf("Var value: got %s, want %s", v, tc.wantValue)
					}
				} else {
					t.Error("Expected Var value")
				}
			}
		})
	}
}
