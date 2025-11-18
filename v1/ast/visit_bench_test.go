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
