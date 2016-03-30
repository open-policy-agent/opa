// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

// Var is the AST type representing a variable.
type Var struct {
	Name string
}

// NewVar returns a new variable named "name".
func NewVar(name string) *Var {
    return &Var{name}
}

// Equal returns true if two variables have the same name.
func (v *Var) Equal(other *Var) bool {
    return v.Name == other.Name
}