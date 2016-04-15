// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "fmt"
import "strings"

import "github.com/open-policy-agent/opa/opalog"

type hashEntry struct {
	k    opalog.Value
	v    opalog.Value
	next *hashEntry
}

type hashMap struct {
	table map[int]*hashEntry
}

func newHashMap() *hashMap {
	return &hashMap{make(map[int]*hashEntry)}
}

func (hm *hashMap) Get(k opalog.Value) opalog.Value {
	hash := k.Hash()
	for entry := hm.table[hash]; entry != nil; entry = entry.next {
		if entry.k.Equal(k) {
			return entry.v
		}
	}
	return nil
}

// Iter invokes the iter function for each element in the hashMap.
// If the iter function returns true, iteration stops and the return value is true.
// If the iter function never returns true, iteration proceeds through all elements
// and the return value is false.
func (hm *hashMap) Iter(iter func(opalog.Value, opalog.Value) bool) bool {
	for _, entry := range hm.table {
		for ; entry != nil; entry = entry.next {
			if iter(entry.k, entry.v) {
				return true
			}
		}
	}
	return false
}

func (hm *hashMap) Put(k opalog.Value, v opalog.Value) {
	hash := k.Hash()
	head := hm.table[hash]
	for entry := head; entry != nil; entry = entry.next {
		if entry.k.Equal(k) {
			entry.v = v
			return
		}
	}
	hm.table[hash] = &hashEntry{k: k, v: v, next: head}
}

func (hm *hashMap) String() string {
	var buf []string
	hm.Iter(func(k opalog.Value, v opalog.Value) bool {
		buf = append(buf, fmt.Sprintf("%v: %v", k, v))
		return false
	})
	return "{" + strings.Join(buf, ", ") + "}"
}
