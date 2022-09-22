// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast/internal/tokens"
)

const (
	testModule = `
# This policy module belongs the opa.example package.
package opa.examples

# Refer to data.servers as servers.
import data.servers
# Refer to the data.networks as networks.
import data.networks
# Refer to the data.ports as ports.
import data.ports

# A server exists in the violations set if...
violations[server] {
    # ...the server exists
    server = servers[i]
    # ...and any of the server’s protocols is HTTP
    server.protocols[j] = "http"
    # ...and the server is public.
    public_servers[server]
}

# A server exists in the public_servers set if...
public_servers[server] {
	# Semicolons are optional. Can group expressions onto one line.
    server = servers[i]; server.ports[j] = ports[k].id 	# ...and the server is connected to a port
    ports[k].networks[l] = networks[m].id; 				# ...and the port is connected to a network
    networks[m].public = true							# ...and the network is public.
}`
)

func TestNumberTerms(t *testing.T) {

	tests := []struct {
		input    string
		expected string
	}{
		{"0", "0"},
		{"100", "100"},
		{"-1", "-1"},
		{"1e6", "1e6"},
		{"1.1e6", "1.1e6"},
		{"-1e-6", "-1e-6"},
		{"1E6", "1E6"},
		{"0.1", "0.1"},
		{".1", "0.1"},
		{".0001", "0.0001"},
		{"-.1", "-0.1"},
		{"-0.0001", "-0.0001"},
		{"1e1000", "1e1000"},
		{"0e1", "0"},
		{"-0.1", "-0.1"},
	}

	for _, tc := range tests {
		result, err := ParseTerm(tc.input)
		if err != nil {
			t.Errorf("Unexpected error for %v: %v", tc.input, err)
		} else {
			e := NumberTerm(json.Number(tc.expected))
			if !result.Equal(e) {
				t.Errorf("Expected %v for %v but got: %v", e, tc.input, result)
			}
		}
	}
}

func TestStringTerms(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`""`, ""},                   // empty
		{`" "`, " "},                 // whitespace
		{`"\""`, `"`},                // escaped quote
		{`"http:\/\/"`, `http://`},   // escaped solidus
		{`"\u0001"`, "\x01"},         // control code
		{`"foo\u005C"`, "foo\u005c"}, // unicode (upper hex)
		{`"foo\u005c"`, "foo\u005C"}, // unicode (lower hex)
		{`"\uD834\uDD1E"`, `𝄞`},      // g-clef
		{"`hi\\there`", `hi\there`},  // basic raw string
		{"`foo\nbar\n    baz`", `foo
bar
    baz`}, // multi-line raw string
	}

	for _, tc := range tests {
		result, err := ParseTerm(tc.input)
		if err != nil {
			t.Errorf("Unexpected error for %v: %v", tc.input, err)
		} else {
			s := StringTerm(tc.expected)
			if !result.Equal(s) {
				t.Errorf("Expected %v for %v but got: %v", s, tc.input, result)
			}
		}
	}
}

func TestScalarTerms(t *testing.T) {
	assertParseOneTerm(t, "null", "null", NullTerm())
	assertParseOneTerm(t, "true", "true", BooleanTerm(true))
	assertParseOneTerm(t, "false", "false", BooleanTerm(false))
	assertParseOneTerm(t, "integer", "53", IntNumberTerm(53))
	assertParseOneTerm(t, "integer2", "-53", IntNumberTerm(-53))
	assertParseOneTerm(t, "float", "16.7", FloatNumberTerm(16.7))
	assertParseOneTerm(t, "float2", "-16.7", FloatNumberTerm(-16.7))
	assertParseOneTerm(t, "exponent", "6e7", FloatNumberTerm(6e7))
	assertParseOneTerm(t, "string", "\"a string\"", StringTerm("a string"))
	assertParseOneTerm(t, "string", "\"a string u6abc7def8abc0def with unicode\"", StringTerm("a string u6abc7def8abc0def with unicode"))
	assertParseErrorContains(t, "hex", "6abc", "illegal number format")
	assertParseErrorContains(t, "non-terminated", "\"foo", "non-terminated string")
	assertParseErrorContains(t, "non-terminated-raw", "`foo", "non-terminated string")
	assertParseErrorContains(t, "non-string", "'a string'", "illegal token")
	assertParseErrorContains(t, "non-number", "6zxy", "illegal number format")
	assertParseErrorContains(t, "non-number2", "6d7", "illegal number format")
	assertParseErrorContains(t, "non-number3", "6\"foo\"", "expected exactly one statement") // ??
	assertParseErrorContains(t, "non-number4", "6true", "illegal number format")
	assertParseErrorContains(t, "non-number5", "6false", "illegal number format")
	assertParseErrorContains(t, "non-number6", "6[null, null]", "illegal ref (head cannot be number)") // ??
	assertParseErrorContains(t, "non-number7", "6{\"foo\": \"bar\"}", "expected exactly one statement")
	assertParseErrorContains(t, "non-number8", ".0.", "expected fraction")
	assertParseErrorContains(t, "non-number9", "0e", "expected exponent")
	assertParseErrorContains(t, "non-number10", "0e.", "expected exponent")
	assertParseErrorContains(t, "non-number11", "0F", "illegal number format")
	assertParseErrorContains(t, "non-number12", "00", "expected number")
	assertParseErrorContains(t, "non-number13", "00.1", "expected number")
	assertParseErrorContains(t, "non-number14", "-00", "expected number")
	assertParseErrorContains(t, "non-number15", "-00.1", "expected number")
	assertParseErrorContains(t, "non-number16", "-00.01", "expected number")
	assertParseErrorContains(t, "non-number17", "00e1", "expected number")
	assertParseErrorContains(t, "non-number18", "-00e1", "expected number")
	assertParseErrorContains(t, "parsing float fails", "7e3000000000", "invalid float")
	assertParseErrorContains(t, "float is +inf", "10245423601e680507880", "number too big")
	assertParseErrorContains(t, "float is -inf", "-10245423601e680507880", "number too big")

	// f := big.NewFloat(1); f.SetMantExp(f, -1e6); f.String() // => 1.010034059e-301030 (this takes ~9s)
	assertParseErrorContains(t, "float exp < -1e5", "1.010034059e-301030", "number too big")

	// g := big.NewFloat(1); g.SetMantExp(g, 1e6); g.String() // => 9.900656229e+301029
	assertParseErrorContains(t, "float exp > 1e5", "9.900656229e+301029", "number too big")
}

func TestVarTerms(t *testing.T) {
	assertParseOneTerm(t, "var", "foo", VarTerm("foo"))
	assertParseOneTerm(t, "var", "foo_bar", VarTerm("foo_bar"))
	assertParseOneTerm(t, "var", "foo0", VarTerm("foo0"))
	assertParseOneTerm(t, "import prefix", "imports", VarTerm("imports"))
	assertParseOneTerm(t, "not prefix", "not_foo", VarTerm("not_foo"))
	assertParseOneTerm(t, `package prefix`, "packages", VarTerm("packages"))
	assertParseOneTerm(t, `true prefix`, "trueish", VarTerm("trueish"))
	assertParseOneTerm(t, `false prefix`, "false_flag", VarTerm("false_flag"))
	assertParseOneTerm(t, `null prefix`, "nullable", VarTerm("nullable"))
	assertParseError(t, "illegal token", `墳`)
	assertParseError(t, "not keyword", "not")
	assertParseError(t, `package keyword`, "package")
	assertParseError(t, "import keyword", "import")
	assertParseError(t, "import invalid path", "import x.")
}

func TestRefTerms(t *testing.T) {
	assertParseOneTerm(t, "constants", "foo.bar.baz", RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")))
	assertParseOneTerm(t, "constants 2", "foo.bar[0].baz", RefTerm(VarTerm("foo"), StringTerm("bar"), IntNumberTerm(0), StringTerm("baz")))
	assertParseOneTerm(t, "variables", "foo.bar[0].baz[i]", RefTerm(VarTerm("foo"), StringTerm("bar"), IntNumberTerm(0), StringTerm("baz"), VarTerm("i")))
	assertParseOneTerm(t, "spaces", "foo[\"white space\"].bar", RefTerm(VarTerm("foo"), StringTerm("white space"), StringTerm("bar")))
	assertParseOneTerm(t, "nested", "foo[baz[1][borge[i]]].bar", RefTerm(
		VarTerm("foo"),
		RefTerm(
			VarTerm("baz"), IntNumberTerm(1), RefTerm(
				VarTerm("borge"), VarTerm("i"),
			),
		),
		StringTerm("bar"),
	))
	assertParseOneTerm(t, "composite operand 1", "foo[[1,2,3]].bar", RefTerm(VarTerm("foo"), ArrayTerm(NumberTerm("1"), NumberTerm("2"), NumberTerm("3")), StringTerm("bar")))
	assertParseOneTerm(t, "composite operand 2", `foo[{"foo": 2}].bar`, RefTerm(VarTerm("foo"), ObjectTerm(Item(StringTerm("foo"), NumberTerm("2"))), StringTerm("bar")))

	assertParseError(t, "missing component 1", "foo.")
	assertParseError(t, "missing component 2", "foo[].bar")
	assertParseError(t, "invalid composite operand", "foo[1,2]")
	assertParseError(t, "invalid call", "bar(..")
	assertParseError(t, "invalid ref", "bar[..")
	assertParseError(t, "invalid ref head type number", "0[0]")
	assertParseError(t, "invalid ref head type boolean", "true[0]")
	assertParseError(t, "invalid ref head type string", `"foo"[0]`)
	assertParseError(t, "invalid ref head type null", `null[0]`)
}

func TestObjectWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{\"abc\": 7, \"def\": 8}", ObjectTerm(Item(StringTerm("abc"), IntNumberTerm(7)), Item(StringTerm("def"), IntNumberTerm(8))))
	assertParseOneTerm(t, "bool", "{\"abc\": false, \"def\": true}", ObjectTerm(Item(StringTerm("abc"), BooleanTerm(false)), Item(StringTerm("def"), BooleanTerm(true))))
	assertParseOneTerm(t, "string", "{\"abc\": \"foo\", \"def\": \"bar\"}", ObjectTerm(Item(StringTerm("abc"), StringTerm("foo")), Item(StringTerm("def"), StringTerm("bar"))))
	assertParseOneTerm(t, "mixed", "{\"abc\": 7, \"def\": null}", ObjectTerm(Item(StringTerm("abc"), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key", "{8: 7, \"def\": null}", ObjectTerm(Item(IntNumberTerm(8), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key 2", "{8.5: 7, \"def\": null}", ObjectTerm(Item(FloatNumberTerm(8.5), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "bool key", "{true: false}", ObjectTerm(Item(BooleanTerm(true), BooleanTerm(false))))
	assertParseOneTerm(t, "trailing comma", `{"a": "bar", "b": 64, }`, ObjectTerm(Item(StringTerm("a"), StringTerm("bar")), Item(StringTerm("b"), IntNumberTerm(64))))
	assertParseOneTerm(t, "leading comma", `{, "a": "bar", "b": 64 }`, ObjectTerm(Item(StringTerm("a"), StringTerm("bar")), Item(StringTerm("b"), IntNumberTerm(64))))
	assertParseOneTerm(t, "leading comma not comprehension", `{, 1 | 1: "bar"}`, ObjectTerm(Item(CallTerm(RefTerm(VarTerm("or")), NumberTerm("1"), NumberTerm("1")), StringTerm("bar"))))
}

func TestObjectWithVars(t *testing.T) {
	assertParseOneTerm(t, "var keys", "{foo: \"bar\", bar: 64}", ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), IntNumberTerm(64))))
	assertParseOneTerm(t, "nested var keys", "{baz: {foo: \"bar\", bar: qux}}", ObjectTerm(Item(VarTerm("baz"), ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), VarTerm("qux"))))))
	assertParseOneTerm(t, "ambiguous or", `{ a: b+c | d }`, ObjectTerm(Item(VarTerm("a"), CallTerm(RefTerm(VarTerm("or")), CallTerm(RefTerm(VarTerm("plus")), VarTerm("b"), VarTerm("c")), VarTerm("d")))))
}

func TestObjectWithRelation(t *testing.T) {
	assertParseOneTerm(t, "relation term value", `{"x": 1+1}`, ObjectTerm(
		Item(StringTerm("x"), CallTerm(RefTerm(VarTerm("plus")), IntNumberTerm(1), IntNumberTerm(1))),
	))
	assertParseError(t, "invalid relation term value", `{"x": 0= }`)
}

func TestObjectFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "{foo: bar, baz: [], qux: corge")
	assertParseError(t, "non-terminated 2", "{foo: bar, baz: [], qux: ")
	assertParseError(t, "non-terminated 3", "{foo: bar, baz: [], qux ")
	assertParseError(t, "non-terminated 4", "{foo: bar, baz: [], ")
	assertParseError(t, "missing separator", "{foo: bar baz: []}")
	assertParseError(t, "missing start", "foo: bar, baz: [], qux: corge}")
	assertParseError(t, "double comma", "{a:1,,b:2}")
	assertParseError(t, "leading double comma", "{,,a:1}")
	assertParseError(t, "trailing double comma", "{a:1,,}")
}

func TestArrayWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "[1,2,3,4.5]", ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), FloatNumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "[true, false, true]", ArrayTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "[\"foo\", \"bar\"]", ArrayTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "[null, true, 42]", ArrayTerm(NullTerm(), BooleanTerm(true), IntNumberTerm(42)))
	assertParseOneTerm(t, "trailing comma - one element", "[null, ]", ArrayTerm(NullTerm()))
	assertParseOneTerm(t, "trailing comma", "[null, true, ]", ArrayTerm(NullTerm(), BooleanTerm(true)))
	assertParseOneTerm(t, "leading comma", "[, null, true]", ArrayTerm(NullTerm(), BooleanTerm(true)))
	assertParseOneTerm(t, "leading comma not comprehension", "[, 1 | 1]", ArrayTerm(CallTerm(RefTerm(VarTerm("or")), NumberTerm("1"), NumberTerm("1"))))
	assertParseOneTerm(t, "ambiguous or", "[ 1 + 2 | 3 ]", ArrayTerm(CallTerm(RefTerm(VarTerm("or")), CallTerm(RefTerm(VarTerm("plus")), NumberTerm("1"), NumberTerm("2")), NumberTerm("3"))))
}

func TestArrayWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "[foo, bar, 42]", ArrayTerm(VarTerm("foo"), VarTerm("bar"), IntNumberTerm(42)))
	assertParseOneTerm(t, "nested var elements", "[[foo, true], [null, bar], 42]", ArrayTerm(ArrayTerm(VarTerm("foo"), BooleanTerm(true)), ArrayTerm(NullTerm(), VarTerm("bar")), IntNumberTerm(42)))
}

func TestArrayFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "[foo, bar")
	assertParseError(t, "non-terminated 2", "[foo, bar, ")
	assertParseError(t, "missing separator", "[foo bar]")
	assertParseError(t, "missing start", "foo, bar, baz]")
	assertParseError(t, "bad term", "[!!!]")
	assertParseError(t, "double comma", "[a,,b]")
	assertParseError(t, "leading double comma", "[,,a]")
	assertParseError(t, "trailing double comma", "[a,,]")
}

func TestSetWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{1,2,3,4.5}", SetTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), FloatNumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "{true, false, true}", SetTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "{\"foo\", \"bar\"}", SetTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "{null, true, 42}", SetTerm(NullTerm(), BooleanTerm(true), IntNumberTerm(42)))
	assertParseOneTerm(t, "trailing comma", "{null, true,}", SetTerm(NullTerm(), BooleanTerm(true)))
	assertParseOneTerm(t, "leading comma", "{, null, true}", SetTerm(NullTerm(), BooleanTerm(true)))
	assertParseOneTerm(t, "leading comma not comprehension", "{, 1 | 1}", SetTerm(CallTerm(RefTerm(VarTerm("or")), NumberTerm("1"), NumberTerm("1"))))
	assertParseOneTerm(t, "ambiguous or", "{ 1 + 2 | 3}", SetTerm(CallTerm(RefTerm(VarTerm("or")), CallTerm(RefTerm(VarTerm("plus")), NumberTerm("1"), NumberTerm("2")), NumberTerm("3"))))
}

func TestSetWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "{foo, bar, 42}", SetTerm(VarTerm("foo"), VarTerm("bar"), IntNumberTerm(42)))
	assertParseOneTerm(t, "nested var elements", "{[foo, true], {null, bar}, set()}", SetTerm(ArrayTerm(VarTerm("foo"), BooleanTerm(true)), SetTerm(NullTerm(), VarTerm("bar")), SetTerm()))
}

func TestSetFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "set(")
	assertParseError(t, "non-terminated 2", "{foo, bar")
	assertParseError(t, "non-terminated 3", "{foo, bar, ")
	assertParseError(t, "missing separator", "{foo bar}")
	assertParseError(t, "missing start", "foo, bar, baz}")
	assertParseError(t, "bad term", "{!!!}")
	assertParseError(t, "double comma", "{a,,b}")
	assertParseError(t, "leading double comma", "{,,a}")
	assertParseError(t, "trailing double comma", "{a,,}")
}

func TestEmptyComposites(t *testing.T) {
	assertParseOneTerm(t, "empty object", "{}", ObjectTerm())
	assertParseOneTerm(t, "empty array", "[]", ArrayTerm())
	assertParseOneTerm(t, "empty set", "set()", SetTerm())
}

func TestNestedComposites(t *testing.T) {
	assertParseOneTerm(t, "nested composites", "[{foo: [\"bar\", {baz}]}]", ArrayTerm(ObjectTerm(Item(VarTerm("foo"), ArrayTerm(StringTerm("bar"), SetTerm(VarTerm("baz")))))))
}

func TestCompositesWithRefs(t *testing.T) {
	ref1 := RefTerm(VarTerm("a"), VarTerm("i"), StringTerm("b"))
	ref2 := RefTerm(VarTerm("c"), IntNumberTerm(0), StringTerm("d"), StringTerm("e"), VarTerm("j"))
	assertParseOneTerm(t, "ref keys", "[{a[i].b: 8, c[0][\"d\"].e[j]: f}]", ArrayTerm(ObjectTerm(Item(ref1, IntNumberTerm(8)), Item(ref2, VarTerm("f")))))
	assertParseOneTerm(t, "ref values", "[{8: a[i].b, f: c[0][\"d\"].e[j]}]", ArrayTerm(ObjectTerm(Item(IntNumberTerm(8), ref1), Item(VarTerm("f"), ref2))))
	assertParseOneTerm(t, "ref values (sets)", `{a[i].b, {c[0]["d"].e[j]}}`, SetTerm(ref1, SetTerm(ref2)))
}

func TestArrayComprehensions(t *testing.T) {

	nestedTerm := `[{"x": [a[i] | xs = [{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"]; xs[j].a[k] = "foo"]}]`
	nestedExpected := ArrayTerm(
		ObjectTerm(Item(
			StringTerm("x"),
			ArrayComprehensionTerm(
				RefTerm(VarTerm("a"), VarTerm("i")),
				NewBody(
					Equality.Expr(
						VarTerm("xs"),
						ArrayComprehensionTerm(
							ObjectTerm(Item(StringTerm("a"), ArrayTerm(StringTerm("baz"), VarTerm("j")))),
							NewBody(
								NewExpr(RefTerm(VarTerm("q"), VarTerm("p"))),
								NotEqual.Expr(RefTerm(VarTerm("p"), StringTerm("a")), StringTerm("bar")),
								Equality.Expr(VarTerm("j"), StringTerm("foo")),
							),
						),
					),
					Equality.Expr(
						RefTerm(VarTerm("xs"), VarTerm("j"), StringTerm("a"), VarTerm("k")),
						StringTerm("foo"),
					),
				),
			),
		)),
	)
	assertParseOneTerm(t, "nested", nestedTerm, nestedExpected)
	assertParseOneTerm(t, "ambiguous or", "[ a | b ]", ArrayComprehensionTerm(
		VarTerm("a"),
		MustParseBody("b"),
	))
}

func TestObjectComprehensions(t *testing.T) {
	nestedTerm := `[{"x": {a[i]: b[i] | xs = {"foo":{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"}; xs[j].a[k] = "foo"}}]`
	nestedExpected := ArrayTerm(
		ObjectTerm(Item(
			StringTerm("x"),
			ObjectComprehensionTerm(
				RefTerm(VarTerm("a"), VarTerm("i")),
				RefTerm(VarTerm("b"), VarTerm("i")),
				NewBody(
					Equality.Expr(
						VarTerm("xs"),
						ObjectComprehensionTerm(
							StringTerm("foo"),
							ObjectTerm(Item(StringTerm("a"), ArrayTerm(StringTerm("baz"), VarTerm("j")))),
							NewBody(
								NewExpr(RefTerm(VarTerm("q"), VarTerm("p"))),
								NotEqual.Expr(RefTerm(VarTerm("p"), StringTerm("a")), StringTerm("bar")),
								Equality.Expr(VarTerm("j"), StringTerm("foo")),
							),
						),
					),
					Equality.Expr(
						RefTerm(VarTerm("xs"), VarTerm("j"), StringTerm("a"), VarTerm("k")),
						StringTerm("foo"),
					),
				),
			),
		)),
	)
	assertParseOneTerm(t, "nested", nestedTerm, nestedExpected)
	assertParseOneTerm(t, "ambiguous or", "{ 1+2: 3 | 4}", ObjectComprehensionTerm(
		CallTerm(RefTerm(VarTerm("plus")), NumberTerm("1"), NumberTerm("2")),
		NumberTerm("3"),
		MustParseBody("4"),
	))
}

func TestObjectComprehensionError(t *testing.T) {
	assertParseError(t, "bad body", "{x: y|!!!}")
}

func TestSetComprehensions(t *testing.T) {
	nestedTerm := `[{"x": {a[i] | xs = {{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"}; xs[j].a[k] = "foo"}}]`
	nestedExpected := ArrayTerm(
		ObjectTerm(Item(
			StringTerm("x"),
			SetComprehensionTerm(
				RefTerm(VarTerm("a"), VarTerm("i")),
				NewBody(
					Equality.Expr(
						VarTerm("xs"),
						SetComprehensionTerm(
							ObjectTerm(Item(StringTerm("a"), ArrayTerm(StringTerm("baz"), VarTerm("j")))),
							NewBody(
								NewExpr(RefTerm(VarTerm("q"), VarTerm("p"))),
								NotEqual.Expr(RefTerm(VarTerm("p"), StringTerm("a")), StringTerm("bar")),
								Equality.Expr(VarTerm("j"), StringTerm("foo")),
							),
						),
					),
					Equality.Expr(
						RefTerm(VarTerm("xs"), VarTerm("j"), StringTerm("a"), VarTerm("k")),
						StringTerm("foo"),
					),
				),
			),
		)),
	)

	assertParseOneTerm(t, "nested", nestedTerm, nestedExpected)
	assertParseOneTerm(t, "ambiguous or", "{ a | b }", SetComprehensionTerm(
		VarTerm("a"),
		MustParseBody("b"),
	))
}

func TestSetComprehensionError(t *testing.T) {
	assertParseError(t, "bad body", "{x|!!!}")
}

func TestSetComprehensionsAlone(t *testing.T) {
	input := `{k | a = [1,2,3]; a[k]}`

	expected := SetComprehensionTerm(
		VarTerm("k"),
		NewBody(
			Equality.Expr(
				VarTerm("a"),
				ArrayTerm(NumberTerm("1"), NumberTerm("2"), NumberTerm("3")),
			),
			&Expr{
				Terms: RefTerm(VarTerm("a"), VarTerm("k")),
			},
		),
	)

	assertParseOneTerm(t, "alone", input, expected)
}

func TestCalls(t *testing.T) {

	assertParseOneExpr(t, "ne", "100 != 200", NotEqual.Expr(IntNumberTerm(100), IntNumberTerm(200)))
	assertParseOneExpr(t, "gt", "17.4 > \"hello\"", GreaterThan.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lt", "17.4 < \"hello\"", LessThan.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "gte", "17.4 >= \"hello\"", GreaterThanEq.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lte", "17.4 <= \"hello\"", LessThanEq.Expr(FloatNumberTerm(17.4), StringTerm("hello")))

	left2 := ArrayTerm(ObjectTerm(Item(FloatNumberTerm(14.2), BooleanTerm(true)), Item(StringTerm("a"), NullTerm())))
	right2 := ObjectTerm(Item(VarTerm("foo"), ObjectTerm(Item(RefTerm(VarTerm("a"), StringTerm("b"), IntNumberTerm(0)), ArrayTerm(IntNumberTerm(10))))))
	assertParseOneExpr(t, "composites", "[{14.2: true, \"a\": null}] != {foo: {a.b[0]: [10]}}", NotEqual.Expr(left2, right2))

	assertParseOneExpr(t, "plus", "1 + 2", Plus.Expr(IntNumberTerm(1), IntNumberTerm(2)))
	assertParseOneExpr(t, "minus", "1 - 2", Minus.Expr(IntNumberTerm(1), IntNumberTerm(2)))
	assertParseOneExpr(t, "mul", "1 * 2", Multiply.Expr(IntNumberTerm(1), IntNumberTerm(2)))
	assertParseOneExpr(t, "div", "1 / 2", Divide.Expr(IntNumberTerm(1), IntNumberTerm(2)))
	assertParseOneExpr(t, "rem", "3 % 2", Rem.Expr(IntNumberTerm(3), IntNumberTerm(2)))
	assertParseOneExpr(t, "and", "{1,2,3} & {2,3,4}", And.Expr(SetTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3)), SetTerm(IntNumberTerm(2), IntNumberTerm(3), IntNumberTerm(4))))
	assertParseOneExpr(t, "or", "{1,2,3} | {3,4,5}", Or.Expr(SetTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3)), SetTerm(IntNumberTerm(3), IntNumberTerm(4), IntNumberTerm(5))))

	assertParseOneExpr(t, "call", "count([true, false])", Count.Expr(ArrayTerm(BooleanTerm(true), BooleanTerm(false))))
	assertParseOneExpr(t, "call-ref", "foo.bar(1)", NewExpr(
		[]*Term{RefTerm(VarTerm("foo"), StringTerm("bar")),
			IntNumberTerm(1)}))
	assertParseOneExpr(t, "call-void", "foo()", NewExpr(
		[]*Term{RefTerm(VarTerm("foo"))}))

	opts := ParserOptions{FutureKeywords: []string{"in"}}
	assertParseOneExpr(t, "internal.member_2", "x in xs", Member.Expr(VarTerm("x"), VarTerm("xs")), opts)
	assertParseOneExpr(t, "internal.member_3", "x, y in xs", MemberWithKey.Expr(VarTerm("x"), VarTerm("y"), VarTerm("xs")), opts)
}

func TestInfixExpr(t *testing.T) {
	assertParseOneExpr(t, "scalars 1", "true = false", Equality.Expr(BooleanTerm(true), BooleanTerm(false)))
	assertParseOneExpr(t, "scalars 2", "3.14 = null", Equality.Expr(FloatNumberTerm(3.14), NullTerm()))
	assertParseOneExpr(t, "scalars 3", "42 = \"hello world\"", Equality.Expr(IntNumberTerm(42), StringTerm("hello world")))
	assertParseOneExpr(t, "vars 1", "hello = world", Equality.Expr(VarTerm("hello"), VarTerm("world")))
	assertParseOneExpr(t, "vars 2", "42 = hello", Equality.Expr(IntNumberTerm(42), VarTerm("hello")))

	ref1 := RefTerm(VarTerm("foo"), IntNumberTerm(0), StringTerm("bar"), VarTerm("x"))
	ref2 := RefTerm(VarTerm("baz"), BooleanTerm(false), StringTerm("qux"), StringTerm("hello"))
	assertParseOneExpr(t, "refs 1", "foo[0].bar[x] = baz[false].qux[\"hello\"]", Equality.Expr(ref1, ref2))

	left1 := ObjectTerm(Item(VarTerm("a"), ArrayTerm(ref1)))
	right1 := ArrayTerm(ObjectTerm(Item(IntNumberTerm(42), BooleanTerm(true))))
	assertParseOneExpr(t, "composites", "{a: [foo[0].bar[x]]} = [{42: true}]", Equality.Expr(left1, right1))

	assertParseOneExpr(t, "plus", "x = 1 + 2", Equality.Expr(VarTerm("x"), Plus.Call(IntNumberTerm(1), IntNumberTerm(2))))
	assertParseOneExpr(t, "plus reverse", "1 + 2 = x", Equality.Expr(Plus.Call(IntNumberTerm(1), IntNumberTerm(2)), VarTerm("x")))

	assertParseOneExpr(t, "call", "count([true, false]) = x", Equality.Expr(Count.Call(ArrayTerm(BooleanTerm(true), BooleanTerm(false))), VarTerm("x")))
	assertParseOneExpr(t, "call-reverse", "x = count([true, false])", Equality.Expr(VarTerm("x"), Count.Call(ArrayTerm(BooleanTerm(true), BooleanTerm(false)))))
}

func TestNegatedExpr(t *testing.T) {
	assertParseOneTermNegated(t, "scalars 1", "not true", BooleanTerm(true))
	assertParseOneTermNegated(t, "scalars 2", "not \"hello\"", StringTerm("hello"))
	assertParseOneTermNegated(t, "scalars 3", "not 100", IntNumberTerm(100))
	assertParseOneTermNegated(t, "scalars 4", "not null", NullTerm())
	assertParseOneTermNegated(t, "var", "not x", VarTerm("x"))
	assertParseOneTermNegated(t, "ref", "not x[y].z", RefTerm(VarTerm("x"), VarTerm("y"), StringTerm("z")))
	assertParseOneExprNegated(t, "vars", "not x = y", Equality.Expr(VarTerm("x"), VarTerm("y")))

	ref1 := RefTerm(VarTerm("x"), VarTerm("y"), StringTerm("z"), VarTerm("a"))

	assertParseOneExprNegated(t, "membership", "not x[y].z[a] = \"b\"", Equality.Expr(ref1, StringTerm("b")))
	assertParseOneExprNegated(t, "misc. builtin", "not sorted(x[y].z[a])", NewExpr([]*Term{RefTerm(VarTerm("sorted")), ref1}))
}

func TestExprWith(t *testing.T) {
	assertParseOneExpr(t, "input", "data.foo with input as baz", &Expr{
		Terms: MustParseTerm("data.foo"),
		With: []*With{
			{
				Target: NewTerm(InputRootRef),
				Value:  VarTerm("baz"),
			},
		},
	})

	assertParseOneExpr(t, "builtin/ref target/composites", `plus(data.foo, 1, x) with input.com.acmecorp.obj as {"count": [{1,2,3}]}`, &Expr{
		Terms: MustParseExpr("plus(data.foo, 1, x)").Terms,
		With: []*With{
			{
				Target: MustParseTerm("input.com.acmecorp.obj"),
				Value:  MustParseTerm(`{"count": [{1,2,3}]}`),
			},
		},
	})

	assertParseOneExpr(t, "multiple", `data.foo with input.obj as baz with input.com.acmecorp.obj as {"count": [{1,2,3}]}`, &Expr{
		Terms: MustParseTerm("data.foo"),
		With: []*With{
			{
				Target: MustParseTerm("input.obj"),
				Value:  VarTerm("baz"),
			},
			{
				Target: MustParseTerm("input.com.acmecorp.obj"),
				Value:  MustParseTerm(`{"count": [{1,2,3}]}`),
			},
		},
	})

	assertParseOneExpr(t, "variable target", "true with x as 1", &Expr{
		Terms: BooleanTerm(true),
		With: []*With{
			{
				Target: VarTerm("x"),
				Value:  IntNumberTerm(1),
			},
		},
	})
}

