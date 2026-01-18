package topdown

import (
	"math/big"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/types"
)

func TestCustomBuiltinIterator(t *testing.T) {
	t.Parallel()

	query := NewQuery(ast.MustParseBody("test(1, x)")).WithBuiltins(map[string]*Builtin{
		"test": {
			Decl: &ast.Builtin{
				Name: "test",
				Decl: types.NewFunction(types.Args(types.N), types.N),
			},
			Func: func(bctx BuiltinContext, terms []*ast.Term, iter func(*ast.Term) error) error {
				if bctx.Context == nil {
					t.Fatal("context must be non-nil")
				}
				n, ok := terms[0].Value.(ast.Number)
				if ok {
					if i, ok := n.Int(); ok {
						return iter(ast.IntNumberTerm(i + 1))
					}
				}
				return nil
			},
		},
	})

	ctx := t.Context()

	rs, err := query.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected one result but got:", rs)
	} else if !rs[0][ast.Var("x")].Equal(ast.IntNumberTerm(2)) {
		t.Fatal("Expected x to be 2 but got:", rs[0])
	}
}

func TestNumberToFloatInto(t *testing.T) {
	t.Parallel()

	first := builtins.NumberToFloatInto(nil, ast.Number("1.5"))
	if first == nil {
		t.Fatal("expected non-nil float")
	}
	if first.Cmp(big.NewFloat(1.5)) != 0 {
		t.Fatalf("expected 1.5, got %v", first)
	}

	reuse := big.NewFloat(0)
	second := builtins.NumberToFloatInto(reuse, ast.Number("2.75"))
	if second != reuse {
		t.Fatal("expected reuse of provided float")
	}
	if reuse.Cmp(big.NewFloat(2.75)) != 0 {
		t.Fatalf("expected 2.75, got %v", reuse)
	}
}
