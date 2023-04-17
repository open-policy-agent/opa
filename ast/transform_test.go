// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

func TestTransform(t *testing.T) {
	module := MustParseModule(`package ex.this

import input.foo
import data.bar.this as qux
import future.keywords.every

p = true { "this" = "that" }
p = "this" { false }
p["this"] { false }
p[y] = {"this": ["this"]} { false }
p = true { ["this" | "this"] }
p = n { count({"this", "that"}, n) with input.foo.this as {"this": true} }
p { false } else = "this" { "this" } else = ["this"] { true }
foo(x) = y { split(x, "this", y) }
p { every x in ["this"] { x == "this" } }
a.b.c.this["this"] = d { d := "this" }
`)

	result, err := Transform(&GenericTransformer{
		func(x interface{}) (interface{}, error) {
			if s, ok := x.(String); ok && s == String("this") {
				return String("that"), nil
			}
			return x, nil
		},
	}, module)

	if err != nil {
		t.Fatalf("Unexpected error during transform: %v", err)
	}

	resultMod, ok := result.(*Module)
	if !ok {
		t.Fatalf("Expected module from transform but got: %v", result)
	}

	expected := MustParseModule(`package ex.that

import input.foo
import data.bar.that as qux
import future.keywords.every

p = true { "that" = "that" }
p = "that" { false }
p["that"] { false }
p[y] = {"that": ["that"]} { false }
p = true { ["that" | "that"] }
p = n { count({"that"}, n) with input.foo.that as {"that": true} }
p { false } else = "that" { "that" } else = ["that"] { true }
foo(x) = y { split(x, "that", y) }
p { every x in ["that"] { x == "that" } }
a.b.c.that["that"] = d { d := "that" }
`)

	if !expected.Equal(resultMod) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected, resultMod)
	}

}

func TestTransformAnnotations(t *testing.T) {

	module, err := ParseModuleWithOpts("test.rego", `package test

# METADATA
# scope: rule
p := 7`, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}

	result, err := Transform(&GenericTransformer{
		func(x interface{}) (interface{}, error) {
			if s, ok := x.(*Annotations); ok {
				cpy := *s
				cpy.Scope = "deadbeef"
				return &cpy, nil
			}
			return x, nil
		},
	}, module)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	resultMod, ok := result.(*Module)
	if !ok {
		t.Fatalf("Expected module from transform but got: %v", result)
	}

	exp, err := ParseModuleWithOpts("test.rego", `package test

# METADATA
# scope: rule
p := 7`, ParserOptions{ProcessAnnotation: true})
	if err != nil {
		t.Fatal(err)
	}

	exp.Annotations[0].Scope = "deadbeef"

	if resultMod.Compare(exp) != 0 {
		t.Fatalf("expected:\n\n%v\n\ngot:\n\n%v", exp, resultMod)
	}

}

func TestTransformRefsAndRuleHeads(t *testing.T) {
	module := MustParseModule(`package test
p.q.this.fo[x] = y { x := "x"; y := "y" }`)

	result, err := TransformRefs(module, func(r Ref) (Value, error) {
		if r[0].Value.Compare(Var("p")) == 0 {
			r[2] = StringTerm("that")
		}
		return r, nil
	})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	resultMod := result.(*Module)
	if exp, act := MustParseRef("p.q.that.fo[x]"), resultMod.Rules[0].Head.Reference; !act.Equal(exp) {
		t.Errorf("expected %v, got %v", exp, act)
	}
}
