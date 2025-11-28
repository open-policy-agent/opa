// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

// BenchmarkRefPtr benchmarks the optimized Ref.Ptr() method
func BenchmarkRefPtr(b *testing.B) {
	ref, err := ParseRef("data.foo.bar.baz.qux")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
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
	for range b.N {
		_ = args.String()
	}
}

// BenchmarkBodyString benchmarks the optimized Body.String() method
func BenchmarkBodyString(b *testing.B) {
	body, err := ParseBody("x := 1; y := 2; z := x + y; a := z * 2")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = body.String()
	}
}

// BenchmarkExprString benchmarks the optimized Expr.String() method
func BenchmarkExprString(b *testing.B) {
	expr, err := ParseExpr("x = y + z with input.foo as 42 with data.bar as \"test\"")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = expr.String()
	}
}

// BenchmarkSetDiff benchmarks the optimized set.Diff() method
func BenchmarkSetDiff(b *testing.B) {
	s1 := NewSet()
	s2 := NewSet()
	for i := range 100 {
		s1.Add(IntNumberTerm(i))
		if i%2 == 0 {
			s2.Add(IntNumberTerm(i))
		}
	}

	b.ResetTimer()
	for range b.N {
		_ = s1.Diff(s2)
	}
}

// BenchmarkSetIntersect benchmarks the optimized set.Intersect() method
func BenchmarkSetIntersect(b *testing.B) {
	s1 := NewSet()
	s2 := NewSet()
	for i := range 100 {
		s1.Add(IntNumberTerm(i))
		if i%2 == 0 {
			s2.Add(IntNumberTerm(i))
		}
	}

	b.ResetTimer()
	for range b.N {
		_ = s1.Intersect(s2)
	}
}

// BenchmarkObjectKeys benchmarks the optimized object.Keys() method
func BenchmarkObjectKeys(b *testing.B) {
	obj := NewObject()
	for i := range 50 {
		obj.Insert(StringTerm(string(rune('a'+i%26))+string(rune('0'+i/26))), IntNumberTerm(i))
	}

	b.ResetTimer()
	for range b.N {
		_ = obj.Keys()
	}
}

// BenchmarkGetRules benchmarks the optimized Compiler.GetRules() method
func BenchmarkGetRules(b *testing.B) {
	module := `
		package test
		
		p contains x if { x := 1 }
		p contains x if { x := 2 }
		q contains x if { x := 3 }
		r := 4
	`

	mod, err := ParseModule("test.rego", module)
	if err != nil {
		b.Fatal(err)
	}

	c := NewCompiler()
	c.Compile(map[string]*Module{
		"test.rego": mod,
	})

	if c.Failed() {
		b.Fatal(c.Errors)
	}

	ref, err := ParseRef("data.test.p")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = c.GetRules(ref)
	}
}
