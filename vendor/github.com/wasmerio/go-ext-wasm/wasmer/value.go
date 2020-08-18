package wasmer

import (
	"fmt"
	"math"
)

// ValueType represents the `Value` type.
type ValueType int

const (
	// TypeI32 represents the WebAssembly `i32` type.
	TypeI32 ValueType = iota

	// TypeI64 represents the WebAssembly `i64` type.
	TypeI64

	// TypeF32 represents the WebAssembly `f32` type.
	TypeF32

	// TypeF64 represents the WebAssembly `f64` type.
	TypeF64

	// TypeVoid represents nothing.
	// WebAssembly doesn't have “void” type, but it is introduced
	// here to represent the returned value of a WebAssembly exported
	// function that returns nothing.
	TypeVoid
)

// Value represents a WebAssembly value of a particular type.
type Value struct {
	// The WebAssembly value (as bits).
	value uint64

	// The WebAssembly value type.
	ty ValueType
}

// I32 constructs a WebAssembly value of type `i32`.
func I32(value int32) Value {
	return Value{
		value: uint64(value),
		ty:    TypeI32,
	}
}

// I64 constructs a WebAssembly value of type `i64`.
func I64(value int64) Value {
	return Value{
		value: uint64(value),
		ty:    TypeI64,
	}
}

// F32 constructs a WebAssembly value of type `f32`.
func F32(value float32) Value {
	return Value{
		value: uint64(math.Float32bits(value)),
		ty:    TypeF32,
	}
}

// F64 constructs a WebAssembly value of type `f64`.
func F64(value float64) Value {
	return Value{
		value: math.Float64bits(value),
		ty:    TypeF64,
	}
}

// void constructs an empty WebAssembly value.
func void() Value {
	return Value{
		value: 0,
		ty:    TypeVoid,
	}
}

// GetType gets the type of the WebAssembly value.
func (value Value) GetType() ValueType {
	return value.ty
}

// ToI32 reads the WebAssembly value bits as an `int32`. The WebAssembly
// value type is ignored.
func (value Value) ToI32() int32 {
	return int32(value.value)
}

// ToI64 reads the WebAssembly value bits as an `int64`. The WebAssembly
// value type is ignored.
func (value Value) ToI64() int64 {
	return int64(value.value)
}

// ToF32 reads the WebAssembly value bits as a `float32`. The WebAssembly
// value type is ignored.
func (value Value) ToF32() float32 {
	return math.Float32frombits(uint32(value.value))
}

// ToF64 reads the WebAssembly value bits as a `float64`. The WebAssembly
// value type is ignored.
func (value Value) ToF64() float64 {
	return math.Float64frombits(value.value)
}

// ToVoid reads the WebAssembly value bits as a `nil`. The WebAssembly
// value type is ignored.
func (value Value) ToVoid() interface{} {
	return nil
}

// String formats the WebAssembly value as a Go string.
func (value Value) String() string {
	switch value.ty {
	case TypeI32:
		return fmt.Sprintf("%d", value.ToI32())
	case TypeI64:
		return fmt.Sprintf("%d", value.ToI64())
	case TypeF32:
		return fmt.Sprintf("%f", value.ToF32())
	case TypeF64:
		return fmt.Sprintf("%f", value.ToF64())
	case TypeVoid:
		return "void"
	default:
		return ""
	}
}
