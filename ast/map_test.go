// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"reflect"
	"sort"
	"testing"
)

func TestValueMapOverwrite(t *testing.T) {

	a := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("x"), String("bar"))
	if a.Get(String("x")) != String("bar") {
		t.Fatalf("Expected a['x'] = 'bar' but got: %v", a.Get(String("x")))
	}

}

func TestValueMapIter(t *testing.T) {
	a := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("y"), String("bar"))
	a.Put(String("z"), String("baz"))
	values := []string{}
	a.Iter(func(k, v Value) bool {
		values = append(values, string(v.(String)))
		return false
	})
	sort.Strings(values)
	expected := []string{"bar", "baz", "foo"}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("Unexpected value from iteration: %v", values)
	}
}

func TestValueMapCopy(t *testing.T) {
	a := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("y"), String("bar"))
	b := a.Copy()
	b.Delete(String("y"))
	if a.Get(String("y")) != String("bar") {
		t.Fatalf("Unexpected a['y'] value: %v", a.Get(String("y")))
	}
}

func TestValueMapEqual(t *testing.T) {
	a := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("y"), String("bar"))
	b := a.Copy()
	if !a.Equal(b) {
		t.Fatalf("Expected a == b but not for: %v / %v", a, b)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("Expected a.Hash() == b.Hash() but not for: %v / %v", a, b)
	}
	a.Delete(String("x"))
	if a.Equal(b) {
		t.Fatalf("Expected a != b but not for: %v / %v", a, b)
	}
}

func TestValueMapGetMissing(t *testing.T) {
	a := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("y"), String("bar"))
	if a.Get(String("z")) != nil {
		t.Fatalf("Expected a['z'] = nil but got: %v", a.Get(String("z")))
	}
}

func TestValueMapString(t *testing.T) {
	a := NewValueMap()
	a.Put(MustParseRef("a.b.c[x]"), String("foo"))
	a.Put(Var("x"), Number("1"))
	result := a.String()
	o1 := `{a.b.c[x]: "foo", x: 1}`
	o2 := `{x: 1, a.b.c[x]: "foo"}`
	if result != o1 && result != o2 {
		t.Fatalf("Expected string to equal either %v or %v but got: %v", o1, o2, result)
	}
}

func TestValueMapNil(t *testing.T) {
	var a *ValueMap
	if a.Copy() != nil {
		t.Fatalf("Expected nil map copy to be nil")
	}
	a.Delete(String("foo"))
	var b *ValueMap
	if !a.Equal(b) {
		t.Fatalf("Expected nil maps to be equal")
	}
	b = NewValueMap()
	if !a.Equal(b) {
		t.Fatalf("Expected nil map to equal non-nil, empty map")
	}
	b.Put(String("foo"), String("bar"))
	if a.Equal(b) {
		t.Fatalf("Expected nil map to not equal non-empty map")
	}
	if b.Equal(a) {
		t.Fatalf("Expected non-nil map to not equal nil map")
	}
	if a.Hash() != 0 {
		t.Fatalf("Expected nil map to hash to zero")
	}
	if a.Iter(func(Value, Value) bool { return true }) {
		t.Fatalf("Expected nil map iteration to return false")
	}
	if a.Len() != 0 {
		t.Fatalf("Expected nil map length to be zero")
	}
	if a.String() != "{}" {
		t.Fatalf("Expected nil map string to be {}")
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected put to panic")
		}
	}()
	a.Put(String("foo"), String("bar"))
}
