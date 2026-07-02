// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types declares data types for Rego values and helper functions to
// operate on these types.
package types

import (
	v1 "github.com/open-policy-agent/opa/v1/types"
)

// Sprint returns the string representation of the type.
func Sprint(x Type) string {
	return v1.Sprint(x)
}

// Type represents a type of a term in the language.
type Type = v1.Type

// Null represents the null type.
type Null = v1.Null

// NewNull returns a new Null type.
func NewNull() Null {
	return v1.NewNull()
}

// NamedType represents a type alias with an arbitrary name and description.
// This is useful for generating documentation for built-in functions.
type NamedType = v1.NamedType

// Named returns the passed type as a named type.
// Named types are only valid at the top level of built-in functions.
// Note that nested named types cause panic.
func Named(name string, t Type) *NamedType {
	return v1.Named(name, t)
}

// Boolean represents the boolean type.
type Boolean = v1.Boolean

// B represents an instance of the boolean type.
var B = NewBoolean()

// NewBoolean returns a new Boolean type.
func NewBoolean() Boolean {
	return v1.NewBoolean()
}

// String represents the string type.
type String = v1.String

// S represents an instance of the string type.
var S = NewString()

// NewString returns a new String type.
func NewString() String {
	return v1.NewString()
}

// Number represents the number type.
type Number = v1.Number

// N represents an instance of the number type.
var N = NewNumber()

// NewNumber returns a new Number type.
func NewNumber() Number {
	return v1.NewNumber()
}

// Array represents the array type.
type Array = v1.Array

// NewArray returns a new Array type.
func NewArray(static []Type, dynamic Type) *Array {
	return v1.NewArray(static, dynamic)
}

// Set represents the set type.
type Set = v1.Set

// NewSet returns a new Set type.
func NewSet(of Type) *Set {
	return v1.NewSet(of)
}

// StaticProperty represents a static object property.
type StaticProperty = v1.StaticProperty

// NewStaticProperty returns a new StaticProperty object.
func NewStaticProperty(key any, value Type) *StaticProperty {
	return v1.NewStaticProperty(key, value)
}

// DynamicProperty represents a dynamic object property.
type DynamicProperty = v1.DynamicProperty

// NewDynamicProperty returns a new DynamicProperty object.
func NewDynamicProperty(key, value Type) *DynamicProperty {
	return v1.NewDynamicProperty(key, value)
}

// Object represents the object type.
type Object = v1.Object

// NewObject returns a new Object type.
func NewObject(static []*StaticProperty, dynamic *DynamicProperty) *Object {
	return v1.NewObject(static, dynamic)
}

// Any represents a dynamic type.
type Any = v1.Any

// A represents the superset of all types.
var A = NewAny()

// NewAny returns a new Any type.
func NewAny(of ...Type) Any {
	return v1.NewAny(of...)
}

// Function represents a function type.
type Function = v1.Function

// Args returns an argument list.
func Args(x ...Type) []Type {
	return v1.Args(x...)
}

// Void returns true if the function has no return value. This function returns
// false if x is not a function.
func Void(x Type) bool {
	return v1.Void(x)
}

// Arity returns the number of arguments in the function signature or zero if x
// is not a function. If the type is unknown, this function returns -1.
func Arity(x Type) int {
	return v1.Arity(x)
}

// NewFunction returns a new Function object of the given argument and result types.
func NewFunction(args []Type, result Type) *Function {
	return v1.NewFunction(args, result)
}

// NewVariadicFunction returns a new Function object. This function sets the
// variadic bit on the signature. Non-void variadic functions are not currently
// supported.
func NewVariadicFunction(args []Type, varargs Type, result Type) *Function {
	return v1.NewVariadicFunction(args, varargs, result)
}

// FuncArgs represents the arguments that can be passed to a function.
type FuncArgs = v1.FuncArgs

// Compare returns -1, 0, 1 based on comparison between a and b.
func Compare(a, b Type) int {
	return v1.Compare(a, b)
}

// Contains returns true if a is a superset or equal to b.
func Contains(a, b Type) bool {
	return v1.Contains(a, b)
}

// Or returns a type that represents the union of a and b. If one type is a
// superset of the other, the superset is returned unchanged.
func Or(a, b Type) Type {
	return v1.Or(a, b)
}

// Select returns a property or item of a.
func Select(a Type, x any) Type {
	return v1.Select(a, x)
}

// Keys returns the type of keys that can be enumerated for a. For arrays, the
// keys are always number types, for objects the keys are always string types,
// and for sets the keys are always the type of the set element.
func Keys(a Type) Type {
	return v1.Keys(a)
}

// Values returns the type of values that can be enumerated for a.
func Values(a Type) Type {
	return v1.Values(a)
}

// Nil returns true if a's type is unknown.
func Nil(a Type) bool {
	return v1.Nil(a)
}

// TypeOf returns the type of the Golang native value.
func TypeOf(x any) Type {
	return v1.TypeOf(x)
}