func TestExprWithLocation(t *testing.T) {
	cases := []struct {
		note     string
		input    string
		expected []*Location
	}{
		{
			note:  "base",
			input: "a with b as c",
			expected: []*Location{
				{
					Row:    1,
					Col:    3,
					Offset: 2,
					Text:   []byte("with b as c"),
				},
			},
		},
		{
			note:  "with line break",
			input: "a with b\nas c",
			expected: []*Location{
				{
					Row:    1,
					Col:    3,
					Offset: 2,
					Text:   []byte("with b\nas c"),
				},
			},
		},
		{
			note:  "multiple withs on single line",
			input: "a with b as c with d as e",
			expected: []*Location{
				{
					Row:    1,
					Col:    3,
					Offset: 2,
					Text:   []byte("with b as c"),
				},
				{
					Row:    1,
					Col:    15,
					Offset: 14,
					Text:   []byte("with d as e"),
				},
			},
		},
		{
			note:  "multiple withs on multiple line",
			input: "a with b as c\n\t\twith d as e",
			expected: []*Location{
				{
					Row:    1,
					Col:    3,
					Offset: 2,
					Text:   []byte("with b as c"),
				},
				{
					Row:    2,
					Col:    3,
					Offset: 16,
					Text:   []byte("with d as e"),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			parsed, err := ParseStatement(tc.input)
			if err != nil {
				t.Errorf("Unexpected error on %s: %s", tc.input, err)
				return
			}

			body := parsed.(Body)
			if len(body) != 1 {
				t.Errorf("Parser returned multiple expressions: %v", body)
				return
			}
			expr := body[0]
			if len(expr.With) != len(tc.expected) {
				t.Fatalf("Expected %d with statements, got %d", len(expr.With), len(tc.expected))
			}
			for i, with := range expr.With {
				if !with.Location.Equal(tc.expected[i]) {
					t.Errorf("Expected location %+v for '%v' but got %+v ", *(tc.expected[i]), with.String(), *with.Location)
				}
			}
		})
	}
}

func TestSomeDeclExpr(t *testing.T) {
	opts := ParserOptions{FutureKeywords: []string{"in"}}

	assertParseOneExpr(t, "one", "some x", &Expr{
		Terms: &SomeDecl{
			Symbols: []*Term{
				VarTerm("x"),
			},
		},
	})

	assertParseOneExpr(t, "internal.member_2", "some x in xs", &Expr{
		Terms: &SomeDecl{
			Symbols: []*Term{
				Member.Call(
					VarTerm("x"),
					VarTerm("xs"),
				),
			},
		},
	}, opts)

	assertParseOneExpr(t, "internal.member_3", "some x, y in xs", &Expr{
		Terms: &SomeDecl{
			Symbols: []*Term{
				MemberWithKey.Call(
					VarTerm("x"),
					VarTerm("y"),
					VarTerm("xs"),
				),
			},
		},
	}, opts)

	assertParseErrorContains(t, "not some", "not some x, y in xs",
		"unexpected some keyword: illegal negation of 'some'",
		opts)

	assertParseErrorContains(t, "some + function call", "some f(x)",
		"expected `x in xs` or `x, y in xs` expression")

	assertParseOneExpr(t, "multiple", "some x, y", &Expr{
		Terms: &SomeDecl{
			Symbols: []*Term{
				VarTerm("x"),
				VarTerm("y"),
			},
		},
	}, opts)

	assertParseOneExpr(t, "multiple split across lines", `some x, y,
		z`, &Expr{
		Terms: &SomeDecl{
			Symbols: []*Term{
				VarTerm("x"),
				VarTerm("y"),
				VarTerm("z"),
			},
		},
	})

	assertParseRule(t, "whitespace separated", `

		p[x] {
			some x
			q[x]
		}
	`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x")),
		Body: NewBody(
			NewExpr(&SomeDecl{Symbols: []*Term{VarTerm("x")}}),
			NewExpr(RefTerm(VarTerm("q"), VarTerm("x"))),
		),
	})

	assertParseRule(t, "whitespace separated, following `in` rule ref", `
	p[x] {
		some x
		in[x]
	}
`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x")),
		Body: NewBody(
			NewExpr(&SomeDecl{Symbols: []*Term{VarTerm("x")}}),
			NewExpr(RefTerm(VarTerm("in"), VarTerm("x"))),
		),
	})

	assertParseErrorContains(t, "some x in ... usage is hinted properly", `
	p[x] {
		some x in {"foo": "bar"}
	}`,
		"unexpected ident token: expected \\n or ; or } (hint: `import future.keywords.in` for `some x in xs` expressions)")

	assertParseErrorContains(t, "some x, y in ... usage is hinted properly", `
	p[y] = x {
		some x, y in {"foo": "bar"}
	}`,
		"unexpected ident token: expected \\n or ; or } (hint: `import future.keywords.in` for `some x in xs` expressions)")

	assertParseRule(t, "whitespace terminated", `

	p[x] {
		some x
		x
	}
`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x")),
		Body: NewBody(
			NewExpr(&SomeDecl{Symbols: []*Term{VarTerm("x")}}),
			NewExpr(VarTerm("x")),
		),
	})

	assertParseOneExpr(t, "with modifier on expr", "some x, y in input with input as []",
		&Expr{
			Terms: &SomeDecl{
				Symbols: []*Term{
					MemberWithKey.Call(
						VarTerm("x"),
						VarTerm("y"),
						NewTerm(MustParseRef("input")),
					),
				},
			},
			With: []*With{{Value: ArrayTerm(), Target: NewTerm(MustParseRef("input"))}},
		}, opts)

	assertParseErrorContains(t, "invalid domain (internal.member_2)", "some internal.member_2()", "illegal domain", opts)
	assertParseErrorContains(t, "invalid domain (internal.member_3)", "some internal.member_3()", "illegal domain", opts)

}

func TestEvery(t *testing.T) {
	opts := ParserOptions{unreleasedKeywords: true, FutureKeywords: []string{"every"}}
	assertParseOneExpr(t, "simple", "every x in xs { true }",
		&Expr{
			Terms: &Every{
				Value:  VarTerm("x"),
				Domain: VarTerm("xs"),
				Body: []*Expr{
					NewExpr(BooleanTerm(true)),
				},
			},
		},
		opts)

	assertParseOneExpr(t, "with key", "every k, v in [1,2] { true }",
		&Expr{
			Terms: &Every{
				Key:    VarTerm("k"),
				Value:  VarTerm("v"),
				Domain: ArrayTerm(IntNumberTerm(1), IntNumberTerm(2)),
				Body: []*Expr{
					NewExpr(BooleanTerm(true)),
				},
			},
		}, opts)

	assertParseErrorContains(t, "arbitrary term", "every 10", "expected `x[, y] in xs { ... }` expression", opts)
	assertParseErrorContains(t, "non-var value", "every 10 in xs { true }", "unexpected { token: expected value to be a variable", opts)
	assertParseErrorContains(t, "non-var key", "every 10, x in xs { true }", "unexpected { token: expected key to be a variable", opts)
	assertParseErrorContains(t, "arbitrary call", "every f(10)", "expected `x[, y] in xs { ... }` expression", opts)
	assertParseErrorContains(t, "no body", "every x in xs", "missing body", opts)
	assertParseErrorContains(t, "invalid body", "every x in xs { + }", "unexpected plus token", opts)
	assertParseErrorContains(t, "not every", "not every x in xs { true }", "unexpected every keyword: illegal negation of 'every'", opts)

	assertParseOneExpr(t, `"every" kw implies "in" kw`, "x in xs", Member.Expr(
		VarTerm("x"),
		VarTerm("xs"),
	), opts)

	assertParseOneExpr(t, "with modifier on expr", "every x in input { x } with input as []",
		&Expr{
			Terms: &Every{
				Value:  VarTerm("x"),
				Domain: NewTerm(MustParseRef("input")),
				Body: []*Expr{
					NewExpr(VarTerm("x")),
				},
			},
			With: []*With{{Value: ArrayTerm(), Target: NewTerm(MustParseRef("input"))}},
		}, opts)

	assertParseErrorContains(t, "every x, y in ... usage is hinted properly", `
	p {
		every x, y in {"foo": "bar"} { is_string(x); is_string(y) }
	}`,
		"unexpected ident token: expected \\n or ; or } (hint: `import future.keywords.every` for `every x in xs { ... }` expressions)")

	assertParseErrorContains(t, "not every 'every' gets a hint", `
	p {
		every x
	}`,
		"unexpected ident token: expected \\n or ; or }\n\tevery x\n", // this asserts that the tail of the error message doesn't contain a hint
	)

	assertParseErrorContains(t, "invalid domain (internal.member_2)", "every internal.member_2()", "illegal domain", opts)
	assertParseErrorContains(t, "invalid domain (internal.member_3)", "every internal.member_3()", "illegal domain", opts)
}

func TestNestedExpressions(t *testing.T) {

	n1 := IntNumberTerm(1)
	n2 := IntNumberTerm(2)
	n3 := IntNumberTerm(3)
	n4 := IntNumberTerm(4)
	n6 := IntNumberTerm(6)
	x := VarTerm("x")
	y := VarTerm("y")
	z := VarTerm("z")
	w := VarTerm("w")
	f := RefTerm(VarTerm("f"))
	g := RefTerm(VarTerm("g"))

	tests := []struct {
		note     string
		input    string
		expected *Expr
	}{
		{"associativity", "1 + 2 * 6 / 3",
			Plus.Expr(
				n1,
				Divide.Call(
					Multiply.Call(
						n2,
						n6),
					n3))},
		{"associativity - factors", "x * y / z % w",
			Rem.Expr(Divide.Call(Multiply.Call(x, y), z), w)},
		{"associativity - factors", "w % z / x * y",
			Multiply.Expr(Divide.Call(Rem.Call(w, z), x), y)},
		{"associativity - arithetic", "x + y - z",
			Minus.Expr(Plus.Call(x, y), z)},
		{"associativity - arithmetic", "z - x + y",
			Plus.Expr(Minus.Call(z, x), y)},
		{"associativity - and", "z & x & y",
			And.Expr(And.Call(z, x), y)},
		{"associativity - or", "z | x | y",
			Or.Expr(Or.Call(z, x), y)},
		{"associativity - relations", "x == y != z",
			NotEqual.Expr(Equal.Call(x, y), z)},
		{"grouping", "(1 + 2 * 6 / 3) > 4",
			GreaterThan.Expr(
				Plus.Call(
					n1,
					Divide.Call(
						Multiply.Call(
							n2,
							n6),
						n3)),
				n4)},
		{"nested parens", "(((1 + 2) * (6 / (3))) > 4) != false",
			NotEqual.Expr(
				GreaterThan.Call(
					Multiply.Call(
						Plus.Call(
							n1,
							n2),
						Divide.Call(
							n6,
							n3)),
					n4,
				),
				BooleanTerm(false))},
		{"bitwise or", "x + 1 | 2", Or.Expr(Plus.Call(x, n1), n2)},
		{"bitwise and", "x + 1 | 2 & 3", Or.Expr(Plus.Call(x, n1), And.Call(n2, n3))},
		{"array", "[x + 1, y > 2, z]", NewExpr(ArrayTerm(Plus.Call(x, n1), GreaterThan.Call(y, n2), z))},
		{"object", "{x * 2: y < 2, z[3]: 1 + 6/2}", NewExpr(
			ObjectTerm(
				Item(Multiply.Call(x, n2), LessThan.Call(y, n2)),
				Item(RefTerm(z, n3), Plus.Call(n1, Divide.Call(n6, n2))),
			),
		)},
		{"set", "{x + 1, y + 2, set()}", NewExpr(
			SetTerm(
				Plus.Call(x, n1),
				Plus.Call(y, n2),
				SetTerm(),
			),
		)},
		{"ref", `x[1][y + z[w + 1]].b`, NewExpr(
			RefTerm(
				x,
				n1,
				Plus.Call(
					y,
					RefTerm(
						z,
						Plus.Call(w, n1))),
				StringTerm("b"),
			),
		)},
		{"call void", "f()", NewExpr([]*Term{f})},
		{"call unary", "f(x)", NewExpr([]*Term{f, x})},
		{"call binary", "f(x, y)", NewExpr([]*Term{f, x, y})},
		{"call embedded", "f([g(x), y+1])", NewExpr([]*Term{
			f,
			ArrayTerm(
				CallTerm(g, x),
				Plus.Call(y, n1))})},
		{"call fqn", "foo.bar(1)", NewExpr([]*Term{
			RefTerm(VarTerm("foo"), StringTerm("bar")),
			n1,
		})},
		{"unify", "x = 1", Equality.Expr(x, n1)},
		{"unify embedded", "1 + x = 2 - y", Equality.Expr(Plus.Call(n1, x), Minus.Call(n2, y))},
		{"not keyword", "not x = y", Equality.Expr(x, y).Complement()},
		{"with keyword", "x with p[q] as f([x+1])", NewExpr(x).IncludeWith(
			RefTerm(VarTerm("p"), VarTerm("q")),
			CallTerm(f, ArrayTerm(Plus.Call(x, n1))),
		)},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			expr, err := ParseExpr(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !expr.Equal(tc.expected) {
				t.Fatalf("Expected %v but got %v", tc.expected, expr)
			}
		})
	}
}

func TestChainedCall(t *testing.T) {
	result, err := ParseExpr("foo.bar(1)[0](1).baz")
	if err != nil {
		t.Fatal(err)
	}

	exp := NewExpr(RefTerm(
		CallTerm(
			RefTerm(
				CallTerm(
					RefTerm(VarTerm("foo"), StringTerm("bar")),
					IntNumberTerm(1)),
				IntNumberTerm(0)),
			IntNumberTerm(1)),
		StringTerm("baz")))

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got: %v", exp, result)
	}
}

func TestMultiLineBody(t *testing.T) {
	tests := []struct {
		note  string
		input string
		exp   Body
	}{
		{
			note: "three definitions",
			input: `
x = 1
y = 2
z = [ i | [x,y] = arr
	arr[_] = i]
`,
			exp: MustParseBody(`x = 1; y = 2; z = [i | [x,y] = arr; arr[_] = i]`),
		},
		{
			note: "three definitions, with comments and w/o enclosing braces",
			input: `
x = 1 ; # comment after semicolon
y = 2   # comment without semicolon
z = [ i | [x,y] = arr  # comment in comprehension
	arr[_] = i]
`,
			exp: MustParseBody(`x = 1; y = 2; z = [i | [x,y] = arr; arr[_] = i]`),
		},
		{
			note:  "array following call w/ whitespace",
			input: "f(x)\n [1]",
			exp: NewBody(
				NewExpr([]*Term{RefTerm(VarTerm("f")), VarTerm("x")}),
				NewExpr(ArrayTerm(IntNumberTerm(1))),
			),
		},
		{
			note:  "set following call w/ semicolon",
			input: "f(x);{1}",
			exp: NewBody(
				NewExpr([]*Term{RefTerm(VarTerm("f")), VarTerm("x")}),
				NewExpr(SetTerm(IntNumberTerm(1))),
			),
		},
		{
			note:  "array following array w/ whitespace",
			input: "[1]\n [2]",
			exp: NewBody(
				NewExpr(ArrayTerm(IntNumberTerm(1))),
				NewExpr(ArrayTerm(IntNumberTerm(2))),
			),
		},
		{
			note:  "array following set w/ whitespace",
			input: "{1}\n [2]",
			exp: NewBody(
				NewExpr(SetTerm(IntNumberTerm(1))),
				NewExpr(ArrayTerm(IntNumberTerm(2))),
			),
		},
		{
			note:  "set following call w/ whitespace",
			input: "f(x)\n {1}",
			exp: NewBody(
				NewExpr([]*Term{RefTerm(VarTerm("f")), VarTerm("x")}),
				NewExpr(SetTerm(IntNumberTerm(1))),
			),
		},
		{
			note:  "set following ref w/ whitespace",
			input: "data.p.q\n {1}",
			exp: NewBody(
				NewExpr(&Term{Value: MustParseRef("data.p.q")}),
				NewExpr(SetTerm(IntNumberTerm(1))),
			),
		},
		{
			note:  "set following variable w/ whitespace",
			input: "input\n {1}",
			exp: NewBody(
				NewExpr(&Term{Value: MustParseRef("input")}),
				NewExpr(SetTerm(IntNumberTerm(1))),
			),
		},
		{
			note:  "set following equality w/ whitespace",
			input: "input = 2 \n {1}",
			exp: NewBody(
				Equality.Expr(&Term{Value: MustParseRef("input")}, IntNumberTerm(2)),
				NewExpr(SetTerm(IntNumberTerm(1))),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseOneBody(t, tc.note, tc.input, tc.exp)
		})
	}
}

