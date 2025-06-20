// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
)

func builtinTypeName(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	switch operands[0].Value.(type) {
	case ast.Null:
		return iter(ast.InternedTerm("null"))
	case ast.Boolean:
		return iter(ast.InternedTerm("boolean"))
	case ast.Number:
		return iter(ast.InternedTerm("number"))
	case ast.String:
		return iter(ast.InternedTerm("string"))
	case *ast.Array:
		return iter(ast.InternedTerm("array"))
	case ast.Object:
		return iter(ast.InternedTerm("object"))
	case ast.Set:
		return iter(ast.InternedTerm("set"))
	}

	return errors.New("illegal value")
}

func init() {
	RegisterBuiltinFunc(ast.TypeNameBuiltin.Name, builtinTypeName)
}
