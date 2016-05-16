// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// Builtins is the registry of built-in functions supported by
// OPA. When adding a new built-in function to OPA, update this
// list.
var Builtins = [...]*Builtin{
	Equality,
}

// BuiltinMap provides a convenient mapping of built-in names to
// built-in definitions.
var BuiltinMap map[Var]*Builtin

// Equality represents the "=" operator/function.
var Equality = &Builtin{
	Name:         Var("="),
	NumArgs:      2,
	RecTargetPos: []int{0, 1},
}

// Builtin represents a built-in function supported by OPA. Every
// built-in function is uniquely identified by a name.
type Builtin struct {
	Name         Var
	NumArgs      int
	TargetPos    []int
	RecTargetPos []int
}

// Unifies returns true if a term in the given position will unify
// non-recursively or recursively.
func (b *Builtin) Unifies(i int) bool {
	for _, x := range b.TargetPos {
		if x == i {
			return true
		}
	}
	return b.UnifiesRecursively(i)
}

// UnifiesRecursively returns true if a term in the given position will
// unify recursively, i.e., variables embedded inside a collection type
// will unify.
func (b *Builtin) UnifiesRecursively(i int) bool {
	for _, x := range b.RecTargetPos {
		if x == i {
			return true
		}
	}
	return false
}

func init() {
	BuiltinMap = map[Var]*Builtin{}
	for _, b := range Builtins {
		BuiltinMap[b.Name] = b
	}
}
