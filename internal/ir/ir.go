// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package ir defines an intermediate representation (IR) for Rego.
//
// The IR specifies an imperative execution model for Rego policies similar to a
// query plan in traditional databases.
package ir

import (
	"fmt"
)

type (
	// Policy represents a planned policy query.
	Policy struct {
		Static Static
		Plan   Plan
	}

	// Static represents a static data segment that is indexed into by the policy.
	Static struct {
		Strings []StringConst
	}

	// Plan represents an ordered series of blocks to execute. All plans contain a
	// final block that returns indicating the plan result was undefined. Plan
	// execution stops when a block returns a value. Blocks are executed in-order.
	Plan struct {
		Blocks []Block
	}

	// Block represents an ordered sequence of statements to execute. Blocks are
	// executed until a return statement is encountered, a statement is undefined,
	// or there are no more statements. If all statements are defined but no return
	// statement is encountered, the block is undefined.
	Block struct {
		Stmts []Stmt
	}

	// Stmt represents an operation (e.g., comparison, loop, dot, etc.) to execute.
	Stmt interface {
	}

	// Local represents a plan-scoped variable.
	Local int

	// Const represents a constant value from the policy.
	Const interface {
		typeMarker()
	}

	// NullConst represents a null value.
	NullConst struct{}

	// BooleanConst represents a boolean value.
	BooleanConst struct {
		Value bool
	}

	// StringConst represents a string value.
	StringConst struct {
		Value string
	}

	// IntConst represents an integer constant.
	IntConst struct {
		Value int64
	}

	// FloatConst represents a floating-point constant.
	FloatConst struct {
		Value float64
	}
)

const (
	// Undefined represents an undefined return value. An undefined return value
	// indicates the policy did not return a definitive answer.
	Undefined int32 = iota

	// Defined represents a defined return value.
	Defined

	// Error indicates a runtime error occurred during evaluation.
	Error
)

const (
	// InputRaw refers to the local variable containing the address of the raw
	// (serialized) input data.
	InputRaw Local = 0

	// InputLen refers to the local variable containing the length of the raw input.
	InputLen Local = 1

	// Input refers to the local variable containing the address of the deserialized
	// input value.
	Input Local = 2
)

func (a Policy) String() string {
	return "Policy"
}

func (a Static) String() string {
	return fmt.Sprintf("Static (%d strings)", len(a.Strings))
}

func (a Plan) String() string {
	return fmt.Sprintf("Plan (%d blocks)", len(a.Blocks))
}

func (a Block) String() string {
	return fmt.Sprintf("Block (%d statements)", len(a.Stmts))
}

func (a BooleanConst) typeMarker() {}
func (a NullConst) typeMarker()    {}
func (a IntConst) typeMarker()     {}
func (a FloatConst) typeMarker()   {}
func (a StringConst) typeMarker()  {}

// ReturnStmt represents a return statement. Return statements halt execution of
// a plan with the given code.
type ReturnStmt struct {
	Code int32 // 32-bit integer for compatibility with languages like JavaScript.
}

// DotStmt represents a lookup operation on a value (e.g., array, object, etc.)
// The source of a DotStmt may be a scalar value in which case the statement
// will be undefined.
type DotStmt struct {
	Source Local
	Key    Local
	Target Local
}

// LoopStmt represents a loop operation on a composite value. The source of a
// LoopStmt may be a scalar in which case the statement will be undefined.
type LoopStmt struct {
	Source Local
	Key    Local
	Value  Local
	Cond   Local
	Block  Block
}

// AssignStmt represents an assignment of a local variable.
type AssignStmt struct {
	Value  Const
	Target Local
}

// MakeStringStmt constructs a local variable that refers to a string constant.
type MakeStringStmt struct {
	Index  int
	Target Local
}

// MakeBooleanStmt constructs a local variable that refers to a boolean value.
type MakeBooleanStmt struct {
	Value  bool
	Target Local
}

// MakeNumberIntStmt constructs a local variable that refers to an integer value.
type MakeNumberIntStmt struct {
	Value  int64
	Target Local
}

// EqualStmt represents an value-equality check of two local variables.
type EqualStmt struct {
	A Local
	B Local
}

// LessThanStmt represents a < check of two local variables.
type LessThanStmt struct {
	A Local
	B Local
}

// LessThanEqualStmt represents a <= check of two local variables.
type LessThanEqualStmt struct {
	A Local
	B Local
}

// GreaterThanStmt represents a > check of two local variables.
type GreaterThanStmt struct {
	A Local
	B Local
}

// GreaterThanEqualStmt represents a >= check of two local variables.
type GreaterThanEqualStmt struct {
	A Local
	B Local
}

// NotEqualStmt represents a != check of two local variables.
type NotEqualStmt struct {
	A Local
	B Local
}
