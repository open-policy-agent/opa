// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package util

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"testing"
)

func TestHashMapPutDelete(t *testing.T) {
	m := stringHashMap()
	m.Put("a", "b")
	m.Put("b", "c")
	m.Delete("b")
	r, _ := m.Get("a")
	if r != "b" {
		t.Fatal("Expected a to be intact")
	}
	r, ok := m.Get("b")
	if ok {
		t.Fatalf("Expected b to be removed: %v", r)
	}
	m.Delete("b")
	r, _ = m.Get("a")
	if r != "b" {
		t.Fatal("Expected a to be intact")
	}
}

func TestHashMapOverwrite(t *testing.T) {
	m := stringHashMap()
	key := "hello"
	expected := "goodbye"
	m.Put(key, "world")
	m.Put(key, expected)
	result, _ := m.Get(key)
	if result != expected {
		t.Errorf("Expected existing value to be overwritten but got %v for key %v", result, key)
	}
}

func TestHashMapIter(t *testing.T) {
	m := NewHashMap(func(a, b T) bool {
		n1 := a.(float64)
		n2 := b.(float64)
		return n1 == n2
	}, func(v T) int {
		n := v.(float64)
		return int(n)
	})
	keys := []float64{1, 2, 1.4}
	value := struct{}{}
	for _, k := range keys {
		m.Put(k, value)
	}
	// 1 and 1.4 should both hash to 1.
	if len(m.table) != 2 {
		panic(fmt.Sprintf("Expected collision: %v", m))
	}
	results := map[T]T{}
	m.Iter(func(k T, v T) bool {
		results[k] = v
		return false
	})
	expected := map[T]T{
		float64(1):   value,
		float64(2):   value,
		float64(1.4): value,
	}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v but got %v", expected, results)
	}
}

func TestHashMapCompare(t *testing.T) {
	m := stringHashMap()
	n := stringHashMap()
	k1 := "k1"
	k2 := "k2"
	k3 := "k3"
	v1 := "hello"
	v2 := "goodbye"

	m.Put(k1, v1)
	if m.Equal(n) {
		t.Errorf("Expected hash maps of different size to be non-equal for %v and %v", m, n)
		return
	}
	n.Put(k1, v1)
	if m.Hash() != n.Hash() {
		t.Errorf("Expected hashes to equal for %v and %v", m, n)
		return
	}
	if !m.Equal(n) {
		t.Errorf("Expected hash maps to be equal for %v and %v", m, n)
		return
	}
	m.Put(k2, v2)
	n.Put(k3, v2)
	if m.Hash() == n.Hash() {
		t.Errorf("Did not expect hashes to equal for %v and %v", m, n)
		return
	}
	if m.Equal(n) {
		t.Errorf("Did not expect hash maps to be equal for %v and %v", m, n)
	}
}

func TestHashMapCopy(t *testing.T) {
	m := stringHashMap()

	k1 := "k1"
	k2 := "k2"
	v1 := "hello"
	v2 := "goodbye"

	m.Put(k1, v1)
	m.Put(k2, v2)

	n := m.Copy()

	if !n.Equal(m) {
		t.Errorf("Expected hash maps to be equal: %v != %v", n, m)
		return
	}

	m.Put(k2, "world")

	if n.Equal(m) {
		t.Errorf("Expected hash maps to be non-equal: %v == %v", n, m)
	}
}

func TestHashMapUpdate(t *testing.T) {
	m := stringHashMap()
	n := stringHashMap()
	x := stringHashMap()

	k1 := "k1"
	k2 := "k2"
	v1 := "hello"
	v2 := "goodbye"

	m.Put(k1, v1)
	n.Put(k2, v2)
	x.Put(k1, v1)
	x.Put(k2, v2)

	o := n.Update(m)

	if !x.Equal(o) {
		t.Errorf("Expected update to merge hash maps: %v != %v", x, o)
	}
}

func TestHashMapString(t *testing.T) {
	x := stringHashMap()
	x.Put("x", "y")
	str := x.String()
	exp := "{x: y}"
	if exp != str {
		t.Errorf("expected x.String() == {x: y}: %v != %v", exp, str)
	}
}

func stringHashMap() *HashMap {
	return NewHashMap(func(a, b T) bool {
		s1 := a.(string)
		s2 := b.(string)
		return s1 == s2
	}, func(v T) int {
		s := v.(string)
		h := fnv.New64a()
		h.Write([]byte(s))
		return int(h.Sum64())
	})
}
