// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"github.com/open-policy-agent/opa/util"
)

// ValueMap represents a key/value map between AST term values. Any type of term
// can be used as a key in the map.
type ValueMap struct {
	hashMap *util.HashMap
}

// NewValueMap returns a new ValueMap.
func NewValueMap() *ValueMap {
	vs := &ValueMap{
		hashMap: util.NewHashMap(valueEq, valueHash),
	}
	return vs
}

// Copy returns a shallow copy of the ValueMap.
func (vs *ValueMap) Copy() *ValueMap {
	cpy := NewValueMap()
	cpy.hashMap = vs.hashMap.Copy()
	return cpy
}

// Equal returns true if this ValueMap equals the other.
func (vs *ValueMap) Equal(other *ValueMap) bool {
	return vs.hashMap.Equal(other.hashMap)
}

// Get returns the value in the map for k.
func (vs *ValueMap) Get(k Value) Value {
	if v, ok := vs.hashMap.Get(k); ok {
		return v.(Value)
	}
	return nil
}

// Hash returns a hash code for this ValueMap.
func (vs *ValueMap) Hash() int {
	return vs.hashMap.Hash()
}

// Iter calls the iter function for each key/value pair in the map. If the iter
// function returns true, iteration stops.
func (vs *ValueMap) Iter(iter func(Value, Value) bool) bool {
	return vs.hashMap.Iter(func(kt, vt util.T) bool {
		k := kt.(Value)
		v := vt.(Value)
		return iter(k, v)
	})
}

// Put inserts a key k into the map with value v.
func (vs *ValueMap) Put(k, v Value) {
	vs.hashMap.Put(k, v)
}

// Delete removes a key k from the map.
func (vs *ValueMap) Delete(k Value) {
	vs.hashMap.Delete(k)
}

// Update returns a new ValueMap that contains the union of this ValueMap and
// the other. Overlapping keys are replaced with values from the other.
func (vs *ValueMap) Update(other *ValueMap) *ValueMap {
	new := vs.hashMap.Update(other.hashMap)
	return &ValueMap{new}
}

func (vs *ValueMap) String() string {
	return vs.hashMap.String()
}

func valueHash(v util.T) int {
	return v.(Value).Hash()
}

func valueEq(a, b util.T) bool {
	av := a.(Value)
	bv := b.(Value)
	return av.Equal(bv)
}
