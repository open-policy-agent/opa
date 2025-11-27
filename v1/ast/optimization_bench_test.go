// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

// BenchmarkRefPtr benchmarks the optimized Ref.Ptr() method
func BenchmarkRefPtr(b *testing.B) {
	ref := MustParseRef("data.foo.bar.baz.qux")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ref.Ptr()
	}
}

// BenchmarkArgsString benchmarks the optimized Args.String() method
func BenchmarkArgsString(b *testing.B) {
	args := Args{
		StringTerm("arg1"),
		StringTerm("arg2"),
		StringTerm("arg3"),
		NumberTerm("42"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = args.String()
	}
}

// BenchmarkBodyString benchmarks the optimized Body.String() method
func BenchmarkBodyString(b *testing.B) {
	body := MustParseBody("x := 1; y := 2; z := x + y; a := z * 2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = body.String()
	}
}

// BenchmarkExprString benchmarks the optimized Expr.String() method
func BenchmarkExprString(b *testing.B) {
	expr := MustParseExpr("x = y + z with input.foo as 42 with data.bar as \"test\"")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = expr.String()
	}
}

// BenchmarkSetDiff benchmarks the optimized set.Diff() method
func BenchmarkSetDiff(b *testing.B) {
	s1 := NewSet()
	s2 := NewSet()
	for i := 0; i < 100; i++ {
		s1.Add(IntNumberTerm(i))
		if i%2 == 0 {
			s2.Add(IntNumberTerm(i))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s1.Diff(s2)
	}
}

// BenchmarkSetIntersect benchmarks the optimized set.Intersect() method
func BenchmarkSetIntersect(b *testing.B) {
	s1 := NewSet()
	s2 := NewSet()
	for i := 0; i < 100; i++ {
		s1.Add(IntNumberTerm(i))
		if i%2 == 0 {
			s2.Add(IntNumberTerm(i))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s1.Intersect(s2)
	}
}

// BenchmarkObjectKeys benchmarks the optimized object.Keys() method
func BenchmarkObjectKeys(b *testing.B) {
	obj := NewObject()
	for i := 0; i < 50; i++ {
		obj.Insert(StringTerm(string(rune('a'+i%26))+string(rune('0'+i/26))), IntNumberTerm(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = obj.Keys()
	}
}

// BenchmarkGetRules benchmarks the optimized Compiler.GetRules() method
func BenchmarkGetRules(b *testing.B) {
	module := `
		package test
		
		p[x] { x := 1 }
		p[x] { x := 2 }
		q[x] { x := 3 }
		r := 4
	`

	c := NewCompiler()
	c.Compile(map[string]*Module{
		"test.rego": MustParseModule(module),
	})

	if c.Failed() {
		b.Fatal(c.Errors)
	}

	ref := MustParseRef("data.test.p")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.GetRules(ref)
	}
}
