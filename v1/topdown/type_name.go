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
		return iter(ast.InternedStringTerm("null"))
	case ast.Boolean:
		return iter(ast.InternedStringTerm("boolean"))
	case ast.Number:
		return iter(ast.InternedStringTerm("number"))
	case ast.String:
		return iter(ast.InternedStringTerm("string"))
	case *ast.Array:
		return iter(ast.InternedStringTerm("array"))
	case ast.Object:
		return iter(ast.InternedStringTerm("object"))
	case ast.Set:
		return iter(ast.InternedStringTerm("set"))
	}

	return errors.New("illegal value")
}

func init() {
	RegisterBuiltinFunc(ast.TypeNameBuiltin.Name, builtinTypeName)
}
