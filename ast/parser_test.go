// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var _ = fmt.Printf

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
	assertParseError(t, "hex", "6abc")
	assertParseError(t, "non-terminated", "\"foo")
	assertParseError(t, "non-terminated-raw", "`foo")
	assertParseError(t, "non-string", "'a string'")
	assertParseError(t, "non-number", "6zxy")
	assertParseError(t, "non-number2", "6d7")
	assertParseError(t, "non-number3", "6\"foo\"")
	assertParseError(t, "non-number4", "6true")
	assertParseError(t, "non-number5", "6false")
	assertParseError(t, "non-number6", "6[null, null]")
	assertParseError(t, "non-number7", "6{\"foo\": \"bar\"}")
}

func TestVarTerms(t *testing.T) {
	assertParseOneTerm(t, "var", "foo", VarTerm("foo"))
	assertParseOneTerm(t, "var", "foo_bar", VarTerm("foo_bar"))
	assertParseOneTerm(t, "var", "foo0", VarTerm("foo0"))
	assertParseOneTerm(t, "import prefix", "imports", VarTerm("imports"))
	assertParseOneTerm(t, "not prefix", "not_foo", VarTerm("not_foo"))
	assertParseOneTerm(t, `package prefix`, "packages", VarTerm("packages"))
	assertParseError(t, "not keyword", "not")
	assertParseError(t, `package keyword`, "package")
	assertParseError(t, "import keyword", "import")
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
}

func TestObjectWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{\"abc\": 7, \"def\": 8}", ObjectTerm(Item(StringTerm("abc"), IntNumberTerm(7)), Item(StringTerm("def"), IntNumberTerm(8))))
	assertParseOneTerm(t, "bool", "{\"abc\": false, \"def\": true}", ObjectTerm(Item(StringTerm("abc"), BooleanTerm(false)), Item(StringTerm("def"), BooleanTerm(true))))
	assertParseOneTerm(t, "string", "{\"abc\": \"foo\", \"def\": \"bar\"}", ObjectTerm(Item(StringTerm("abc"), StringTerm("foo")), Item(StringTerm("def"), StringTerm("bar"))))
	assertParseOneTerm(t, "mixed", "{\"abc\": 7, \"def\": null}", ObjectTerm(Item(StringTerm("abc"), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key", "{8: 7, \"def\": null}", ObjectTerm(Item(IntNumberTerm(8), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "number key 2", "{8.5: 7, \"def\": null}", ObjectTerm(Item(FloatNumberTerm(8.5), IntNumberTerm(7)), Item(StringTerm("def"), NullTerm())))
	assertParseOneTerm(t, "bool key", "{true: false}", ObjectTerm(Item(BooleanTerm(true), BooleanTerm(false))))
}

func TestObjectWithVars(t *testing.T) {
	assertParseOneTerm(t, "var keys", "{foo: \"bar\", bar: 64}", ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), IntNumberTerm(64))))
	assertParseOneTerm(t, "nested var keys", "{baz: {foo: \"bar\", bar: qux}}", ObjectTerm(Item(VarTerm("baz"), ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), VarTerm("qux"))))))
	assertParseOneTerm(t, "trailing comma", "{foo: \"bar\", bar: 64, }", ObjectTerm(Item(VarTerm("foo"), StringTerm("bar")), Item(VarTerm("bar"), IntNumberTerm(64))))
}

func TestObjectFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "{foo: bar, baz: [], qux: corge")
	assertParseError(t, "non-terminated 2", "{foo: bar, baz: [], qux: ")
	assertParseError(t, "non-terminated 3", "{foo: bar, baz: [], qux ")
	assertParseError(t, "non-terminated 4", "{foo: bar, baz: [], ")
	assertParseError(t, "missing separator", "{foo: bar baz: []}")
	assertParseError(t, "missing start", "foo: bar, baz: [], qux: corge}")
}

func TestArrayWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "[1,2,3,4.5]", ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), FloatNumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "[true, false, true]", ArrayTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "[\"foo\", \"bar\"]", ArrayTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "[null, true, 42]", ArrayTerm(NullTerm(), BooleanTerm(true), IntNumberTerm(42)))
	assertParseOneTerm(t, "trailing comma", "[null, true, ]", ArrayTerm(NullTerm(), BooleanTerm(true)))
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
}

func TestSetWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{1,2,3,4.5}", SetTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), FloatNumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "{true, false, true}", SetTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "{\"foo\", \"bar\"}", SetTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "{null, true, 42}", SetTerm(NullTerm(), BooleanTerm(true), IntNumberTerm(42)))
	assertParseOneTerm(t, "trailing comma", "{null, true,}", SetTerm(NullTerm(), BooleanTerm(true)))
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

	input := `[{"x": [a[i] | xs = [{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"]; xs[j].a[k] = "foo"]}]`

	expected := ArrayTerm(
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

	assertParseOneTerm(t, "nested", input, expected)
}

