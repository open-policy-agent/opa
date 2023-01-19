// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast/location"
	"github.com/open-policy-agent/opa/util"
)

func TestModuleJSONRoundTrip(t *testing.T) {

	mod, err := ParseModuleWithOpts("test.rego", `package a.b.c

import future.keywords
import data.x.y as z
import data.u.i

p = [1, 2, {"foo": 3.14}] { r[x] = 1; not q[x] }
r[y] = v { i[1] = y; v = i[2] }
q[x] { a = [true, false, null, {"x": [1, 2, 3]}]; a[i] = x }
t = true { xs = [{"x": a[i].a} | a[i].n = "bob"; b[x]] }
big = 1e+1000 { true }
odd = -0.1 { true }
s = {1, 2, 3} { true }
s = set() { false }
empty_obj = true { {} }
empty_arr = true { [] }
empty_set = true { set() }
using_with = true { x = data.foo + 1 with input.foo as bar }
x = 2 { input = null }
default allow = true
f(x) = y { y = x }
a = true { xs = {a: b | input.y[a] = "foo"; b = input.z["bar"]} }
b = true { xs = {{"x": a[i].a} | a[i].n = "bob"; b[x]} }
call_values { f(x) != g(x) }
assigned := 1
rule.having.ref.head[1] = x if x := 2

# METADATA
# scope: rule
metadata := 7
`, ParserOptions{ProcessAnnotation: true})

	if err != nil {
		t.Fatal(err)
	}

	bs, err := json.Marshal(mod)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	roundtrip := &Module{}

	err = util.UnmarshalJSON(bs, roundtrip)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !roundtrip.Equal(mod) {
		t.Fatalf("Expected roundtripped module to be equal to original:\nExpected:\n\n%v\n\nGot:\n\n%v\n", mod, roundtrip)
	}

	if mod.Rules[3].Path().String() != "data.a.b.c.t" {
		t.Fatal("expected path data.a.b.c.t for 4th rule in module but got:", mod.Rules[3].Path())
	}

	if len(roundtrip.Annotations) != 1 {
		t.Fatal("expected exactly one annotation")
	}
}

func TestBodyEmptyJSON(t *testing.T) {
	var body Body
	bs := util.MustMarshalJSON(body)
	if string(bs) != "[]" {
		t.Fatalf("Unexpected JSON value for empty body")
	}
	body = Body{}
	bs = util.MustMarshalJSON(body)
	if string(bs) != "[]" {
		t.Fatalf("Unexpected JSON value for empty body")
	}
}

func TestPackageEquals(t *testing.T) {
	pkg1 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	pkg2 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	pkg3 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("qux"), StringTerm("baz")).Value.(Ref)}
	assertPackagesEqual(t, pkg1, pkg1)
	assertPackagesEqual(t, pkg1, pkg2)
	assertPackagesNotEqual(t, pkg1, pkg3)
	assertPackagesNotEqual(t, pkg2, pkg3)
}

func TestPackageString(t *testing.T) {
	pkg1 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	result1 := pkg1.String()
	expected1 := `package bar.baz`
	if result1 != expected1 {
		t.Errorf("Expected %v but got %v", expected1, result1)
	}

	var nilPkg *Package

	expNil := "<illegal nil package>"
	if nilPkg.String() != expNil {
		t.Fatal("Unexpected package repr:", nilPkg.String())
	}

	badPathPkg := &Package{
		Path: RefTerm(VarTerm("x")).Value.(Ref),
	}

	expBadPath := "package <illegal path \"x\">"
	if badPathPkg.String() != expBadPath {
		t.Fatal("Unexpected package repr:", badPathPkg.String())
	}

}

func TestImportEquals(t *testing.T) {
	imp1 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp11 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp2 := &Import{Path: VarTerm("foo")}
	imp3 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux")), Alias: Var("corge")}
	imp33 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux")), Alias: Var("corge")}
	imp4 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux"))}
	assertImportsEqual(t, imp1, imp1)
	assertImportsEqual(t, imp1, imp11)
	assertImportsEqual(t, imp3, imp3)
	assertImportsEqual(t, imp3, imp33)
	imps := []*Import{imp1, imp2, imp3, imp4}
	for i := range imps {
		for j := range imps {
			if i != j {
				assertImportsNotEqual(t, imps[i], imps[j])
			}
		}
	}
}

