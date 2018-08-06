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
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
