// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "fmt"
import "strings"

import "github.com/open-policy-agent/opa/ast"

type hashEntry struct {
	k    ast.Value
	v    ast.Value
	next *hashEntry
}

type hashMap struct {
	table map[int]*hashEntry
	size  int
}

func newHashMap() *hashMap {
	return &hashMap{make(map[int]*hashEntry), 0}
}

func (hm *hashMap) Copy() *hashMap {
	cpy := newHashMap()
	hm.Iter(func(k, v ast.Value) bool {
		cpy.Put(k, v)
		return false
	})
	return cpy
}

func (hm *hashMap) Equal(other *hashMap) bool {
	if hm.Len() != other.Len() {
		return false
	}
	return !hm.Iter(func(k, v ast.Value) bool {
		ov := other.Get(k)
		if ov == nil {
			return true
		}
		return !v.Equal(ov)
	})
}

func (hm *hashMap) Get(k ast.Value) ast.Value {
	hash := k.Hash()
	for entry := hm.table[hash]; entry != nil; entry = entry.next {
		if entry.k.Equal(k) {
			return entry.v
		}
	}
	return nil
}

func (hm *hashMap) Hash() int {
	var hash int
	hm.Iter(func(k, v ast.Value) bool {
		hash += k.Hash() + v.Hash()
		return false
	})
	return hash
}

// Iter invokes the iter function for each element in the hashMap.
// If the iter function returns true, iteration stops and the return value is true.
// If the iter function never returns true, iteration proceeds through all elements
// and the return value is false.
func (hm *hashMap) Iter(iter func(ast.Value, ast.Value) bool) bool {
	for _, entry := range hm.table {
		for ; entry != nil; entry = entry.next {
			if iter(entry.k, entry.v) {
				return true
			}
		}
	}
	return false
}

func (hm *hashMap) Len() int {
	return hm.size
}

func (hm *hashMap) Put(k ast.Value, v ast.Value) {
	hash := k.Hash()
	head := hm.table[hash]
	for entry := head; entry != nil; entry = entry.next {
		if entry.k.Equal(k) {
			entry.v = v
			return
		}
	}
	hm.table[hash] = &hashEntry{k: k, v: v, next: head}
	hm.size++
}

func (hm *hashMap) String() string {
	var buf []string
	hm.Iter(func(k ast.Value, v ast.Value) bool {
		buf = append(buf, fmt.Sprintf("%v: %v", k, v))
		return false
	})
	return "{" + strings.Join(buf, ", ") + "}"
}

// Update returns a new hashMap with elements from the other hashMap put into this hashMap.
// If the other hashMap contains elements with the same key as this hashMap, the value
// from the other hashMap overwrites the value from this hashMap.
func (hm *hashMap) Update(other *hashMap) *hashMap {
	updated := hm.Copy()
	other.Iter(func(k, v ast.Value) bool {
		updated.Put(k, v)
		return false
	})
	return updated
}
