// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

func TestTransform(t *testing.T) {
	mod := module(`package ex.this

import input.foo
import data.bar.this as qux
import future.keywords.every

p = true if { "this" = "that" }
p = "this" if { false }
p contains "this" if { false }
p[y] = {"this": ["this"]} if { false }
p = true if { ["this" | "this"] }
p = n if { count({"this", "that"}, n) with input.foo.this as {"this": true} }
p if { false } else = "this" if { "this" } else = ["this"] if { true }
foo(x) = y if { split(x, "this", y) }
p if { every x in ["this"] { x == "this" } }
a.b.c.this["this"] = d if { d := "this" }
`)

	result, err := Transform(&GenericTransformer{
		func(x interface{}) (interface{}, error) {
			if s, ok := x.(String); ok && s == String("this") {
				return String("that"), nil
			}
			return x, nil
		},
	}, mod)

	if err != nil {
		t.Fatalf("Unexpected error during transform: %v", err)
	}

	resultMod, ok := result.(*Module)
	if !ok {
		t.Fatalf("Expected module from transform but got: %v", result)
	}

	expected := module(`package ex.that

import input.foo
import data.bar.that as qux
import future.keywords.every

p = true if { "that" = "that" }
p = "that" if { false }
p contains "that" if  { false }
p[y] = {"that": ["that"]} if { false }
p = true if { ["that" | "that"] }
p = n if { count({"that"}, n) with input.foo.that as {"that": true} }
p if { false } else = "that" if { "that" } else = ["that"] if { true }
foo(x) = y if { split(x, "that", y) }
p if { every x in ["that"] { x == "that" } }
a.b.c.that["that"] = d if { d := "that" }
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
	module := module(`package test
p.q.this.fo[x] = y if { x := "x"; y := "y" }`)

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
