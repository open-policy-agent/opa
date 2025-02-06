package ast

import "testing"

// BenchmarkTypeName-10    32207775	   38.93 ns/op    8 B/op    1 allocs/op
func BenchmarkTypeName(b *testing.B) {
	term := StringTerm("foo")
	b.ResetTimer()

	for range b.N {
		name := TypeName(term.Value)
		if name != "string" {
			b.Fatalf("expected string but got %v", name)
		}
	}
}

// BenchmarkValueName-10    508312227    2.374 ns/op    0 B/op    0 allocs/op
func BenchmarkValueName(b *testing.B) {
	term := StringTerm("foo")
	b.ResetTimer()

	for range b.N {
		name := ValueName(term.Value)
		if name != "string" {
			b.Fatalf("expected string but got %v", name)
		}
	}
}
