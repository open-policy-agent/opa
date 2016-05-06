// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/opalog"
)

func TestHashmapOverwrite(t *testing.T) {
	m := newHashMap()
	key := opalog.String("hello")
	expected := opalog.String("goodbye")
	m.Put(key, opalog.String("world"))
	m.Put(key, expected)
	result := m.Get(key)
	if result != expected {
		t.Errorf("Expected existing value to be overwritten but got %v for key %v", result, key)
	}
}

func TestHashmapIter(t *testing.T) {
	m := newHashMap()
	keys := []opalog.Number{opalog.Number(1), opalog.Number(2), opalog.Number(1.4)}
	value := opalog.Null{}
	for _, k := range keys {
		m.Put(k, value)
	}
	// 1 and 1.4 should both hash to 1.
	if len(m.table) != 2 {
		panic(fmt.Sprintf("Expected collision: %v", m))
	}
	results := map[opalog.Value]opalog.Value{}
	m.Iter(func(k opalog.Value, v opalog.Value) bool {
		results[k] = v
		return false
	})
	expected := map[opalog.Value]opalog.Value{
		opalog.Number(1):   value,
		opalog.Number(2):   value,
		opalog.Number(1.4): value,
	}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v but got %v", expected, results)
	}
}

func TestHashmapCompare(t *testing.T) {
	m := newHashMap()
	n := newHashMap()
	k1 := opalog.String("k1")
	k2 := opalog.String("k2")
	k3 := opalog.String("k3")
	v1 := parseTerm(`[{"a": 1, "b": 2}, {"c": 3}]`).Value
	v2 := parseTerm(`[{"a": 1, "b": 2}, {"c": 4}]`).Value
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