func TestObjectComprehensions(t *testing.T) {
	input := `[{"x": {a[i]: b[i] | xs = {"foo":{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"}; xs[j].a[k] = "foo"}}]`

	expected := ArrayTerm(
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

	assertParseOneTerm(t, "nested", input, expected)
}

func TestSetComprehensions(t *testing.T) {
	input := `[{"x": {a[i] | xs = {{"a": ["baz", j]} | q[p]; p.a != "bar"; j = "foo"}; xs[j].a[k] = "foo"}}]`

	expected := ArrayTerm(
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

	assertParseOneTerm(t, "nested", input, expected)
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

func TestMultiLineBody(t *testing.T) {

	input1 := `
	{
		x = 1
		y = 2
		z = [ i | [x,y] = arr
				   arr[_] = i]
	}
	`

	body1, err := ParseBody(input1)
	if err != nil {
		t.Fatalf("Unexpected parse error on enclosed body: %v", err)
	}

	expected1 := MustParseBody(`x = 1; y = 2; z = [i | [x,y] = arr; arr[_] = i]`)

	if !body1.Equal(expected1) {
		t.Errorf("Expected enclosed body to equal %v but got: %v", expected1, body1)
	}

	// Check that parser can handle multiple expressions w/o enclsoing braces.
	input2 := `
		x = 1 ; # comment after semicolon
		y = 2   # comment without semicolon
		z = [ i | [x,y] = arr  # comment in comprehension
				   arr[_] = i]
	`

	body2, err := ParseBody(input2)
	if err != nil {
		t.Fatalf("Unexpected parse error on enclosed body: %v", err)
	}

	if !body2.Equal(expected1) {
		t.Errorf("Expected unenclosed body to equal %v but got: %v", expected1, body1)
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
	assertParseErrorEquals(t, "non-ground ref", "import data.foo[x]", "rego_parse_error: invalid path data.foo[x]: path elements must be strings")
	assertParseErrorEquals(t, "non-string", "import input.foo[0]", "rego_parse_error: invalid path input.foo[0]: path elements must be strings")
	assertParseErrorEquals(t, "unknown root", "import foo.bar", "rego_parse_error: invalid path foo.bar: path must begin with input or data")
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

	assertParseErrorEquals(t, "empty body", `f(_) = y {}`, "rego_parse_error: body must be non-empty")
	assertParseErrorEquals(t, "object composite key", "p[[x,y]] = z { true }", "rego_parse_error: object key must be string, var, or ref, not array")
	assertParseErrorEquals(t, "default ref value", "default p = [data.foo]", "rego_parse_error: default rule value cannot contain ref")
	assertParseErrorEquals(t, "default var value", "default p = [x]", "rego_parse_error: default rule value cannot contain var")
	assertParseErrorEquals(t, "empty rule body", "p {}", "rego_parse_error: body must be non-empty")

	assertParseErrorContains(t, "no output", `f(_) = { "foo" = "bar" }`, "rego_parse_error: no match found")
	assertParseErrorContains(t, "unmatched braces", `f(x) = y { trim(x, ".", y) `, "rego_parse_error: no match found")

	// TODO(tsandall): improve error checking here. This is a common mistake
	// and the current error message is not very good. Need to investigate if the
	// parser can be improved.
	assertParseError(t, "dangling semicolon", "p { true; false; }")
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

	if err == nil || !strings.Contains(err.Error(), "unexpected 'else' keyword") {
		t.Fatalf("Expected parse error but got: %v", err)
	}

	_, err = ParseModule("", `
	package test
	p { false } { false } else { true }
	`)

	if err == nil || !strings.Contains(err.Error(), "unexpected 'else' keyword") {
		t.Fatalf("Expected parse error but got: %v", err)
	}

	_, err = ParseModule("", `
	package test
	p { false } else { false } { true }
	`)

	if err == nil || !strings.Contains(err.Error(), "expected 'else' keyword") {
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
	if err != nil {
		t.Errorf("Expected nil for empty module: %s", err)
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

    q # interruptting

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

pi = 3.14159 { true }
p[x] { x = 1 }
greeting = "hello" { true }
cores = [{0: 1}, {1: 2}] { true }
wrapper = cores[0][1] { true }
pi = [3, 1, 4, x, y, z] { true }
foo["bar"] = "buz"
foo["9"] = "10"
foo.buz = "bar"
bar[1]
bar[[{"foo":"baz"}]]
input = 1
data = 2
f(1) = 2
f(1)
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
			MustParseRule(`input = 1 { true }`),
			MustParseRule(`data = 2 { true }`),
			MustParseRule(`f(1) = 2 { true }`),
			MustParseRule(`f(1) = true { true }`),
		},
	})

	mockModule := `package ex

input = {"foo": 1} { true }
data = {"bar": 2} { true }`

	assertParseModule(t, "rule name: input/data", mockModule, &Module{
		Package: MustParsePackage(`package ex`),
		Rules: []*Rule{
			MustParseRule(`input = {"foo": 1} { true }`),
			MustParseRule(`data = {"bar": 2} { true }`),
		},
	})

	multipleExprs := `
    package a.b.c

    pi = 3.14159, pi > 3
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

	assertParseModuleError(t, "multiple expressions", multipleExprs)
	assertParseModuleError(t, "non-equality", nonEquality)
	assertParseModuleError(t, "non-var name", nonVarName)
	assertParseModuleError(t, "with expr", withExpr)
	assertParseModuleError(t, "bad ref (too long)", badRefLen1)
	assertParseModuleError(t, "bad ref (too long)", badRefLen2)
	assertParseModuleError(t, "negated", negated)
	assertParseModuleError(t, "non ref term", nonRefTerm)
	assertParseModuleError(t, "zero args", zeroArgs)
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

	expected := "1 error occurred: foo.rego:5: rego_parse_error: no match found"

	if !strings.HasPrefix(err.Error(), expected) {
		t.Fatalf("Bad parse error, expected %v but got: %v", expected, err)
	}

	mod = `package test

	p { true // <-- parse error: no match`

	_, err = ParseModule("foo.rego", mod)

	loc := NewLocation(nil, "foo.rego", 3, 12)

	if err.(Errors)[0].Location.File != "foo.rego" || err.(Errors)[0].Location.Row != 3 {
		t.Fatalf("Expected %v but got: %v", loc, err)
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
		{`foo(1,2,3).bar`, nil, true},
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

func assertParse(t *testing.T, msg string, input string, correct func([]Statement)) {
	p, _, err := ParseStatements("", input)
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, input, err)
		return
	}
	correct(p)
}

func assertParseError(t *testing.T, msg string, input string) {
	assertParseErrorFunc(t, msg, input, func(string) {})
}

func assertParseErrorContains(t *testing.T, msg string, input string, expected string) {
	assertParseErrorFunc(t, msg, input, func(result string) {
		if !strings.Contains(result, expected) {
			t.Errorf("Error on test %s: expected parse error to contain %v but got: %v", msg, expected, result)
		}
	})
}

func assertParseErrorEquals(t *testing.T, msg string, input string, expected string) {
	assertParseErrorFunc(t, msg, input, func(result string) {
		if result != expected {
			t.Errorf("Error on test %s: expected parse error to equal %v but got: %v", msg, expected, result)
		}
	})
}

func assertParseErrorFunc(t *testing.T, msg string, input string, f func(string)) {
	p, err := ParseStatement(input)
	if err == nil {
		t.Errorf("Error on test %s: expected parse error: %v (parsed)", msg, p)
		return
	}
	result := err.Error()
	// error occurred: <line>:<col>: <message>
	parts := strings.SplitN(result, ":", 4)
	result = strings.TrimSpace(parts[len(parts)-1])
	f(result)
}

func assertParseImport(t *testing.T, msg string, input string, correct *Import) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		imp := parsed.(*Import)
		if !imp.Equal(correct) {
			t.Errorf("Error on test %s: imports not equal: %v (parsed), %v (correct)", msg, imp, correct)
		}
	})
}

func assertParseModule(t *testing.T, msg string, input string, correct *Module) {

	m, err := ParseModule("", input)
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, input, err)
		return
	}

	if !m.Equal(correct) {
		t.Errorf("Error on test %s: modules not equal: %v (parsed), %v (correct)", msg, m, correct)
	}

}

