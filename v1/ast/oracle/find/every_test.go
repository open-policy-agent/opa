package find

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestEveryLocatorApplicable(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected bool
	}{
		"every declaration in expression": {
			stack: []ast.Node{
				&ast.Expr{
					Terms: &ast.Every{
						Domain: ast.VarTerm("x"),
						Key:    ast.VarTerm("k"),
						Value:  ast.VarTerm("v"),
						Body: ast.Body{
							&ast.Expr{Terms: ast.BooleanTerm(true)},
						},
					},
				},
			},
			expected: true,
		},
		"no every declaration - some": {
			stack: []ast.Node{
				&ast.SomeDecl{
					Symbols: []*ast.Term{ast.VarTerm("x")},
				},
			},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewEveryLocator()
			result := locator.Applicable(tc.stack)

			if result != tc.expected {
				t.Fatalf("EveryLocator.Applicable() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestEveryLocatorFind(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected *ast.Location
	}{
		"finds variable": {
			stack: []ast.Node{
				&ast.Rule{
					Head: &ast.Head{
						Args: []*ast.Term{
							{
								Value: ast.Var("items"),
								Location: &ast.Location{
									File: "test.rego",
									Row:  4,
									Col:  2,
									Text: []byte("items"),
								},
							},
						},
					},
				},
				&ast.Expr{
					Terms: &ast.Every{
						Domain: ast.VarTerm("items"),
					},
				},
			},
			expected: &ast.Location{
				File: "test.rego",
				Row:  4,
				Col:  2,
				Text: []byte("items"),
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
			locator := NewEveryLocator()
			result := locator.Find(tc.stack, nil, nil)

			if tc.expected == nil {
				if result != nil {
					t.Fatalf("Expected nil result, but got: %v", result)
				}
				return
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Fatalf("EveryLocator.Find() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
