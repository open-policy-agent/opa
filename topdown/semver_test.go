// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestSemVerCompare(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"a < b", []string{`p = x { x = semver.compare("1.0.0", "2.0.0") }`}, "-1"},
		{"a > b", []string{`p = x { x = semver.compare("2.0.0", "1.0.0") }`}, "1"},
		{"a == b", []string{`p = x { x = semver.compare("1.0.0", "1.0.0") }`}, "0"},
		{
			"invalid type a",
			[]string{`p = x { x = semver.compare(1, "1.0.0") }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "semver.compare: invalid argument(s)")},
		},
		{
			"invalid type b",
			[]string{`p = x { x = semver.compare("1.0.0", false) }`},
			ast.Errors{ast.NewError(ast.TypeErr, nil, "semver.compare: invalid argument(s)")},
		},
		{
			"invalid version a",
			[]string{`p = x { x = semver.compare("1", "1.0.0") }`},
			&Error{Code: BuiltinErr, Message: `semver.compare("1", "1.0.0"): eval_builtin_error: semver.compare: operand 1: string "1" is not a valid SemVer`},
		},
		{
			"invalid version b",
			[]string{`p = x { x = semver.compare("1.0.0", "1") }`},
			&Error{Code: BuiltinErr, Message: `semver.compare("1.0.0", "1"): eval_builtin_error: semver.compare: operand 2: string "1" is not a valid SemVer`},
		},
	}

	data := map[string]interface{}{}

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestSemVerIsValid(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"valid", []string{`p = x { x = semver.is_valid("1.0.0") }`}, "true"},
		{"invalid version", []string{`p = x { x = semver.is_valid("1") }`}, "false"},
		{"invalid type", []string{`p = x { x = semver.is_valid(1) }`}, "false"},
	}

	data := map[string]interface{}{}

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
