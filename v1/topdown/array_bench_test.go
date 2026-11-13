package topdown

import (
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// BenchmarkArraySlice/start_=_0,_stop_=_length-16       89784783       13.3 ns/op       0 B/op       0 allocs/op
// BenchmarkArraySlice/start_>=_length-16                166829112       7.1 ns/op       0 B/op       0 allocs/op
// BenchmarkArraySlice/normal-16                         37044658       32.1 ns/op      88 B/op       2 allocs/op
func BenchmarkArraySlice(b *testing.B) {
	b.Run("start = 0, stop = length", func(b *testing.B) {
		bctx := BuiltinContext{}
		have := []*ast.Term{
			ast.ArrayTerm(ast.InternedBooleanTrue),
			ast.InternedTerm(0),
			ast.InternedTerm(1),
		}
		want := eqIter(ast.ArrayTerm(ast.InternedBooleanTrue))
		for b.Loop() {
			_ = builtinArraySlice(bctx, have, want)
		}
	})

	b.Run("start >= length", func(b *testing.B) {
		bctx := BuiltinContext{}
		have := []*ast.Term{
			ast.ArrayTerm(ast.InternedBooleanTrue),
			ast.InternedTerm(1),
			ast.InternedTerm(2),
		}
		want := eqIter(ast.InternedEmptyArray)
		for b.Loop() {
			_ = builtinArraySlice(bctx, have, want)
		}
	})

	b.Run("normal", func(b *testing.B) {
		bctx := BuiltinContext{}
		have := []*ast.Term{
			ast.ArrayTerm(ast.InternedBooleanTrue, ast.InternedBooleanTrue, ast.InternedBooleanTrue),
			ast.InternedTerm(1),
			ast.InternedTerm(2),
		}
		want := eqIter(ast.ArrayTerm(ast.InternedBooleanTrue))
		for b.Loop() {
			_ = builtinArraySlice(bctx, have, want)
		}
	})
}

// 1618 ns/op	    3672 B/op	       4 allocs/op
func BenchmarkArrayConcat(b *testing.B) {
	b.Run("concat two non-empty arrays", func(b *testing.B) {
		ints := slices.Collect(ast.InternedIntRange(0, 100))
		bctx := BuiltinContext{}
		have := []*ast.Term{
			ast.ArrayTerm(ints...),
			ast.ArrayTerm(ints...),
		}
		want := eqIter(ast.ArrayTerm(slices.Concat(ints, ints)...))
		for b.Loop() {
			_ = builtinArrayConcat(bctx, have, want)
		}
	})
}
