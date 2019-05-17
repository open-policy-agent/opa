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
		Static *Static
		Plan   *Plan
		Funcs  *Funcs
	}

	// Static represents a static data segment that is indexed into by the policy.
	Static struct {
		Strings []*StringConst
	}

	// Funcs represents a collection of planned functions to include in the
	// policy.
	Funcs struct {
		Funcs map[string]*Func
	}

	// Func represents a named plan (function) that can be invoked. Functions
	// accept one or more parameters and return a value. By convention, the
	// input document is always passed as the first argument.
	Func struct {
		Name   string
		Params []Local
		Return Local
		Blocks []*Block // TODO(tsandall): should this be a plan?
	}

	// Plan represents an ordered series of blocks to execute. All plans contain a
	// final block that returns indicating the plan result was undefined. Plan
	// execution stops when a block returns a value. Blocks are executed in-order.
	Plan struct {
		Blocks []*Block
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
	//
	// TODO(tsandall): should this be int32 for safety?
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

func (a *Policy) String() string {
	return "Policy"
}

func (a *Static) String() string {
	return fmt.Sprintf("Static (%d strings)", len(a.Strings))
}

func (a *Funcs) String() string {
	return fmt.Sprintf("Funcs (%d funcs)", len(a.Funcs))
}

func (a *Func) String() string {
	return fmt.Sprintf("%v (%d params: %v, %d blocks)", a.Name, len(a.Params), a.Params, len(a.Blocks))
}

func (a *Plan) String() string {
	return fmt.Sprintf("Plan (%d blocks)", len(a.Blocks))
}

func (a *Block) String() string {
	return fmt.Sprintf("Block (%d statements)", len(a.Stmts))
}

func (a *BooleanConst) typeMarker() {}
func (a *NullConst) typeMarker()    {}
func (a *IntConst) typeMarker()     {}
func (a *FloatConst) typeMarker()   {}
func (a *StringConst) typeMarker()  {}

// ReturnStmt represents a return statement. Return statements halt execution of
// a plan with the given code.
type ReturnStmt struct {
	Code int32 // 32-bit integer for compatibility with languages like JavaScript.
}

// ReturnLocalStmt represents a return statement that yields a local value.
type ReturnLocalStmt struct {
	Source Local
}

// CallStmt represents a named function call. The result should be stored in the
// result local.
type CallStmt struct {
	Func   string
	Args   []Local
	Result Local
}

// BlockStmt represents a nested block. Nested blocks and break statements can
// be used to short-circuit execution.
type BlockStmt struct {
	Blocks []*Block
}

func (a *BlockStmt) String() string {
	return fmt.Sprintf("BlockStmt (%d blocks)", len(a.Blocks))
}

// BreakStmt represents a jump out of the current block. The index specifies how
// many blocks to jump starting from zero (the current block). Execution will
// continue from the end of the block that is jumped to.
type BreakStmt struct {
	Index uint32
}

// DotStmt represents a lookup operation on a value (e.g., array, object, etc.)
// The source of a DotStmt may be a scalar value in which case the statement
// will be undefined.
type DotStmt struct {
	Source Local
	Key    Local
	Target Local
}

// LenStmt represents a length() operation on a local variable. The
// result is stored in the target local variable.
type LenStmt struct {
	Source Local
	Target Local
}

// ScanStmt represents a linear scan over a composite value. The
// source may be a scalar in which case the block will never execute.
type ScanStmt struct {
	Source Local
	Key    Local
	Value  Local
	Block  *Block
}

// NotStmt represents a negated statement. The last statement in the negation
// block will set the condition to false.
type NotStmt struct {
	Cond  Local
	Block *Block
}

// AssignBooleanStmt represents an assignment of a boolean value to a local variable.
type AssignBooleanStmt struct {
	Value  bool
	Target Local
}

// AssignIntStmt represents an assignment of an integer value to a
// local variable.
type AssignIntStmt struct {
	Value  int64
	Target Local
}

// AssignVarStmt represents an assignment of one local variable to another.
type AssignVarStmt struct {
	Source Local
	Target Local
}

// AssignVarOnceStmt represents an assignment of one local variable to another.
// If the target is defined, execution aborts with a conflict error.
//
// TODO(tsandall): is there a better name for this?
type AssignVarOnceStmt struct {
	Target Local
	Source Local
}

// MakeStringStmt constructs a local variable that refers to a string constant.
type MakeStringStmt struct {
	Index  int
	Target Local
}

// MakeNullStmt constructs a local variable that refers to a null value.
type MakeNullStmt struct {
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

// MakeArrayStmt constructs a local variable that refers to an array value.
type MakeArrayStmt struct {
	Capacity int32
	Target   Local
}

// MakeObjectStmt constructs a local variable that refers to an object value.
type MakeObjectStmt struct {
	Target Local
}

// MakeSetStmt constructs a local variable that refers to a set value.
type MakeSetStmt struct {
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

// IsArrayStmt represents a dynamic type check on a local variable.
type IsArrayStmt struct {
	Source Local
}

// IsObjectStmt represents a dynamic type check on a local variable.
type IsObjectStmt struct {
	Source Local
}

// IsDefinedStmt represents a check of whether a local variable is defined.
type IsDefinedStmt struct {
	Source Local
}

// IsUndefinedStmt represents a check of whether local variable is undefined.
type IsUndefinedStmt struct {
	Source Local
}

// ArrayAppendStmt represents a dynamic append operation of a value
// onto an array.
type ArrayAppendStmt struct {
	Value Local
	Array Local
}

// ObjectInsertStmt represents a dynamic insert operation of a
// key/value pair into an object.
type ObjectInsertStmt struct {
	Key    Local
	Value  Local
	Object Local
}

// ObjectInsertOnceStmt represents a dynamic insert operation of a key/value
// pair into an object. If the key already exists and the value differs,
// execution aborts with a conflict error.
type ObjectInsertOnceStmt struct {
	Key    Local
	Value  Local
	Object Local
}

// SetAddStmt represents a dynamic add operation of an element into a set.
type SetAddStmt struct {
	Value Local
	Set   Local
}
