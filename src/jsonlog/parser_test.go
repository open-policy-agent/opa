// Copyright 2015 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package jsonlog

import (
	"testing"
	// "fmt"
)

func testParse1Term(t *testing.T, msg string, expr string, correct *Term) interface{} {
	p, err := Parse("", []byte(expr))
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, expr, err)
	}
	parsed := p.([]interface{})
	if len(parsed) != 1 {
		t.Errorf("Error on test %s: failed to parse 1 element from %s: %v",
			msg, expr, parsed)
	}
	term := parsed[0].(*Term)
	if !term.Equal(correct) {
		t.Errorf("Error on test %s: wrong result on %s.  Actual = %v; Correct = %v",
			msg, expr, term, correct)
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
	testParse1Term(t, "null", "null", NewTerm1(nil, NULL))
	testParse1Term(t, "true", "true", NewTerm1(true, BOOLEAN))
	testParse1Term(t, "false", "false", NewTerm1(false, BOOLEAN))
	testParse1Term(t, "integer", "53", NewTerm1(53, NUMBER))
	testParse1Term(t, "integer2", "-53", NewTerm1(-53, NUMBER))
	testParse1Term(t, "float", "16.7", NewTerm1(16.7, NUMBER))
	testParse1Term(t, "float2", "-16.7", NewTerm1(-16.7, NUMBER))
	testParse1Term(t, "exponent", "6e7", NewTerm1(6e7, NUMBER))
	testParse1Term(t, "string", "\"a string\"", NewTerm1("a string", STRING))
	testParse1Term(t, "string", "\"a string u6abc7def8abc0def with unicode\"",
		NewTerm1("a string u6abc7def8abc0def with unicode", STRING))

	testParse1TermFail(t, "hex", "6abc")
	testParse1TermFail(t, "non-string", "'a string'")
	testParse1TermFail(t, "non-bool", "True")
	testParse1TermFail(t, "non-bool", "False")
	testParse1TermFail(t, "non-number", "6zxy")
	testParse1TermFail(t, "non-number2", "6d7")
}

// func TestVariables(t *testing.T) {
// 	testParse(t, "variable", "\"a string\"")
// }

// NewTerm1 creates a NewTerm for testing using a couple default values
func NewTerm1(x interface{}, kind int) *Term {
	var val interface{}
	switch x.(type) {
		case uint, uint8, uint16, uint32, uint64, int8, int16, int32, int64, int:
			val = float64(x.(int))
		case float32:
			val = float64(x.(float32))
		default:
			val = x
	}
    return NewTerm(val, kind, []byte(""), "", 0, 0)
}
