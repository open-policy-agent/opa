package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// sprintf renders -0 as 0 and the golden cases compare numbers as float64, so neither can
// tell a negative zero quotient from a zero one.
func TestDivideNegativeZero(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ x, y string }{{"0", "-1"}, {"-0", "1"}, {"0.0", "-7"}} {
		operands := []*ast.Term{ast.NewTerm(ast.Number(tc.x)), ast.NewTerm(ast.Number(tc.y))}

		var got string
		if err := opDivide.builtin()(BuiltinContext{}, operands, func(term *ast.Term) error {
			got = term.String()
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if got != "-0" {
			t.Errorf("%s / %s: expected -0 but got %s", tc.x, tc.y, got)
		}
	}
}

// A number that is not an integer, and cannot be parsed as a float either, is an error and
// not a panic.
func TestRemOutOfRangeExponent(t *testing.T) {
	t.Parallel()

	operands := []*ast.Term{ast.NewTerm(ast.Number("1e2147483648")), ast.IntNumberTerm(2)}

	err := opRem.builtin()(BuiltinContext{}, operands, func(*ast.Term) error { return nil })
	if err == nil || err.Error() != "modulo on floating-point number" {
		t.Fatalf("expected modulo on floating-point number error but got %v", err)
	}
}
