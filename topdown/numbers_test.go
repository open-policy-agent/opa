// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package topdown

import (
	"testing"
)

func TestBuiltinNumbersRange(t *testing.T) {
	cases := []struct {
		note string
		stmt string
		exp  interface{}
	}{
		{
			note: "one",
			stmt: "p = x { x := numbers.range(0, 0) }",
			exp:  "[0]",
		},
		{
			note: "ascending",
			stmt: "p = x { x := numbers.range(-2, 3) }",
			exp:  "[-2, -1, 0, 1, 2, 3]",
		},
		{
			note: "descending",
			stmt: "p = x { x := numbers.range(2, -3) }",
			exp:  "[2, 1, 0, -1, -2, -3]",
		},
		{
			note: "precision",
			stmt: "p { numbers.range(49649733057, 49649733060, [49649733057, 49649733058, 49649733059, 49649733060]) }",
			exp:  "true",
		},
		{
			note: "error: floating-point number pos 1",
			stmt: "p { numbers.range(3.14, 4) }",
			exp:  &Error{Code: TypeErr, Message: "numbers.range: operand 1 must be integer number but got floating-point number"},
		},
		{
			note: "error: floating-point number pos 2",
			stmt: "p { numbers.range(3, 3.14) }",
			exp:  &Error{Code: TypeErr, Message: "numbers.range: operand 2 must be integer number but got floating-point number"},
		},
	}

	for _, tc := range cases {
		runTopDownTestCase(t, map[string]interface{}{}, tc.note, []string{tc.stmt}, tc.exp)
	}
}
