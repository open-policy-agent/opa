// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/pkg/errors"
)

type (
	// FunctionalBuiltin1 defines an interface for simple functional built-ins.
	//
	// Implement this interface if your built-in function takes one input and
	// produces one output.
	//
	// If an error occurs, the functional built-in should return a descriptive
	// message. The message should not be prefixed with the built-in name as the
	// framework takes care of this.
	FunctionalBuiltin1 func(op1 ast.Value) (output ast.Value, err error)

	// FunctionalBuiltin2 defines an interface for simple functional built-ins.
	//
	// Implement this interface if your built-in function takes two inputs and
	// produces one output.
	//
	// If an error occurs, the functional built-in should return a descriptive
	// message. The message should not be prefixed with the built-in name as the
	// framework takes care of this.
	FunctionalBuiltin2 func(op1, op2 ast.Value) (output ast.Value, err error)

	// FunctionalBuiltin3 defines an interface for simple functional built-ins.
	//
	// Implement this interface if your built-in function takes three inputs and
	// produces one output.
	//
	// If an error occurs, the functional built-in should return a descriptive
	// message. The message should not be prefixed with the built-in name as the
	// framework takes care of this.
	FunctionalBuiltin3 func(op1, op2, op3 ast.Value) (output ast.Value, err error)

	// FunctionalBuiltinVoid2 defines an interface for simple functional built-ins.
	//
	// Implement this interface if your built-in function takes two inputs and
	// produces no outputs.
	//
	// If an error occurs, the functional built-in should return a descriptive
	// message. The message should not be prefixed with the built-in name as the
	// framework takes care of this.
	FunctionalBuiltinVoid2 func(op1, op2 ast.Value) (err error)

	// BuiltinFunc defines the interface that the evaluation engine uses to
	// invoke built-in functions (built-ins). In most cases, custom built-ins
	// can be implemented using the FunctionalBuiltin interfaces (which provide
	// less control but are much simpler).
	//
	// Users can implement their own built-ins and register them with OPA.
	//
	// Built-ins are given the current evaluation context t with the expression expr
	// to be evaluated. Built-ins can assume that the expression has been plugged
	// with bindings from the current context however references to base documents
	// will not have been resolved. If the built-in determines that the expression
	// has evaluated successfully it should bind any output variables and invoke the
	// iterator with the context produced by binding the output variables. Built-ins
	// must be sure to unbind the outputs after the iterator returns.
	BuiltinFunc func(t *Topdown, expr *ast.Expr, iter Iterator) (err error)
)

// RegisterBuiltinFunc adds a new built-in function to the evaluation engine.
func RegisterBuiltinFunc(name ast.String, fun BuiltinFunc) {
	builtinFunctions[name] = fun
}

// RegisterFunctionalBuiltinVoid2 adds a new built-in function to the evaluation
// engine.
func RegisterFunctionalBuiltinVoid2(name ast.String, fun FunctionalBuiltinVoid2) {
	builtinFunctions[name] = functionalWrapperVoid2(name, fun)
}

// RegisterFunctionalBuiltin1 adds a new built-in function to the evaluation
// engine.
func RegisterFunctionalBuiltin1(name ast.String, fun FunctionalBuiltin1) {
	builtinFunctions[name] = functionalWrapper1(name, fun)
}

// RegisterFunctionalBuiltin2 adds a new built-in function to the evaluation
// engine.
func RegisterFunctionalBuiltin2(name ast.String, fun FunctionalBuiltin2) {
	builtinFunctions[name] = functionalWrapper2(name, fun)
}

// RegisterFunctionalBuiltin3 adds a new built-in function to the evaluation
// engine.
func RegisterFunctionalBuiltin3(name ast.String, fun FunctionalBuiltin3) {
	builtinFunctions[name] = functionalWrapper3(name, fun)
}

// BuiltinEmpty is used to signal that the built-in function evaluated, but the
// result is undefined so evaluation should not continue.
type BuiltinEmpty struct{}

func (BuiltinEmpty) Error() string {
	return "<empty>"
}

var builtinFunctions = map[ast.String]BuiltinFunc{}

func functionalWrapperVoid2(name ast.String, fn FunctionalBuiltinVoid2) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		operands := expr.Terms.([]*ast.Term)[1:]
		resolved, err := resolveN(t, name, operands, 2)
		if err != nil {
			return err
		}
		err = fn(resolved[0], resolved[1])
		if err == nil {
			return iter(t)
		}
		return handleFunctionalBuiltinErr(name, expr.Location, err)
	}
}

func functionalWrapper1(name ast.String, fn FunctionalBuiltin1) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		operands := expr.Terms.([]*ast.Term)[1:]
		resolved, err := resolveN(t, name, operands, 1)
		if err != nil {
			return err
		}
		result, err := fn(resolved[0])
		if err != nil {
			return handleFunctionalBuiltinErr(name, expr.Location, err)
		}
		return unifyAndContinue(t, iter, result, operands[1].Value)
	}
}

func functionalWrapper2(name ast.String, fn FunctionalBuiltin2) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		operands := expr.Terms.([]*ast.Term)[1:]
		resolved, err := resolveN(t, name, operands, 2)
		if err != nil {
			return err
		}
		result, err := fn(resolved[0], resolved[1])
		if err != nil {
			return handleFunctionalBuiltinErr(name, expr.Location, err)
		}
		return unifyAndContinue(t, iter, result, operands[2].Value)
	}
}

func functionalWrapper3(name ast.String, fn FunctionalBuiltin3) BuiltinFunc {
	return func(t *Topdown, expr *ast.Expr, iter Iterator) error {
		operands := expr.Terms.([]*ast.Term)[1:]
		resolved, err := resolveN(t, name, operands, 3)
		if err != nil {
			return err
		}
		result, err := fn(resolved[0], resolved[1], resolved[2])
		if err != nil {
			return handleFunctionalBuiltinErr(name, expr.Location, err)
		}
		return unifyAndContinue(t, iter, result, operands[3].Value)
	}
}

func handleFunctionalBuiltinErr(name ast.String, loc *ast.Location, err error) error {
	switch err := err.(type) {
	case BuiltinEmpty:
		return nil
	case builtins.ErrOperand:
		return &Error{
			Code:     TypeErr,
			Message:  fmt.Sprintf("%v: %v", string(name), err.Error()),
			Location: loc,
		}
	default:
		return &Error{
			Code:     InternalErr,
			Message:  fmt.Sprintf("%v: %v", string(name), err.Error()),
			Location: loc,
		}
	}
}

func resolveN(t *Topdown, name ast.String, ops []*ast.Term, n int) ([]ast.Value, error) {
	result := make([]ast.Value, n)
	for i := 0; i < n; i++ {
		op, err := ResolveRefs(ops[i].Value, t)
		if err != nil {
			return nil, errors.Wrapf(err, "resolving operand %v of %v", i+1, name)
		}
		result[i] = op
	}
	return result, nil
}

func unifyAndContinue(t *Topdown, iter Iterator, result, output ast.Value) error {
	undo, err := evalEqUnify(t, result, output, nil, iter)
	t.Unbind(undo)
	return err
}
