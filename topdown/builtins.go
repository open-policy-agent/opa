// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

type (
	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin1 = v1.FunctionalBuiltin1 //nolint:staticcheck // SA1019: Intentional use of deprecated type.

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin2 = v1.FunctionalBuiltin2 //nolint:staticcheck // SA1019: Intentional use of deprecated type.

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin3 = v1.FunctionalBuiltin3 //nolint:staticcheck // SA1019: Intentional use of deprecated type.

	// Deprecated: Functional-style builtins are deprecated. Use BuiltinFunc instead.
	FunctionalBuiltin4 = v1.FunctionalBuiltin4 //nolint:staticcheck // SA1019: Intentional use of deprecated type.

	// BuiltinContext contains context from the evaluator that may be used by
	// built-in functions.
	BuiltinContext = v1.BuiltinContext

	// BuiltinFunc defines an interface for implementing built-in functions.
	// The built-in function is called with the plugged operands from the call
	// (including the output operands.) The implementation should evaluate the
	// operands and invoke the iterator for each successful/defined output
	// value.
	BuiltinFunc = v1.BuiltinFunc
)

// RegisterBuiltinFunc adds a new built-in function to the evaluation engine.
func RegisterBuiltinFunc(name string, f BuiltinFunc) {
	v1.RegisterBuiltinFunc(name, f)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin1(name string, fun FunctionalBuiltin1) {
	v1.RegisterFunctionalBuiltin1(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin2(name string, fun FunctionalBuiltin2) {
	v1.RegisterFunctionalBuiltin2(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin3(name string, fun FunctionalBuiltin3) {
	v1.RegisterFunctionalBuiltin3(name, fun)
}

// Deprecated: Functional-style builtins are deprecated. Use RegisterBuiltinFunc instead.
func RegisterFunctionalBuiltin4(name string, fun FunctionalBuiltin4) {
	v1.RegisterFunctionalBuiltin4(name, fun)
}

// GetBuiltin returns a built-in function implementation, nil if no built-in found.
func GetBuiltin(name string) BuiltinFunc {
	return v1.GetBuiltin(name)
}

// Deprecated: The BuiltinEmpty type is no longer needed. Use nil return values instead.
type BuiltinEmpty = v1.Builtin
