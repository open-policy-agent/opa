package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// BenchmarkNumbersRange/interned-12         	  824348	      1443 ns/op	    1880 B/op	       4 allocs/op
// BenchmarkNumbersRange/not_interned-12     	   99547	     12052 ns/op	   15968 B/op	     533 allocs/op
func BenchmarkNumbersRange(b *testing.B) {
	bctx := BuiltinContext{}
	expect100Items := expectCountIter(b, 100)
	tests := []struct {
		name     string
		operands []*ast.Term
	}{
		{
			name:     "interned",
			operands: []*ast.Term{ast.InternedTerm(0), ast.InternedTerm(99)},
		},
		{
			name:     "not interned",
			operands: []*ast.Term{ast.IntNumberTerm(1000), ast.IntNumberTerm(1099)},
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			for b.Loop() {
				if err := builtinNumbersRange(bctx, test.operands, expect100Items); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Performs and should perform identically to BenchmarkNumbersRange
func BenchmarkNumbersRangeStep(b *testing.B) {
	bctx := BuiltinContext{}
	expect100Items := expectCountIter(b, 100)
	step := ast.InternedTerm(2)
	tests := []struct {
		name     string
		operands []*ast.Term
	}{
		{
			name:     "interned",
			operands: []*ast.Term{ast.InternedTerm(0), ast.InternedTerm(199), step},
		},
		{
			name:     "not interned",
			operands: []*ast.Term{ast.IntNumberTerm(1000), ast.IntNumberTerm(1199), step},
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			for b.Loop() {
				if err := builtinNumbersRangeStep(bctx, test.operands, expect100Items); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func expectCountIter(b *testing.B, expected int) func(*ast.Term) error {
	b.Helper()
	return func(term *ast.Term) error {
		if a, ok := term.Value.(*ast.Array); ok && a.Len() == expected {
			return nil
		}
		return fmt.Errorf("expected an array of %d items, got %v", expected, term.Value)
	}
}