func TestImportName(t *testing.T) {
	imp1 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp2 := &Import{Path: VarTerm("foo")}
	imp3 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux")), Alias: Var("corge")}
	imp4 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux"))}
	imp5 := &Import{Path: DefaultRootDocument}
	expected := []Var{
		"bar",
		"foo",
		"corge",
		"qux",
		"data",
	}
	tests := []*Import{
		imp1, imp2, imp3, imp4, imp5,
	}
	for i := range tests {
		result := tests[i].Name()
		if !result.Equal(expected[i]) {
			t.Errorf("Expected %v but got: %v", expected[i], result)
		}
	}
}

func TestImportString(t *testing.T) {
	imp1 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp2 := &Import{Path: VarTerm("foo")}
	imp3 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux")), Alias: Var("corge")}
	imp4 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux"))}
	assertImportToString(t, imp1, "import foo as bar")
	assertImportToString(t, imp2, "import foo")
	assertImportToString(t, imp3, "import bar.baz.qux as corge")
	assertImportToString(t, imp4, "import bar.baz.qux")
}

func TestExprEquals(t *testing.T) {

	// Scalars
	expr1 := &Expr{Terms: BooleanTerm(true)}
	expr2 := &Expr{Terms: BooleanTerm(true)}
	expr3 := &Expr{Terms: StringTerm("true")}
	assertExprEqual(t, expr1, expr2)
	assertExprNotEqual(t, expr1, expr3)

	// Vars, refs, and composites
	ref1 := RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"))
	ref2 := RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"))
	obj1 := ObjectTerm(Item(ref1, ArrayTerm(IntNumberTerm(1), NullTerm())))
	obj2 := ObjectTerm(Item(ref2, ArrayTerm(IntNumberTerm(1), NullTerm())))
	obj3 := ObjectTerm(Item(ref2, ArrayTerm(StringTerm("1"), NullTerm())))
	expr10 := &Expr{Terms: obj1}
	expr11 := &Expr{Terms: obj2}
	expr12 := &Expr{Terms: obj3}
	assertExprEqual(t, expr10, expr11)
	assertExprNotEqual(t, expr10, expr12)

	// Builtins and negation
	expr20 := &Expr{
		Negated: true,
		Terms:   []*Term{StringTerm("="), VarTerm("x"), ref1},
	}
	expr21 := &Expr{
		Negated: true,
		Terms:   []*Term{StringTerm("="), VarTerm("x"), ref1},
	}
	expr22 := &Expr{
		Negated: false,
		Terms:   []*Term{StringTerm("="), VarTerm("x"), ref1},
	}
	expr23 := &Expr{
		Negated: true,
		Terms:   []*Term{StringTerm("="), VarTerm("y"), ref1},
	}
	assertExprEqual(t, expr20, expr21)
	assertExprNotEqual(t, expr20, expr22)
	assertExprNotEqual(t, expr20, expr23)

	// Modifiers
	expr30 := &Expr{
		Terms: MustParseTerm("data.foo.bar"),
		With: []*With{
			{
				Target: MustParseTerm("input"),
				Value:  MustParseTerm("bar"),
			},
		},
	}

	expr31 := &Expr{
		Terms: MustParseTerm("data.foo.bar"),
		With: []*With{
			{
				Target: MustParseTerm("input"),
				Value:  MustParseTerm("bar"),
			},
		},
	}

	expr32 := &Expr{
		Terms: MustParseTerm("data.foo.bar"),
		With: []*With{
			{
				Target: MustParseTerm("input.foo"),
				Value:  MustParseTerm("baz"),
			},
		},
	}

	assertExprEqual(t, expr30, expr31)
	assertExprNotEqual(t, expr30, expr32)
}

func TestBodyIsGround(t *testing.T) {
	if MustParseBody(`a.b[0] = 1; a = [1, 2, x]`).IsGround() {
		t.Errorf("Expected body to be non-ground")
	}
}

