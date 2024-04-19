// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"strings"
)

type hashEntry[K, V any] struct {
	k    K
	v    V
	next *hashEntry[K, V]
}

// HashMap represents a key/value map.
type HashMap[K, V any] struct {
	eq    func(any, any) bool
	hash  func(any) int
	table map[int]*hashEntry[K, V]
	size  int
}

// NewHashMap returns a new empty HashMap.
func NewHashMap[K, V any](eq func(any, any) bool, hash func(any) int) *HashMap[K, V] {
	return &HashMap[K, V]{
		eq:    eq,
		hash:  hash,
		table: make(map[int]*hashEntry[K, V]),
		size:  0,
	}
}

// Copy returns a shallow copy of this HashMap.
func (h *HashMap[K, V]) Copy() *HashMap[K, V] {
	cpy := NewHashMap[K, V](h.eq, h.hash)
	h.Iter(func(k K, v V) bool {
		cpy.Put(k, v)
		return false
	})
	return cpy
}

// Equal returns true if this HashMap equals the other HashMap.
// Two hash maps are equal if they contain the same key/value pairs.
func (h *HashMap[K, V]) Equal(other *HashMap[K, V]) bool {
	if h.Len() != other.Len() {
		return false
	}
	return !h.Iter(func(k K, v V) bool {
		ov, ok := other.Get(k)
		if !ok {
			return true
		}
		return !h.eq(v, ov)
	})
}

// Get returns the value for k.
func (h *HashMap[K, V]) Get(k K) (V, bool) {
	hash := h.hash(k)
	for entry := h.table[hash]; entry != nil; entry = entry.next {
		if h.eq(entry.k, k) {
			return entry.v, true
		}
	}
	var empty V
	return empty, false
}

// Delete removes the key k.
func (h *HashMap[K, V]) Delete(k K) {
	hash := h.hash(k)
	var prev *hashEntry[K, V]
	for entry := h.table[hash]; entry != nil; entry = entry.next {
		if h.eq(entry.k, k) {
			if prev != nil {
				prev.next = entry.next
			} else {
				h.table[hash] = entry.next
			}
			h.size--
			return
		}
		prev = entry
	}
}

// Hash returns the hash code for this hash map.
func (h *HashMap[K, V]) Hash() int {
	var hash int
	h.Iter(func(k K, v V) bool {
		hash += h.hash(k) + h.hash(v)
		return false
	})
	return hash
}

// Iter invokes the iter function for each element in the HashMap.
// If the iter function returns true, iteration stops and the return value is true.
// If the iter function never returns true, iteration proceeds through all elements
// and the return value is false.
func (h *HashMap[K, V]) Iter(iter func(K, V) bool) bool {
	for _, entry := range h.table {
		for ; entry != nil; entry = entry.next {
			if iter(entry.k, entry.v) {
				return true
			}
		}
	}
	return false
}

// Len returns the current size of this HashMap.
func (h *HashMap[K, V]) Len() int {
	return h.size
}

// Put inserts a key/value pair into this HashMap. If the key is already present, the existing
// value is overwritten.
func (h *HashMap[K, V]) Put(k K, v V) {
	hash := h.hash(k)
	head := h.table[hash]
	for entry := head; entry != nil; entry = entry.next {
		if h.eq(entry.k, k) {
			entry.v = v
			return
		}
	}
	h.table[hash] = &hashEntry[K, V]{k: k, v: v, next: head}
	h.size++
}

func (h *HashMap[K, V]) String() string {
	var buf []string
	h.Iter(func(k K, v V) bool {
		buf = append(buf, fmt.Sprintf("%v: %v", k, v))
		return false
	})
	return "{" + strings.Join(buf, ", ") + "}"
}

// Update returns a new HashMap with elements from the other HashMap put into this HashMap.
// If the other HashMap contains elements with the same key as this HashMap, the value
// from the other HashMap overwrites the value from this HashMap.
func (h *HashMap[K, V]) Update(other *HashMap[K, V]) *HashMap[K, V] {
	updated := h.Copy()
	other.Iter(func(k K, v V) bool {
		updated.Put(k, v)
		return false
	})
	return updated
}