func TestBitwiseOrVsComprehension(t *testing.T) {

	x := VarTerm("x")
	y := VarTerm("y")
	z := VarTerm("z")
	a := VarTerm("a")
	b := VarTerm("b")

	tests := []struct {
		note  string
		input string
		exp   *Term
	}{
		{
			note:  "array containing bitwise or",
			input: "[x|y,z]",
			exp:   ArrayTerm(Or.Call(x, y), z),
		},
		{
			note:  "array containing bitwise or - last element",
			input: "[z,x|y]",
			exp:   ArrayTerm(z, Or.Call(x, y)),
		},
		{
			note:  "array containing bitwise or - middle",
			input: "[z,x|y,a]",
			exp:   ArrayTerm(z, Or.Call(x, y), a),
		},
		{
			note:  "array containing single bitwise or",
			input: "[x|y,]",
			exp:   ArrayTerm(Or.Call(x, y)),
		},
		{
			note:  "set containing bitwise or",
			input: "{x|y,z}",
			exp:   SetTerm(Or.Call(x, y), z),
		},
		{
			note:  "set containing bitwise or - last element",
			input: "{z,x|y}",
			exp:   SetTerm(z, Or.Call(x, y)),
		},
		{
			note:  "set containing bitwise or - middle",
			input: "{z,x|y,a}",
			exp:   SetTerm(z, Or.Call(x, y), a),
		},
		{
			note:  "set containing single bitwise or",
			input: "{x|y,}",
			exp:   SetTerm(Or.Call(x, y)),
		},
		{
			note:  "object containing bitwise or",
			input: "{x:y|z,a:b}",
			exp:   ObjectTerm([2]*Term{x, Or.Call(y, z)}, [2]*Term{a, b}),
		},
		{
			note:  "object containing single bitwise or",
			input: "{x:y|z,}",
			exp:   ObjectTerm([2]*Term{x, Or.Call(y, z)}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			term, err := ParseTerm(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			if !term.Equal(tc.exp) {
				t.Fatalf("Expected %v but got %v", tc.exp, term)
			}
		})
	}

}

func TestPackage(t *testing.T) {
	ref1 := RefTerm(DefaultRootDocument, StringTerm("foo"))
	assertParsePackage(t, "single", `package foo`, &Package{Path: ref1.Value.(Ref)})
	ref2 := RefTerm(DefaultRootDocument, StringTerm("f00"), StringTerm("bar_baz"), StringTerm("qux"))
	assertParsePackage(t, "multiple", `package f00.bar_baz.qux`, &Package{Path: ref2.Value.(Ref)})
	ref3 := RefTerm(DefaultRootDocument, StringTerm("foo"), StringTerm("bar baz"))
	assertParsePackage(t, "space", `package foo["bar baz"]`, &Package{Path: ref3.Value.(Ref)})
	assertParseError(t, "non-ground ref", "package foo[x]")
	assertParseError(t, "non-string value", "package foo.bar[42].baz")
	assertParseError(t, "invalid term", "package 42")
	assertParseError(t, "scanner error", "package foo.")
	assertParseError(t, "non-string first value", "package e().s")
}

func TestImport(t *testing.T) {
	foo := RefTerm(VarTerm("input"), StringTerm("foo"))
	foobarbaz := RefTerm(VarTerm("input"), StringTerm("foo"), StringTerm("bar"), StringTerm("baz"))
	whitespace := RefTerm(VarTerm("input"), StringTerm("foo"), StringTerm("bar"), StringTerm("white space"))
	assertParseImport(t, "single-input", "import input", &Import{Path: RefTerm(InputRootDocument)})
	assertParseImport(t, "single-data", "import data", &Import{Path: RefTerm(DefaultRootDocument)})
	assertParseImport(t, "multiple", "import input.foo.bar.baz", &Import{Path: foobarbaz})
	assertParseImport(t, "single alias", "import input.foo as bar", &Import{Path: foo, Alias: Var("bar")})
	assertParseImport(t, "multiple alias", "import input.foo.bar.baz as qux", &Import{Path: foobarbaz, Alias: Var("qux")})
	assertParseImport(t, "white space", "import input.foo.bar[\"white space\"]", &Import{Path: whitespace})
	assertParseErrorContains(t, "non-ground ref", "import data.foo[x]", "rego_parse_error: unexpected var token: expecting string")
	assertParseErrorContains(t, "non-string", "import input.foo[0]", "rego_parse_error: unexpected number token: expecting string")
	assertParseErrorContains(t, "unknown root", "import foo.bar", "rego_parse_error: unexpected import path, must begin with one of: {data, future, input}, got: foo")
	assertParseErrorContains(t, "bad variable term", "import input as A(", "rego_parse_error: unexpected eof token: expected var")

	_, _, err := ParseStatements("", "package foo\nimport bar.data\ndefault foo=1")
	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}
	if len(err.(Errors)) > 1 {
		t.Fatalf("Expected a single error, got %s", err)
	}
	txt := err.(Errors)[0].Details.Lines()[0]
	expected := "import bar.data"
	if txt != expected {
		t.Fatalf("Expected error detail text '%s' but got '%s'", expected, txt)
	}
}

func TestFutureImports(t *testing.T) {
	assertParseErrorContains(t, "future", "import future", "invalid import, must be `future.keywords`")
	assertParseErrorContains(t, "future.a", "import future.a", "invalid import, must be `future.keywords`")
	assertParseErrorContains(t, "unknown keyword", "import future.keywords.xyz", "unexpected keyword, must be one of [contains every if in]")
	assertParseErrorContains(t, "all keyword import + alias", "import future.keywords as xyz", "`future` imports cannot be aliased")
	assertParseErrorContains(t, "keyword import + alias", "import future.keywords.in as xyz", "`future` imports cannot be aliased")

	assertParseImport(t, "import kw with kw in options",
		"import future.keywords.in", &Import{Path: RefTerm(VarTerm("future"), StringTerm("keywords"), StringTerm("in"))},
		ParserOptions{FutureKeywords: []string{"in"}})
	assertParseImport(t, "import kw with all kw in options",
		"import future.keywords.in", &Import{Path: RefTerm(VarTerm("future"), StringTerm("keywords"), StringTerm("in"))},
		ParserOptions{AllFutureKeywords: true})

	mod := `
		package p
		import future.keywords
		import future.keywords.in
	`
	parsed := Module{
		Package: MustParseStatement(`package p`).(*Package),
		Imports: []*Import{
			MustParseStatement("import future.keywords").(*Import),
			MustParseStatement("import future.keywords.in").(*Import),
		},
	}
	assertParseModule(t, "multiple imports, all kw in options", mod, &parsed, ParserOptions{AllFutureKeywords: true})
	assertParseModule(t, "multiple imports, single in options", mod, &parsed, ParserOptions{FutureKeywords: []string{"in"}})
}

func TestFutureImportsExtraction(t *testing.T) {
	// These tests assert that "import future..." statements in policies cause
	// the proper keywords to be added to the parser's list of known keywords.
	tests := []struct {
		note, imp string
		exp       map[string]tokens.Token
	}{
		{
			note: "simple import",
			imp:  "import future.keywords.in",
			exp:  map[string]tokens.Token{"in": tokens.In},
		},
		{
			note: "all keywords imported",
			imp:  "import future.keywords",
			exp:  map[string]tokens.Token{"in": tokens.In},
		},
		{
			note: "all keywords + single keyword imported",
			imp: `
				import future.keywords
				import future.keywords.in`,
			exp: map[string]tokens.Token{"in": tokens.In},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			parser := NewParser().WithFilename("").WithReader(bytes.NewBufferString(tc.imp))
			_, _, errs := parser.Parse()
			if exp, act := 0, len(errs); exp != act {
				t.Fatalf("expected %d errors, got %d: %v", exp, act, errs)
			}
			for kw, exp := range tc.exp {
				act := parser.s.s.Keyword(kw)
				if act != exp {
					t.Errorf("expected keyword %q to yield token %v, got %v", kw, exp, act)
				}
			}
		})
	}
}

func TestIsValidImportPath(t *testing.T) {
	tests := []struct {
		path     string
		expected error
	}{
		{"[1,2,3]", fmt.Errorf("invalid path [1, 2, 3]: path must be ref or var")},
	}

	for _, tc := range tests {
		path := MustParseTerm(tc.path).Value
		result := IsValidImportPath(path)
		if tc.expected == nil && result != nil {
			t.Errorf("Unexpected error for %v: %v", path, result)
		} else if !reflect.DeepEqual(tc.expected, result) {
			t.Errorf("For %v expected %v but got: %v", path, tc.expected, result)
		}
	}

}

