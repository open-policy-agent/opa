package topdown

import (
	"fmt"
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

// BenchmarkJSONSize/json.size/10-16         	 4335858	       560.3 ns/op	      43 B/op	       3 allocs/op
// BenchmarkJSONSize/json.marshal/10-16      	  345904	      6793 ns/op	    9947 B/op	     218 allocs/op
// BenchmarkJSONSize/json.size/1000-16       	   36680	     64019 ns/op	      45 B/op	       3 allocs/op
// BenchmarkJSONSize/json.marshal/1000-16    	    3049	    787611 ns/op	 1063859 B/op	   21017 allocs/op
// BenchmarkJSONSize/json.size/100000-16     	     338	   6791184 ns/op	      48 B/op	       3 allocs/op
// BenchmarkJSONSize/json.marshal/100000-16  	      25	  91886153 ns/op	114875544 B/op	2100286 allocs/op
func BenchmarkJSONSize(b *testing.B) {
	for _, n := range []int{10, 1_000, 100_000} {
		items := make([][2]*ast.Term, 0, n)
		for i := range n {
			items = append(items, [2]*ast.Term{
				ast.StringTerm(fmt.Sprintf("key-%d", i)),
				ast.ObjectTerm(
					[2]*ast.Term{ast.StringTerm("name"), ast.StringTerm(fmt.Sprintf("some <name> héllo %d", i))},
					[2]*ast.Term{ast.StringTerm("n"), ast.IntNumberTerm(i)},
					[2]*ast.Term{ast.StringTerm("tags"), ast.ArrayTerm(ast.StringTerm("a"), ast.StringTerm("b"), ast.BooleanTerm(true))},
				),
			})
		}
		operands := []*ast.Term{ast.NewTerm(ast.NewObject(items...))}

		b.Run(fmt.Sprintf("json.size/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := builtinJSONSize(BuiltinContext{}, operands, func(*ast.Term) error { return nil }); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("json.marshal/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := builtinJSONMarshal(BuiltinContext{}, operands, func(*ast.Term) error { return nil }); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
