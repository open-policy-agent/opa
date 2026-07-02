package cases

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
)

func TestLoadExtended(t *testing.T) {
	// If a test case fails to create an IR plan an error will be returned
	// Seems unnecessary to check each individual test if the plan was generated correctly
	_, err := LoadIrExtendedTestCases()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadIrExtendedFiltered(t *testing.T) {
	// Load a capability file that only supports the plus builtin
	c, err := ast.LoadCapabilitiesFile("testdata/plus_sign_builtin_only.json")
	if err != nil {
		t.Fatal(err)
	}
	expectedBuiltin := &ir.BuiltinFunc{Name: c.Builtins[0].Name, Decl: c.Builtins[0].Decl}

	testCases, err := LoadIrExtendedTestCasesFiltered(CapabilitiesFilter(c))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range testCases {
		for _, testCase := range tc.Cases {
			if testCase.Ignore {
				if slices.Contains(testCase.Plan.Static.BuiltinFuncs, expectedBuiltin) {
					t.Fatalf("Expected built-in functions to not contain %v but got %v", expectedBuiltin, testCase.Plan.Static.BuiltinFuncs)
				}
			} else {
				if !slices.Contains(testCase.Plan.Static.BuiltinFuncs, expectedBuiltin) {
					t.Fatalf("Expected built-in functions to contain %v but got %v", expectedBuiltin, testCase.Plan.Static.BuiltinFuncs)
				}
			}

		}
	}
}
