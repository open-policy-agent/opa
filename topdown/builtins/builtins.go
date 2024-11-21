// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package builtins contains utilities for implementing built-in functions.
package builtins

import (
	"math/big"

	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/topdown/builtins"
)

// Cache defines the built-in cache used by the top-down evaluation. The keys
// must be comparable and should not be of type string.
type Cache = v1.Cache

// We use an ast.Object for the cached keys/values because a naive
// map[ast.Value]ast.Value will not correctly detect value equality of
// the member keys.
type NDBCache = v1.NDBCache

// ErrOperand represents an invalid operand has been passed to a built-in
// function. Built-ins should return ErrOperand to indicate a type error has
// occurred.
type ErrOperand = v1.ErrOperand

// NewOperandErr returns a generic operand error.
func NewOperandErr(pos int, f string, a ...interface{}) error {
	return v1.NewOperandErr(pos, f, a...)
}

// NewOperandTypeErr returns an operand error indicating the operand's type was wrong.
func NewOperandTypeErr(pos int, got ast.Value, expected ...string) error {
	return v1.NewOperandTypeErr(pos, got, expected...)
}

// NewOperandElementErr returns an operand error indicating an element in the
// composite operand was wrong.
func NewOperandElementErr(pos int, composite ast.Value, got ast.Value, expected ...string) error {
	return v1.NewOperandElementErr(pos, composite, got, expected...)
}

// NewOperandEnumErr returns an operand error indicating a value was wrong.
func NewOperandEnumErr(pos int, expected ...string) error {
	return v1.NewOperandEnumErr(pos, expected...)
}

// IntOperand converts x to an int. If the cast fails, a descriptive error is
// returned.
func IntOperand(x ast.Value, pos int) (int, error) {
	return v1.IntOperand(x, pos)
}

// BigIntOperand converts x to a big int. If the cast fails, a descriptive error
// is returned.
func BigIntOperand(x ast.Value, pos int) (*big.Int, error) {
	return v1.BigIntOperand(x, pos)
}

// NumberOperand converts x to a number. If the cast fails, a descriptive error is
// returned.
func NumberOperand(x ast.Value, pos int) (ast.Number, error) {
	return v1.NumberOperand(x, pos)
}

// SetOperand converts x to a set. If the cast fails, a descriptive error is
// returned.
func SetOperand(x ast.Value, pos int) (ast.Set, error) {
	return v1.SetOperand(x, pos)
}

// StringOperand converts x to a string. If the cast fails, a descriptive error is
// returned.
func StringOperand(x ast.Value, pos int) (ast.String, error) {
	return v1.StringOperand(x, pos)
}

// ObjectOperand converts x to an object. If the cast fails, a descriptive
// error is returned.
func ObjectOperand(x ast.Value, pos int) (ast.Object, error) {
	return v1.ObjectOperand(x, pos)
}

// ArrayOperand converts x to an array. If the cast fails, a descriptive
// error is returned.
func ArrayOperand(x ast.Value, pos int) (*ast.Array, error) {
	return v1.ArrayOperand(x, pos)
}

// NumberToFloat converts n to a big float.
func NumberToFloat(n ast.Number) *big.Float {
	return v1.NumberToFloat(n)
}

// FloatToNumber converts f to a number.
func FloatToNumber(f *big.Float) ast.Number {
	return v1.FloatToNumber(f)
}

// NumberToInt converts n to a big int.
// If n cannot be converted to an big int, an error is returned.
func NumberToInt(n ast.Number) (*big.Int, error) {
	return v1.NumberToInt(n)
}

// IntToNumber converts i to a number.
func IntToNumber(i *big.Int) ast.Number {
	return v1.IntToNumber(i)
}

// StringSliceOperand converts x to a []string. If the cast fails, a descriptive error is
// returned.
func StringSliceOperand(a ast.Value, pos int) ([]string, error) {
	return v1.StringSliceOperand(a, pos)
}

// RuneSliceOperand converts x to a []rune. If the cast fails, a descriptive error is
// returned.
func RuneSliceOperand(x ast.Value, pos int) ([]rune, error) {
	return v1.RuneSliceOperand(x, pos)
}
