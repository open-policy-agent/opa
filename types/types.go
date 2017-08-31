// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types declares data types for Rego values and helper functions to
// operate on these types.
package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

// Sprint returns the string representation of the type.
func Sprint(x Type) string {
	if x == nil {
		return "???"
	}
	return x.String()
}

// Type represents a type of a term in the language.
type Type interface {
	String() string
	typeMarker()
}

func (Null) typeMarker()    {}
func (Boolean) typeMarker() {}
func (Number) typeMarker()  {}
func (String) typeMarker()  {}
func (*Array) typeMarker()  {}
func (*Object) typeMarker() {}
func (*Set) typeMarker()    {}
func (Any) typeMarker()     {}

// Null represents the null type.
type Null struct{}

// NewNull returns a new Null type.
func NewNull() Null {
	return Null{}
}

func (t Null) String() string {
	return "null"
}

// Boolean represents the boolean type.
type Boolean struct{}

// B represents an instance of the boolean type.
var B = NewBoolean()

// NewBoolean returns a new Boolean type.
func NewBoolean() Boolean {
	return Boolean{}
}

func (t Boolean) String() string {
	return "boolean"
}

// String represents the string type.
type String struct{}

// S represents an instance of the string type.
var S = NewString()

// NewString returns a new String type.
func NewString() String {
	return String{}
}

func (t String) String() string {
	return "string"
}

// Number represents the number type.
type Number struct{}

// N represents an instance of the number type.
var N = NewNumber()

// NewNumber returns a new Number type.
func NewNumber() Number {
	return Number{}
}

func (Number) String() string {
	return "number"
}

// Array represents the array type.
type Array struct {
	static  []Type // static items
	dynamic Type   // dynamic items
}

// NewArray returns a new Array type.
func NewArray(static []Type, dynamic Type) *Array {
	return &Array{
		static:  static,
		dynamic: dynamic,
	}
}

func (t *Array) String() string {
	prefix := "array"
	buf := []string{}
	for _, tpe := range t.static {
		buf = append(buf, Sprint(tpe))
	}
	var repr = prefix
	if len(buf) > 0 {
		repr += "<" + strings.Join(buf, ", ") + ">"
	}
	if t.dynamic != nil {
		repr += "[" + t.dynamic.String() + "]"
	}
	return repr
}

// Dynamic returns the type of the array's dynamic elements.
func (t *Array) Dynamic() Type {
	return t.dynamic
}

// Len returns the number of static array elements.
func (t *Array) Len() int {
	return len(t.static)
}

// Select returns the type of element at the zero-based pos.
func (t *Array) Select(pos int) Type {
	if len(t.static) > pos {
		return t.static[pos]
	}
	if t.dynamic != nil {
		return t.dynamic
	}
	return nil
}

// Set represents the set type.
type Set struct {
	of Type
}

// NewSet returns a new Set type.
func NewSet(of Type) *Set {
	return &Set{
		of: of,
	}
}

func (t *Set) String() string {
	prefix := "set"
	return prefix + "[" + Sprint(t.of) + "]"
}

// StaticProperty represents a static object property.
type StaticProperty struct {
	Key   interface{}
	Value Type
}

// NewStaticProperty returns a new Property object.
func NewStaticProperty(key interface{}, value Type) *StaticProperty {
	return &StaticProperty{
		Key:   key,
		Value: value,
	}
}

// Object represents the object type.
type Object struct {
	static  []*StaticProperty // constant properties
	dynamic Type              // dynamic properties
}

// NewObject returns a new Object type.
func NewObject(static []*StaticProperty, dynamic Type) *Object {
	sort.Slice(static, func(i, j int) bool {
		cmp := util.Compare(static[i].Key, static[j].Key)
		return cmp == -1
	})
	return &Object{
		static:  static,
		dynamic: dynamic,
	}
}

func (t *Object) String() string {
	prefix := "object"
	buf := make([]string, 0, len(t.static))
	for _, p := range t.static {
		buf = append(buf, fmt.Sprintf("%v: %v", p.Key, Sprint(p.Value)))
	}
	var repr = prefix
	if len(buf) > 0 {
		repr += "<" + strings.Join(buf, ", ") + ">"
	}
	if t.dynamic != nil {
		repr += "[" + t.dynamic.String() + "]"
	}
	return repr
}

// Dynamic returns the type of the object's dynamic elements.
func (t *Object) Dynamic() Type {
	return t.dynamic
}