func TestExprString(t *testing.T) {
	expr1 := &Expr{
		Terms: RefTerm(VarTerm("q"), StringTerm("r"), VarTerm("x")),
	}
	expr2 := &Expr{
		Negated: true,
		Terms:   RefTerm(VarTerm("q"), StringTerm("r"), VarTerm("x")),
	}
	expr3 := Equality.Expr(StringTerm("a"), FloatNumberTerm(17.1))
	expr4 := NotEqual.Expr(
		ObjectTerm(Item(VarTerm("foo"), ArrayTerm(
			IntNumberTerm(1), RefTerm(VarTerm("a"), StringTerm("b")),
		))),
		BooleanTerm(false),
	)
	expr5 := &Expr{
		Terms: BooleanTerm(true),
		With: []*With{
			{
				Target: VarTerm("foo"),
				Value:  VarTerm("bar"),
			},
			{
				Target: VarTerm("baz"),
				Value:  VarTerm("qux"),
			},
		},
	}
	expr6 := Plus.Expr(
		IntNumberTerm(1),
		IntNumberTerm(2),
		IntNumberTerm(3),
	)
	expr7 := Count.Expr(
		StringTerm("foo"),
		VarTerm("x"),
	)
	expr8 := &Expr{
		Terms: []*Term{
			RefTerm(VarTerm("data"), StringTerm("test"), StringTerm("f")),
			IntNumberTerm(1),
			VarTerm("x"),
		},
	}
	expr9 := Contains.Expr(StringTerm("foo.bar"), StringTerm("."))
	expr10 := Member.Expr(StringTerm("foo"), VarTerm("xs"))
	expr11 := MemberWithKey.Expr(VarTerm("x"), StringTerm("foo"), VarTerm("xs"))
	assertExprString(t, expr1, "q.r[x]")
	assertExprString(t, expr2, "not q.r[x]")
	assertExprString(t, expr3, "\"a\" = 17.1")
	assertExprString(t, expr4, "neq({foo: [1, a.b]}, false)")
	assertExprString(t, expr5, "true with foo as bar with baz as qux")
	assertExprString(t, expr6, "plus(1, 2, 3)")
	assertExprString(t, expr7, "count(\"foo\", x)")
	assertExprString(t, expr8, "data.test.f(1, x)")
	assertExprString(t, expr9, `contains("foo.bar", ".")`)
	assertExprString(t, expr10, `internal.member_2("foo", xs)`)
	assertExprString(t, expr11, `internal.member_3(x, "foo", xs)`)
}

func TestExprBadJSON(t *testing.T) {

	assert := func(js string, exp error) {
		expr := Expr{}
		err := util.UnmarshalJSON([]byte(js), &expr)
		if !reflect.DeepEqual(exp, err) {
			t.Errorf("For %v Expected %v but got: %v", js, exp, err)
		}
	}

	js := `
	{
		"negated": 100,
		"terms": {
			"value": "foo",
			"type": "string"
		},
		"index": 0
	}
	`

	exp := fmt.Errorf("ast: unable to unmarshal negated field with type: json.Number (expected true or false)")
	assert(js, exp)

	js = `
	{
		"terms": [
			"foo"
		],
		"index": 0
	}
	`
	exp = fmt.Errorf("ast: unable to unmarshal term")
	assert(js, exp)

	js = `
	{
		"terms": "bad value",
		"index": 0
	}
	`
	exp = fmt.Errorf(`ast: unable to unmarshal terms field with type: string (expected {"value": ..., "type": ...} or [{"value": ..., "type": ...}, ...])`)
	assert(js, exp)

	js = `
	{
		"terms": {"value": "foo", "type": "string"}
	}`
	exp = fmt.Errorf("ast: unable to unmarshal index field with type: <nil> (expected integer)")
	assert(js, exp)
}

func TestExprEveryCopy(t *testing.T) {
	opts := ParserOptions{AllFutureKeywords: true}
	newEvery := func() *Expr {
		return MustParseBodyWithOpts(
			`every k, v in [1,2,3] { true }`, opts,
		)[0]
	}
	e0 := newEvery()
	e1 := e0.Copy()
	e1.Terms.(*Every).Body = NewBody(NewExpr(BooleanTerm(false)))
	if exp := newEvery(); exp.Compare(e0) != 0 {
		t.Errorf("expected e0 unchanged (%v), found %v", exp, e0)
	}
}

