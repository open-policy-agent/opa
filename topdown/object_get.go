package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinObjectGet(a, b, c ast.Value) (ast.Value, error) {
	object, err := builtins.ObjectOperand(a, 1)
	if err != nil {
		return nil, err
	}

	if ret := object.Get(&ast.Term{Value: b}); ret != nil {
		return ret.Value, nil
	}

	return c, nil
}

func init() {
	RegisterFunctionalBuiltin3(ast.ObjectGet.Name, builtinObjectGet)
}
