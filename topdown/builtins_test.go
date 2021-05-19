package topdown

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
)

func TestCustomBuiltinIterator(t *testing.T) {

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

	ctx := context.Background()

	rs, err := query.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected one result but got:", rs)
	} else if !rs[0][ast.Var("x")].Equal(ast.IntNumberTerm(2)) {
		t.Fatal("Expected x to be 2 but got:", rs[0])
	}
}
