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
	b := NewValueMap()
	a.Put(String("x"), String("foo"))
	a.Put(String("x"), String("bar"))
	if a.Get(String("x")) != String("bar") {
		t.Fatalf("Expected a['x'] = 'bar' but got: %v", a.Get(String("x")))
	}

	a.Put(String("z"), String("corge"))
	b.Put(String("y"), String("baz"))
	b.Put(String("x"), String("qux"))

	c := a.Update(b)

	if c.Get(String("x")) != String("qux") {
		t.Fatalf("Expected c['x'] = 'qux' but got: %v", c.Get(String("x")))
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
	a.Put(Var("x"), Number(1))
	result := a.String()
	term := MustParseTerm(result)
	obj := term.Value.(Object)
	expected := `{a.b.c[x]: "foo", x: 1}`
	if !obj.Equal(MustParseTerm(expected).Value) {
		t.Fatalf("Expected string to equal %v but got: %v", expected, result)
	}
}
