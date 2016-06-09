// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// Builtins is the registry of built-in functions supported by OPA.
// Call RegisterBuiltin to add a new built-in.
var Builtins []*Builtin

// RegisterBuiltin adds a new built-in function to the registry.
func RegisterBuiltin(b *Builtin) {
	Builtins = append(Builtins, b)
	BuiltinMap[b.Name] = b
	ReservedVars.Add(b.Name)
}

// DefaultBuiltins is the registry of built-in functions supported in OPA
// by default. When adding a new built-in function to OPA, update this
// list.
var DefaultBuiltins = [...]*Builtin{
	Equality,
	GreaterThan, GreaterThanEq, LessThan, LessThanEq, NotEqual,
}

// BuiltinMap provides a convenient mapping of built-in names to
// built-in definitions.
var BuiltinMap map[Var]*Builtin

// Equality represents the "=" operator.
var Equality = &Builtin{
	Name:      Var("="),
	Alias:     Var("eq"),
	NumArgs:   2,
	TargetPos: []int{0, 1},
}

// GreaterThan represents the ">" comparison operator.
var GreaterThan = &Builtin{
	Name:    Var(">"),
	Alias:   Var("gt"),
	NumArgs: 2,
}

// GreaterThanEq represents the ">=" comparison operator.
var GreaterThanEq = &Builtin{
	Name:    Var(">="),
	Alias:   Var("gte"),
	NumArgs: 2,
}

// LessThan represents the "<" comparison operator.
var LessThan = &Builtin{
	Name:    Var("<"),
	Alias:   Var("lt"),
	NumArgs: 2,
}

// LessThanEq represents the "<=" comparison operator.
var LessThanEq = &Builtin{
	Name:    Var("<="),
	Alias:   Var("lte"),
	NumArgs: 2,
}

// NotEqual represents the "!=" comparison operator.
var NotEqual = &Builtin{
	Name:    Var("!="),
	Alias:   Var("neq"),
	NumArgs: 2,
}

// Builtin represents a built-in function supported by OPA. Every
// built-in function is uniquely identified by a name.
type Builtin struct {
	Name      Var
	Alias     Var
	NumArgs   int
	TargetPos []int
}

// GetPrintableName returns a printable name for the builtin.
// Some built-ins have names that are used for infix operators
// but when printing we want to use something a bit more readable,
// e.g., "gte(a,b)" instead of ">=(a,b)".
func (b *Builtin) GetPrintableName() string {
	if len(b.Alias) > 0 {
		return b.Alias.String()
	}
	return b.Name.String()
}

// IsTargetPos returns true if a variable in the i-th position will be
// bound when the expression is evaluated.
func (b *Builtin) IsTargetPos(i int) bool {
	for _, x := range b.TargetPos {
		if x == i {
			return true
		}
	}
	return false
}

func init() {
	BuiltinMap = map[Var]*Builtin{}
	for _, b := range DefaultBuiltins {
		RegisterBuiltin(b)
	}
}
