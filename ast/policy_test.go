// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestModuleJSONRoundTrip(t *testing.T) {

	mod := MustParseModule(`package a.b.c

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
`)

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
			&With{
				Target: MustParseTerm("input"),
				Value:  MustParseTerm("bar"),
			},
		},
	}

	expr31 := &Expr{
		Terms: MustParseTerm("data.foo.bar"),
		With: []*With{
			&With{
				Target: MustParseTerm("input"),
				Value:  MustParseTerm("bar"),
			},
		},
	}

	expr32 := &Expr{
		Terms: MustParseTerm("data.foo.bar"),
		With: []*With{
			&With{
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
	assertExprString(t, expr1, "q.r[x]")
	assertExprString(t, expr2, "not q.r[x]")
	assertExprString(t, expr3, "\"a\" = 17.1")
	assertExprString(t, expr4, "neq({foo: [1, a.b]}, false)")
	assertExprString(t, expr5, "true with foo as bar with baz as qux")
	assertExprString(t, expr6, "plus(1, 2, 3)")
	assertExprString(t, expr7, "count(\"foo\", x)")
	assertExprString(t, expr8, "data.test.f(1, x)")
	assertExprString(t, expr9, `contains("foo.bar", ".")`)
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

func TestRuleHeadEquals(t *testing.T) {
	assertHeadsEqual(t, &Head{}, &Head{})

	// Same name/key/value
	assertHeadsEqual(t, &Head{Name: Var("p")}, &Head{Name: Var("p")})
	assertHeadsEqual(t, &Head{Key: VarTerm("x")}, &Head{Key: VarTerm("x")})
	assertHeadsEqual(t, &Head{Value: VarTerm("x")}, &Head{Value: VarTerm("x")})
	assertHeadsEqual(t, &Head{Args: []*Term{VarTerm("x"), VarTerm("y")}}, &Head{Args: []*Term{VarTerm("x"), VarTerm("y")}})

	// Different name/key/value
	assertHeadsNotEqual(t, &Head{Name: Var("p")}, &Head{Name: Var("q")})
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
}

func TestRuleString(t *testing.T) {

	rule1 := &Rule{
		Head: NewHead(Var("p")),
		Body: NewBody(
			Equality.Expr(StringTerm("foo"), StringTerm("bar")),
		),
	}

	rule2 := &Rule{
		Head: NewHead(Var("p"), VarTerm("x"), VarTerm("y")),
		Body: NewBody(
			Equality.Expr(StringTerm("foo"), VarTerm("x")),
			&Expr{
				Negated: true,
				Terms:   RefTerm(VarTerm("a"), StringTerm("b"), VarTerm("x")),
			},
			Equality.Expr(StringTerm("b"), VarTerm("y")),
		),
	}

	rule3 := &Rule{
		Default: true,
		Head:    NewHead("p", nil, BooleanTerm(true)),
	}

	rule4 := &Rule{
		Head: &Head{
			Name:  Var("f"),
			Args:  Args{VarTerm("x"), VarTerm("y")},
			Value: VarTerm("z"),
		},
		Body: NewBody(Plus.Expr(VarTerm("x"), VarTerm("y"), VarTerm("z"))),
	}

	assertRuleString(t, rule1, `p { "foo" = "bar" }`)
	assertRuleString(t, rule2, `p[x] = y { "foo" = x; not a.b[x]; "b" = y }`)
	assertRuleString(t, rule3, `default p = true`)
	assertRuleString(t, rule4, "f(x, y) = z { plus(x, y, z) }")
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

func assertExprEqual(t *testing.T, a, b *Expr) {
	if !a.Equal(b) {
		t.Errorf("Expressions are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertExprNotEqual(t *testing.T, a, b *Expr) {
	if a.Equal(b) {
		t.Errorf("Expressions are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertExprString(t *testing.T, expr *Expr, expected string) {
	result := expr.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertImportsEqual(t *testing.T, a, b *Import) {
	if !a.Equal(b) {
		t.Errorf("Imports are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertImportsNotEqual(t *testing.T, a, b *Import) {
	if a.Equal(b) {
		t.Errorf("Imports are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertImportToString(t *testing.T, imp *Import, expected string) {
	result := imp.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertPackagesEqual(t *testing.T, a, b *Package) {
	if !a.Equal(b) {
		t.Errorf("Packages are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertPackagesNotEqual(t *testing.T, a, b *Package) {
	if a.Equal(b) {
		t.Errorf("Packages are not equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRulesEqual(t *testing.T, a, b *Rule) {
	if !a.Equal(b) {
		t.Errorf("Rules are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertRulesNotEqual(t *testing.T, a, b *Rule) {
	if a.Equal(b) {
		t.Errorf("Rules are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertHeadsEqual(t *testing.T, a, b *Head) {
	if !a.Equal(b) {
		t.Errorf("Heads are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertHeadsNotEqual(t *testing.T, a, b *Head) {
	if a.Equal(b) {
		t.Errorf("Heads are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRuleString(t *testing.T, rule *Rule, expected string) {
	result := rule.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}
