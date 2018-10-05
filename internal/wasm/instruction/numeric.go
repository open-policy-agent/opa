// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package instruction

import (
	"github.com/open-policy-agent/opa/internal/wasm/opcode"
)

// I32Const represents the WASM i32.const instruction.
type I32Const struct {
	Value int32
}

// Op returns the opcode of the instruction.
func (I32Const) Op() opcode.Opcode {
	return opcode.I32Const
}

// ImmediateArgs returns the i32 value to push onto the stack.
func (i I32Const) ImmediateArgs() []interface{} {
	return []interface{}{i.Value}
}

// I64Const represents the WASM i64.const instruction.
type I64Const struct {
	Value int64
}

// Op returns the opcode of the instruction.
func (I64Const) Op() opcode.Opcode {
	return opcode.I64Const
}

// ImmediateArgs returns the i64 value to push onto the stack.
func (i I64Const) ImmediateArgs() []interface{} {
	return []interface{}{i.Value}
}

// I32Eqz represents the WASM i32.eqz instruction.
type I32Eqz struct {
	NoImmediateArgs
}

// Op returns the opcode of the instruction.
func (I32Eqz) Op() opcode.Opcode {
	return opcode.I32Eqz
}