// Keys returns the keys of the object's static elements.
func (t *Object) Keys() []interface{} {
	sl := make([]interface{}, 0, len(t.static))
	for _, p := range t.static {
		sl = append(sl, p.Key)
	}
	return sl
}

// Select returns the type of the named property.
func (t *Object) Select(name interface{}) Type {
	for _, p := range t.static {
		if util.Compare(p.Key, name) == 0 {
			return p.Value
		}
	}
	if t.dynamic != nil {
		return t.dynamic
	}
	return nil
}

// Any represents a dynamic type.
type Any []Type

// A represents the superset of all types.
var A = NewAny()

// NewAny returns a new Any type.
func NewAny(of ...Type) Any {
	sl := make(Any, len(of))
	for i := range sl {
		sl[i] = of[i]
	}
	return sl
}

// Contains returns true if t is a superset of other.
func (t Any) Contains(other Type) bool {
	for i := range t {
		if Compare(t[i], other) == 0 {
			return true
		}
	}
	return len(t) == 0
}

// Merge return a new Any type that is the superset of t and other.
func (t Any) Merge(other Type) Any {
	if otherAny, ok := other.(Any); ok {
		return t.Union(otherAny)
	}
	if t.Contains(other) {
		return t
	}
	return append(t, other)
}

// Union returns a new Any type that is the union of the two Any types.
func (t Any) Union(other Any) Any {
	if len(t) == 0 {
		return t
	}
	if len(other) == 0 {
		return other
	}
	cpy := make(Any, len(t))
	for i := range cpy {
		cpy[i] = t[i]
	}
	for i := range other {
		if !cpy.Contains(other[i]) {
			cpy = append(cpy, other[i])
		}
	}
	return cpy
}

func (t Any) String() string {
	prefix := "any"
	if len(t) == 0 {
		return prefix
	}
	buf := make([]string, len(t))
	for i := range t {
		buf[i] = Sprint(t[i])
	}
	return prefix + "<" + strings.Join(buf, ", ") + ">"
}

// Compare returns -1, 0, 1 based on comparison between a and b.
func Compare(a, b Type) int {
	x := typeOrder(a)
	y := typeOrder(b)
	if x > y {
		return 1
	} else if x < y {
		return -1
	}
	switch a.(type) {
	case nil:
		return 0
	case Null, Boolean, Number, String:
		return 0
	case *Array:
		arrA := a.(*Array)
		arrB := b.(*Array)
		if arrA.dynamic != nil && arrB.dynamic == nil {
			return 1
		} else if arrB.dynamic != nil && arrA.dynamic == nil {
			return -1
		}
		if arrB.dynamic != nil && arrA.dynamic != nil {
			if cmp := Compare(arrA.dynamic, arrB.dynamic); cmp != 0 {
				return cmp
			}
		}
		return typeSliceCompare(arrA.static, arrB.static)
	case *Object:
		objA := a.(*Object)
		objB := b.(*Object)
		if objA.dynamic != nil && objB.dynamic == nil {
			return 1
		} else if objB.dynamic != nil && objA.dynamic == nil {
			return -1
		}
		if objA.dynamic != nil && objB.dynamic != nil {
			if cmp := Compare(objA.dynamic, objB.dynamic); cmp != 0 {
				return cmp
			}
		}

		lenStaticA := len(objA.static)
		lenStaticB := len(objB.static)

		minLen := lenStaticA
		if lenStaticB < minLen {
			minLen = lenStaticB
		}

		for i := 0; i < minLen; i++ {
			if cmp := util.Compare(objA.static[i].Key, objB.static[i].Key); cmp != 0 {
				return cmp
			}
			if cmp := Compare(objA.static[i].Value, objB.static[i].Value); cmp != 0 {
				return cmp
			}
		}

		if lenStaticA < lenStaticB {
			return -1
		} else if lenStaticB < lenStaticA {
			return 1
		}

		return 0
	case *Set:
		setA := a.(*Set)
		setB := b.(*Set)
		if setA.of == nil && setB.of == nil {
			return 0
		} else if setA.of == nil {
			return -1
		} else if setB.of == nil {
			return 1
		}
		return Compare(setA.of, setB.of)
	case Any:
		sl1 := typeSlice(a.(Any))
		sl2 := typeSlice(b.(Any))
		sort.Sort(sl1)
		sort.Sort(sl2)
		return typeSliceCompare(sl1, sl2)
	default:
		panic("unreachable")
	}
}

