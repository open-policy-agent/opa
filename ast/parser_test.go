// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
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
	assertParseOneTerm(t, "integer", "53", NumberTerm(53))
	assertParseOneTerm(t, "integer2", "-53", NumberTerm(-53))
	assertParseOneTerm(t, "float", "16.7", NumberTerm(16.7))
	assertParseOneTerm(t, "float2", "-16.7", NumberTerm(-16.7))
	assertParseOneTerm(t, "exponent", "6e7", NumberTerm(6e7))
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
	assertParseError(t, "non-var", "foo-bar")
	assertParseError(t, "non-var2", "foo-7")
	for _, v := range Keywords {
		assertParseError(t, "keyword", v)
	}
}

func TestRefTerms(t *testing.T) {
	assertParseOneTerm(t, "constants", "foo.bar.baz", RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")))
	assertParseOneTerm(t, "constants 2", "foo.bar[0].baz", RefTerm(VarTerm("foo"), StringTerm("bar"), NumberTerm(0), StringTerm("baz")))
	assertParseOneTerm(t, "variables", "foo.bar[0].baz[i]", RefTerm(VarTerm("foo"), StringTerm("bar"), NumberTerm(0), StringTerm("baz"), VarTerm("i")))
	assertParseOneTerm(t, "spaces", "foo[\"white space\"].bar", RefTerm(VarTerm("foo"), StringTerm("white space"), StringTerm("bar")))
	assertParseError(t, "missing component 1", "foo.")
	assertParseError(t, "missing component 2", "foo[].bar")
	assertParseError(t, "composite operand 1", "foo[[1,2,3]].bar")
	assertParseError(t, "composite operand 2", "foo[{1: 2}].bar")
	// TODO(tsandall): this may be allowed some day
	assertParseError(t, "nested refs", "foo[baz.qux].bar")
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

func TestObjectFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "{foo: bar, baz: [], qux: corge")
	assertParseError(t, "non-terminated 2", "{foo: bar, baz: [], qux: ")
	assertParseError(t, "non-terminated 3", "{foo: bar, baz: [], qux ")
	assertParseError(t, "non-terminated 4", "{foo: bar, baz: [], ")
	assertParseError(t, "missing separator", "{foo: bar baz: []}")
	assertParseError(t, "missing start", "foo: bar, baz: [], qux: corge}")
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

func TestArrayFail(t *testing.T) {
	assertParseError(t, "non-terminated 1", "[foo, bar")
	assertParseError(t, "non-terminated 2", "[foo, bar, ")
	assertParseError(t, "missing element", "[foo, bar, ]")
	assertParseError(t, "missing separator", "[foo bar]")
	assertParseError(t, "missing start", "foo, bar, baz]")
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

func TestInfixExpr(t *testing.T) {
	assertParseOneExpr(t, "scalars 1", "true = false", NewBuiltinExpr(VarTerm("="), BooleanTerm(true), BooleanTerm(false)))
	assertParseOneExpr(t, "scalars 2", "3.14 = null", NewBuiltinExpr(VarTerm("="), NumberTerm(3.14), NullTerm()))
	assertParseOneExpr(t, "scalars 3", "42 = \"hello world\"", NewBuiltinExpr(VarTerm("="), NumberTerm(42), StringTerm("hello world")))
	assertParseOneExpr(t, "vars 1", "hello = world", NewBuiltinExpr(VarTerm("="), VarTerm("hello"), VarTerm("world")))
	assertParseOneExpr(t, "vars 2", "42 = hello", NewBuiltinExpr(VarTerm("="), NumberTerm(42), VarTerm("hello")))

	ref1 := RefTerm(VarTerm("foo"), NumberTerm(0), StringTerm("bar"), VarTerm("x"))
	ref2 := RefTerm(VarTerm("baz"), BooleanTerm(false), StringTerm("qux"), StringTerm("hello"))
	assertParseOneExpr(t, "refs 1", "foo[0].bar[x] = baz[false].qux[\"hello\"]", NewBuiltinExpr(VarTerm("="), ref1, ref2))

	left1 := ObjectTerm(Item(VarTerm("a"), ArrayTerm(ref1)))
	right1 := ArrayTerm(ObjectTerm(Item(NumberTerm(42), BooleanTerm(true))))
	assertParseOneExpr(t, "composites", "{a: [foo[0].bar[x]]} = [{42: true}]", NewBuiltinExpr(VarTerm("="), left1, right1))

	assertParseOneExpr(t, "ne", "100 != 200", NewBuiltinExpr(VarTerm("!="), NumberTerm(100), NumberTerm(200)))
	assertParseOneExpr(t, "gt", "17.4 > \"hello\"", NewBuiltinExpr(VarTerm(">"), NumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lt", "17.4 < \"hello\"", NewBuiltinExpr(VarTerm("<"), NumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "gte", "17.4 >= \"hello\"", NewBuiltinExpr(VarTerm(">="), NumberTerm(17.4), StringTerm("hello")))
	assertParseOneExpr(t, "lte", "17.4 <= \"hello\"", NewBuiltinExpr(VarTerm("<="), NumberTerm(17.4), StringTerm("hello")))

	left2 := ArrayTerm(ObjectTerm(Item(NumberTerm(14.2), BooleanTerm(true)), Item(StringTerm("a"), NullTerm())))
	right2 := ObjectTerm(Item(VarTerm("foo"), ObjectTerm(Item(RefTerm(VarTerm("a"), StringTerm("b"), NumberTerm(0)), ArrayTerm(NumberTerm(10))))))
	assertParseOneExpr(t, "composites", "[{14.2: true, \"a\": null}] != {foo: {a.b[0]: [10]}}", NewBuiltinExpr(VarTerm("!="), left2, right2))
}

func TestMiscBuiltinExpr(t *testing.T) {
	xyz := VarTerm("xyz")
	assertParseOneExpr(t, "empty", "xyz()", NewBuiltinExpr(xyz))
	assertParseOneExpr(t, "single", "xyz(abc)", NewBuiltinExpr(xyz, VarTerm("abc")))
	assertParseOneExpr(t, "multiple", "xyz(abc, {\"one\": [1,2,3]})", NewBuiltinExpr(xyz, VarTerm("abc"), ObjectTerm(Item(StringTerm("one"), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3))))))
}

func TestNegatedExpr(t *testing.T) {
	assertParseOneTermNegated(t, "scalars 1", "not true", BooleanTerm(true))
	assertParseOneTermNegated(t, "scalars 2", "not \"hello\"", StringTerm("hello"))
	assertParseOneTermNegated(t, "scalars 3", "not 100", NumberTerm(100))
	assertParseOneTermNegated(t, "scalars 4", "not null", NullTerm())
	assertParseOneTermNegated(t, "var", "not x", VarTerm("x"))
	assertParseOneTermNegated(t, "ref", "not x[y].z", RefTerm(VarTerm("x"), VarTerm("y"), StringTerm("z")))
	assertParseOneExprNegated(t, "vars", "not x = y", NewBuiltinExpr(VarTerm("="), VarTerm("x"), VarTerm("y")))

	ref1 := RefTerm(VarTerm("x"), VarTerm("y"), StringTerm("z"), VarTerm("a"))

	assertParseOneExprNegated(t, "membership", "not x[y].z[a] = \"b\"", NewBuiltinExpr(VarTerm("="), ref1, StringTerm("b")))
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
}

func TestRule(t *testing.T) {

	assertParseRule(t, "identity", "p = true :- true", &Rule{
		Name:  Var("p"),
		Value: BooleanTerm(true),
		Body: []*Expr{
			&Expr{Terms: BooleanTerm(true)},
		},
	})

	assertParseRule(t, "set", "p[x] :- x = 42", &Rule{
		Name: Var("p"),
		Key:  VarTerm("x"),
		Body: []*Expr{
			NewBuiltinExpr(VarTerm("="), VarTerm("x"), NumberTerm(42)),
		},
	})

	assertParseRule(t, "object", "p[x] = y :- x = 42, y = \"hello\"", &Rule{
		Name:  Var("p"),
		Key:   VarTerm("x"),
		Value: VarTerm("y"),
		Body: []*Expr{
			NewBuiltinExpr(VarTerm("="), VarTerm("x"), NumberTerm(42)),
			NewBuiltinExpr(VarTerm("="), VarTerm("y"), StringTerm("hello")),
		},
	})

	assertParseRule(t, "constant composite", "p = [{\"foo\": [1,2,3,4]}] :- true", &Rule{
		Name: Var("p"),
		Value: ArrayTerm(
			ObjectTerm(Item(StringTerm("foo"), ArrayTerm(NumberTerm(1), NumberTerm(2), NumberTerm(3), NumberTerm(4)))),
		),
		Body: []*Expr{
			&Expr{Terms: BooleanTerm(true)},
		},
	})

	assertParseRule(t, "true", "p :- true", &Rule{
		Name:  Var("p"),
		Value: BooleanTerm(true),
		Body: []*Expr{
			&Expr{Terms: BooleanTerm(true)},
		},
	})

	assertParseError(t, "constant key", "p[100] :- true")
	assertParseError(t, "composite key", "p[[1,2,x]] :- x = true")
	assertParseError(t, "dangling comma", "p :- true, false,")
}

func TestEmptyModule(t *testing.T) {
	r, err := ParseModule("    ")
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

func TestConstantRules(t *testing.T) {
	testModule := `
    package a.b.c

    pi = 3.14159

    # intersperse a regular rule
    p[x] :- x = 1

    greeting = "hello"

    cores = [{0: 1}, {1: 2}]
    `

	assertParseModule(t, "constant rules", testModule, &Module{
		Package: MustParseStatement("package a.b.c").(*Package),
		Rules: []*Rule{
			MustParseStatement("pi = 3.14159 :- true").(*Rule),
			MustParseStatement("p[x] :- x = 1").(*Rule),
			MustParseStatement("greeting = \"hello\" :- true").(*Rule),
			MustParseStatement("cores = [{0: 1}, {1: 2}] :- true").(*Rule),
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

	ungroundValue := `
    package a.b.c

    pi = [3, 1, 4, x, y, z]
    `

	assertParseError(t, "multiple expressions", multipleExprs)
	assertParseError(t, "non-equality", nonEquality)
	assertParseError(t, "non-var name", nonVarName)
	assertParseError(t, "unground value", ungroundValue)
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

	assertParseOneExpr(t, "expr", `_ = [a[_]]`, &Expr{
		Terms: []*Term{
			VarTerm("="),
			VarTerm("$0"),
			ArrayTerm(
				RefTerm(VarTerm("a"), VarTerm("$1")),
			),
		},
	})
}

func assertParse(t *testing.T, msg string, input string, correct func([]interface{})) {
	p, err := ParseStatements(input)
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

func assertParseImport(t *testing.T, msg string, input string, correct *Import) {
	assertParseOne(t, msg, input, func(parsed interface{}) {
		imp := parsed.(*Import)
		if !imp.Equal(correct) {
			t.Errorf("Error on test %s: imports not equal: %v (parsed), %v (correct)", msg, imp, correct)
		}
	})
}

func assertParseModule(t *testing.T, msg string, input string, correct *Module) {

	m, err := ParseModule(input)
	if err != nil {
		t.Errorf("Error on test %s: parse error on %s: %s", msg, input, err)
		return
	}

	if !m.Equal(correct) {
		t.Errorf("Error on test %s: modules not equal: %v (parsed), %v (correct)", msg, m, correct)
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
			t.Errorf("Error on test %s: expressions not equal: %v (parsed), %v (correct)", msg, expr, correct)
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