func TestRule(t *testing.T) {

	assertParseRule(t, "constant", `p = true { true }`, &Rule{
		Head: NewHead(Var("p"), nil, BooleanTerm(true)),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "set", `p[x] { x = 42 }`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x")),
		Body: NewBody(
			Equality.Expr(VarTerm("x"), IntNumberTerm(42)),
		),
	})

	assertParseRule(t, "object", `p[x] = y { x = 42; y = "hello" }`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x"), VarTerm("y")),
		Body: NewBody(
			Equality.Expr(VarTerm("x"), IntNumberTerm(42)),
			Equality.Expr(VarTerm("y"), StringTerm("hello")),
		),
	})

	assertParseRule(t, "constant composite", `p = [{"foo": [1, 2, 3, 4]}] { true }`, &Rule{
		Head: NewHead(Var("p"), nil, ArrayTerm(
			ObjectTerm(Item(StringTerm("foo"), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), IntNumberTerm(4)))))),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "true", `p = true { true }`, &Rule{
		Head: NewHead(Var("p"), nil, BooleanTerm(true)),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "composites in head", `p[[{"x": [a, b]}]] { a = 1; b = 2 }`, &Rule{
		Head: NewHead(Var("p"), ArrayTerm(
			ObjectTerm(
				Item(StringTerm("x"), ArrayTerm(VarTerm("a"), VarTerm("b"))),
			),
		)),
		Body: NewBody(
			Equality.Expr(VarTerm("a"), IntNumberTerm(1)),
			Equality.Expr(VarTerm("b"), IntNumberTerm(2)),
		),
	})

	assertParseRule(t, "refs in head", `p = data.foo[x] { x = 1 }`, &Rule{
		Head: NewHead(Var("p"), nil, &Term{
			Value: MustParseRef("data.foo[x]"),
		}),
		Body: MustParseBody("x = 1"),
	})

	assertParseRule(t, "refs in head", `p[data.foo[x]] { true }`, &Rule{
		Head: NewHead(Var("p"), &Term{
			Value: MustParseRef("data.foo[x]"),
		}),
		Body: MustParseBody("true"),
	})

	assertParseRule(t, "refs in head", `p[data.foo[x]] = data.bar[y] { true }`, &Rule{
		Head: NewHead(Var("p"), &Term{
			Value: MustParseRef("data.foo[x]"),
		}, &Term{
			Value: MustParseRef("data.bar[y]"),
		}),
		Body: MustParseBody("true"),
	})

	assertParseRule(t, "data", `data = true { true }`, &Rule{
		Head: NewHead(Var("data"), nil, MustParseTerm("true")),
		Body: MustParseBody("true"),
	})

	assertParseRule(t, "input", `input = true { true }`, &Rule{
		Head: NewHead(Var("input"), nil, MustParseTerm("true")),
		Body: MustParseBody("true"),
	})

	assertParseRule(t, "default", `default allow = false`, &Rule{
		Default: true,
		Head:    NewHead(Var("allow"), nil, MustParseTerm("false")),
		Body:    NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "default w/ assignment", `default allow := false`, &Rule{
		Default: true,
		Head: &Head{
			Name:   "allow",
			Value:  BooleanTerm(false),
			Assign: true,
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "default w/ comprehension", `default widgets = [x | x = data.fooz[_]]`, &Rule{
		Default: true,
		Head:    NewHead(Var("widgets"), nil, MustParseTerm(`[x | x = data.fooz[_]]`)),
		Body:    NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "one line with braces", `p[x] { x = data.a[_]; count(x, 3) }`, &Rule{
		Head: NewHead(Var("p"), VarTerm("x")),
		Body: MustParseBody(`x = data.a[_]; count(x, 3)`),
	})

	assertParseRule(t, "multiple lines with braces", `p[[x, y]] { [data.a[0]] = [{"x": x}]; count(x, 3); sum(x, y); y > 100 }`,

		&Rule{
			Head: NewHead(Var("p"), MustParseTerm("[x, y]")),
			Body: MustParseBody(`[data.a[0]] = [{"x": x}]; count(x, 3); sum(x, y); y > 100`),
		})

	fxy := &Head{
		Name:  Var("f"),
		Args:  Args{VarTerm("x")},
		Value: VarTerm("y"),
	}

	assertParseRule(t, "identity", `f(x) = y { y = x }`, &Rule{
		Head: fxy,
		Body: NewBody(
			Equality.Expr(VarTerm("y"), VarTerm("x")),
		),
	})

	assertParseRule(t, "composite arg", `f([x, y]) = z { split(x, y, z) }`, &Rule{
		Head: &Head{
			Name:  Var("f"),
			Args:  Args{ArrayTerm(VarTerm("x"), VarTerm("y"))},
			Value: VarTerm("z"),
		},
		Body: NewBody(
			Split.Expr(VarTerm("x"), VarTerm("y"), VarTerm("z")),
		),
	})

	assertParseRule(t, "composite result", `f(1) = [x, y] { split("foo.bar", x, y) }`, &Rule{
		Head: &Head{
			Name:  Var("f"),
			Args:  Args{IntNumberTerm(1)},
			Value: ArrayTerm(VarTerm("x"), VarTerm("y")),
		},
		Body: NewBody(
			Split.Expr(StringTerm("foo.bar"), VarTerm("x"), VarTerm("y")),
		),
	})

	assertParseRule(t, "expr terms: key", `p[f(x) + g(x)] { true }`, &Rule{
		Head: &Head{
			Name: Var("p"),
			Key: Plus.Call(
				CallTerm(RefTerm(VarTerm("f")), VarTerm("x")),
				CallTerm(RefTerm(VarTerm("g")), VarTerm("x")),
			),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "expr terms: value", `p = f(x) + g(x) { true }`, &Rule{
		Head: &Head{
			Name: Var("p"),
			Value: Plus.Call(
				CallTerm(RefTerm(VarTerm("f")), VarTerm("x")),
				CallTerm(RefTerm(VarTerm("g")), VarTerm("x")),
			),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "expr terms: args", `p(f(x) + g(x)) { true }`, &Rule{
		Head: &Head{
			Name: Var("p"),
			Args: Args{
				Plus.Call(
					CallTerm(RefTerm(VarTerm("f")), VarTerm("x")),
					CallTerm(RefTerm(VarTerm("g")), VarTerm("x")),
				),
			},
			Value: BooleanTerm(true),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "assignment operator", `x := 1 { true }`, &Rule{
		Head: &Head{
			Name:   Var("x"),
			Value:  IntNumberTerm(1),
			Assign: true,
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "else assignment", `x := 1 { false } else := 2`, &Rule{
		Head: &Head{
			Name:   "x",
			Value:  IntNumberTerm(1),
			Assign: true,
		},
		Body: NewBody(NewExpr(BooleanTerm(false))),
		Else: &Rule{
			Head: &Head{
				Name:   "x",
				Value:  IntNumberTerm(2),
				Assign: true,
			},
			Body: NewBody(NewExpr(BooleanTerm(true))),
		},
	})

	assertParseRule(t, "partial assignment", `p[x] := y { true }`, &Rule{
		Head: &Head{
			Name:   "p",
			Value:  VarTerm("y"),
			Key:    VarTerm("x"),
			Assign: true,
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	assertParseRule(t, "function assignment", `f(x) := y { true }`, &Rule{
		Head: &Head{
			Name:  "f",
			Value: VarTerm("y"),
			Args: Args{
				VarTerm("x"),
			},
			Assign: true,
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})

	// TODO: expect expressions instead?
	assertParseErrorContains(t, "empty body", `f(_) = y {}`, "rego_parse_error: found empty body")
	assertParseErrorContains(t, "empty rule body", "p {}", "rego_parse_error: found empty body")
	assertParseErrorContains(t, "unmatched braces", `f(x) = y { trim(x, ".", y) `, `rego_parse_error: unexpected eof token: expected \n or ; or }`)

	// TODO: how to highlight that assignment is incorrect here?
	assertParseErrorContains(t, "no output", `f(_) = { "foo" = "bar" }`, "rego_parse_error: unexpected eq token: expected rule value term")
	assertParseErrorContains(t, "no output", `f(_) := { "foo" = "bar" }`, "rego_parse_error: unexpected assign token: expected function value term")
	assertParseErrorContains(t, "no output", `f := { "foo" = "bar" }`, "rego_parse_error: unexpected assign token: expected rule value term")
	assertParseErrorContains(t, "no output", `f[_] := { "foo" = "bar" }`, "rego_parse_error: unexpected assign token: expected partial rule value term")
	assertParseErrorContains(t, "no output", `default f :=`, "rego_parse_error: unexpected assign token: expected default rule value term")

	// TODO(tsandall): improve error checking here. This is a common mistake
	// and the current error message is not very good. Need to investigate if the
	// parser can be improved.
	assertParseError(t, "dangling semicolon", "p { true; false; }")

	assertParseErrorContains(t, "default invalid rule name", `default 0[0`, "unexpected default keyword")
	assertParseErrorContains(t, "default invalid rule value", `default a[0`, "illegal default rule (must have a value)")
	assertParseRule(t, "default missing value", `default a`, &Rule{
		Default: true,
		Head: &Head{
			Name:  Var("a"),
			Value: BooleanTerm(true),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})
	assertParseRule(t, "empty arguments", `f() { x := 1 }`, &Rule{
		Head: &Head{
			Name:  "f",
			Value: BooleanTerm(true),
		},
		Body: MustParseBody(`x := 1`),
	})

	assertParseErrorContains(t, "default invalid rule head ref", `default a = b.c.d`, "illegal default rule (value cannot contain ref)")
	assertParseErrorContains(t, "default invalid rule head call", `default a = g(x)`, "illegal default rule (value cannot contain call)")
	assertParseErrorContains(t, "default invalid rule head builtin call", `default a = upper("foo")`, "illegal default rule (value cannot contain call)")
	assertParseErrorContains(t, "default invalid rule head call", `default a = b`, "illegal default rule (value cannot contain var)")

	assertParseError(t, "extra braces", `{ a := 1 }`)
	assertParseError(t, "invalid rule name dots", `a.b = x { x := 1 }`)
	assertParseError(t, "invalid rule name dots and call", `a.b(x) { x := 1 }`)
	assertParseError(t, "invalid rule name hyphen", `a-b = x { x := 1 }`)

	assertParseRule(t, "wildcard name", `_ { x == 1 }`, &Rule{
		Head: &Head{
			Name:  "$0",
			Value: BooleanTerm(true),
		},
		Body: MustParseBody(`x == 1`),
	})

	assertParseRule(t, "partial object array key", `p[[a, 1, 2]] = x { a := 1; x := "foo" }`, &Rule{
		Head: &Head{
			Name:  "p",
			Key:   ArrayTerm(VarTerm("a"), NumberTerm("1"), NumberTerm("2")),
			Value: VarTerm("x"),
		},
		Body: MustParseBody(`a := 1; x := "foo"`),
	})
	assertParseError(t, "invalid rule body no separator", `p { a = "foo"bar }`)
	assertParseError(t, "invalid rule body no newline", `p { a b c }`)
}

func TestRuleContains(t *testing.T) {
	opts := ParserOptions{FutureKeywords: []string{"contains"}}

	tests := []struct {
		note string
		rule string
		exp  *Rule
	}{
		{
			note: "simple",
			rule: `p contains "x" { true }`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("x")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "no body",
			rule: `p contains "x"`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("x")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "set with var element",
			rule: `deny contains msg { msg := "nonono" }`,
			exp: &Rule{
				Head: NewHead(Var("deny"), VarTerm("msg")),
				Body: MustParseBody(`msg := "nonono"`),
			},
		},
		{
			note: "set with object elem",
			rule: `deny contains {"allow": false, "msg": msg} { msg := "nonono" }`,
			exp: &Rule{
				Head: NewHead(Var("deny"), MustParseTerm(`{"allow": false, "msg": msg}`)),
				Body: MustParseBody(`msg := "nonono"`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseRule(t, tc.note, tc.rule, tc.exp, opts)
		})
	}
}

func TestRuleIf(t *testing.T) {
	opts := ParserOptions{FutureKeywords: []string{"contains", "if", "every"}}

	tests := []struct {
		note string
		rule string
		exp  *Rule
	}{
		{
			note: "complete",
			rule: `p if { true }`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "else",
			rule: `p if { true } else if { true }`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(NewExpr(BooleanTerm(true))),
				Else: &Rule{
					Head: NewHead(Var("p"), nil, BooleanTerm(true)),
					Body: NewBody(NewExpr(BooleanTerm(true))),
				},
			},
		},
		{
			note: "complete, normal body",
			rule: `p if { x := 10; x > y }`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: MustParseBody(`x := 10; x > y`),
			},
		},
		{
			note: "complete+else, normal bodies, assign",
			rule: `p := "yes" if { 10 > y } else := "no" { 10 <= y }`,
			exp: &Rule{
				Head: &Head{
					Name:   Var("p"),
					Value:  StringTerm("yes"),
					Assign: true,
				},
				Body: MustParseBody(`10 > y`),
				Else: &Rule{
					Head: &Head{
						Name:   Var("p"),
						Value:  StringTerm("no"),
						Assign: true,
					},
					Body: MustParseBody(`10 <= y`),
				},
			},
		},
		{
			note: "complete+else, normal bodies, assign; if",
			rule: `p := "yes" if { 10 > y } else := "no" if { 10 <= y }`,
			exp: &Rule{
				Head: &Head{
					Name:   Var("p"),
					Value:  StringTerm("yes"),
					Assign: true,
				},
				Body: MustParseBody(`10 > y`),
				Else: &Rule{
					Head: &Head{
						Name:   Var("p"),
						Value:  StringTerm("no"),
						Assign: true,
					},
					Body: MustParseBody(`10 <= y`),
				},
			},
		},
		{
			note: "complete, shorthand",
			rule: `p if true`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "complete, else, shorthand",
			rule: `p if true else if true`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(NewExpr(BooleanTerm(true))),
				Else: &Rule{
					Head: NewHead(Var("p"), nil, BooleanTerm(true)),
					Body: NewBody(NewExpr(BooleanTerm(true))),
				},
			},
		},
		{
			note: "complete, else, assignment+shorthand",
			rule: `p if true else := 3 if 2 < 1`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(NewExpr(BooleanTerm(true))),
				Else: &Rule{
					Head: NewHead(Var("p"), nil, IntNumberTerm(3)),
					Body: NewBody(LessThan.Expr(IntNumberTerm(2), IntNumberTerm(1))),
				},
			},
		},
		{
			note: "complete+not, shorthand",
			rule: `p if not q`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: MustParseBody(`not q`),
			},
		},
		{
			note: "complete+else, shorthand",
			rule: `p if 1 > 2 else = 42 { 2 > 1 }`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: MustParseBody(`1 > 2`),
				Else: &Rule{
					Head: &Head{
						Name:  Var("p"),
						Value: NumberTerm("42"),
					},
					Body: MustParseBody(`2 > 1`),
				},
			},
		},
		{
			note: "complete+call, shorthand",
			rule: `p if count(q) > 0`,
			exp: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: MustParseBody(`count(q) > 0`),
			},
		},
		{
			note: "function, shorthand",
			rule: `f(x) = y if y := x + 1`,
			exp: &Rule{
				Head: &Head{
					Name:  Var("f"),
					Args:  []*Term{VarTerm("x")},
					Value: VarTerm("y"),
				},
				Body: MustParseBody(`y := x + 1`),
			},
		},
		{
			note: "function+every, shorthand",
			rule: `f(xs) if every x in xs { x != 0 }`,
			exp: &Rule{
				Head: &Head{
					Name:  Var("f"),
					Args:  []*Term{VarTerm("xs")},
					Value: BooleanTerm(true),
				},
				Body: MustParseBodyWithOpts(`every x in xs { x != 0 }`, opts),
			},
		},
		{
			note: "object",
			rule: `p["foo"] = "bar" if { true }`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("foo"), StringTerm("bar")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "object, shorthand",
			rule: `p["foo"] = "bar" if true`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("foo"), StringTerm("bar")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "object with vars",
			rule: `p[x] = y if {
				x := "foo"
				y := "bar"
			}`,
			exp: &Rule{
				Head: NewHead(Var("p"), VarTerm("x"), VarTerm("y")),
				Body: MustParseBody(`x := "foo"; y := "bar"`),
			},
		},
		{
			note: "set",
			rule: `p contains "foo" if { true }`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("foo")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "set, shorthand",
			rule: `p contains "foo" if true`,
			exp: &Rule{
				Head: NewHead(Var("p"), StringTerm("foo")),
				Body: NewBody(NewExpr(BooleanTerm(true))),
			},
		},
		{
			note: "set+var+shorthand",
			rule: `p contains x if { x := "foo" }`,
			exp: &Rule{
				Head: NewHead(Var("p"), VarTerm("x")),
				Body: MustParseBody(`x := "foo"`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			assertParseRule(t, tc.note, tc.rule, tc.exp, opts)
		})
	}

	errors := []struct {
		note string
		rule string
		err  string
	}{
		{
			note: "partial set+if, shorthand",
			rule: `p[x] if x := 1`,
			err:  "rego_parse_error: unexpected if keyword: invalid for partial set rule p (use `contains`)",
		},
		{
			note: "partial set+if",
			rule: `p[x] if { x := 1 }`,
			err:  "rego_parse_error: unexpected if keyword: invalid for partial set rule p (use `contains`)",
		},
	}
	for _, tc := range errors {
		t.Run(tc.note, func(t *testing.T) {
			assertParseErrorContains(t, tc.note, tc.rule, tc.err, opts)
		})
	}
}

func TestRuleElseKeyword(t *testing.T) {
	mod := `package test

	p {
		"p0"
	}

	p {
		"p1"
	} else {
		"p1_e1"
	} else = [null] {
		"p1_e2"
	} else = x {
		x = "p1_e3"
	}

	p {
		"p2"
	}

	f(x) {
		x < 100
	} else = false {
		x > 200
	} else {
		x != 150
	}

	_ {
		x > 0
	} else {
	    x == -1
	} else {
		x > -100
	}

	nobody = 1 {
		false
	} else = 7

	nobody_f(x) = 1 {
		false
	} else = 7
	`

	parsed, err := ParseModule("", mod)
	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	name := Var("p")
	tr := BooleanTerm(true)
	head := &Head{Name: name, Value: tr}

	expected := &Module{
		Package: MustParsePackage(`package test`),
		Rules: []*Rule{
			{
				Head: head,
				Body: MustParseBody(`"p0"`),
			},
			{
				Head: head,
				Body: MustParseBody(`"p1"`),
				Else: &Rule{
					Head: head,
					Body: MustParseBody(`"p1_e1"`),
					Else: &Rule{
						Head: &Head{
							Name:  Var("p"),
							Value: ArrayTerm(NullTerm()),
						},
						Body: MustParseBody(`"p1_e2"`),
						Else: &Rule{
							Head: &Head{
								Name:  name,
								Value: VarTerm("x"),
							},
							Body: MustParseBody(`x = "p1_e3"`),
						},
					},
				},
			},
			{
				Head: head,
				Body: MustParseBody(`"p2"`),
			},
			{
				Head: &Head{
					Name:  Var("f"),
					Args:  Args{VarTerm("x")},
					Value: BooleanTerm(true),
				},
				Body: MustParseBody(`x < 100`),
				Else: &Rule{
					Head: &Head{
						Name:  Var("f"),
						Args:  Args{VarTerm("x")},
						Value: BooleanTerm(false),
					},
					Body: MustParseBody(`x > 200`),
					Else: &Rule{
						Head: &Head{
							Name:  Var("f"),
							Args:  Args{VarTerm("x")},
							Value: BooleanTerm(true),
						},
						Body: MustParseBody(`x != 150`),
					},
				},
			},

			{
				Head: &Head{
					Name:  Var("$0"),
					Value: BooleanTerm(true),
				},
				Body: MustParseBody(`x > 0`),
				Else: &Rule{
					Head: &Head{
						Name:  Var("$0"),
						Value: BooleanTerm(true),
					},
					Body: MustParseBody(`x == -1`),
					Else: &Rule{
						Head: &Head{
							Name:  Var("$0"),
							Value: BooleanTerm(true),
						},
						Body: MustParseBody(`x > -100`),
					},
				},
			},
			{
				Head: &Head{
					Name:  Var("nobody"),
					Value: IntNumberTerm(1),
				},
				Body: MustParseBody("false"),
				Else: &Rule{
					Head: &Head{
						Name:  Var("nobody"),
						Value: IntNumberTerm(7),
					},
					Body: MustParseBody("true"),
				},
			},
			{
				Head: &Head{
					Name:  Var("nobody_f"),
					Args:  Args{VarTerm("x")},
					Value: IntNumberTerm(1),
				},
				Body: MustParseBody("false"),
				Else: &Rule{
					Head: &Head{
						Name:  Var("nobody_f"),
						Args:  Args{VarTerm("x")},
						Value: IntNumberTerm(7),
					},
					Body: MustParseBody("true"),
				},
			},
		},
	}

	if parsed.Compare(expected) != 0 {
		t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, parsed)
	}

	notExpected := &Module{
		Package: MustParsePackage(`package test`),
		Rules: []*Rule{
			{
				Head: head,
				Body: MustParseBody(`"p0"`),
			},
			{
				Head: head,
				Body: MustParseBody(`"p1"`),
				Else: &Rule{
					Head: head,
					Body: MustParseBody(`"p1_e1"`),
					Else: &Rule{
						Head: &Head{
							Name:  Var("p"),
							Value: ArrayTerm(NullTerm()),
						},
						Body: MustParseBody(`"p1_e2"`),
						Else: &Rule{
							Head: &Head{
								Name:  name,
								Value: VarTerm("x"),
							},
							Body: MustParseBody(`x = "p1_e4"`),
						},
					},
				},
			},
			{
				Head: head,
				Body: MustParseBody(`"p2"`),
			},
		},
	}

	if parsed.Compare(notExpected) != -1 {
		t.Fatalf("Expected not equal:\n%v\n\nGot:\n%v", parsed, notExpected)
	}

	_, err = ParseModule("", `
	package test
	p[1] { false } else { true }
	`)

	if err == nil || !strings.Contains(err.Error(), "else keyword cannot be used on partial rules") {
		t.Fatalf("Expected parse error but got: %v", err)
	}

	_, err = ParseModule("", `
	package test
	p { false } { false } else { true }
	`)

	if err == nil || !strings.Contains(err.Error(), "unexpected else keyword") {
		t.Fatalf("Expected parse error but got: %v", err)
	}

	_, err = ParseModule("", `
	package test
	p { false } else { false } { true }
	`)

	if err == nil || !strings.Contains(err.Error(), "expected else keyword") {
		t.Fatalf("Expected parse error but got: %v", err)
	}

}

func TestMultipleEnclosedBodies(t *testing.T) {

	result, err := ParseModule("", `package ex

p[x] = y {
	x = "a"
	y = 1
} {
	x = "b"
	y = 2
}

q = 1

f(x) {
	x < 10
} {
	x > 1000
}
`,
	)

	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	expected := MustParseModule(`package ex

p[x] = y { x = "a"; y = 1 }
p[x] = y { x = "b"; y = 2 }
q = 1 { true }
f(x) { x < 10 }
f(x) { x > 1000 }`,
	)

	if !expected.Equal(result) {
		t.Fatal("Expected modules to be equal but got:\n\n", result, "\n\nExpected:\n\n", expected)
	}

}

func TestEmptyModule(t *testing.T) {
	r, err := ParseModule("", "    ")
	if err == nil {
		t.Error("Expected error for empty module")
		return
	}
	if r != nil {
		t.Errorf("Expected nil for empty module: %v", r)
	}
}

func TestComments(t *testing.T) {

	testModule := `package a.b.c

    import input.e.f as g  # end of line
    import input.h

    # by itself

    p[x] = y { y = "foo";
        # inside a rule
        x = "bar";
        x != y;
        q[x]
	}

    import input.xyz.abc

    q # interrupting

	[a] # the head of a rule

	{ m = [1,2,
    3, ];
    a = m[i]

	}

	r[x] { x = [ a | # inside comprehension
					  a = z[i]
	                  b[i].a = a ]

		y = { a | # inside set comprehension
				a = z[i]
			b[i].a = a}

		z = {a: i | # inside object comprehension
				a = z[i]
			b[i].a = a}
					  }`

	assertParseModule(t, "module comments", testModule, &Module{
		Package: MustParseStatement(`package a.b.c`).(*Package),
		Imports: []*Import{
			MustParseStatement("import input.e.f as g").(*Import),
			MustParseStatement("import input.h").(*Import),
			MustParseStatement("import input.xyz.abc").(*Import),
		},
		Rules: []*Rule{
			MustParseStatement(`p[x] = y { y = "foo"; x = "bar"; x != y; q[x] }`).(*Rule),
			MustParseStatement(`q[a] { m = [1, 2, 3]; a = m[i] }`).(*Rule),
			MustParseStatement(`r[x] { x = [a | a = z[i]; b[i].a = a]; y = {a |  a = z[i]; b[i].a = a}; z = {a: i | a = z[i]; b[i].a = a} }`).(*Rule),
		},
	})

	module, err := ParseModule("test.rego", testModule)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	exp := []struct {
		text string
		row  int
		col  int
	}{
		{text: "end of line", row: 3, col: 28},
		{text: "by itself", row: 6, col: 5},
		{text: "inside a rule", row: 9, col: 9},
		{text: "interrupting", row: 17, col: 7},
		{text: "the head of a rule", row: 19, col: 6},
		{text: "inside comprehension", row: 27, col: 19},
		{text: "inside set comprehension", row: 31, col: 13},
		{text: "inside object comprehension", row: 35, col: 15},
	}

	if len(module.Comments) != len(exp) {
		t.Fatalf("Expected %v comments but got %v", len(exp), len(module.Comments))
	}

	for i := range exp {

		expc := &Comment{
			Text: []byte(" " + exp[i].text),
			Location: &Location{
				File: "test.rego",
				Text: []byte("# " + exp[i].text),
				Row:  exp[i].row,
				Col:  exp[i].col,
			},
		}

		if !expc.Equal(module.Comments[i]) {
			comment := module.Comments[i]
			fmt.Printf("comment: %v %v %v %v\n", comment.Location.File, comment.Location.Text, comment.Location.Col, comment.Location.Row)
			fmt.Printf("expcomm: %v %v %v %v\n", expc.Location.File, expc.Location.Text, expc.Location.Col, expc.Location.Row)
			t.Errorf("Expected %q but got: %q (want: %d:%d, got: %d:%d)", expc, comment, exp[i].row, exp[i].col, comment.Location.Row, comment.Location.Col)
		}
	}
}

func TestCommentsWhitespace(t *testing.T) {
	cases := []struct {
		note     string
		module   string
		expected []string
	}{
		{
			note:     "trailing spaces",
			module:   "# a comment    \t   \n",
			expected: []string{" a comment    \t   "},
		},
		{
			note:     "trailing carriage return",
			module:   "# a comment\r\n",
			expected: []string{" a comment"},
		},
		{
			note:     "trailing carriage return double newline",
			module:   "# a comment\r\n\n",
			expected: []string{" a comment"},
		},
		{
			note:     "double trailing carriage return newline",
			module:   "#\r\r\n",
			expected: []string{"\r"},
		},
		{
			note:     "double trailing carriage return",
			module:   "#\r\r",
			expected: []string{"\r"},
		},
		{
			note:     "carriage return",
			module:   "#\r",
			expected: []string{""},
		},
		{
			note:     "carriage return in comment",
			module:   "# abc\rdef\r\n",
			expected: []string{" abc\rdef"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			_, comments, err := ParseStatements("", tc.module)
			if err != nil {
				t.Fatalf("Unexpected parse error: %s", err)
			}

			for i, exp := range tc.expected {
				actual := string(comments[i].Text)
				if exp != actual {
					t.Errorf("Expected comment text (len %d):\n\n\t%q\n\nbut got (len %d):\n\n\t%q\n\n", len(exp), exp, len(actual), actual)
				}
			}
		})
	}
}

func TestExample(t *testing.T) {
	assertParseModule(t, "example module", testModule, &Module{
		Package: MustParseStatement(`package opa.examples`).(*Package),
		Imports: []*Import{
			MustParseStatement("import data.servers").(*Import),
			MustParseStatement("import data.networks").(*Import),
			MustParseStatement("import data.ports").(*Import),
		},
		Rules: []*Rule{
			MustParseStatement(`violations[server] { server = servers[i]; server.protocols[j] = "http"; public_servers[server] }`).(*Rule),
			MustParseStatement(`public_servers[server] { server = servers[i]; server.ports[j] = ports[k].id; ports[k].networks[l] = networks[m].id; networks[m].public = true }`).(*Rule),
		},
	})
}

func TestModuleParseErrors(t *testing.T) {
	input := `
	x = 1			# expect package
	package a  		# unexpected package
	1 = 2			# non-var head
	1 != 2			# non-equality expr
	x = y; x = 1    # multiple exprs
	`

	mod, err := ParseModule("test.rego", input)
	if err == nil {
		t.Fatalf("Expected error but got: %v", mod)
	}

	errs, ok := err.(Errors)
	if !ok {
		panic("unexpected error value")
	}

	if len(errs) != 5 {
		t.Fatalf("Expected exactly 5 errors but got: %v", err)
	}
}

func TestLocation(t *testing.T) {
	mod, err := ParseModule("test", testModule)
	if err != nil {
		t.Errorf("Unexpected error while parsing test module: %v", err)
		return
	}
	expr := mod.Rules[0].Body[0]
	if expr.Location.Col != 5 {
		t.Errorf("Expected column of %v to be 5 but got: %v", expr, expr.Location.Col)
	}
	if expr.Location.Row != 15 {
		t.Errorf("Expected row of %v to be 8 but got: %v", expr, expr.Location.Row)
	}
	if expr.Location.File != "test" {
		t.Errorf("Expected file of %v to be test but got: %v", expr, expr.Location.File)
	}
}

func TestRuleFromBody(t *testing.T) {
	testModule := `package a.b.c

pi = 3.14159
p[x] { x = 1 }
greeting = "hello"
cores = [{0: 1}, {1: 2}]
wrapper = cores[0][1]
pi = [3, 1, 4, x, y, z]
foo["bar"] = "buz"
foo["9"] = "10"
foo.buz = "bar"
bar[1]
bar[[{"foo":"baz"}]]
bar.qux
input = 1
data = 2
f(1) = 2
f(1)
d1 := 1234
`

	assertParseModule(t, "rules from bodies", testModule, &Module{
		Package: MustParseStatement(`package a.b.c`).(*Package),
		Rules: []*Rule{
			MustParseRule(`pi = 3.14159 { true }`),
			MustParseRule(`p[x] { x = 1 }`),
			MustParseRule(`greeting = "hello" { true }`),
			MustParseRule(`cores = [{0: 1}, {1: 2}] { true }`),
			MustParseRule(`wrapper = cores[0][1] { true }`),
			MustParseRule(`pi = [3, 1, 4, x, y, z] { true }`),
			MustParseRule(`foo["bar"] = "buz" { true }`),
			MustParseRule(`foo["9"] = "10" { true }`),
			MustParseRule(`foo["buz"] = "bar" { true }`),
			MustParseRule(`bar[1] { true }`),
			MustParseRule(`bar[[{"foo":"baz"}]] { true }`),
			MustParseRule(`bar["qux"] { true }`),
			MustParseRule(`input = 1 { true }`),
			MustParseRule(`data = 2 { true }`),
			MustParseRule(`f(1) = 2 { true }`),
			MustParseRule(`f(1) = true { true }`),
			MustParseRule("d1 := 1234 { true }"),
		},
	})

	// Verify the rule and rule and rule head col/loc values
	module, err := ParseModule("test.rego", testModule)
	if err != nil {
		t.Fatal(err)
	}

	for i := range module.Rules {
		col := module.Rules[i].Location.Col
		if col != 1 {
			t.Fatalf("expected rule %v column to be 1 but got %v", module.Rules[i].Head.Name, col)
		}
		row := module.Rules[i].Location.Row
		if row != 3+i { // 'pi' rule stats on row 3
			t.Fatalf("expected rule %v row to be %v but got %v", module.Rules[i].Head.Name, 3+i, row)
		}
		col = module.Rules[i].Head.Location.Col
		if col != 1 {
			t.Fatalf("expected rule head %v column to be 1 but got %v", module.Rules[i].Head.Name, col)
		}
		row = module.Rules[i].Head.Location.Row
		if row != 3+i { // 'pi' rule stats on row 3
			t.Fatalf("expected rule head %v row to be %v but got %v", module.Rules[i].Head.Name, 3+i, row)
		}
	}

	mockModule := `package ex

input = {"foo": 1}
data = {"bar": 2}`

	assertParseModule(t, "rule name: input/data", mockModule, &Module{
		Package: MustParsePackage(`package ex`),
		Rules: []*Rule{
			MustParseRule(`input = {"foo": 1} { true }`),
			MustParseRule(`data = {"bar": 2} { true }`),
		},
	})

	multipleExprs := `
    package a.b.c

    pi = 3.14159; pi > 3
    `

	nonEquality := `
    package a.b.c

    pi > 3
    `

	nonVarName := `
    package a.b.c

    "pi" = 3
    `

	withExpr := `
	package a.b.c

	foo = input with input as 1
	`

	badRefLen1 := `
	package a.b.c

	p["x"].y = 1`

	badRefLen2 := `
	package a.b.c

	p["x"].y`

	negated := `
	package a.b.c

	not p = 1`

	nonRefTerm := `
	package a.b.c

	p`

	zeroArgs := `
	package a.b.c

	p()`

	assignToTerm := `
	package a.b.c

	"foo" := 1`

	someDecl := `
	package a

	some x`

	arrayTerm := `
	package a
	[][0]
	`

	callWithRuleKeyPartialSet := `
	package a
	f(x)[x] { true }`

	callWithRuleKeyPartialObject := `
	package a
	f(x)[x] = x { true }`

	assignNoOperands := `
	package a
	assign()`

	assignOneOperand := `
	package a
	assign(x)`

	eqNoOperands := `
	package a
	eq()`

	eqOneOperand := `
	package a
	eq(x)`

	assertParseModuleError(t, "multiple expressions", multipleExprs)
	assertParseModuleError(t, "non-equality", nonEquality)
	assertParseModuleError(t, "non-var name", nonVarName)
	assertParseModuleError(t, "with expr", withExpr)
	assertParseModuleError(t, "bad ref (too long)", badRefLen1)
	assertParseModuleError(t, "bad ref (too long)", badRefLen2)
	assertParseModuleError(t, "negated", negated)
	assertParseModuleError(t, "non ref term", nonRefTerm)
	assertParseModuleError(t, "zero args", zeroArgs)
	assertParseModuleError(t, "assign to term", assignToTerm)
	assertParseModuleError(t, "some decl", someDecl)
	assertParseModuleError(t, "array term", arrayTerm)
	assertParseModuleError(t, "call in ref partial set", "package test\nf().x {}")
	assertParseModuleError(t, "call in ref partial object", "package test\nf().x = y {}")
	assertParseModuleError(t, "number in ref", "package a\n12[3]()=4")
	assertParseModuleError(t, "rule with args and key", callWithRuleKeyPartialObject)
	assertParseModuleError(t, "rule with args and key", callWithRuleKeyPartialSet)
	assertParseModuleError(t, "assign without operands", assignNoOperands)
	assertParseModuleError(t, "assign with only one operand", assignOneOperand)
	assertParseModuleError(t, "eq without operands", eqNoOperands)
	assertParseModuleError(t, "eq with only one operand", eqOneOperand)

	if _, err := ParseRuleFromExpr(&Module{}, &Expr{
		Terms: struct{}{},
	}); err == nil {
		t.Fatal("expected error for unknown expression term type")
	}
}

func TestWildcards(t *testing.T) {

	assertParseOneTerm(t, "ref", "a.b[_].c[_]", RefTerm(
		VarTerm("a"),
		StringTerm("b"),
		VarTerm("$0"),
		StringTerm("c"),
		VarTerm("$1"),
	))

	assertParseOneTerm(t, "nested", `[{"a": a[_]}, _, {"b": _}]`, ArrayTerm(
		ObjectTerm(
			Item(StringTerm("a"), RefTerm(VarTerm("a"), VarTerm("$0"))),
		),
		VarTerm("$1"),
		ObjectTerm(
			Item(StringTerm("b"), VarTerm("$2")),
		),
	))

	assertParseOneExpr(t, "expr", `_ = [a[_]]`, Equality.Expr(
		VarTerm("$0"),
		ArrayTerm(
			RefTerm(VarTerm("a"), VarTerm("$1")),
		)))

	assertParseOneExpr(t, "comprehension", `_ = [x | a = a[_]]`, Equality.Expr(
		VarTerm("$0"),
		ArrayComprehensionTerm(
			VarTerm("x"),
			NewBody(
				Equality.Expr(
					VarTerm("a"),
					RefTerm(VarTerm("a"), VarTerm("$1")),
				),
			),
		)))

	assertParseRule(t, "functions", `f(_) = y { true }`, &Rule{
		Head: &Head{
			Name: Var("f"),
			Args: Args{
				VarTerm("$0"),
			},
			Value: VarTerm("y"),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	})
}

func TestRuleModulePtr(t *testing.T) {
	mod := `package test

	p { true }
	p { true }
	q { true }
	r = 1
	default s = 2
	`

	parsed, err := ParseModule("", mod)
	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	for _, rule := range parsed.Rules {
		if rule.Module != parsed {
			t.Fatalf("Expected module ptr to be %p but got %p", parsed, rule.Module)
		}
	}
}

func TestNoMatchError(t *testing.T) {
	mod := `package test

	p { true;
		 1 != 0; # <-- parse error: no match
	}`

	_, err := ParseModule("foo.rego", mod)

	expected := "1 error occurred: foo.rego:5: rego_parse_error: unexpected } token"

	if !strings.HasPrefix(err.Error(), expected) {
		t.Fatalf("Bad parse error, expected %v but got: %v", expected, err)
	}

	mod = `package test

	p { true // <-- parse error: no match`

	_, err = ParseModule("foo.rego", mod)

	loc := NewLocation([]byte{'/'}, "foo.rego", 3, 12)

	if !loc.Equal(err.(Errors)[0].Location) {
		t.Fatalf("Expected %v but got: %v", loc, err)
	}
}

func TestBraceBracketParenMatchingErrors(t *testing.T) {
	// Checks to prevent regression on issue #4672.
	// Error location is important here, which is why we check
	// the error strings directly.
	tests := []struct {
		note  string
		err   string
		input string
	}{
		{
			note: "Unmatched ')' case",
			err: `1 error occurred: test.rego:4: rego_parse_error: unexpected , token: expected \n or ; or }
	y := contains("a"), "b")
	                  ^`,
			input: `package test
p {
	x := 5
	y := contains("a"), "b")
}`,
		},
		{
			note: "Unmatched '}' case",
			err: `1 error occurred: test.rego:4: rego_parse_error: unexpected , token: expected \n or ; or }
	y := {"a", "b", "c"}, "a"}
	                    ^`,
			input: `package test
p {
	x := 5
	y := {"a", "b", "c"}, "a"}
}`,
		},
		{
			note: "Unmatched ']' case",
			err: `1 error occurred: test.rego:4: rego_parse_error: unexpected , token: expected \n or ; or }
	y := ["a", "b", "c"], "a"]
	                    ^`,
			input: `package test
p {
	x := 5
	y := ["a", "b", "c"], "a"]
}`,
		},
		{
			note: "Unmatched '(' case",
			err: `1 error occurred: test.rego:5: rego_parse_error: unexpected } token: expected "," or ")"
	}
	^`,
			input: `package test
p {
	x := 5
	y := contains("a", "b"
}`,
		},
		{
			note: "Unmatched '{' case",

			err: `1 error occurred: test.rego:5: rego_parse_error: unexpected eof token: expected \n or ; or }
	}
	^`,
			input: `package test
p {
	x := 5
	y := {{"a", "b", "c"}, "a"
}`,
		},
		{
			note: "Unmatched '[' case",
			err: `1 error occurred: test.rego:5: rego_parse_error: unexpected } token: expected "," or "]"
	}
	^`,
			input: `package test
p {
	x := 5
	y := [["a", "b", "c"], "a"
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := ParseModule("test.rego", tc.input)
			if err == nil {
				t.Fatal("Expected error")
			}
			if tc.err != "" && tc.err != err.Error() {
				t.Fatalf("Expected error string %q but got: %q", tc.err, err.Error())
			}
		})
	}
}

func TestParseErrorDetails(t *testing.T) {

	tests := []struct {
		note  string
		exp   *ParserErrorDetail
		err   string
		input string
	}{
		{
			note: "no match: bad rule name",
			exp: &ParserErrorDetail{
				Line: ".",
				Idx:  0,
			},
			input: `
package test
.`,
		},
		{
			note: "no match: bad termination for comprehension",
			exp: &ParserErrorDetail{
				Line: "p = [true | true}",
				Idx:  16,
			},
			input: `
package test
p = [true | true}`},
		{
			note: "no match: non-terminated comprehension",
			exp: &ParserErrorDetail{
				Line: "p = [true | true",
				Idx:  15,
			},
			input: `
package test
p = [true | true`},
		{
			note: "no match: expected expression",
			exp: &ParserErrorDetail{
				Line: "p { true; }",
				Idx:  10,
			},
			input: `
package test
p { true; }`},
		{
			note: "empty body",
			exp: &ParserErrorDetail{
				Line: "p { }",
				Idx:  4,
			},
			input: `
package test
p { }`},
		{
			note: "non-terminated string",
			exp: &ParserErrorDetail{
				Line: `p = "foo`,
				Idx:  4,
			},
			input: `
package test
p = "foo`},
		{
			note: "rule with error begins with one tab",
			exp: &ParserErrorDetail{
				Line: "\tas",
				Idx:  1,
			},
			input: `
package test
	as`,
			err: `1 error occurred: test.rego:3: rego_parse_error: unexpected as keyword
	as
	^`},
		{
			note: "rule term with error begins with two tabs",
			exp: &ParserErrorDetail{
				Line: "\t\tas",
				Idx:  2,
			},
			input: `
package test
p = true {
		as
}`,
			err: `1 error occurred: test.rego:4: rego_parse_error: unexpected as keyword
	as
	^`},
		{
			note: "input is tab and space tokens only",
			exp: &ParserErrorDetail{
				Line: "\t\v\f ",
				Idx:  0,
			},
			input: "\t\v\f ",
			// NOTE(sr): With the unprintable control characters, the output is pretty
			// useless. But it's also quite an edge case.
			err: "1 error occurred: test.rego:1: rego_parse_error: illegal token\n\t\v\f \n\t^",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := ParseModule("test.rego", tc.input)
			if err == nil {
				t.Fatal("Expected error")
			}
			detail := err.(Errors)[0].Details
			if !reflect.DeepEqual(detail, tc.exp) {
				t.Errorf("Expected %v but got: %v", tc.exp, detail)
			}
			if tc.err != "" && tc.err != err.Error() {
				t.Fatalf("Expected error string %q but got: %q", tc.err, err.Error())
			}
		})
	}
}

func TestNamespacedBuiltins(t *testing.T) {

	tests := []struct {
		expr     string
		expected *Term
		wantErr  bool
	}{
		{`foo.bar.baz(1, 2)`, MustParseTerm("foo.bar.baz"), false},
		{`foo.(1,2)`, nil, true},
		{`foo.#.bar(1,2)`, nil, true},
	}

	for _, tc := range tests {
		expr, err := ParseExpr(tc.expr)
		if !tc.wantErr {
			if err != nil {
				t.Fatalf("Unexpected parse error: %v", err)
			}
			terms, ok := expr.Terms.([]*Term)
			if !ok {
				t.Fatalf("Expected terms not: %T", expr.Terms)
			}
			if !terms[0].Equal(tc.expected) {
				t.Fatalf("Expected builtin-name to equal %v but got: %v", tc.expected, terms)
			}
		} else if err == nil {
			t.Fatalf("Expected error from %v but got: %v", tc.expr, expr)
		}
	}
}

func TestRuleHeadLocation(t *testing.T) {

	const input = `package pkg

p[x] {
	x = "hi"
} {
	x = "bye"
}

f(x) {
	false
} else = false {
	true
}
`

	module := MustParseModule(input)

	for _, tc := range []struct {
		note         string
		location     *Location
		expectedRow  int
		expectedText string
	}{
		{
			note:        "partial rule",
			location:    module.Rules[0].Location,
			expectedRow: 3,
			expectedText: `
p[x] {
	x = "hi"
}
			`,
		},
		{
			note:         "partial rule head",
			location:     module.Rules[0].Head.Location,
			expectedRow:  3,
			expectedText: `p[x]`,
		},
		{
			note:         "partial rule head key",
			location:     module.Rules[0].Head.Key.Location,
			expectedRow:  3,
			expectedText: `x`,
		},
		{
			note:        "chained rule",
			location:    module.Rules[1].Location,
			expectedRow: 5,
			expectedText: `
{
	x = "bye"
}
			`,
		},
		{
			note:        "chained rule head",
			location:    module.Rules[1].Head.Location,
			expectedRow: 5,
			expectedText: `
{
	x = "bye"
}
			`,
		},
		{
			note:        "chained rule head key",
			location:    module.Rules[1].Head.Key.Location,
			expectedRow: 5,
			expectedText: `
{
	x = "bye"
}
			`,
		},
		{
			note:        "rule with args",
			location:    module.Rules[2].Location,
			expectedRow: 9,
			expectedText: `
f(x) {
	false
} else = false {
	true
}
			`,
		},
		{
			note:         "rule with args head",
			location:     module.Rules[2].Head.Location,
			expectedRow:  9,
			expectedText: `f(x)`,
		},
		{
			note:         "rule with args head arg 0",
			location:     module.Rules[2].Head.Args[0].Location,
			expectedRow:  9,
			expectedText: `x`,
		},
		{
			note:        "else with args",
			location:    module.Rules[2].Else.Location,
			expectedRow: 11,
			expectedText: `
else = false {
	true
}
			`,
		},
		{
			note:         "else with args head",
			location:     module.Rules[2].Else.Head.Location,
			expectedRow:  11,
			expectedText: `else = false`,
		},
		{
			note:         "else with args head arg 0",
			location:     module.Rules[2].Else.Head.Args[0].Location,
			expectedRow:  9,
			expectedText: `x`,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if tc.location.Row != tc.expectedRow {
				t.Errorf("Expected %d but got %d", tc.expectedRow, tc.location.Row)
			}
			exp := strings.TrimSpace(tc.expectedText)
			if string(tc.location.Text) != exp {
				t.Errorf("Expected text:\n%s\n\ngot:\n%s\n\n", exp, tc.location.Text)
			}
		})
	}
}

func TestParserText(t *testing.T) {

	tests := []struct {
		note  string
		input string
		want  string
	}{
		{
			note:  "relational term",
			input: `(1 == (2 > 3))`,
		},
		{
			note:  "array - empty",
			input: `[ ]`,
		},
		{
			note:  "array - one element",
			input: `[ 1 ]`,
		},
		{
			note:  "array - multiple elements",
			input: `[1 , 2 , 3]`,
		},
		{
			note:  "object - empty",
			input: `{ }`,
		},
		{
			note:  "object - one element",
			input: `{ "foo": 1 }`,
		},
		{
			note:  "object - multiple elements",
			input: `{"foo": 1, "bar": 2}`,
		},
		{
			note:  "set - one element",
			input: `{ 1 }`,
		},
		{
			note:  "set - multiple elements",
			input: `{1 , 2 , 3}`,
		},
		{
			note:  "idents",
			input: "foo",
		},
		{
			note:  "ref",
			input: `data.foo[x].bar`,
		},
		{
			note:  "call",
			input: `data.foo.bar(x)`,
		},
		{
			note:  "ref and call",
			input: `data.foo[1](x).bar(y)[z]`,
		},
		{
			note:  "infix",
			input: "input = 1",
		},
		{
			note:  "negated",
			input: "not x = 1",
		},
		{
			note:  "expr with statements",
			input: "x = 1 with input as 2 with input as 3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, suffix := range []string{"", "\t\n "} {
				input := tc.input + suffix

				stmts, _, err := ParseStatements("test.rego", input)
				if err != nil {
					t.Fatal(err)
				}

				if len(stmts) != 1 {
					t.Fatal("expected exactly one statement but got:", stmts)
				}

				result := string(stmts[0].Loc().Text)

				if result != tc.input {
					t.Fatalf("expected %q but got: %q", tc.input, result)
				}
			}
		})
	}
}

func TestRuleText(t *testing.T) {
	input := ` package test

r[x] = y {
	x = input.a
	x = "foo"
} {
	x = input.b
	x = "bar"
} {
	x = input.c
	x = "baz"
}

r[x] = y {
	x = input.d
	x = "qux"
}
`

	mod := MustParseModule(input)
	rules := mod.Rules

	if len(rules) != 4 {
		t.Fatalf("Expected 4 rules, got %d", len(rules))
	}

	expectedRuleText := []string{
		`
r[x] = y {
	x = input.a
	x = "foo"
}
		`,
		`
{
	x = input.b
	x = "bar"
}
		`,
		`
{
	x = input.c
	x = "baz"
}
		`,
		`
r[x] = y {
	x = input.d
	x = "qux"
}
		`,
	}

	assertLocationText(t, strings.TrimSpace(expectedRuleText[0]), rules[0].Location)
	assertLocationText(t, "r[x] = y", rules[0].Head.Location)
	assertLocationText(t, "y", rules[0].Head.Value.Location)

	// Chained rules recursively set text on heads to be the full rule
	for i := 1; i < len(expectedRuleText)-1; i++ {
		text := strings.TrimSpace(expectedRuleText[i])
		assertLocationText(t, text, rules[i].Location)
		assertLocationText(t, text, rules[i].Head.Location)
		assertLocationText(t, text, rules[i].Head.Value.Location)
	}

	assertLocationText(t, strings.TrimSpace(expectedRuleText[3]), rules[3].Location)
	assertLocationText(t, "r[x] = y", rules[3].Head.Location)
	assertLocationText(t, "y", rules[3].Head.Value.Location)
}

func TestRuleElseText(t *testing.T) {
	input := `
r1 = x {
	a == "foo"
} else = y {
	b == "bar"
}

else {
	c == "baz"
}

else = {
	"k1": 1,
	"k2": 2
} {
	true
}
`

	rule := MustParseRule(input)
	assertLocationText(t, strings.TrimSpace(input), rule.Location)
	assertLocationText(t, "r1 = x", rule.Head.Location)
	assertLocationText(t, "x", rule.Head.Value.Location)

	curElse := rule.Else
	if curElse == nil {
		t.Fatalf("Expected an else block, got nil")
	}
	assertLocationText(t, strings.TrimSpace(`
else = y {
	b == "bar"
}

else {
	c == "baz"
}

else = {
	"k1": 1,
	"k2": 2
} {
	true
}
	`), curElse.Location)
	assertLocationText(t, "else = y", curElse.Head.Location)
	assertLocationText(t, "y", curElse.Head.Value.Location)

	curElse = curElse.Else
	if curElse == nil {
		t.Fatalf("Expected an else block, got nil")
	}
	assertLocationText(t, strings.TrimSpace(`
else {
	c == "baz"
}

else = {
	"k1": 1,
	"k2": 2
} {
	true
}
	`), curElse.Location)
	assertLocationText(t, "else", curElse.Head.Location)
	if curElse.Head.Value.Location != nil {
		t.Errorf("Expected a nil location")
	}

	curElse = curElse.Else
	if curElse == nil {
		t.Fatalf("Expected an else block, got nil")
	}
	assertLocationText(t, strings.TrimSpace(`
else = {
	"k1": 1,
	"k2": 2
} {
	true
}
	`), curElse.Location)
	assertLocationText(t, strings.TrimSpace(`
else = {
	"k1": 1,
	"k2": 2
}
	`), curElse.Head.Location)
	assertLocationText(t, strings.TrimSpace(`
{
	"k1": 1,
	"k2": 2
}
	`), curElse.Head.Value.Location)
}

func TestAnnotations(t *testing.T) {

	dataServers := MustParseRef("data.servers")
	dataNetworks := MustParseRef("data.networks")
	dataPorts := MustParseRef("data.ports")

	schemaServers := MustParseRef("schema.servers")
	schemaNetworks := MustParseRef("schema.networks")
	schemaPorts := MustParseRef("schema.ports")

	stringSchemaAsMap := map[string]interface{}{
		"type": "string",
	}
	var stringSchema interface{} = stringSchemaAsMap

	tests := []struct {
		note           string
		module         string
		expNumComments int
		expAnnotations []*Annotations
		expError       string
	}{
		{
			note: "Single valid annotation",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 4,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Multiple annotations on multiple lines",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 6,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
						{Path: dataNetworks, Schema: schemaNetworks},
						{Path: dataPorts, Schema: schemaPorts},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Comment in between metadata and rule (valid)",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports

# This is a comment after the metadata YAML
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 7,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
						{Path: dataNetworks, Schema: schemaNetworks},
						{Path: dataPorts, Schema: schemaPorts},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Empty comment line in between metadata and rule (valid)",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports
#
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 7,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
						{Path: dataNetworks, Schema: schemaNetworks},
						{Path: dataPorts, Schema: schemaPorts},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Ill-structured (invalid) metadata start",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports
# METADATA
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expError: "test.rego:14: rego_parse_error: yaml: line 7: could not find expected ':'",
		},
		{
			note: "Ill-structured (invalid) annotation document path",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data/servers: schema.servers
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 4,
			expError:       "rego_parse_error: invalid document reference",
		},
		{
			note: "Ill-structured (invalid) annotation schema path",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema/servers
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 4,
			expError:       "rego_parse_error: invalid schema reference",
		},
		{
			note: "Ill-structured (invalid) annotation",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers= schema
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 5,
			expError:       "rego_parse_error: yaml: unmarshal errors:\n  line 3: cannot unmarshal !!str",
		},
		{
			note: "Indentation error in yaml",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
# - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 6,
			expError:       "rego_parse_error: yaml: line 3: did not find expected key",
		},
		{
			note: "Multiple rules with and without metadata",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
#   - data.networks: schema.networks
#   - data.ports: schema.ports
public_servers[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}

public_servers_1[server] {
	server = servers[i]; server.ports[j] = ports[k].id
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
	server.typo  # won't catch this type error since rule has no schema metadata
}`,
			expNumComments: 7,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
						{Path: dataNetworks, Schema: schemaNetworks},
						{Path: dataPorts, Schema: schemaPorts},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Multiple rules with metadata",
			module: `
package opa.examples

import data.servers
import data.networks
import data.ports

# METADATA
# scope: rule
# schemas:
#   - data.servers: schema.servers
public_servers[server] {
	server = servers[i]
}

# METADATA
# scope: rule
# schemas:
#   - data.networks: schema.networks
#   - data.ports: schema.ports
public_servers_1[server] {
	ports[k].networks[l] = networks[m].id;
	networks[m].public = true
}`,
			expNumComments: 9,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: dataServers, Schema: schemaServers},
					},
					Scope: annotationScopeRule,
					node:  MustParseRule(`public_servers[server] { server = servers[i] }`),
				},
				{
					Schemas: []*SchemaAnnotation{

						{Path: dataNetworks, Schema: schemaNetworks},
						{Path: dataPorts, Schema: schemaPorts},
					},
					Scope: annotationScopeRule,
					node:  MustParseRule(`public_servers_1[server] { ports[k].networks[l] = networks[m].id; networks[m].public = true }`),
				},
			},
		},
		{
			note: "multiple metadata blocks on a single rule",
			module: `package test

# METADATA
# title: My rule

# METADATA
# title: My rule 2
p { input = "str" }`,
			expNumComments: 4,
			expAnnotations: []*Annotations{
				{
					Scope: annotationScopeRule,
					Title: "My rule",
				},
				{
					Scope: annotationScopeRule,
					Title: "My rule 2",
				},
			},
		},
		{
			note: "Empty annotation error due to whitespace following METADATA hint",
			module: `package test

# METADATA

# scope: rule
p { input.x > 7 }`,
			expError: "test.rego:3: rego_parse_error: expected METADATA block, found whitespace",
		},
		{
			note: "Annotation on constant",
			module: `
package test

# METADATA
# scope: rule
p := 7`,
			expNumComments: 2,
			expAnnotations: []*Annotations{
				{Scope: annotationScopeRule},
			},
		},
		{
			note: "annotation on package",
			module: `# METADATA
# title: My package
package test

p { input = "str" }`,
			expNumComments: 2,
			expAnnotations: []*Annotations{
				{
					Scope: annotationScopePackage,
					Title: "My package",
				},
			},
		},
		{
			note: "annotation on import",
			module: `package test

# METADATA
# title: My import
import input.foo

p { input = "str" }`,
			expNumComments: 2,
			expError:       "1 error occurred: test.rego:3: rego_parse_error: invalid annotation scope 'import'",
		},
		{
			note: "Default rule scope",
			module: `
package test

# METADATA
# {}
p := 7`,
			expNumComments: 2,
			expAnnotations: []*Annotations{
				{Scope: annotationScopeRule},
			},
		},
		{
			note: "Unknown scope",
			module: `
package test

# METADATA
# scope: deadbeef
p := 7`,
			expNumComments: 2,
			expError:       "invalid annotation scope 'deadbeef'",
		},
		{
			note: "Invalid rule scope/attachment",
			module: `
# METADATA
# scope: rule
package test

p := 7`,
			expNumComments: 2,
			expError:       "test.rego:2: rego_parse_error: annotation scope 'rule' must be applied to rule (have package)",
		},
		{
			note: "Scope attachment error: document on import",
			module: `package test
# METADATA
# scope: document
import data.foo.bar`,
			expError: "test.rego:2: rego_parse_error: annotation scope 'document' must be applied to rule (have import)",
		},
		{
			note: "Scope attachment error: unattached",
			module: `package test

# METADATA
# scope: package`,
			expError: "test.rego:3: rego_parse_error: annotation scope 'package' must be applied to package",
		},
		{
			note: "Scope attachment error: package on non-package",
			module: `package test
# METADATA
# scope: package
import data.foo`,
			expError: "test.rego:2: rego_parse_error: annotation scope 'package' must be applied to package (have import)",
		},
		{
			note: "Inline schema definition",
			module: `package test

# METADATA
# schemas:
# - input: {"type": "string"}
p { input = "str" }`,
			expNumComments: 3,
			expAnnotations: []*Annotations{
				{
					Schemas: []*SchemaAnnotation{
						{Path: InputRootRef, Definition: &stringSchema},
					},
					Scope: annotationScopeRule,
				},
			},
		},
		{
			note: "Rich meta",
			module: `package test

# METADATA
# title: My rule
# description: |
#  My rule has a
#  multiline description.
# organizations:
# - Acme Corp.
# - Soylent Corp.
# - Tyrell Corp.
# related_resources:
# - https://example.com
# - 
#  ref: http://john:123@do.re/mi?foo=bar#baz
#  description: foo bar
# authors:
# - John Doe <john@example.com>
# - name: Jane Doe
#   email: jane@example.com
# custom:
#  list:
#   - a
#   - b
#  map:
#   a: 1
#   b: 2.2
#   c:
#    "3": d
#    "4": e
#  number: 42
#  string: foo bar baz
#  flag:
p { input = "str" }`,
			expNumComments: 31,
			expAnnotations: []*Annotations{
				{
					Scope:         annotationScopeRule,
					Title:         "My rule",
					Description:   "My rule has a\nmultiline description.\n",
					Organizations: []string{"Acme Corp.", "Soylent Corp.", "Tyrell Corp."},
					RelatedResources: []*RelatedResourceAnnotation{
						{
							Ref: mustParseURL("https://example.com"),
						},
						{
							Ref:         mustParseURL("http://john:123@do.re/mi?foo=bar#baz"),
							Description: "foo bar",
						},
					},
					Authors: []*AuthorAnnotation{
						{
							Name:  "John Doe",
							Email: "john@example.com",
						},
						{
							Name:  "Jane Doe",
							Email: "jane@example.com",
						},
					},
					Custom: map[string]interface{}{
						"list": []interface{}{
							"a", "b",
						},
						"map": map[string]interface{}{
							"a": 1,
							"b": 2.2,
							"c": map[string]interface{}{
								"3": "d",
								"4": "e",
							},
						},
						"number": 42,
						"string": "foo bar baz",
						"flag":   nil,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod, err := ParseModuleWithOpts("test.rego", tc.module, ParserOptions{
				ProcessAnnotation: true,
			})
			if err != nil {
				if tc.expError == "" || !strings.Contains(err.Error(), tc.expError) {
					t.Fatalf("Unexpected parse error when getting annotations: %v", err)
				}
				return
			} else if tc.expError != "" {
				t.Fatalf("Expected err: %v but no error from parse module", tc.expError)
			}

			if len(mod.Comments) != tc.expNumComments {
				t.Fatalf("Expected %v comments but got %v", tc.expNumComments, len(mod.Comments))
			}

			if annotationsCompare(tc.expAnnotations, mod.Annotations) != 0 {
				t.Fatalf("expected %v but got %v", tc.expAnnotations, mod.Annotations)
			}
		})
	}
}

func TestAuthorAnnotation(t *testing.T) {
	tests := []struct {
		note     string
		raw      interface{}
		expected interface{}
	}{
		{
			note:     "no name",
			raw:      "",
			expected: fmt.Errorf("author is an empty string"),
		},
		{
			note:     "only whitespaces",
			raw:      " \t",
			expected: fmt.Errorf("author is an empty string"),
		},
		{
			note:     "one name only",
			raw:      "John",
			expected: AuthorAnnotation{Name: "John"},
		},
		{
			note:     "multiple names",
			raw:      "John Jr.\tDoe",
			expected: AuthorAnnotation{Name: "John Jr. Doe"},
		},
		{
			note:     "email only",
			raw:      "<john@example.com>",
			expected: AuthorAnnotation{Email: "john@example.com"},
		},
		{
			note:     "name and email",
			raw:      "John Doe <john@example.com>",
			expected: AuthorAnnotation{Name: "John Doe", Email: "john@example.com"},
		},
		{
			note:     "empty email",
			raw:      "John Doe <>",
			expected: AuthorAnnotation{Name: "John Doe"},
		},
		{
			note:     "name with reserved characters",
			raw:      "John Doe < >",
			expected: AuthorAnnotation{Name: "John Doe < >"},
		},
		{
			note:     "name with reserved characters (email with space)",
			raw:      "<john@ example.com>",
			expected: AuthorAnnotation{Name: "<john@ example.com>"},
		},
		{
			note: "map with name",
			raw: map[string]interface{}{
				"name": "John Doe",
			},
			expected: AuthorAnnotation{Name: "John Doe"},
		},
		{
			note: "map with email",
			raw: map[string]interface{}{
				"email": "john@example.com",
			},
			expected: AuthorAnnotation{Email: "john@example.com"},
		},
		{
			note: "map with name and email",
			raw: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
			expected: AuthorAnnotation{Name: "John Doe", Email: "john@example.com"},
		},
		{
			note: "map with extra entry",
			raw: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
				"foo":   "bar",
			},
			expected: AuthorAnnotation{Name: "John Doe", Email: "john@example.com"},
		},
		{
			note:     "empty map",
			raw:      map[string]interface{}{},
			expected: fmt.Errorf("'name' and/or 'email' values required in object"),
		},
		{
			note: "map with empty name",
			raw: map[string]interface{}{
				"name": "",
			},
			expected: fmt.Errorf("'name' and/or 'email' values required in object"),
		},
		{
			note: "map with email and empty name",
			raw: map[string]interface{}{
				"name":  "",
				"email": "john@example.com",
			},
			expected: AuthorAnnotation{Email: "john@example.com"},
		},
		{
			note: "map with empty email",
			raw: map[string]interface{}{
				"email": "",
			},
			expected: fmt.Errorf("'name' and/or 'email' values required in object"),
		},
		{
			note: "map with name and empty email",
			raw: map[string]interface{}{
				"name":  "John Doe",
				"email": "",
			},
			expected: AuthorAnnotation{Name: "John Doe"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			parsed, err := parseAuthor(tc.raw)

			switch expected := tc.expected.(type) {
			case AuthorAnnotation:
				if err != nil {
					t.Fatal(err)
				}

				if parsed.Compare(&expected) != 0 {
					t.Fatalf("expected %v but got %v", tc.expected, parsed)
				}
			case error:
				if err == nil {
					t.Fatalf("expected '%v' error but got %v", tc.expected, parsed)
				}

				if strings.Compare(expected.Error(), err.Error()) != 0 {
					t.Fatalf("expected %v but got %v", tc.expected, err)
				}
			default:
				t.Fatalf("Unexpected result type: %T", expected)
			}
		})
	}
}

func TestRelatedResourceAnnotation(t *testing.T) {
	tests := []struct {
		note     string
		raw      interface{}
		expected interface{}
	}{
		{
			note:     "empty ref URL",
			raw:      "",
			expected: fmt.Errorf("ref URL may not be empty string"),
		},
		{
			note:     "only whitespaces in ref URL",
			raw:      " \t",
			expected: fmt.Errorf("parse \" \\t\": net/url: invalid control character in URL"),
		},
		{
			note:     "invalid ref URL",
			raw:      "https://foo:bar",
			expected: fmt.Errorf("parse \"https://foo:bar\": invalid port \":bar\" after host"),
		},
		{
			note:     "ref URL as string",
			raw:      "https://example.com/foo?bar#baz",
			expected: RelatedResourceAnnotation{Ref: mustParseURL("https://example.com/foo?bar#baz")},
		},
		{
			note: "map with only ref",
			raw: map[string]interface{}{
				"ref": "https://example.com/foo?bar#baz",
			},
			expected: RelatedResourceAnnotation{Ref: mustParseURL("https://example.com/foo?bar#baz")},
		},
		{
			note: "map with only description",
			raw: map[string]interface{}{
				"description": "foo bar",
			},
			expected: fmt.Errorf("'ref' value required in object"),
		},
		{
			note: "map with ref and description",
			raw: map[string]interface{}{
				"ref":         "https://example.com/foo?bar#baz",
				"description": "foo bar",
			},
			expected: RelatedResourceAnnotation{
				Ref:         mustParseURL("https://example.com/foo?bar#baz"),
				Description: "foo bar",
			},
		},
		{
			note: "map with ref and description",
			raw: map[string]interface{}{
				"ref":         "https://example.com/foo?bar#baz",
				"description": "foo bar",
				"foo":         "bar",
			},
			expected: RelatedResourceAnnotation{
				Ref:         mustParseURL("https://example.com/foo?bar#baz"),
				Description: "foo bar",
			},
		},
		{
			note:     "empty map",
			raw:      map[string]interface{}{},
			expected: fmt.Errorf("'ref' value required in object"),
		},
		{
			note: "map with empty ref",
			raw: map[string]interface{}{
				"ref": "",
			},
			expected: fmt.Errorf("'ref' value required in object"),
		},
		{
			note: "map with only whitespace in ref",
			raw: map[string]interface{}{
				"ref": " \t",
			},
			expected: fmt.Errorf("'ref' value required in object"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			parsed, err := parseRelatedResource(tc.raw)

			switch expected := tc.expected.(type) {
			case RelatedResourceAnnotation:
				if err != nil {
					t.Fatal(err)
				}

				if parsed.Compare(&expected) != 0 {
					t.Fatalf("expected %v but got %v", tc.expected, parsed)
				}
			case error:
				if err == nil {
					t.Fatalf("expected '%v' error but got %v", tc.expected, parsed)
				}

				if strings.Compare(expected.Error(), err.Error()) != 0 {
					t.Fatalf("expected %v but got %v", tc.expected, err)
				}
			default:
				t.Fatalf("Unexpected result type: %T", expected)
			}
		})
	}
}

func assertLocationText(t *testing.T, expected string, actual *Location) {
	t.Helper()
	if actual == nil || actual.Text == nil {
		t.Errorf("Expected a non nil location and text")
		return
	}
	if string(actual.Text) != expected {
		t.Errorf("Unexpected Location text, got:\n%s\n\nExpected:\n%s\n\n", actual.Text, expected)
	}
}

func assertParseError(t *testing.T, msg string, input string) {
	t.Helper()
	t.Run(msg, func(t *testing.T) {
		assertParseErrorFunc(t, msg, input, func(string) {})
	})
}

func assertParseErrorContains(t *testing.T, msg string, input string, expected string, opts ...ParserOptions) {
	t.Helper()
	assertParseErrorFunc(t, msg, input, func(result string) {
		t.Helper()
		if !strings.Contains(result, expected) {
			t.Errorf("Error on test \"%s\": expected parse error to contain:\n\n%v\n\nbut got:\n\n%v", msg, expected, result)
		}
	}, opts...)
}

func assertParseErrorFunc(t *testing.T, msg string, input string, f func(string), opts ...ParserOptions) {
	t.Helper()
	opt := ParserOptions{}
	if len(opts) == 1 {
		opt = opts[0]
	}
	stmts, _, err := ParseStatementsWithOpts("", input, opt)
	if err == nil && len(stmts) != 1 {
		err = fmt.Errorf("expected exactly one statement")
	}
	if err == nil {
		t.Errorf("Error on test \"%s\": expected parse error on %s: expected no statements, got %d: %v", msg, input, len(stmts), stmts)
		return
	}
	result := err.Error()
	// error occurred: <line>:<col>: <message>
	parts := strings.SplitN(result, ":", 4)
	result = strings.TrimSpace(parts[len(parts)-1])
	f(result)
}

func assertParseImport(t *testing.T, msg string, input string, correct *Import, opts ...ParserOptions) {
	t.Helper()
	assertParseOne(t, msg, input, func(parsed interface{}) {
		t.Helper()
		imp := parsed.(*Import)
		if !imp.Equal(correct) {
			t.Errorf("Error on test \"%s\": imports not equal: %v (parsed), %v (correct)", msg, imp, correct)
		}
	}, opts...)
}

func assertParseModule(t *testing.T, msg string, input string, correct *Module, opts ...ParserOptions) {
	opt := ParserOptions{}
	if len(opts) == 1 {
		opt = opts[0]
	}
	m, err := ParseModuleWithOpts("", input, opt)
	if err != nil {
		t.Errorf("Error on test \"%s\": parse error on %s: %s", msg, input, err)
		return
	}

	if !m.Equal(correct) {
		t.Errorf("Error on test %s: modules not equal: %v (parsed), %v (correct)", msg, m, correct)
	}

}

func assertParseModuleError(t *testing.T, msg, input string) {
	m, err := ParseModule("", input)
	if err == nil {
		t.Errorf("Error on test \"%s\": expected parse error: %v (parsed)", msg, m)
	}
}

func assertParsePackage(t *testing.T, msg string, input string, correct *Package) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		pkg := parsed.(*Package)
		if !pkg.Equal(correct) {
			t.Errorf("Error on test \"%s\": packages not equal: %v (parsed), %v (correct)", msg, pkg, correct)
		}
	})
}

func assertParseOne(t *testing.T, msg string, input string, correct func(interface{}), opts ...ParserOptions) {
	t.Helper()
	opt := ParserOptions{}
	if len(opts) == 1 {
		opt = opts[0]
	}
	stmts, _, err := ParseStatementsWithOpts("", input, opt)
	if err != nil {
		t.Errorf("Error on test \"%s\": parse error on %s: %s", msg, input, err)
		return
	}
	if len(stmts) != 1 {
		t.Errorf("Error on test \"%s\": parse error on %s: expected exactly one statement, got %d: %v", msg, input, len(stmts), stmts)
		return
	}
	correct(stmts[0])
}

func assertParseOneBody(t *testing.T, msg string, input string, correct Body) {
	t.Helper()
	body, err := ParseBody(input)
	if err != nil {
		t.Fatal(err)
	}
	if !body.Equal(correct) {
		t.Fatalf("Error on test \"%s\": bodies not equal:\n%v (parsed)\n%v (correct)", msg, body, correct)
	}
}

func assertParseOneExpr(t *testing.T, msg string, input string, correct *Expr, opts ...ParserOptions) {
	t.Helper()
	assertParseOne(t, msg, input, func(parsed interface{}) {
		t.Helper()
		body := parsed.(Body)
		if len(body) != 1 {
			t.Errorf("Error on test \"%s\": parser returned multiple expressions: %v", msg, body)
			return
		}
		expr := body[0]
		if !expr.Equal(correct) {
			t.Errorf("Error on test \"%s\": expressions not equal:\n%v (parsed)\n%v (correct)", msg, expr, correct)
		}
	}, opts...)
}

func assertParseOneExprNegated(t *testing.T, msg string, input string, correct *Expr) {
	correct.Negated = true
	assertParseOneExpr(t, msg, input, correct)
}

func assertParseOneTerm(t *testing.T, msg string, input string, correct *Term) {
	t.Helper()
	t.Run(msg, func(t *testing.T) {
		assertParseOneExpr(t, msg, input, &Expr{Terms: correct})
	})
}

func assertParseOneTermNegated(t *testing.T, msg string, input string, correct *Term) {
	t.Helper()
	assertParseOneExprNegated(t, msg, input, &Expr{Terms: correct})
}

func assertParseRule(t *testing.T, msg string, input string, correct *Rule, opts ...ParserOptions) {
	t.Helper()
	assertParseOne(t, msg, input, func(parsed interface{}) {
		t.Helper()
		rule := parsed.(*Rule)
		if !rule.Equal(correct) {
			t.Errorf("Error on test \"%s\": rules not equal: %v (parsed), %v (correct)", msg, rule, correct)
		}
	},
		opts...)
}
