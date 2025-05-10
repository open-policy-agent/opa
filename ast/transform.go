// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	v1 "github.com/open-policy-agent/opa/v1/ast"
)

// Transformer defines the interface for transforming AST elements. If the
// transformer returns nil and does not indicate an error, the AST element will
// be set to nil and no transformations will be applied to children of the
// element.
type Transformer = v1.Transformer

// Transform iterates the AST and calls the Transform function on the
// Transformer t for x before recursing.
func Transform(t Transformer, x any) (any, error) {
	return v1.Transform(t, x)
}

// TransformRefs calls the function f on all references under x.
func TransformRefs(x any, f func(Ref) (Value, error)) (any, error) {
	return v1.TransformRefs(x, f)
}

// TransformVars calls the function f on all vars under x.
func TransformVars(x any, f func(Var) (Value, error)) (any, error) {
	return v1.TransformVars(x, f)
}

// TransformComprehensions calls the functio nf on all comprehensions under x.
func TransformComprehensions(x any, f func(any) (Value, error)) (any, error) {
	return v1.TransformComprehensions(x, f)
}

// GenericTransformer implements the Transformer interface to provide a utility
// to transform AST nodes using a closure.
type GenericTransformer = v1.GenericTransformer

// NewGenericTransformer returns a new GenericTransformer that will transform
// AST nodes using the function f.
func NewGenericTransformer(f func(x any) (any, error)) *GenericTransformer {
	return v1.NewGenericTransformer(f)
}
