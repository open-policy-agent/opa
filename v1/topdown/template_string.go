// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func builtinTemplateString(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	arr, err := builtins.ArrayOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	buf := make([]string, arr.Len())

	err = builtinPrintCrossProductOperands(bctx, buf, arr, 0, func(buf []string) error {
		return nil
	})

	if err != nil {
		return err
	}

	str := ast.StringTerm(strings.Join(buf, ""))
	return iter(str)
}

func init() {
	RegisterBuiltinFunc(ast.InternalTemplateString.Name, builtinTemplateString)
}
