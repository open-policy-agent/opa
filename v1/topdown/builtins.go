// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/topdown/print"
	"github.com/open-policy-agent/opa/v1/tracing"
)

type (
	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin1 func(op1 ast.Value) (output ast.Value, err error)

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin2 func(op1, op2 ast.Value) (output ast.Value, err error)

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin3 func(op1, op2, op3 ast.Value) (output ast.Value, err error)

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin4 func(op1, op2, op3, op4 ast.Value) (output ast.Value, err error)

	// BuiltinContext contains context from the evaluator that may be used by
	// built-in functions.
	BuiltinContext struct {
		InterQueryBuiltinCache      cache.InterQueryCache
		Metrics                     metrics.Metrics
		Seed                        io.Reader
		PrintHook                   print.Hook
		Cancel                      Cancel
		Context                     context.Context
		InterQueryBuiltinValueCache cache.InterQueryValueCache
		Location                    *ast.Location
		Time                        *ast.Term
		NDBuiltinCache              builtins.NDBCache
		Runtime                     *ast.Term
		Cache                       builtins.Cache
		Capabilities                *ast.Capabilities
		rand                        *rand.Rand
		RoundTripper                CustomizeRoundTripper
		Tracers                     []Tracer
		DistributedTracingOpts      tracing.Options
		QueryTracers                []QueryTracer
		ParentID                    uint64
		QueryID                     uint64
		TraceEnabled                bool
	}

	// BuiltinFunc defines an interface for implementing built-in functions.
	// The built-in function is called with the plugged operands from the call
	// (including the output operands.) The implementation should evaluate the
	// operands and invoke the iterator for each successful/defined output
	// value.
	BuiltinFunc func(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error
)

// Rand returns a random number generator based on the Seed for this built-in
// context. The random number will be re-used across multiple calls to this
// function. If a random number generator cannot be created, an error is
// returned.
func (bctx *BuiltinContext) Rand() (*rand.Rand, error) {

	if bctx.rand != nil {
		return bctx.rand, nil
	}

	seed, err := readInt64(bctx.Seed)
	if err != nil {
		return nil, err
	}

	bctx.rand = rand.New(rand.NewSource(seed))
	return bctx.rand, nil
}

// RegisterBuiltinFunc adds a new built-in function to the evaluation engine.
func RegisterBuiltinFunc(name string, f BuiltinFunc) {
	builtinFunctions[name] = builtinErrorWrapper(name, f)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin1(name string, fun FunctionalBuiltin1) {
	builtinFunctions[name] = functionalWrapper1(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin2(name string, fun FunctionalBuiltin2) {
	builtinFunctions[name] = functionalWrapper2(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin3(name string, fun FunctionalBuiltin3) {
	builtinFunctions[name] = functionalWrapper3(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin4(name string, fun FunctionalBuiltin4) {
	builtinFunctions[name] = functionalWrapper4(name, fun)
}

// GetBuiltin returns a built-in function implementation, nil if no built-in found.
func GetBuiltin(name string) BuiltinFunc {
	return builtinFunctions[name]
}

// Deprecated: The BuiltinEmpty type is no longer needed. Use nil return values instead.
type BuiltinEmpty struct{}

func (BuiltinEmpty) Error() string {
	return "<empty>"
}

var builtinFunctions = map[string]BuiltinFunc{}

func builtinErrorWrapper(name string, fn BuiltinFunc) BuiltinFunc {
	return func(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
		err := fn(bctx, args, iter)
		if err == nil {
			return nil
		}
		return handleBuiltinErr(name, bctx.Location, err)
	}
}

func functionalWrapper1(name string, fn FunctionalBuiltin1) BuiltinFunc {
	return func(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
		result, err := fn(args[0].Value)
		if err == nil {
			return iter(ast.NewTerm(result))
		}
		return handleBuiltinErr(name, bctx.Location, err)
	}
}

func functionalWrapper2(name string, fn FunctionalBuiltin2) BuiltinFunc {
	return func(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
		result, err := fn(args[0].Value, args[1].Value)
		if err == nil {
			return iter(ast.NewTerm(result))
		}
		return handleBuiltinErr(name, bctx.Location, err)
	}
}

func functionalWrapper3(name string, fn FunctionalBuiltin3) BuiltinFunc {
	return func(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
		result, err := fn(args[0].Value, args[1].Value, args[2].Value)
		if err == nil {
			return iter(ast.NewTerm(result))
		}
		return handleBuiltinErr(name, bctx.Location, err)
	}
}

func functionalWrapper4(name string, fn FunctionalBuiltin4) BuiltinFunc {
	return func(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
		result, err := fn(args[0].Value, args[1].Value, args[2].Value, args[3].Value)
		if err == nil {
			return iter(ast.NewTerm(result))
		}
		if _, empty := err.(BuiltinEmpty); empty {
			return nil
		}
		return handleBuiltinErr(name, bctx.Location, err)
	}
}

func handleBuiltinErr(name string, loc *ast.Location, err error) error {
	switch err := err.(type) {
	case BuiltinEmpty:
		return nil
	case *Error, Halt:
		return err
	case builtins.ErrOperand:
		e := &Error{
			Code:     TypeErr,
			Message:  fmt.Sprintf("%v: %v", name, err.Error()),
			Location: loc,
		}
		return e.Wrap(err)
	default:
		e := &Error{
			Code:     BuiltinErr,
			Message:  fmt.Sprintf("%v: %v", name, err.Error()),
			Location: loc,
		}
		return e.Wrap(err)
	}
}

func readInt64(r io.Reader) (int64, error) {
	bs := make([]byte, 8)
	n, err := io.ReadFull(r, bs)
	if n != len(bs) || err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(bs)), nil
}

// Used to get older-style (ast.Term, error) tuples out of newer functions.
func getResult(fn BuiltinFunc, operands ...*ast.Term) (*ast.Term, error) {
	var result *ast.Term
	extractionFn := func(r *ast.Term) error {
		result = r
		return nil
	}
	err := fn(BuiltinContext{}, operands, extractionFn)
	if err != nil {
		return nil, err
	}
	return result, nil
}