// Or returns a type that represents the union of a and b. If one type is a
// superset of the other, the superset is returned unchanged.
func Or(a, b Type) Type {
	if a == nil {
		return b
	} else if b == nil {
		return a
	}
	anyA, ok1 := a.(Any)
	anyB, ok2 := b.(Any)
	if ok1 {
		return anyA.Merge(b)
	}
	if ok2 {
		return anyB.Merge(a)
	}
	if Compare(a, b) == 0 {
		return a
	}
	return NewAny(a, b)
}

// Select returns a property or item of a.
func Select(a Type, x interface{}) Type {
	switch a := a.(type) {
	case *Array:
		n, ok := x.(json.Number)
		if !ok {
			return nil
		}
		pos, err := n.Int64()
		if err != nil {
			return nil
		}
		return a.Select(int(pos))
	case *Object:
		return a.Select(x)
	case *Set:
		tpe := TypeOf(x)
		if Compare(a.of, tpe) == 0 {
			return a.of
		}
		if any, ok := a.of.(Any); ok {
			if any.Contains(tpe) {
				return tpe
			}
		}
		return nil
	case Any:
		if Compare(a, A) == 0 {
			return A
		}
		var tpe Type
		for i := range a {
			// TODO(tsandall): test nil/nil
			tpe = Or(Select(a[i], x), tpe)
		}
		return tpe
	default:
		return nil
	}
}

// Keys returns the type of keys that can be enumerated for a. For arrays, the
// keys are always number types, for objects the keys are always string types,
// and for sets the keys are always the type of the set element.
func Keys(a Type) Type {
	switch a := a.(type) {
	case *Array:
		return N
	case *Object:
		return S
	case *Set:
		return a.of
	case Any:
		// TODO(tsandall): ditto test
		if Compare(a, A) == 0 {
			return A
		}
		var tpe Type
		for i := range a {
			tpe = Or(Keys(a[i]), tpe)
		}
		return tpe
	}
	return nil
}

// Values returns the type of values that can be enumerated for a.
func Values(a Type) Type {
	switch a := a.(type) {
	case *Array:
		var tpe Type
		for i := range a.static {
			tpe = Or(tpe, a.static[i])
		}
		return Or(tpe, a.dynamic)
	case *Object:
		var tpe Type
		for _, v := range a.static {
			tpe = Or(tpe, v.Value)
		}
		return Or(tpe, a.dynamic)
	case *Set:
		return a.of
	case Any:
		if Compare(a, A) == 0 {
			return A
		}
		var tpe Type
		for i := range a {
			tpe = Or(Values(a[i]), tpe)
		}
		return tpe
	}
	return nil
}

// Nil returns true if a's type is unknown.
func Nil(a Type) bool {
	switch a := a.(type) {
	case nil:
		return true
	case *Array:
		for i := range a.static {
			if Nil(a.static[i]) {
				return true
			}
		}
	case *Object:
		for i := range a.static {
			if Nil(a.static[i].Value) {
				return true
			}
		}
	case *Set:
		return Nil(a.of)
	}
	return false
}

// TypeOf returns the type of the Golang native value.
func TypeOf(x interface{}) Type {
	switch x := x.(type) {
	case nil:
		return NewNull()
	case bool:
		return B
	case string:
		return S
	case json.Number:
		return N
	case map[interface{}]interface{}:
		static := make([]*StaticProperty, 0, len(x))
		for k, v := range x {
			static = append(static, NewStaticProperty(k, TypeOf(v)))
		}
		return NewObject(static, nil)
	case []interface{}:
		static := make([]Type, len(x))
		for i := range x {
			static[i] = TypeOf(x[i])
		}
		return NewArray(static, nil)
	}
	panic("unreachable")
}

type typeSlice []Type

func (s typeSlice) Less(i, j int) bool { return Compare(s[i], s[j]) < 0 }
func (s typeSlice) Swap(i, j int)      { x := s[i]; s[i] = s[j]; s[j] = x }
func (s typeSlice) Len() int           { return len(s) }

func typeSliceCompare(a, b []Type) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if cmp := Compare(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	} else if len(b) < len(a) {
		return 1
	}
	return 0
}

func typeOrder(x Type) int {
	switch x.(type) {
	case Null:
		return 0
	case Boolean:
		return 1
	case Number:
		return 2
	case String:
		return 3
	case *Array:
		return 4
	case *Object:
		return 5
	case *Set:
		return 6
	case Any:
		return 7
	case nil:
		return -1
	}
	panic("unreachable")
}
