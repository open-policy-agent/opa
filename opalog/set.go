// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import "fmt"

// Set is a collection of objects that we can't use a map for.
type Set struct {
	Values    []interface{}
	EqualFunc func(interface{}, interface{}) bool
}

// KeyValue represents a single key-value pair for a dictionary
type KeyValue struct {
	Key   *Term
	Value *Term
}

// NewKeyValue creates a key-value pair
func NewKeyValue(key *Term, value *Term) *KeyValue {
	kv := KeyValue{Key: key, Value: value}
	return &kv
}

// String converts a KeyValue into a string
func (kv *KeyValue) String() string {
	return fmt.Sprintf("%s: %s", kv.Key.String(), kv.Value.String())
}

// Equal returns T if the keys and values are the same
func (kv1 *KeyValue) Equal(kv2 *KeyValue) bool {
	return kv1.Key.Equal(kv2.Key) && kv1.Value.Equal(kv2.Value)
}

// NewSet returns a new set
func NewSet(equal func(interface{}, interface{}) bool) *Set {
	s := Set{Values: make([]interface{}, 0), EqualFunc: equal}
	return &s
}

// NewKeyValueSet returns a set for KeyValue pairs
func NewKeyValueSet() *Set {
	f := func(x interface{}, y interface{}) bool {
		kvx := x.(*KeyValue)
		kvy := y.(*KeyValue)
		return kvx.Equal(kvy)
	}
	return NewSet(f)
}

// Add an element
func (s *Set) Add(x interface{}) {
	if !s.Contains(x) {
		s.Values = append(s.Values, x)
	}
}

// Contains returns true if the set contains element x
func (s *Set) Contains(x interface{}) bool {
	for _, elem := range s.Values {
		if s.EqualFunc(x, elem) {
			return true
		}
	}
	return false
}

// Length returns number of elements
func (s *Set) Length() int {
	return len(s.Values)
}

// Equal returns True if the 2 sets have all the same elements
func (set1 *Set) Equal(set2 *Set) bool {
	if len(set1.Values) != len(set2.Values) {
		return false
	}

	// TODO: reimplement natively so we don't use memory
	diff12 := set1.Difference(set2)
	if diff12.Length() > 0 {
		return false
	}
	diff21 := set2.Difference(set1)
	if diff21.Length() > 0 {
		return false
	}
	return true
}

// Difference returns a new set that has all the elements of set1 except those in set2
func (set1 *Set) Difference(set2 *Set) *Set {
	newset := NewSet(set1.EqualFunc)
	for _, elem := range set1.Values {
		if !set2.Contains(elem) {
			newset.Add(elem)
		}
	}
	return newset
}