func TestRuleHeadJSON(t *testing.T) {
	// NOTE(sr): we may get to see Rule objects that aren't the result of parsing, but
	// fed as-is into the compiler. We need to be able to make sense of their refs, too.
	head := Head{
		Name: Var("allow"),
	}

	rule := Rule{
		Head: &head,
	}
	bs, err := json.Marshal(&rule)
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := `{"body":[],"head":{"name":"allow","ref":[{"type":"var","value":"allow"}]}}`, string(bs); act != exp {
		t.Errorf("expected %q, got %q", exp, act)
	}

	var readRule Rule
	if err := json.Unmarshal(bs, &readRule); err != nil {
		t.Fatal(err)
	}
	if exp, act := 1, len(readRule.Head.Reference); act != exp {
		t.Errorf("expected unmarshalled rule to have Reference, got %v", readRule.Head.Reference)
	}
	bs0, err := json.Marshal(&readRule)
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := string(bs), string(bs0); exp != act {
		t.Errorf("expected json repr to match %q, got %q", exp, act)
	}

	var readAgainRule Rule
	if err := json.Unmarshal(bs, &readAgainRule); err != nil {
		t.Fatal(err)
	}
	if !readAgainRule.Equal(&readRule) {
		t.Errorf("expected roundtripped rule reference to match %v, got %v", readRule.Head.Reference, readAgainRule.Head.Reference)
	}
}

func TestRuleHeadEquals(t *testing.T) {
	assertHeadsEqual(t, &Head{}, &Head{})

	// Same name/ref/key/value
	assertHeadsEqual(t, &Head{Name: Var("p")}, &Head{Name: Var("p")})
	assertHeadsEqual(t, &Head{Reference: Ref{VarTerm("p"), StringTerm("r")}}, &Head{Reference: Ref{VarTerm("p"), StringTerm("r")}}) // TODO: string for first section
	assertHeadsEqual(t, &Head{Key: VarTerm("x")}, &Head{Key: VarTerm("x")})
	assertHeadsEqual(t, &Head{Value: VarTerm("x")}, &Head{Value: VarTerm("x")})
	assertHeadsEqual(t, &Head{Args: []*Term{VarTerm("x"), VarTerm("y")}}, &Head{Args: []*Term{VarTerm("x"), VarTerm("y")}})

	// Different name/ref/key/value
	assertHeadsNotEqual(t, &Head{Name: Var("p")}, &Head{Name: Var("q")})
	assertHeadsNotEqual(t, &Head{Reference: Ref{VarTerm("p")}}, &Head{Reference: Ref{VarTerm("q")}}) // TODO: string for first section
	assertHeadsNotEqual(t, &Head{Key: VarTerm("x")}, &Head{Key: VarTerm("y")})
	assertHeadsNotEqual(t, &Head{Value: VarTerm("x")}, &Head{Value: VarTerm("y")})
	assertHeadsNotEqual(t, &Head{Args: []*Term{VarTerm("x"), VarTerm("z")}}, &Head{Args: []*Term{VarTerm("x"), VarTerm("y")}})
}

func TestRuleBodyEquals(t *testing.T) {

	true1 := &Expr{Terms: []*Term{BooleanTerm(true)}}
	true2 := &Expr{Terms: []*Term{BooleanTerm(true)}}
	false1 := &Expr{Terms: []*Term{BooleanTerm(false)}}
	head := NewHead(Var("p"))

	ruleTrue1 := &Rule{Head: head, Body: NewBody(true1)}
	ruleTrue12 := &Rule{Head: head, Body: NewBody(true1, true2)}
	ruleTrue2 := &Rule{Head: head, Body: NewBody(true2)}
	ruleTrue12_2 := &Rule{Head: head, Body: NewBody(true1, true2)}
	ruleFalse1 := &Rule{Head: head, Body: NewBody(false1)}
	ruleTrueFalse := &Rule{Head: head, Body: NewBody(true1, false1)}
	ruleFalseTrue := &Rule{Head: head, Body: NewBody(false1, true1)}

	// Same expressions
	assertRulesEqual(t, ruleTrue1, ruleTrue2)
	assertRulesEqual(t, ruleTrue12, ruleTrue12_2)

	// Different expressions/different order
	assertRulesNotEqual(t, ruleTrue1, ruleFalse1)
	assertRulesNotEqual(t, ruleTrueFalse, ruleFalseTrue)

	// Assigned versus not.
	assigned := ruleTrue1.Copy()
	assigned.Head.Assign = true
	assertRulesNotEqual(t, ruleTrue1, assigned)
}

