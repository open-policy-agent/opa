package topdown

import (
	"context"
	"errors"
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

func TestTopdownWithBuiltinErrors(t *testing.T) {
	var errs []error
	query := NewQuery(ast.MustParseBody(`startswith(input, "foo")`)).
		WithInput(ast.MustParseTerm("{}")).
		WithBuiltinErrors(func(es ...error) { errs = append(errs, es...) })

	ctx := context.Background()

	rs, err := query.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := 0, len(rs); exp != act {
		t.Fatalf("Expected %d results but got: %d", exp, act)
	}
	if exp, act := 1, len(errs); exp != act {
		t.Fatalf("Expected %d errors but got: %d", exp, act)
	}
	var exp *Error
	if act := errs[0]; !errors.As(act, &exp) {
		t.Fatalf("expected %[1]T, got %[2]T: %[2]v", exp, act)
	}
}
