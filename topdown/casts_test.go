package topdown

import (
	"fmt"
	"github.com/open-policy-agent/opa/ast"
	"testing"
)

func TestToArray(t *testing.T) {

	// expected result
	expectedResult := []interface{}{1, 2, 3}
	resultObj, err := ast.InterfaceToValue(expectedResult)
	if err != nil {
		panic(err)
	}

	typeErr := fmt.Errorf("type")

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"array input", []string{`p = x { to_array([1,2,3], x) }`}, resultObj.String()},
		{"set input", []string{`p = x { to_array({1,2,3}, x) }`}, resultObj.String()},
		{"bad type", []string{`p = x { to_array("hello", x) }`}, typeErr},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestToSet(t *testing.T) {

	typeErr := fmt.Errorf("type")

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"array input", []string{`p = x { to_set([1,1,1], x) }`}, "[1]"},
		{"set input", []string{`p = x { to_set({1,1,2,3}, x) }`}, "[1,2,3]"},
		{"bad type", []string{`p = x { to_set("hello", x) }`}, typeErr},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
