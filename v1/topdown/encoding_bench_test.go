package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// 7447 ns/op	   22984 B/op	     142 allocs/op
// 7243 ns/op	   22855 B/op	     139 allocs/op
func BenchmarkYAMLMarshal(b *testing.B) {
	obj := ast.ObjectTerm(
		ast.Item(ast.InternedTerm("foo"), ast.ObjectTerm(
			ast.Item(ast.InternedTerm("bar"), ast.ArrayTerm(
				ast.InternedTerm("baz"),
				ast.InternedTerm("qux"),
			)),
			ast.Item(ast.InternedTerm("num"), ast.InternedTerm(42)),
		)),
	)
	expect := ast.InternedTerm(`foo:
  bar:
  - baz
  - qux
  num: 42
`)

	operands := []*ast.Term{obj}
	bctx := BuiltinContext{}
	iter := eqIter(expect)

	for b.Loop() {
		if err := builtinYAMLMarshal(bctx, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}

// 5393 ns/op	   11066 B/op	     146 allocs/op
func BenchmarkYAMLUnmarshal(b *testing.B) {
	yamlTerm := ast.InternedTerm(`foo:
  bar:
  - baz
  - qux
  num: 42
`)
	expect := ast.ObjectTerm(
		ast.Item(ast.InternedTerm("foo"), ast.ObjectTerm(
			ast.Item(ast.InternedTerm("bar"), ast.ArrayTerm(
				ast.InternedTerm("baz"),
				ast.InternedTerm("qux"),
			)),
			ast.Item(ast.InternedTerm("num"), ast.InternedTerm(42)),
		)),
	)

	operands := []*ast.Term{yamlTerm}
	bctx := BuiltinContext{}
	iter := eqIter(expect)

	for b.Loop() {
		if err := builtinYAMLUnmarshal(bctx, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}
