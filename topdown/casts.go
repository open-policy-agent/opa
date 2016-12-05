// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

// evalToNumber implements the BuiltinFunc type to provide support for casting
// values to numbers. For details on the signature, see builtins.go. This
// function will only be called by evaluation enigne when the expression refers
// to the "to_number" built-in.
func evalToNumber(ctx *Context, expr *ast.Expr, iter Iterator) error {

	// Step 1. unpack the expression to get access to the operands. The Terms
	// contains the parsed expression: ["to_number", <value>, <output>]
	ops := expr.Terms.([]*ast.Term)
	a, b := ops[1].Value, ops[2].Value

	// Step 2. convert input to native Go value. This is a common step for
	// built-ins that are mostly pass-through to Go standard library functions.
	// ValueToInterface will handle references contained inside the value. If
	// your built-in function is specific to certain types (e.g., strings) see
	// the other variants of the ValueToInterface function at
	// https://godoc.org/github.com/open-policy-agent/opa/topdown.
	x, err := ValueToInterface(a, ctx)
	if err != nil {
		return errors.Wrapf(err, "to_number")
	}

	// Step 3. conversion logic. This logic is specific to this built-in.
	var n ast.Number

	switch x := x.(type) {
	case string:
		_, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return errors.Wrapf(err, "to_number")
		}
		n = ast.Number(json.Number(x))
	case json.Number:
		n = ast.Number(x)
	case bool:
		if x {
			n = ast.Number("1")
		} else {
			n = ast.Number("0")
		}
	default:
		return fmt.Errorf("to_number: source must be a string, boolean, or number: %T", a)
	}

	// Step 4. unify the result with the output value. If the output value is
	// ground then the unification will act as an equality test. If the output
	// value is non-ground (e.g., a variable), the unification will bind the
	// result to the variable. The unification will also invoke the iterator to
	// continue evaluation when necessary.
	//
	// Alternatively, if your built-in has no outputs, just call the iterator if
	// the expression evaluated successfully, for example:
	//
	//	success := <logic>
	//
	//  if success {
	// 	  return iter(ctx)
	//  }
	//
	//  return nil
	undo, err := evalEqUnify(ctx, n, b, nil, iter)

	// Step 5. at this point, evaluation is backtracking so the bindings must be undone.
	ctx.Unbind(undo)

	// Step 6. finished, return error (which may be nil).
	return err
}