func TestRuleString(t *testing.T) {
	trueBody := NewBody(NewExpr(BooleanTerm(true)))

	tests := []struct {
		rule *Rule
		exp  string
	}{
		{
			rule: &Rule{
				Head: NewHead(Var("p"), nil, BooleanTerm(true)),
				Body: NewBody(
					Equality.Expr(StringTerm("foo"), StringTerm("bar")),
				),
			},
			exp: `p = true { "foo" = "bar" }`,
		},
		{
			rule: &Rule{
				Head: NewHead(Var("p"), VarTerm("x")),
				Body: trueBody,
			},
			exp: `p[x] { true }`,
		},
		{
			rule: &Rule{
				Head: RefHead(MustParseRef("p[x]"), BooleanTerm(true)),
				Body: MustParseBody("x = 1"),
			},
			exp: `p[x] = true { x = 1 }`,
		},
		{
			rule: &Rule{
				Head: RefHead(MustParseRef("p.q.r[x]"), BooleanTerm(true)),
				Body: MustParseBody("x = 1"),
			},
			exp: `p.q.r[x] = true { x = 1 }`,
		},
		{
			rule: &Rule{
				Head: &Head{
					Reference: MustParseRef("p.q.r"),
					Key:       VarTerm("1"),
				},
				Body: MustParseBody("x = 1"),
			},
			exp: `p.q.r contains 1 { x = 1 }`,
		},
		{
			rule: &Rule{
				Head: NewHead(Var("p"), VarTerm("x"), VarTerm("y")),
				Body: NewBody(
					Equality.Expr(StringTerm("foo"), VarTerm("x")),
					&Expr{
						Negated: true,
						Terms:   RefTerm(VarTerm("a"), StringTerm("b"), VarTerm("x")),
					},
					Equality.Expr(StringTerm("b"), VarTerm("y")),
				),
			},
			exp: `p[x] = y { "foo" = x; not a.b[x]; "b" = y }`,
		},
		{
			rule: &Rule{
				Default: true,
				Head:    NewHead("p", nil, BooleanTerm(true)),
			},
			exp: `default p = true`,
		},
		{
			rule: &Rule{
				Head: &Head{
					Name:  Var("f"),
					Args:  Args{VarTerm("x"), VarTerm("y")},
					Value: VarTerm("z"),
				},
				Body: NewBody(Plus.Expr(VarTerm("x"), VarTerm("y"), VarTerm("z"))),
			},
			exp: "f(x, y) = z { plus(x, y, z) }",
		},
		{
			rule: &Rule{
				Head: &Head{
					Name:   Var("p"),
					Value:  BooleanTerm(true),
					Assign: true,
				},
				Body: NewBody(
					Equality.Expr(StringTerm("foo"), StringTerm("bar")),
				),
			},
			exp: `p := true { "foo" = "bar" }`,
		},
		{
			rule: &Rule{
				Head: RefHead(MustParseRef("p.q.r")),
				Body: trueBody,
			},
			exp: `p.q.r { true }`,
		},
		{
			rule: &Rule{
				Head: RefHead(MustParseRef("p.q.r"), StringTerm("foo")),
				Body: trueBody,
			},
			exp: `p.q.r = "foo" { true }`,
		},
		{
			rule: &Rule{
				Head: RefHead(MustParseRef("p.q.r[x]"), StringTerm("foo")),
				Body: MustParseBody(`x := 1`),
			},
			exp: `p.q.r[x] = "foo" { assign(x, 1) }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.exp, func(t *testing.T) {
			assertRuleString(t, tc.rule, tc.exp)
		})
	}
}

func TestRulePath(t *testing.T) {
	ruleWithMod := func(r string) Ref {
		mod := MustParseModule("package pkg\n" + r)
		return mod.Rules[0].Path()
	}
	if exp, act := MustParseRef("data.pkg.p.q.r"), ruleWithMod("p.q.r { true }"); !exp.Equal(act) {
		t.Errorf("expected %v, got %v", exp, act)
	}

	if exp, act := MustParseRef("data.pkg.p"), ruleWithMod("p { true }"); !exp.Equal(act) {
		t.Errorf("expected %v, got %v", exp, act)
	}
}

func TestModuleString(t *testing.T) {

	input := `package a.b.c

import data.foo.bar
import input.xyz

p = true { not bar }
q = true { xyz.abc = 2 }
wildcard = true { bar[_] = 1 }`

	mod := MustParseModule(input)

	roundtrip, err := ParseModule("", mod.String())
	if err != nil {
		t.Fatalf("Unexpected error while parsing roundtripped module: %v", err)
	}

	if !roundtrip.Equal(mod) {
		t.Fatalf("Expected roundtripped to equal original but:\n\nExpected:\n\n%v\n\nDoes not equal result:\n\n%v", mod, roundtrip)
	}
}

func TestModuleCopy(t *testing.T) {

	input := `package foo

	# comment
	p := 7`

	mod := MustParseModule(input)
	cpy := mod.Copy()
	cpy.Comments[0].Text[0] = 'X'

	if !bytes.Equal(mod.Comments[0].Text, []byte(" comment")) {
		t.Fatal("expected comment text to be unchanged")
	}
}

func TestWithString(t *testing.T) {

	with1 := &With{
		Target: VarTerm("foo"),
		Value:  VarTerm("bar"),
	}

	result := with1.String()
	expected := "with foo as bar"
	if result != expected {
		t.Fatalf("Expected %v but got %v", expected, result)
	}

	with2 := &With{
		Target: MustParseTerm("com.example.input"),
		Value:  MustParseTerm(`{[1,2,3], {"x": y}}`),
	}

	result = with2.String()
	expected = `with com.example.input as {[1, 2, 3], {"x": y}}`

	if result != expected {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestSomeDeclString(t *testing.T) {

	decl := &SomeDecl{
		Symbols: []*Term{
			VarTerm("a"),
			VarTerm("b"),
		},
	}

	result := decl.String()
	expected := "some a, b"

	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}

	s := &SomeDecl{
		Symbols: []*Term{Member.Call(VarTerm("x"), VarTerm("xs"))},
	}
	if exp, act := "some x in xs", s.String(); act != exp {
		t.Errorf("Expected %v but got %v", exp, act)
	}

	s1 := &SomeDecl{
		Symbols: []*Term{Member.Call(VarTerm("x"), VarTerm("y"), VarTerm("xs"))},
	}
	if exp, act := "some x, y in xs", s1.String(); act != exp {
		t.Errorf("Expected %v but got %v", exp, act)
	}
}

func TestEveryString(t *testing.T) {
	tests := []struct {
		every Every
		exp   string
	}{
		{
			exp: `every x in ["foo", "bar"] { true; true }`,
			every: Every{
				Value:  VarTerm("x"),
				Domain: ArrayTerm(StringTerm("foo"), StringTerm("bar")),
				Body: []*Expr{
					{
						Terms: BooleanTerm(true),
					},
					{
						Terms: BooleanTerm(true),
					},
				},
			},
		},
		{
			exp: `every k, v in ["foo", "bar"] { true; true }`,
			every: Every{
				Key:    VarTerm("k"),
				Value:  VarTerm("v"),
				Domain: ArrayTerm(StringTerm("foo"), StringTerm("bar")),
				Body: []*Expr{
					{
						Terms: BooleanTerm(true),
					},
					{
						Terms: BooleanTerm(true),
					},
				},
			},
		},
	}
	for _, tc := range tests {
		if act := tc.every.String(); act != tc.exp {
			t.Errorf("expected %q, got %q", tc.exp, act)
		}
	}
}

func TestAnnotationsString(t *testing.T) {
	a := &Annotations{
		Scope:       "foo",
		Title:       "bar",
		Description: "baz",
		Authors: []*AuthorAnnotation{
			{
				Name:  "John Doe",
				Email: "john@example.com",
			},
			{
				Name: "Jane Doe",
			},
		},
		Organizations: []string{"mi", "fa"},
		RelatedResources: []*RelatedResourceAnnotation{
			{
				Ref: mustParseURL("https://example.com"),
			},
			{
				Ref:         mustParseURL("https://example.com/2"),
				Description: "Some resource",
			},
		},
		Schemas: []*SchemaAnnotation{
			{
				Path:   MustParseRef("data.bar"),
				Schema: MustParseRef("schema.baz"),
			},
		},
		Custom: map[string]interface{}{
			"list": []int{
				1, 2, 3,
			},
			"map": map[string]interface{}{
				"one": 1,
				"two": map[int]interface{}{
					3: "three",
				},
			},
			"flag": true,
		},
	}

	// NOTE(tsandall): for now, annotations are represented as JSON objects
	// which are a subset of YAML. We could improve this in the future.
	exp := `{"authors":[{"name":"John Doe","email":"john@example.com"},{"name":"Jane Doe"}],"custom":{"flag":true,"list":[1,2,3],"map":{"one":1,"two":{"3":"three"}}},"description":"baz","organizations":["mi","fa"],"related_resources":[{"ref":"https://example.com"},{"description":"Some resource","ref":"https://example.com/2"}],"schemas":[{"path":[{"type":"var","value":"data"},{"type":"string","value":"bar"}],"schema":[{"type":"var","value":"schema"},{"type":"string","value":"baz"}]}],"scope":"foo","title":"bar"}`

	if got := a.String(); exp != got {
		t.Fatalf("expected\n%s\nbut got\n%s", exp, got)
	}
}

func mustParseURL(str string) url.URL {
	parsed, err := url.Parse(str)
	if err != nil {
		panic(err)
	}
	return *parsed
}

func TestModuleStringAnnotations(t *testing.T) {
	module, err := ParseModuleWithOpts("test.rego", `package test

# METADATA
# scope: rule
p := 7`, ParserOptions{ProcessAnnotation: true})

	if err != nil {
		t.Fatal(err)
	}

	exp := `package test

# METADATA
# {"scope":"rule"}
p := 7 { true }`

	if module.String() != exp {
		t.Fatalf("expected %q but got %q", exp, module.String())
	}
}

func TestCommentCopy(t *testing.T) {
	comment := &Comment{
		Text:     []byte("foo bar baz"),
		Location: &location.Location{}, // location must be set for comment equality
	}

	cpy := comment.Copy()
	if !cpy.Equal(comment) {
		t.Fatal("expected copy to be equal")
	}

	comment.Text[1] = '0'

	if cpy.Equal(comment) {
		t.Fatal("expected copy to be unmodified")
	}
}

func assertExprEqual(t *testing.T, a, b *Expr) {
	t.Helper()
	if !a.Equal(b) {
		t.Errorf("Expressions are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertExprNotEqual(t *testing.T, a, b *Expr) {
	t.Helper()
	if a.Equal(b) {
		t.Errorf("Expressions are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertExprString(t *testing.T, expr *Expr, expected string) {
	t.Helper()
	result := expr.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertImportsEqual(t *testing.T, a, b *Import) {
	t.Helper()
	if !a.Equal(b) {
		t.Errorf("Imports are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertImportsNotEqual(t *testing.T, a, b *Import) {
	t.Helper()
	if a.Equal(b) {
		t.Errorf("Imports are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertImportToString(t *testing.T, imp *Import, expected string) {
	t.Helper()
	result := imp.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertPackagesEqual(t *testing.T, a, b *Package) {
	t.Helper()
	if !a.Equal(b) {
		t.Errorf("Packages are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertPackagesNotEqual(t *testing.T, a, b *Package) {
	t.Helper()
	if a.Equal(b) {
		t.Errorf("Packages are not equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRulesEqual(t *testing.T, a, b *Rule) {
	t.Helper()
	if !a.Equal(b) {
		t.Errorf("Rules are not equal (expected equal):\na=%v\nb=%v", a, b)
	}
}

func assertRulesNotEqual(t *testing.T, a, b *Rule) {
	t.Helper()
	if a.Equal(b) {
		t.Errorf("Rules are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertHeadsEqual(t *testing.T, a, b *Head) {
	t.Helper()
	if !a.Equal(b) {
		t.Errorf("Heads are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertHeadsNotEqual(t *testing.T, a, b *Head) {
	t.Helper()
	if a.Equal(b) {
		t.Errorf("Heads are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRuleString(t *testing.T, rule *Rule, expected string) {
	t.Helper()
	result := rule.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}
