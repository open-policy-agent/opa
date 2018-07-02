// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

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
		{"array input", []string{`p = x { cast_array([1,2,3], x) }`}, resultObj.String()},
		{"set input", []string{`p = x { cast_array({1,2,3}, x) }`}, resultObj.String()},
		{"bad type", []string{`p = x { cast_array("hello", x) }`}, typeErr},
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
		{"array input", []string{`p = x { cast_set([1,1,1], x) }`}, "[1]"},
		{"set input", []string{`p = x { cast_set({1,1,2,3}, x) }`}, "[1,2,3]"},
		{"bad type", []string{`p = x { cast_set("hello", x) }`}, typeErr},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestCasts(t *testing.T) {
	typeErr := fmt.Errorf("type")

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"null valid", []string{`p = x { cast_null(null, x) }`}, "null"},
		{"null invalid", []string{`p = x { cast_null({}, x) }`}, typeErr},
		//{"string valid", []string{`p = x { cast_string("potato", x) }`}, "potato"},
		{"string invalid", []string{`p = x { cast_string({1,1,2,3}, x) }`}, typeErr},
		{"boolean valid", []string{`p = x { cast_boolean(false, x) }`}, "false"},
		{"boolean valid", []string{`p = x { cast_boolean(1, x) }`}, typeErr},
		{"obj valid", []string{`p = x { cast_object({}, x) }`}, "{}"},
		{"obj invalid", []string{`p = x { cast_object([1,2,3], x) }`}, typeErr},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