func assertParseModuleError(t *testing.T, msg, input string) {
	m, err := ParseModule("", input)
	if err == nil {
		t.Errorf("Error on test %v: expected parse error: %v (parsed)", msg, m)
	}
}

func assertParsePackage(t *testing.T, msg string, input string, correct *Package) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		pkg := parsed.(*Package)
		if !pkg.Equal(correct) {
			t.Errorf("Error on test %s: packages not equal: %v (parsed), %v (correct)", msg, pkg, correct)
		}
	})
}

func assertParseOne(t *testing.T, msg string, input string, correct func(interface{})) {
	p, err := ParseStatement(input)
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, input, err)
		return
	}
	correct(p)
}

func assertParseOneExpr(t *testing.T, msg string, input string, correct *Expr) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		body := parsed.(Body)
		if len(body) != 1 {
			t.Errorf("Error on test %s: parser returned multiple expressions: %v", msg, body)
			return
		}
		expr := body[0]
		if !expr.Equal(correct) {
			t.Errorf("Error on test %s: expressions not equal:\n%v (parsed)\n%v (correct)", msg, expr, correct)
		}
	})
}

func assertParseOneExprNegated(t *testing.T, msg string, input string, correct *Expr) {
	correct.Negated = true
	assertParseOneExpr(t, msg, input, correct)
}

func assertParseOneTerm(t *testing.T, msg string, input string, correct *Term) {
	assertParseOneExpr(t, msg, input, &Expr{Terms: correct})
}

func assertParseOneTermNegated(t *testing.T, msg string, input string, correct *Term) {
	assertParseOneExprNegated(t, msg, input, &Expr{Terms: correct})
}

func assertParseRule(t *testing.T, msg string, input string, correct *Rule) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		rule := parsed.(*Rule)
		if !rule.Equal(correct) {
			t.Errorf("Error on test %s: rules not equal: %v (parsed), %v (correct)", msg, rule, correct)
		}
	})
}
