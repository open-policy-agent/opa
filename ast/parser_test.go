// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var _ = fmt.Printf

const (
	testModule = `
package opa.examples                            # this policy belongs the opa.examples package

import data.servers                             # import the data.servers document to refer to it as "servers" instead of "data.servers"
import data.networks                            # same but for data.networks
import data.ports                               # same but for data.ports

violations[server] :-                           # a server exists in the violations set if:
    server = servers[i],                        # the server exists in the servers collection
    server.protocols[j] = "http",               # and the server has http in its protocols collection
    public_servers[server]                      # and the server exists in the public_servers set

public_servers[server] :-                       # a server exists in the public_servers set if:
    server = servers[i],                        # the server exists in the servers collection
    server.ports[j] = ports[k].id,              # and the server is connected to a port in the ports collection
    ports[k].networks[l] = networks[m].id,      # and the port is connected to a network in the networks collection
    networks[m].public = true                   # and the network is public
    `
)

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
	assertParseError(t, "non-string", "'a string'")
	assertParseError(t, "non-number", "6zxy")
	assertParseError(t, "non-number2", "6d7")
	assertParseError(t, "non-number3", "6\"foo\"")
	assertParseError(t, "non-number4", "6true")
	assertParseError(t, "non-number5", "6false")
	assertParseError(t, "non-number6", "6[null, null]")
	assertParseError(t, "non-number7", "6{\"foo\": \"bar\"}")
	assertParseError(t, "out-of-range", "1e1000")
}

func TestVarTerms(t *testing.T) {
	assertParseOneTerm(t, "var", "foo", VarTerm("foo"))
	assertParseOneTerm(t, "var", "foo_bar", VarTerm("foo_bar"))
	assertParseOneTerm(t, "var", "foo0", VarTerm("foo0"))
	assertParseOneTerm(t, "import prefix", "imports", VarTerm("imports"))
	assertParseOneTerm(t, "not prefix", "not_foo", VarTerm("not_foo"))
	assertParseOneTerm(t, "package prefix", "packages", VarTerm("packages"))
	assertParseError(t, "non-var", "foo-bar")
	assertParseError(t, "non-var2", "foo-7")
	assertParseError(t, "not keyword", "not")
	assertParseError(t, "package keyword", "package")
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
	assertParseError(t, "missing component 1", "foo.")
	assertParseError(t, "missing component 2", "foo[].bar")
	assertParseError(t, "composite operand 1", "foo[[1,2,3]].bar")
	assertParseError(t, "composite operand 2", "foo[{1: 2}].bar")
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
}

func TestArrayWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "[foo, bar, 42]", ArrayTerm(VarTerm("foo"), VarTerm("bar"), IntNumberTerm(42)))
	assertParseOneTerm(t, "nested var elements", "[[foo, true], [null, bar], 42]", ArrayTerm(ArrayTerm(VarTerm("foo"), BooleanTerm(true)), ArrayTerm(NullTerm(), VarTerm("bar")), IntNumberTerm(42)))
}

func TestArrayFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "[foo, bar")
	assertParseError(t, "non-terminated 2", "[foo, bar, ")
	assertParseError(t, "missing element", "[foo, bar, ]")
	assertParseError(t, "missing separator", "[foo bar]")
	assertParseError(t, "missing start", "foo, bar, baz]")
}

func TestSetWithScalars(t *testing.T) {
	assertParseOneTerm(t, "number", "{1,2,3,4.5}", SetTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), FloatNumberTerm(4.5)))
	assertParseOneTerm(t, "bool", "{true, false, true}", SetTerm(BooleanTerm(true), BooleanTerm(false), BooleanTerm(true)))
	assertParseOneTerm(t, "string", "{\"foo\", \"bar\"}", SetTerm(StringTerm("foo"), StringTerm("bar")))
	assertParseOneTerm(t, "mixed", "{null, true, 42}", SetTerm(NullTerm(), BooleanTerm(true), IntNumberTerm(42)))
}

func TestSetWithVars(t *testing.T) {
	assertParseOneTerm(t, "var elements", "{foo, bar, 42}", SetTerm(VarTerm("foo"), VarTerm("bar"), IntNumberTerm(42)))
	assertParseOneTerm(t, "nested var elements", "{[foo, true], {null, bar}, set()}", SetTerm(ArrayTerm(VarTerm("foo"), BooleanTerm(true)), SetTerm(NullTerm(), VarTerm("bar")), SetTerm()))
}

func TestSetFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "set(")
	assertParseError(t, "non-terminated 2", "{foo, bar")
	assertParseError(t, "non-terminated 3", "{foo, bar, ")
	assertParseError(t, "missing element", "{foo, bar, }")
	assertParseError(t, "missing separator", "{foo bar}")
	assertParseError(t, "missing start", "foo, bar, baz}")
}

func TestEmptyComposites(t *testing.T) {
	assertParseOneTerm(t, "empty object", "{}", ObjectTerm())
	assertParseOneTerm(t, "emtpy array", "[]", ArrayTerm())
	assertParseOneTerm(t, "emtpy set", "set()", SetTerm())
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

	input := `[
		{"x": [a[i] | xs = [{"a": ["baz", j]} | q[p], p.a != "bar", j = "foo"],
					  xs[j].a[k] = "foo"]}
	]`

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
								&Expr{
									Terms: RefTerm(VarTerm("q"), VarTerm("p")),
								},
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

	assertParseOneExpr(t, "ne", "100 != 200", NotEqual.Expr(IntNumberTerm(100), IntNumberTerm(200)))
	assertParseOneExpr(t, "gt", "17.4 > \"hello\"", GreaterThan.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lt", "17.4 < \"hello\"", LessThan.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "gte", "17.4 >= \"hello\"", GreaterThanEq.Expr(FloatNumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lte", "17.4 <= \"hello\"", LessThanEq.Expr(FloatNumberTerm(17.4), StringTerm("hello")))

	left2 := ArrayTerm(ObjectTerm(Item(FloatNumberTerm(14.2), BooleanTerm(true)), Item(StringTerm("a"), NullTerm())))
	right2 := ObjectTerm(Item(VarTerm("foo"), ObjectTerm(Item(RefTerm(VarTerm("a"), StringTerm("b"), IntNumberTerm(0)), ArrayTerm(IntNumberTerm(10))))))
	assertParseOneExpr(t, "composites", "[{14.2: true, \"a\": null}] != {foo: {a.b[0]: [10]}}", NotEqual.Expr(left2, right2))
}

func TestMiscBuiltinExpr(t *testing.T) {
	xyz := VarTerm("xyz")
	assertParseOneExpr(t, "empty", "xyz()", NewBuiltinExpr(xyz))
	assertParseOneExpr(t, "single", "xyz(abc)", NewBuiltinExpr(xyz, VarTerm("abc")))
	assertParseOneExpr(t, "multiple", "xyz(abc, {\"one\": [1,2,3]})", NewBuiltinExpr(xyz, VarTerm("abc"), ObjectTerm(Item(StringTerm("one"), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3))))))
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
	assertParseOneExprNegated(t, "misc. builtin", "not sorted(x[y].z[a])", NewBuiltinExpr(VarTerm("sorted"), ref1))
}

func TestPackage(t *testing.T) {
	ref1 := RefTerm(DefaultRootDocument, StringTerm("foo"))
	assertParsePackage(t, "single", "package foo", &Package{Path: ref1.Value.(Ref)})
	ref2 := RefTerm(DefaultRootDocument, StringTerm("f00"), StringTerm("bar_baz"), StringTerm("qux"))
	assertParsePackage(t, "multiple", "package f00.bar_baz.qux", &Package{Path: ref2.Value.(Ref)})
	ref3 := RefTerm(DefaultRootDocument, StringTerm("foo"), StringTerm("bar baz"))
	assertParsePackage(t, "space", "package foo[\"bar baz\"]", &Package{Path: ref3.Value.(Ref)})
	assertParseError(t, "non-ground ref", "package foo[x]")
	assertParseError(t, "non-string value", "package foo.bar[42].baz")
}

func TestImport(t *testing.T) {
	assertParseImport(t, "single", "import foo", &Import{Path: VarTerm("foo")})
	ref := RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz"))
	assertParseImport(t, "multiple", "import foo.bar.baz", &Import{Path: ref})
	assertParseImport(t, "single alias", "import foo as bar", &Import{Path: VarTerm("foo"), Alias: Var("bar")})
	assertParseImport(t, "multiple alias", "import foo.bar.baz as qux", &Import{Path: ref, Alias: Var("qux")})
	ref2 := RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("white space"))
	assertParseImport(t, "white space", "import foo.bar[\"white space\"]", &Import{Path: ref2})
	assertParseError(t, "non-ground ref", "import foo[x]")
	assertParseError(t, "non-string", "import foo[0]")
}

