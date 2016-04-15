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

func TestIter(t *testing.T) {
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
