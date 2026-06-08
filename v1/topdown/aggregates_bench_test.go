package topdown

import (
	"fmt"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// 36.80 ns/op	       0 B/op	       0 allocs/op
func BenchmarkSumIntArray(b *testing.B) {
	bcx := BuiltinContext{}
	arr := ast.ArrayTerm(
		ast.InternedTerm(1),
		ast.InternedTerm(2),
		ast.InternedTerm(3),
		ast.InternedTerm(4),
		ast.InternedTerm(5),
		ast.InternedTerm(6),
	)
	exp := ast.InternedTerm(21)

	verify := func(x *ast.Term) error {
		// Can do simple equality check since we are using interned terms
		if x != exp {
			return fmt.Errorf("expected %v, got %v", exp.Value, x.Value)
		}
		return nil
	}

	for b.Loop() {
		err := builtinSum(bcx, []*ast.Term{arr}, verify)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// 1857 ns/op	    2736 B/op	      80 allocs/op
func BenchmarkSumFloatArray(b *testing.B) {
	bcx := BuiltinContext{}
	arr := ast.ArrayTerm(
		ast.FloatNumberTerm(1.1),
		ast.FloatNumberTerm(2.2),
		ast.FloatNumberTerm(3.3),
		ast.FloatNumberTerm(4.4),
		ast.FloatNumberTerm(5.5),
		ast.FloatNumberTerm(6.6),
	)
	exp := ast.FloatNumberTerm(23.1)

	verify := func(x *ast.Term) error {
		if x.Value != exp.Value {
			return fmt.Errorf("expected %v, got %v", exp.Value, x.Value)
		}
		return nil
	}

	for b.Loop() {
		err := builtinSum(bcx, []*ast.Term{arr}, verify)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkSumIntSet(b *testing.B) {
	bcx := BuiltinContext{}
	set := ast.SetTerm(
		ast.InternedTerm(1),
		ast.InternedTerm(2),
		ast.InternedTerm(3),
		ast.InternedTerm(4),
		ast.InternedTerm(5),
		ast.InternedTerm(6),
	)
	exp := ast.InternedTerm(21)

	verify := func(x *ast.Term) error {
		if x != exp {
			return fmt.Errorf("expected %v, got %v", exp.Value, x.Value)
		}
		return nil
	}

	for b.Loop() {
		err := builtinSum(bcx, []*ast.Term{set}, verify)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkSumFloatSet(b *testing.B) {
	bcx := BuiltinContext{}
	set := ast.SetTerm(
		ast.FloatNumberTerm(1.1),
		ast.FloatNumberTerm(2.2),
		ast.FloatNumberTerm(3.3),
		ast.FloatNumberTerm(4.4),
		ast.FloatNumberTerm(5.5),
		ast.FloatNumberTerm(6.6),
	)
	exp := ast.FloatNumberTerm(23.1)

	verify := func(x *ast.Term) error {
		if x.Value != exp.Value {
			return fmt.Errorf("expected %v, got %v", exp.Value, x.Value)
		}
		return nil
	}

	for b.Loop() {
		err := builtinSum(bcx, []*ast.Term{set}, verify)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkMember/set-16         	81601833	        14.58 ns/op	       0 B/op	       0 allocs/op
// BenchmarkMember/array-16       	  579356	         1988 ns/op	       0 B/op	       0 allocs/op
// BenchmarkMember/object-16      	90483236	        13.14 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMember(b *testing.B) {
	key := ast.InternedTerm(99)
	val := key
	exp := eqIter(ast.InternedTerm(true))

	b.Run("set", func(b *testing.B) {
		bcx := BuiltinContext{}
		set := ast.SetTerm(slices.Collect(ast.InternedIntRange(1, 100))...)
		ops := []*ast.Term{key, set}

		for b.Loop() {
			if err := builtinMember(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("array", func(b *testing.B) {
		bcx := BuiltinContext{}
		arr := ast.ArrayTerm(slices.Collect(ast.InternedIntRange(1, 100))...)
		ops := []*ast.Term{key, arr}

		for b.Loop() {
			if err := builtinMember(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("object", func(b *testing.B) {
		bcx := BuiltinContext{}
		obj := ast.NewObject()
		for i := 1; i <= 100; i++ {
			obj.Insert(ast.InternedTerm(i), ast.InternedTerm(i))
		}
		ops := []*ast.Term{key, val, ast.NewTerm(obj)}

		for b.Loop() {
			if err := builtinMemberWithKey(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}

// Was
// BenchmarkMemberWithKey/set-16         	error
// BenchmarkMemberWithKey/array-16       	58668218	        20.41 ns/op	       0 B/op	       0 allocs/op
// BenchmarkMemberWithKey/object-16      	61149481	        20.33 ns/op	       0 B/op	       0 allocs/op
//
// Now
// BenchmarkMemberWithKey/set-16         	79370059	        15.04 ns/op	       0 B/op	       0 allocs/op
// BenchmarkMemberWithKey/array-16       	97242430	        12.00 ns/op	       0 B/op	       0 allocs/op
// BenchmarkMemberWithKey/object-16      	96117901	        12.63 ns/op	       0 B/op	       0 allocs/op
func BenchmarkMemberWithKey(b *testing.B) {
	key, val := ast.InternedTerm(99), ast.InternedTerm(99)
	exp := eqIter(ast.InternedTerm(true))

	b.Run("set", func(b *testing.B) {
		bcx := BuiltinContext{}
		ops := []*ast.Term{key, key, ast.SetTerm(slices.Collect(ast.InternedIntRange(1, 100))...)}

		for b.Loop() {
			if err := builtinMemberWithKey(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("array", func(b *testing.B) {
		bcx := BuiltinContext{}
		key := ast.InternedTerm(98)
		ops := []*ast.Term{key, val, ast.ArrayTerm(slices.Collect(ast.InternedIntRange(1, 100))...)}

		for b.Loop() {
			if err := builtinMemberWithKey(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("object", func(b *testing.B) {
		bcx := BuiltinContext{}
		obj := ast.NewObject()
		for i := 1; i <= 100; i++ {
			obj.Insert(ast.InternedTerm(i), ast.InternedTerm(i))
		}
		ops := []*ast.Term{key, val, ast.NewTerm(obj)}

		for b.Loop() {
			if err := builtinMemberWithKey(bcx, ops, exp); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}
