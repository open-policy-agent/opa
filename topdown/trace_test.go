// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestEventEqual(t *testing.T) {

	a := ast.NewValueMap()
	a.Put(ast.String("foo"), ast.Number(1))
	b := ast.NewValueMap()
	b.Put(ast.String("foo"), ast.Number(2))

	tests := []struct {
		a     Event
		b     Event
		equal bool
	}{
		{Event{}, Event{}, true},
		{Event{Op: EvalOp}, Event{Op: EnterOp}, false},
		{Event{QueryID: 1}, Event{QueryID: 2}, false},
		{Event{ParentID: 1}, Event{ParentID: 2}, false},
		{Event{Node: ast.MustParseBody("true")}, Event{Node: ast.MustParseBody("false")}, false},
		{Event{Node: ast.MustParseBody("true")[0]}, Event{Node: ast.MustParseBody("false")[0]}, false},
		{Event{Node: ast.MustParseRule("p :- true")}, Event{Node: ast.MustParseRule("p :- false")}, false},
		{Event{Node: "foo"}, Event{Node: "foo"}, false}, // test some unsupported node type
	}

	for _, tc := range tests {
		if tc.a.Equal(tc.b) != tc.equal {
			var s string
			if tc.equal {
				s = "=="
			} else {
				s = "!="
			}
			t.Errorf("Expected %v %v %v", tc.a, s, tc.b)
		}
	}

}
