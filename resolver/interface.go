package resolver

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
)

type Resolver interface {
	Eval(context.Context, Input) (Result, error)
}

type Input struct {
	Ref   ast.Ref
	Input *ast.Term
}

type Result struct {
	Value ast.Value
}
