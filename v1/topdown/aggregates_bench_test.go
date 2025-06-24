package topdown

import (
	"fmt"
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

	for range b.N {
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

	for range b.N {
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

	for range b.N {
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

	for range b.N {
		err := builtinSum(bcx, []*ast.Term{set}, verify)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