func TestRule(t *testing.T) {

	assertParseRule(t, "identity", "p = true :- true", &Rule{
		Name:  Var("p"),
		Value: BooleanTerm(true),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "set", "p[x] :- x = 42", &Rule{
		Name: Var("p"),
		Key:  VarTerm("x"),
		Body: NewBody(
			Equality.Expr(VarTerm("x"), IntNumberTerm(42)),
		),
	})

	assertParseRule(t, "object", "p[x] = y :- x = 42, y = \"hello\"", &Rule{
		Name:  Var("p"),
		Key:   VarTerm("x"),
		Value: VarTerm("y"),
		Body: NewBody(
			Equality.Expr(VarTerm("x"), IntNumberTerm(42)),
			Equality.Expr(VarTerm("y"), StringTerm("hello")),
		),
	})

	assertParseRule(t, "constant composite", "p = [{\"foo\": [1,2,3,4]}] :- true", &Rule{
		Name: Var("p"),
		Value: ArrayTerm(
			ObjectTerm(Item(StringTerm("foo"), ArrayTerm(IntNumberTerm(1), IntNumberTerm(2), IntNumberTerm(3), IntNumberTerm(4)))),
		),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "true", "p :- true", &Rule{
		Name:  Var("p"),
		Value: BooleanTerm(true),
		Body: NewBody(
			&Expr{Terms: BooleanTerm(true)},
		),
	})

	assertParseRule(t, "composites in head", `p[[{"x": [a,b]}]] :- a = 1, b = 2`, &Rule{
		Name: Var("p"),
		Key: ArrayTerm(
			ObjectTerm(
				Item(StringTerm("x"), ArrayTerm(VarTerm("a"), VarTerm("b"))),
			),
		),
		Body: NewBody(
			Equality.Expr(VarTerm("a"), IntNumberTerm(1)),
			Equality.Expr(VarTerm("b"), IntNumberTerm(2)),
		),
	})

	assertParseRule(t, "refs in head", "p = data.foo[x] :- x = 1", &Rule{
		Name: Var("p"),
		Value: &Term{
			Value: MustParseRef("data.foo[x]"),
		},
		Body: MustParseBody("x = 1"),
	})

	assertParseRule(t, "refs in head", "p[data.foo[x]] :- true", &Rule{
		Name: Var("p"),
		Key: &Term{
			Value: MustParseRef("data.foo[x]"),
		},
		Body: MustParseBody("true"),
	})

	assertParseRule(t, "refs in head", "p[data.foo[x]] = data.bar[y] :- true", &Rule{
		Name: Var("p"),
		Key: &Term{
			Value: MustParseRef("data.foo[x]"),
		},
		Value: &Term{
			Value: MustParseRef("data.bar[y]"),
		},
		Body: MustParseBody("true"),
	})

	assertParseErrorEquals(t, "object composite key", "p[[x,y]] = z :- true", "head of object rule must have string, var, or ref key ([x, y] is not allowed)")
	assertParseErrorEquals(t, "closure in key", "p[[1 | true]] :- true", "head cannot contain closures ([1 | true] appears in key)")
	assertParseErrorEquals(t, "closure in value", "p = [[1 | true]] :- true", "head cannot contain closures ([1 | true] appears in value)")

	// TODO(tsandall): improve error checking here. This is a common mistake
	// and the current error message is not very good. Need to investigate if the
	// parser can be improved.
	assertParseError(t, "dangling comma", "p :- true, false,")
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

	testModule := `
    package a.b.c

    import e.f as g  # end of line
    import h

    # by itself

    p[x] = y :- y = "foo",
        # inside a rule
        x = "bar",
        x != y,
        q[x]

    import xyz.abc

    q # interruptting
    [a] # the head of a rule
    :- m = [1,2,
    3],
    a = m[i]

	r[x] :- x = [ a | # inside comprehension
					  a = z[i],
	                  b[i].a = a ]
    `

	assertParseModule(t, "module comments", testModule, &Module{
		Package: MustParseStatement("package a.b.c").(*Package),
		Imports: []*Import{
			MustParseStatement("import e.f as g").(*Import),
			MustParseStatement("import h").(*Import),
			MustParseStatement("import xyz.abc").(*Import),
		},
		Rules: []*Rule{
			MustParseStatement("p[x] = y :- y = \"foo\", x = \"bar\", x != y, q[x]").(*Rule),
			MustParseStatement("q[a] :- m = [1,2,3], a = m[i]").(*Rule),
			MustParseStatement("r[x] :- x = [a | a = z[i], b[i].a = a]").(*Rule),
		},
	})
}

func TestExample(t *testing.T) {
	assertParseModule(t, "example module", testModule, &Module{
		Package: MustParseStatement("package opa.examples").(*Package),
		Imports: []*Import{
			MustParseStatement("import data.servers").(*Import),
			MustParseStatement("import data.networks").(*Import),
			MustParseStatement("import data.ports").(*Import),
		},
		Rules: []*Rule{
			MustParseStatement(`violations[server] :-
                         server = servers[i],
                         server.protocols[j] = "http",
                         public_servers[server]`).(*Rule),
			MustParseStatement(`public_servers[server] :-
                         server = servers[i],
                         server.ports[j] = ports[k].id,
                         ports[k].networks[l] = networks[m].id,
                         networks[m].public = true`).(*Rule),
		},
	})
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
	if expr.Location.Row != 9 {
		t.Errorf("Expected row of %v to be 9 but got: %v", expr, expr.Location.Row)
	}
	if expr.Location.File != "test" {
		t.Errorf("Expected file of %v to be test but got: %v", expr, expr.Location.File)
	}
}

func TestRuleFromBody(t *testing.T) {
	testModule := `
    package a.b.c

    pi = 3.14159
    # intersperse a regular rule
    p[x] :- x = 1
    greeting = "hello"
    cores = [{0: 1}, {1: 2}]
	wrapper = cores[0][1]
	pi = [3, 1, 4, x, y, z]
    `

	assertParseModule(t, "rules from bodies", testModule, &Module{
		Package: MustParseStatement("package a.b.c").(*Package),
		Rules: []*Rule{
			MustParseStatement("pi = 3.14159 :- true").(*Rule),
			MustParseStatement("p[x] :- x = 1").(*Rule),
			MustParseStatement("greeting = \"hello\" :- true").(*Rule),
			MustParseStatement("cores = [{0: 1}, {1: 2}] :- true").(*Rule),
			MustParseStatement("wrapper = cores[0][1] :- true").(*Rule),
			MustParseStatement("pi = [3, 1, 4, x, y, z] :- true").(*Rule),
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

	assertParseModuleError(t, "multiple expressions", multipleExprs)
	assertParseModuleError(t, "non-equality", nonEquality)
	assertParseModuleError(t, "non-var name", nonVarName)
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
			Item(StringTerm("a"), RefTerm(VarTerm("a"), VarTerm("$1"))),
		),
		VarTerm("$0"),
		ObjectTerm(
			Item(StringTerm("b"), VarTerm("$2")),
		),
	))

	assertParseOneExpr(t, "expr", `eq(_, [a[_]])`, Equality.Expr(
		VarTerm("$0"),
		ArrayTerm(
			RefTerm(VarTerm("a"), VarTerm("$1")),
		)))

	assertParseOneExpr(t, "comprehension", "eq(_, [x | a = a[_]])", Equality.Expr(
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
}

func TestNoMatchError(t *testing.T) {
	mod := `package test

	p :- true,
		 1 != 0, // <-- parse error: no match`

	_, err := ParseModule("foo.rego", mod)

	expected := "1 error occurred: foo.rego:4: no match found, unexpected '/'"

	if err.Error() != expected {
		t.Fatalf("Bad parse error, expected %v but got: %v", expected, err)
	}

	mod = `package test

	p :- true// <-- parse error: no match`

	_, err = ParseModule("foo.rego", mod)

	loc := NewLocation(nil, "foo.rego", 3, 12)

	if !reflect.DeepEqual(err.(Errors)[0].Location, loc) {
		t.Fatalf("Expected %v but got: %v", loc, err)
	}
}

func assertParse(t *testing.T, msg string, input string, correct func([]interface{})) {
	p, err := ParseStatements("", input)
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, input, err)
		return
	}
	correct(p)
}

// TODO(tsandall): add assertions to check that error message is as expected
func assertParseError(t *testing.T, msg string, input string) {
	p, err := ParseStatement(input)
	if err == nil {
		t.Errorf("Error on test %s: expected parse error: %v (parsed)", msg, p)
		return
	}
}

func assertParseErrorEquals(t *testing.T, msg string, input string, expected string) {
	p, err := ParseStatement(input)
	if err == nil {
		t.Errorf("Error on test %s: expected parse error: %v (parsed)", msg, p)
		return
	}
	result := err.Error()
	// error occurred: <line>:<col>: <message>
	parts := strings.SplitN(result, ":", 4)
	result = strings.TrimSpace(parts[len(parts)-1])

	if result != expected {
		t.Errorf("Error on test %s: expected parse error to equal %v but got: %v", msg, expected, result)
	}
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
