// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

// Bindings represents a mapping between key/value pairs.
// The key/value pairs are AST values contained in expressions.
// Insertion of a key/value pair represents unification of the
// two values.
type Bindings struct {
	hashMap *util.HashMap
}

// NewBindings returns a new empty set of bindings.
func NewBindings() *Bindings {
	b := &Bindings{
		hashMap: util.NewHashMap(bindingsEq, bindingsHash),
	}
	return b
}

// Copy returns a shallow copy of these bindings.
func (b *Bindings) Copy() *Bindings {
	cpy := NewBindings()
	cpy.hashMap = b.hashMap.Copy()
	return cpy
}

// Equal returns true if these bindings equal the other bindings.
// Two bindings are equal if they contain the same key/value pairs.
func (b *Bindings) Equal(other *Bindings) bool {
	return b.hashMap.Equal(other.hashMap)
}

// Get returns the binding for the given key.
func (b *Bindings) Get(k ast.Value) ast.Value {
	if v, ok := b.hashMap.Get(k); ok {
		return v.(ast.Value)
	}
	return nil
}

// Hash returns the hash code for the bindings.
func (b *Bindings) Hash() int {
	return b.hashMap.Hash()
}

// Iter iterates the bindings and calls the "iter" function for each key/value pair.
func (b *Bindings) Iter(iter func(ast.Value, ast.Value) bool) bool {
	return b.hashMap.Iter(func(kt, vt util.T) bool {
		k := kt.(ast.Value)
		v := vt.(ast.Value)
		return iter(k, v)
	})
}

// Put inserts a key/value pair.
func (b *Bindings) Put(k, v ast.Value) {
	b.hashMap.Put(k, v)
}

// Update returns new bindings that are the union of these bindings and the other bindings.
func (b *Bindings) Update(other *Bindings) *Bindings {
	new := b.hashMap.Update(other.hashMap)
	return &Bindings{new}
}

func (b *Bindings) String() string {
	return b.hashMap.String()
}

func bindingsHash(v util.T) int {
	return v.(ast.Value).Hash()
}

func bindingsEq(a, b util.T) bool {
	av := a.(ast.Value)
	bv := b.(ast.Value)
	return av.Equal(bv)
}
