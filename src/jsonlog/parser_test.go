// Copyright 2015 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package jsonlog

import (
	"testing"
	"fmt"
)

func testTermEqual(t *testing.T, x *Term, y *Term) {
	if !x.Equal(y) {
		t.Errorf("Failure on equality: \n%s and \n%s\n", x, y)
	}
}

func testTermNotEqual(t *testing.T, x *Term, y *Term) {
	if x.Equal(y) {
		t.Errorf("Failure on non-equality: \n%s and \n%s\n", x, y)
	}
}

// Test equality on pure-json terms
func TestEqualJsonTerms(t *testing.T) {
	testTermEqual(t, NewNull(), NewNull())
	testTermEqual(t, GoTerm(true), GoTerm(true))
	testTermEqual(t, GoTerm(5), GoTerm(5))
	testTermEqual(t, GoTerm("a string"), GoTerm("a string"))
	testTermEqual(t, GoTerm(map[int]int{1:2}), GoTerm(map[int]int{1:2}))
	testTermEqual(t, GoTerm(map[int]int{1:2, 3:4}), GoTerm(map[int]int{1:2, 3:4}))
	testTermEqual(t, GoTerm([]int{1, 2, 3}), GoTerm([]int{1, 2, 3}))

	testTermNotEqual(t, NewNull(), GoTerm(true))
	testTermNotEqual(t, GoTerm(true), GoTerm(false))
	testTermNotEqual(t, GoTerm(5), GoTerm(7))
	testTermNotEqual(t, GoTerm("a string"), GoTerm("abc"))
	testTermNotEqual(t, GoTerm(map[int]int{3:2}), GoTerm(map[int]int{1:2}))
	testTermNotEqual(t, GoTerm(map[int]int{1:2, 3:7}), GoTerm(map[int]int{1:2, 3:4}))
	testTermNotEqual(t, GoTerm(5), GoTerm("a string"))
	testTermNotEqual(t, GoTerm(1), GoTerm(true))
	testTermNotEqual(t, GoTerm(map[int]int{1:2, 3:7}), GoTerm([]int{1, 2, 3, 7}))
	testTermNotEqual(t, GoTerm([]int{1, 2, 3}), GoTerm([]int{1, 2, 4}))
}

func testParse1Term(t *testing.T, msg string, expr string, correct *Term) interface{} {
	p, err := Parse("", []byte(expr))
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, expr, err)
		return nil
	}
	parsed := p.([]interface{})
	if len(parsed) != 1 {
		t.Errorf("Error on test %s: failed to parse 1 element from %s: %v",
			msg, expr, parsed)
		return nil
	}
	term := parsed[0].(*Term)
	if !term.Equal(correct) {
		t.Errorf("Error on test %s: wrong result on %s.  Actual = %v; Correct = %v",
			msg, expr, term, correct)
		return nil
	}
	return parsed[0]
}

func testParse1TermFail(t *testing.T, msg string, expr string) {
	p, err := Parse("", []byte(expr))
	if err != nil {
		return
	}
	parsed := p.([]interface{})
	if len(parsed) != 1 {
		t.Errorf("Error on test %s: failed to parse 1 element from %s: %v", msg, expr, parsed)
	} else {
		t.Errorf("Error on test %s: failed to error when parsing %v: %v", msg, expr, parsed)
	}
}

func TestScalarTerms(t *testing.T) {
	testParse1Term(t, "null", "null", NewNull())
	testParse1Term(t, "true", "true", GoTerm(true))
	testParse1Term(t, "false", "false", GoTerm(false))
	testParse1Term(t, "integer", "53", GoTerm(53))
	testParse1Term(t, "integer2", "-53", GoTerm(-53))
	testParse1Term(t, "float", "16.7", GoTerm(16.7))
	testParse1Term(t, "float2", "-16.7", GoTerm(-16.7))
	testParse1Term(t, "exponent", "6e7", GoTerm(6e7))
	testParse1Term(t, "string", "\"a string\"", GoTerm("a string"))
	testParse1Term(t, "string", "\"a string u6abc7def8abc0def with unicode\"",
		GoTerm("a string u6abc7def8abc0def with unicode"))

	testParse1TermFail(t, "hex", "6abc")
	testParse1TermFail(t, "non-string", "'a string'")
	testParse1TermFail(t, "non-bool", "True")
	testParse1TermFail(t, "non-bool", "False")
	testParse1TermFail(t, "non-number", "6zxy")
	testParse1TermFail(t, "non-number2", "6d7")
}

func TestDictionaryTerms(t *testing.T) {
	correct := GoTerm(map[string]int{"abc": 7, "def": 8})
	testParse1Term(t, "simple dict", "{\"abc\": 7, \"def\": 8}", correct)
}

// func TestVariables(t *testing.T) {
// 	testParse(t, "variable", "\"a string\"")
// }

// NewNull creates a new NULL term for testing.
//  Special case since nil could be either NULL or
//  an empty array.
func NewNull() *Term {
    return NewTerm(nil, NULL, []byte(""), "", 0, 0)
}


