// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package ir defines an intermediate representation (IR) for Rego.
//
// The IR specifies an imperative execution model for Rego policies similar to a
// query plan in traditional databases.
package ir

import (
	v1 "github.com/open-policy-agent/opa/v1/ir"
)

type (
	// Policy represents a planned policy query.
	Policy = v1.Policy

	// Static represents a static data segment that is indexed into by the policy.
	Static = v1.Static

	// BuiltinFunc represents a built-in function that may be required by the
	// policy.
	BuiltinFunc = v1.BuiltinFunc

	// Plans represents a collection of named query plans to expose in the policy.
	Plans = v1.Plans

	// Funcs represents a collection of planned functions to include in the
	// policy.
	Funcs = v1.Funcs

	// Func represents a named plan (function) that can be invoked. Functions
	// accept one or more parameters and return a value. By convention, the
	// input document and data documents are always passed as the first and
	// second arguments (respectively).
	Func = v1.Func

	// Plan represents an ordered series of blocks to execute. Plan execution
	// stops when a return statement is reached. Blocks are executed in-order.
	Plan = v1.Plan

	// Block represents an ordered sequence of statements to execute. Blocks are
	// executed until a return statement is encountered, a statement is undefined,
	// or there are no more statements. If all statements are defined but no return
	// statement is encountered, the block is undefined.
	Block = v1.Block

	// Stmt represents an operation (e.g., comparison, loop, dot, etc.) to execute.
	Stmt = v1.Stmt

	// Local represents a plan-scoped variable.
	//
	// TODO(tsandall): should this be int32 for safety?
	Local = v1.Local

	// StringConst represents a string value.
	StringConst = v1.StringConst
)

const (
	// Input is the local variable that refers to the global input document.
	Input = v1.Input

	// Data is the local variable that refers to the global data document.
	Data = v1.Data

	// Unused is the free local variable that can be allocated in a plan.
	Unused = v1.Unused
)

// Operand represents a value that a statement operates on.
type Operand = v1.Operand

// Val represents an abstract value that statements operate on. There are currently
// 3 types of values:
//
// 1. Local - a local variable that can refer to any type.
// 2. StringIndex - a string constant that refers to a compiled string.
// 3. Bool - a boolean constant.
type Val = v1.Val

// StringIndex represents the index into the plan's list of constant strings
// of a constant string.
type StringIndex = v1.StringIndex

// Bool represents a constant boolean.
type Bool = v1.Bool

// ReturnLocalStmt represents a return statement that yields a local value.
type ReturnLocalStmt = v1.ReturnLocalStmt

// CallStmt represents a named function call. The result should be stored in the
// result local.
type CallStmt = v1.CallStmt

// CallDynamicStmt represents an indirect (data) function call. The result should
// be stored in the result local.
type CallDynamicStmt = v1.CallDynamicStmt

// BlockStmt represents a nested block. Nested blocks and break statements can
// be used to short-circuit execution.
type BlockStmt = v1.BlockStmt

// BreakStmt represents a jump out of the current block. The index specifies how
// many blocks to jump starting from zero (the current block). Execution will
// continue from the end of the block that is jumped to.
type BreakStmt = v1.BreakStmt

// DotStmt represents a lookup operation on a value (e.g., array, object, etc.)
// The source of a DotStmt may be a scalar value in which case the statement
// will be undefined.
type DotStmt = v1.DotStmt

// LenStmt represents a length() operation on a local variable. The
// result is stored in the target local variable.
type LenStmt = v1.LenStmt

// ScanStmt represents a linear scan over a composite value. The
// source may be a scalar in which case the block will never execute.
type ScanStmt = v1.ScanStmt

// NotStmt represents a negated statement.
type NotStmt = v1.NotStmt

// AssignIntStmt represents an assignment of an integer value to a
// local variable.
type AssignIntStmt = v1.AssignIntStmt

// AssignVarStmt represents an assignment of one local variable to another.
type AssignVarStmt = v1.AssignVarStmt

// AssignVarOnceStmt represents an assignment of one local variable to another.
// If the target is defined, execution aborts with a conflict error.
//
// TODO(tsandall): is there a better name for this?
type AssignVarOnceStmt = v1.AssignVarOnceStmt

// ResetLocalStmt resets a local variable to 0.
type ResetLocalStmt = v1.ResetLocalStmt

// MakeNullStmt constructs a local variable that refers to a null value.
type MakeNullStmt = v1.MakeNullStmt

// MakeNumberIntStmt constructs a local variable that refers to an integer value.
type MakeNumberIntStmt = v1.MakeNumberIntStmt

// MakeNumberRefStmt constructs a local variable that refers to a number stored as a string.
type MakeNumberRefStmt = v1.MakeNumberRefStmt

// MakeArrayStmt constructs a local variable that refers to an array value.
type MakeArrayStmt = v1.MakeArrayStmt

// MakeObjectStmt constructs a local variable that refers to an object value.
type MakeObjectStmt = v1.MakeObjectStmt

// MakeSetStmt constructs a local variable that refers to a set value.
type MakeSetStmt = v1.MakeSetStmt

// EqualStmt represents an value-equality check of two local variables.
type EqualStmt = v1.EqualStmt

// NotEqualStmt represents a != check of two local variables.
type NotEqualStmt = v1.NotEqualStmt

// IsArrayStmt represents a dynamic type check on a local variable.
type IsArrayStmt = v1.IsArrayStmt

// IsObjectStmt represents a dynamic type check on a local variable.
type IsObjectStmt = v1.IsObjectStmt

// IsSetStmt represents a dynamic type check on a local variable.
type IsSetStmt = v1.IsSetStmt

// IsDefinedStmt represents a check of whether a local variable is defined.
type IsDefinedStmt = v1.IsDefinedStmt

// IsUndefinedStmt represents a check of whether local variable is undefined.
type IsUndefinedStmt = v1.IsUndefinedStmt

// ArrayAppendStmt represents a dynamic append operation of a value
// onto an array.
type ArrayAppendStmt = v1.ArrayAppendStmt

// ObjectInsertStmt represents a dynamic insert operation of a
// key/value pair into an object.
type ObjectInsertStmt = v1.ObjectInsertStmt

// ObjectInsertOnceStmt represents a dynamic insert operation of a key/value
// pair into an object. If the key already exists and the value differs,
// execution aborts with a conflict error.
type ObjectInsertOnceStmt = v1.ObjectInsertOnceStmt

// ObjectMergeStmt performs a recursive merge of two object values. If either of
// the locals refer to non-object values this operation will abort with a
// conflict error. Overlapping object keys are merged recursively.
type ObjectMergeStmt = v1.ObjectMergeStmt

// SetAddStmt represents a dynamic add operation of an element into a set.
type SetAddStmt = v1.SetAddStmt

// WithStmt replaces the Local or a portion of the document referred to by the
// Local with the Value and executes the contained block. If the Path is
// non-empty, the Value is upserted into the Local. If the intermediate nodes in
// the Local referred to by the Path do not exist, they will be created. When
// the WithStmt finishes the Local is reset to it's original value.
type WithStmt = v1.WithStmt

// NopStmt adds a nop instruction. Useful during development and debugging only.
type NopStmt = v1.NopStmt

// ResultSetAddStmt adds a value into the result set returned by the query plan.
type ResultSetAddStmt = v1.ResultSetAddStmt

// Location records the filen index, and the row and column inside that file
// that a statement can be connected to.
type Location = v1.Location
