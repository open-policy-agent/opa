package main

import "testing"

// With Unicode classes in the grammar:
// BenchmarkParseUnicodeClass          2000            548233 ns/op           96615 B/op        978 allocs/op
//
// With Unicode classes in a go map:
// BenchmarkParseUnicodeClass          5000            272224 ns/op           37990 B/op        482 allocs/op
func BenchmarkParseUnicodeClass(b *testing.B) {
	input := []byte("a = [\\p{Latin}]")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", input); err != nil {
			b.Fatal(err)
		}
	}
}

// With keywords in the grammar:
// BenchmarkParseKeyword       5000            315189 ns/op           50175 B/op        530 allocs/op
//
// With keywords in a go map:
// BenchmarkParseKeyword      10000            201175 ns/op           27017 B/op        331 allocs/op
func BenchmarkParseKeyword(b *testing.B) {
	input := []byte("a = uint32:'a'")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", input); err == nil {
			// error IS expected, fatal if none
			b.Fatal(err)
		}
	}
}
