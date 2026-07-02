package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

var (
	unmarshalled = ast.ObjectTerm(
		ast.Item(ast.InternedTerm("foo"), ast.ObjectTerm(
			ast.Item(ast.InternedTerm("bar"), ast.ArrayTerm(ast.InternedTerm("baz"), ast.InternedTerm("qux"))),
			ast.Item(ast.InternedTerm("num"), ast.InternedTerm(42)),
		)),
	)
	marshalled = `foo:
  bar:
  - baz
  - qux
  num: 42
`
)

// 7447 ns/op	   22984 B/op	     142 allocs/op
// 7343 ns/op	   22872 B/op	     140 allocs/op
func BenchmarkYAMLMarshal(b *testing.B) {
	expect := ast.InternedTerm(marshalled)
	operands := []*ast.Term{unmarshalled}
	iter := eqIter(expect)

	for b.Loop() {
		if err := builtinYAMLMarshal(BuiltinContext{}, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}

// 5393 ns/op	   11066 B/op	     146 allocs/op
// 5210 ns/op	   10980 B/op	     144 allocs/op
func BenchmarkYAMLUnmarshal(b *testing.B) {
	operands := []*ast.Term{ast.InternedTerm(marshalled)}
	iter := eqIter(unmarshalled)

	for b.Loop() {
		if err := builtinYAMLUnmarshal(BuiltinContext{}, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}
