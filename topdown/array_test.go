// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"
)

func TestTopDownArray(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"concat", []string{`p = x { x = array.concat([1,2], [3,4]) }`}, "[1,2,3,4]"},
		{"concat: err", []string{`p = x { x = array.concat(data.b, [3,4]) }`}, fmt.Errorf("operand 1 must be array")},
		{"concat: err rhs", []string{`p = x { x = array.concat([1,2], data.b) }`}, fmt.Errorf("operand 2 must be array")},

		{"slice", []string{`p = x { x = array.slice([1,2,3,4,5], 1, 3) }`}, "[2,3]"},
		{"slice: empty slice", []string{`p = x { x = array.slice([1,2,3], 0, 0) }`}, "[]"},
		{"slice: negative indices", []string{`p = x { x = array.slice([1,2,3,4,5], -4, -1) }`}, "[]"},
		{"slice: stopIndex < startIndex", []string{`p = x { x = array.slice([1,2,3,4,5], 4, 1) }`}, "[]"},
		{"slice: clamp startIndex", []string{`p = x { x = array.slice([1,2,3,4,5], -1, 2) }`}, `[1,2]`},
		{"slice: clamp stopIndex", []string{`p = x {x = array.slice([1,2,3,4,5], 3, 6) }`}, `[4,5]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
