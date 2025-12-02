package find

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestRefLocatorApplicable(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		stack    []ast.Node
		expected bool
	}{
		"simple reference": {
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Ref{ast.VarTerm("q")},
				},
			},
			expected: true,
		},
		"no reference": {
			stack: []ast.Node{
				ast.StringTerm("foobar"),
			},
			expected: false,
		},
		"deeply nested reference": {
			stack: []ast.Node{
				&ast.Rule{},
				ast.Body{},
				&ast.Expr{},
				&ast.Term{
					Value: ast.Ref{
						ast.VarTerm("data"),
						ast.StringTerm("foo"),
						ast.StringTerm("bar"),
					},
				},
			},
			expected: true,
		},
		"deeply nested, no reference": {
			stack: []ast.Node{
				&ast.Rule{},
				ast.Body{},
				&ast.Expr{},
				ast.StringTerm("foobar"),
			},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			locator := NewRefLocator()
			result := locator.Applicable(tc.stack)

			if result != tc.expected {
				t.Fatalf("RefLocator.Applicable() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestRefLocatorFind(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		module   string
		stack    []ast.Node
		expected *ast.Location
	}{
		"finds rule definition": {
			module: `package test

test_rule := true`,
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Ref{
						ast.VarTerm("data"),
						ast.StringTerm("test"),
						ast.StringTerm("test_rule"),
					},
				},
			},
			expected: &ast.Location{
				Text:   []byte("test_rule := true"),
				File:   "test.rego",
				Row:    3,
				Col:    1,
				Offset: 14,
				Tabs:   []int{},
			},
		},
		"rule not found": {
			module: `package test

some_rule := true`,
			stack: []ast.Node{
				&ast.Term{
					Value: ast.Ref{
						ast.VarTerm("data"),
						ast.StringTerm("test"),
						ast.StringTerm("nonexistent_rule"),
					},
				},
			},
			expected: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ast.ParseModule("test.rego", tc.module)
			if err != nil {
				t.Fatal(err)
			}

			compiler := ast.NewCompiler()
			compiler.Modules = map[string]*ast.Module{
				"test.rego": parsed,
			}

			compiler.Compile(compiler.Modules)
			if compiler.Failed() {
				t.Fatal(compiler.Errors)
			}

			locator := NewRefLocator()

			result := locator.Find(tc.stack, compiler, parsed)

			if tc.expected == nil {
				if result != nil {
					t.Fatalf("Expected nil result, but got: %v", result)
				}
				return
			}

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Fatalf("RefLocator.Find() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
