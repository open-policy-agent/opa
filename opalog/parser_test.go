// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import (
	"fmt"
	"testing"
)

var _ = fmt.Printf

func TestScalarTerms(t *testing.T) {
	assertParseOneTerm(t, "null", "null", NullTerm())
	assertParseOneTerm(t, "true", "true", BooleanTerm(true))
	assertParseOneTerm(t, "false", "false", BooleanTerm(false))
	assertParseOneTerm(t, "integer", "53", NumberTerm(53))
	assertParseOneTerm(t, "integer2", "-53", NumberTerm(-53))
	assertParseOneTerm(t, "float", "16.7", NumberTerm(16.7))
	assertParseOneTerm(t, "float2", "-16.7", NumberTerm(-16.7))
	assertParseOneTerm(t, "exponent", "6e7", NumberTerm(6e7))
	assertParseOneTerm(t, "string", "\"a string\"", StringTerm("a string"))
	assertParseOneTerm(t, "string", "\"a string u6abc7def8abc0def with unicode\"", StringTerm("a string u6abc7def8abc0def with unicode"))
	assertParseOneTermFail(t, "hex", "6abc")
	assertParseOneTermFail(t, "non-string", "'a string'")
	assertParseOneTermFail(t, "non-number", "6zxy")
	assertParseOneTermFail(t, "non-number2", "6d7")
	assertParseOneTermFail(t, "non-number3", "6\"foo\"")
	assertParseOneTermFail(t, "non-number4", "6true")
	assertParseOneTermFail(t, "non-number5", "6false")
	assertParseOneTermFail(t, "non-number6", "6[null, null]")
	assertParseOneTermFail(t, "non-number7", "6{\"foo\": \"bar\"}")
	assertParseOneTermFail(t, "out-of-range", "1e1000")
}

func TestVarTerms(t *testing.T) {
	assertParseOneTerm(t, "var", "foo", VarTerm("foo"))
	assertParseOneTerm(t, "var", "foo_bar", VarTerm("foo_bar"))
	assertParseOneTerm(t, "var", "foo0", VarTerm("foo0"))
	assertParseOneTermFail(t, "non-var", "foo-bar")
	assertParseOneTermFail(t, "non-var2", "foo-7")
}

func TestRefTerms(t *testing.T) {
	assertParseOneTerm(t, "constants", "foo.bar.baz", RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")))
	assertParseOneTerm(t, "constants 2", "foo.bar[0].baz", RefTerm(VarTerm("foo"), StringTerm("bar"), NumberTerm(0), StringTerm("baz")))
	assertParseOneTerm(t, "variables", "foo.bar[0].baz[i]", RefTerm(VarTerm("foo"), StringTerm("bar"), NumberTerm(0), StringTerm("baz"), VarTerm("i")))
}

func TestObjectWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{\"abc\": 7, \"def\": 8}", ObjectTerm(Item(StringTerm("abc"), NumberTerm(7)), Item(StringTerm("def"), NumberTerm(8))))
	assertParseOneTerm(t, "bool", "{\"abc\": false, \"def\": true}", ObjectTerm(Item(StringTerm("abc"), BooleanTerm(false)), Item(StringTerm("def"), BooleanTerm(true))))
	assertParseOneTerm(t, "string", "{\"abc\": \"foo\", \"def\": \"bar\"}", ObjectTerm(Item(StringTerm("abc"), StringTerm("foo")), Item(StringTerm("def"), StringTerm("bar"))))
	assertParseOneTerm(t, "mixed", "{\"abc\": 7, \"def\": null}", ObjectTerm(Item(StringTerm("abc"), NumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key", "{8: 7, \"def\": null}", ObjectTerm(Item(NumberTerm(8), NumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key 2", "{8.5: 7, \"def\": null}", ObjectTerm(Item(NumberTerm(8.5), NumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "bool key", "{true: false}", ObjectTerm(Item(BooleanTerm(true), BooleanTerm(false))))
}

func TestObjectWithVars(t *testing.T) {
	assertParseOneTerm(t, "var keys", "{foo: \"bar\", bar: 64}", ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), NumberTerm(64))))
	assertParseOneTerm(t, "nested var keys", "{baz: {foo: \"bar\", bar: qux}}", ObjectTerm(Item(VarTerm("baz"), ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), VarTerm("qux"))))))
}

func TestArrayWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "[1,2,3,4.5]", ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3), NumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "[true, false, true]", ArrayTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "[\"foo\", \"bar\"]", ArrayTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "[null, true, 42]", ArrayTerm(NullTerm(), BooleanTerm(true), NumberTerm(42)))
}

func TestArrayWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "[foo, bar, 42]", ArrayTerm(VarTerm("foo"), VarTerm("bar"), NumberTerm(42)))
	assertParseOneTerm(t, "nested var elements", "[[foo, true], [null, bar], 42]", ArrayTerm(ArrayTerm(VarTerm("foo"), BooleanTerm(true)), ArrayTerm(NullTerm(), VarTerm("bar")), NumberTerm(42)))
}

func TestEmptyComposites(t *testing.T) {
    assertParseOneTerm(t, "empty object", "{}", ObjectTerm())
    assertParseOneTerm(t, "emtpy array", "[]", ArrayTerm())
}

func TestNestedComposites(t *testing.T) {
	assertParseOneTerm(t, "nested composites", "[{foo: [\"bar\", baz]}]", ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(StringTerm("bar"), VarTerm("baz"))))))
}

func TestCompositesWithRefs(t *testing.T) {
	ref1 := RefTerm(VarTerm("a"), VarTerm("i"), StringTerm("b"))
	ref2 := RefTerm(VarTerm("c"), NumberTerm(0), StringTerm("d"), StringTerm("e"), VarTerm("j"))
	assertParseOneTerm(t, "ref keys", "[{a[i].b: 8, c[0][\"d\"].e[j]: f}]", ArrayTerm(ObjectTerm(Item(ref1, NumberTerm(8)), Item(ref2, VarTerm("f")))))
	assertParseOneTerm(t, "ref values", "[{8: a[i].b, f: c[0][\"d\"].e[j]}]", ArrayTerm(ObjectTerm(Item(NumberTerm(8), ref1), Item(VarTerm("f"), ref2))))
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

func assertParseOneTerm(t *testing.T, msg string, expr string, correct *Term) interface{} {
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

func assertParseOneTermFail(t *testing.T, msg string, expr string) {
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
