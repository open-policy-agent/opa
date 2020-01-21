package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinObjectGet(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	object, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	if ret := object.Get(operands[1]); ret != nil {
		return iter(ret)
	}

	return iter(operands[2])
}

func init() {
	RegisterBuiltinFunc(ast.ObjectGet.Name, builtinObjectGet)
}
