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

	var count int
	err = builtinPrintCrossProductOperands(bctx.Location, buf, arr, 0, func(buf []string) error {
		count += 1
		// Precautionary run-time assertion that template-strings can't produce multiple outputs; e.g. for custom relation type built-ins not known at compile-time.
		if count > 1 {
			return Halt{Err: &Error{
				Code:     ConflictErr,
				Location: bctx.Location,
				Message:  "template-strings must not produce multiple outputs",
			}}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return iter(ast.StringTerm(strings.Join(buf, "")))
}

func init() {
	RegisterBuiltinFunc(ast.InternalTemplateString.Name, builtinTemplateString)
}
