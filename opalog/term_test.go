// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "testing"

func TestEqualTerms(t *testing.T) {
	assertTermEqual(t, NullTerm(), NullTerm())
	assertTermEqual(t, BooleanTerm(true), BooleanTerm(true))
	assertTermEqual(t, NumberTerm(5), NumberTerm(5))
	assertTermEqual(t, StringTerm("a string"), StringTerm("a string"))
	assertTermEqual(t, ObjectTerm(), ObjectTerm())
	assertTermEqual(t, ArrayTerm(), ArrayTerm())
	assertTermEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2))))
	assertTermEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))))
	assertTermEqual(t, ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)))
	assertTermEqual(t, VarTerm("foo"), VarTerm("foo"))
	assertTermEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)), RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)))
	assertTermNotEqual(t, NullTerm(), BooleanTerm(true))
	assertTermNotEqual(t, BooleanTerm(true), BooleanTerm(false))
	assertTermNotEqual(t, NumberTerm(5), NumberTerm(7))
	assertTermNotEqual(t, StringTerm("a string"), StringTerm("abc"))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(3), NumberTerm(2))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2))))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(7))), ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(4))))
	assertTermNotEqual(t, NumberTerm(5), StringTerm("a string"))
	assertTermNotEqual(t, NumberTerm(1), BooleanTerm(true))
	assertTermNotEqual(t, ObjectTerm(Item(NumberTerm(1), NumberTerm(2)), Item(NumberTerm(3), NumberTerm(7))), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(7)))
	assertTermNotEqual(t, ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3)), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(4)))
	assertTermNotEqual(t, VarTerm("foo"), VarTerm("bar"))
	assertTermNotEqual(t, RefTerm(VarTerm("foo"), VarTerm("i"), NumberTerm(2)), RefTerm(VarTerm("foo"), StringTerm("i"), NumberTerm(2)))
}

func TestTermsToString(t *testing.T) {
	assertToString(t, Null{}, "null")
	assertToString(t, Boolean(true), "true")
	assertToString(t, Boolean(false), "false")
	assertToString(t, Number(4), "4")
	assertToString(t, Number(42.1), "42.1")
	assertToString(t, Number(6e7), "6E+07")
	assertToString(t, String("foo"), "\"foo\"")
	assertToString(t, String("\"foo\""), "\"\\\"foo\\\"\"")
	assertToString(t, String("foo bar"), "\"foo bar\"")
	assertToString(t, Var("foo"), "foo")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar")).Value, "foo.bar")
	assertToString(t, RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"), NumberTerm(0), StringTerm("baz")).Value, "foo.bar[i][0].baz")
	assertToString(t, RefTerm(VarTerm("foo"), BooleanTerm(false), NullTerm(), StringTerm("bar")).Value, "foo[false][null].bar")
	assertToString(t, ArrayTerm().Value, "[]")
	assertToString(t, ObjectTerm().Value, "{}")
	assertToString(t, ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(RefTerm(VarTerm("bar"), VarTerm("i"))))), StringTerm("foo"), BooleanTerm(true), NullTerm(), NumberTerm(42.1)).Value, "[{foo: [bar[i]]}, \"foo\", true, null, 42.1]")
}

func assertTermEqual(t *testing.T, x *Term, y *Term) {
	if !x.Equal(y) {
		t.Errorf("Failure on equality: \n%s and \n%s\n", x, y)
	}
}

func assertTermNotEqual(t *testing.T, x *Term, y *Term) {
	if x.Equal(y) {
		t.Errorf("Failure on non-equality: \n%s and \n%s\n", x, y)
	}
}

func assertToString(t *testing.T, val Value, expected string) {
	result := val.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}
