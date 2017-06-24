package main

import (
	"io/ioutil"
	"testing"
)

func BenchmarkParsePigeonNoMemo(b *testing.B) {
	d, err := ioutil.ReadFile("../../../grammar/pigeon.peg")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", d, Memoize(false)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParsePigeonMemo(b *testing.B) {
	d, err := ioutil.ReadFile("../../../grammar/pigeon.peg")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", d, Memoize(true)); err != nil {
			b.Fatal(err)
		}
	}
}
