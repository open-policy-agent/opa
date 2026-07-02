package ast

import "testing"

// Simple benchmark to demonstrate the cost of boxing to `any`, and why it's
// good not to when it's possible to avoid it.
//
// BenchmarkVarVisitorWalkAnyVsSpecific/Walk-12         33266721        36.70 ns/op      24 B/op       1 allocs/op
// BenchmarkVarVisitorWalkAnyVsSpecific/WalkBody-12     70195105        17.41 ns/op       0 B/op       0 allocs/op
func BenchmarkVarVisitorWalkAnyVsSpecific(b *testing.B) {
	bod := MustParseBody("foo")
	vis := NewVarVisitor()

	b.Run("Walk", func(b *testing.B) {
		for b.Loop() {
			vis.Walk(bod)
		}
	})

	if len(vis.vars) != 1 {
		b.Fatalf("Expected exactly one variable in AST but got %d: %v", len(vis.vars), vis.vars)
	}

	vis.Clear()
	b.ResetTimer()

	b.Run("WalkBody", func(b *testing.B) {
		for b.Loop() {
			vis.WalkBody(bod)
		}
	})

	if len(vis.vars) != 1 {
		b.Fatalf("Expected exactly one variable in AST but got %d: %v", len(vis.vars), vis.vars)
	}
}

// Example benchmark of us over-allocating in place like [outputVarsForExprEq]
//
// BenchmarkVarSetUpdateEmpty-12    	15169982	        79.69 ns/op	     128 B/op	       4 allocs/op
func BenchmarkVarSetUpdateEmpty(b *testing.B) {
	ref := MustParseRef("foo.bar.baz")
	used := NewVarSet()

	for b.Loop() {
		for _, t := range ref[1:] {
			vars := t.Vars()
			used.Update(vars)
		}
	}
}

// This benchmark demonstratesthe cost of walking "blindly", i.e. across
// all nodes even when most of them can't contain what we're looking for.
// While the TypeVisitor is much more efficient even in a default walk, the
// difference becomes massive when walking nodes selectively. The downside
// is there'll be a lot more code needed for type-specific visitors.
//
// GenericVisitor-16              879.1 ns/op     624 B/op     30 allocs/op
// TypeVisitor_term-16            475.5 ns/op       0 B/op      0 allocs/op
// TypeVisitor_via_WalkRules-16   16.11 ns/op       0 B/op      0 allocs/op
func BenchmarkGenericVisitorWalkVsTypeVisitor(b *testing.B) {
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

	b.Run("GenericVisitor", func(b *testing.B) {
		vis := NewGenericVisitor(func(x any) bool {
			return false
		})

		for b.Loop() {
			vis.Walk(mod)
		}
	})

	b.Run("TypeVisitor term", func(b *testing.B) {
		for b.Loop() {
			termTypeVisitor.walk(mod, func(x *Term) bool {
				return false
			})
		}
	})

	b.Run("TypeVisitor via WalkRules", func(b *testing.B) {
		for b.Loop() {
			WalkRules(mod, func(r *Rule) bool {
				return false
			})
		}
	})
}
