package find

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestVarLocatorApplicable(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected bool
	}{
		"simple variable": {
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Var("x"),
				},
			},
			expected: true,
		},
		"variable in nested stack": {
			stack: []ast.Node{
				&ast.Rule{},
				ast.Body{},
				&ast.Term{
					Value: ast.Var("count"),
				},
			},
			expected: true,
		},
		"no variable - string": {
			stack: []ast.Node{
				ast.StringTerm("foobar"),
			},
			expected: false,
		},
		"no variable - ref": {
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Ref{ast.VarTerm("data")},
				},
			},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewVarLocator()
			result := locator.Applicable(tc.stack)

			if result != tc.expected {
				t.Fatalf("VarLocator.Applicable() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestVarLocatorFind(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected *ast.Location
	}{
		"finds variable in rule head": {
			stack: []ast.Node{
				&ast.Rule{
					Head: &ast.Head{
						Args: []*ast.Term{
							{
								Value: ast.Var("x"),
								Location: &ast.Location{
									File: "test.rego",
									Row:  2,
									Col:  5,
									Text: []byte("x"),
								},
							},
						},
					},
				},
				&ast.Term{
					Value: ast.Var("x"),
				},
			},
			expected: &ast.Location{
				File: "test.rego",
				Row:  2,
				Col:  5,
				Text: []byte("x"),
			},
		},
		"variable not found": {
			stack: []ast.Node{
				&ast.Rule{
					Head: &ast.Head{Args: []*ast.Term{}},
					Body: ast.Body{},
				},
				&ast.Term{
					Value: ast.Var("nonexistent"),
				},
			},
			expected: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewVarLocator()
			result := locator.Find(tc.stack, nil, nil)

			if tc.expected == nil {
				if result != nil {
					t.Fatalf("Expected nil result, but got: %v", result)
				}
				return
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Fatalf("VarLocator.Find() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
