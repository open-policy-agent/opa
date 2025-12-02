package find

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestSomeLocatorApplicable(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected bool
	}{
		"some declaration in expression": {
			stack: []ast.Node{
				&ast.Expr{
					Terms: &ast.SomeDecl{
						Symbols: []*ast.Term{ast.VarTerm("x")},
					},
				},
			},
			expected: true,
		},
		"direct some declaration": {
			stack: []ast.Node{
				&ast.SomeDecl{
					Symbols: []*ast.Term{ast.VarTerm("y")},
				},
			},
			expected: true,
		},
		"no some declaration - variable": {
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Var("x"),
				},
			},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewSomeLocator()
			result := locator.Applicable(tc.stack)

			if result != tc.expected {
				t.Fatalf("SomeLocator.Applicable() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestSomeLocatorFind(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected *ast.Location
	}{
		"finds variable through varLocator": {
			stack: []ast.Node{
				&ast.Rule{
					Head: &ast.Head{
						Args: []*ast.Term{
							{
								Value: ast.Var("x"),
								Location: &ast.Location{
									File: "test.rego",
									Row:  3,
									Col:  7,
									Text: []byte("x"),
								},
							},
						},
					},
				},
				&ast.Expr{
					Terms: &ast.SomeDecl{
						Symbols: []*ast.Term{
							{
								Value: ast.Call{
									{Value: ast.Var("x")},
								},
							},
						},
					},
				},
			},
			expected: &ast.Location{
				File: "test.rego",
				Row:  3,
				Col:  7,
				Text: []byte("x"),
			},
		},
		"returns nil for non-applicable case": {
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Var("x"),
				},
			},
			expected: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewSomeLocator()
			result := locator.Find(tc.stack, nil, nil)

			if tc.expected == nil {
				if result != nil {
					t.Fatalf("Expected nil result, but got: %v", result)
				}
				return
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Fatalf("SomeLocator.Find() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
