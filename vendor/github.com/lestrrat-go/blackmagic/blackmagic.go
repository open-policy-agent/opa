package blackmagic

import (
	"fmt"
	"reflect"
)

type errInvalidValue struct{}

func (*errInvalidValue) Error() string {
	return "invalid value (probably an untyped nil)"
}

// InvalidValueError is a sentinel error that can be used to
// indicate that a value is invalid. This can happen when the
// source value is an untyped nil, and we have no further information
// about the type of the value, obstructing the assignment.
func InvalidValueError() error {
	return &errInvalidValue{}
}

// AssignField is a convenience function to assign a value to
// an optional struct field. In Go, an optional struct field is
// usually denoted by a pointer to T instead of T:
//
//	type Object struct {
//	  Optional *T
//	}
//
// This gets a bit cumbersome when you want to assign literals
// or you do not want to worry about taking the address of a
// variable.
//
//	Object.Optional = &"foo" // doesn't compile!
//
// Instead you can use this function to do it in one line:
//
//	blackmagic.AssignOptionalField(&Object.Optionl, "foo")
func AssignOptionalField(dst, src interface{}) error {
	dstRV := reflect.ValueOf(dst)
	srcRV := reflect.ValueOf(src)
	if dstRV.Kind() != reflect.Pointer || dstRV.Elem().Kind() != reflect.Pointer {
		return fmt.Errorf(`dst must be a pointer to a field that is turn a pointer of src (%T)`, src)
	}

	if !dstRV.Elem().CanSet() {
		return fmt.Errorf(`dst (%T) is not assignable`, dstRV.Elem().Interface())
	}
	if !reflect.PointerTo(srcRV.Type()).AssignableTo(dstRV.Elem().Type()) {
		return fmt.Errorf(`cannot assign src (%T) to dst (%T)`, src, dst)
	}

	ptr := reflect.New(srcRV.Type())
	ptr.Elem().Set(srcRV)
	dstRV.Elem().Set(ptr)
	return nil
}

// AssignIfCompatible is a convenience function to safely
// assign arbitrary values. dst must be a pointer to an
// empty interface, or it must be a pointer to a compatible
// variable type that can hold src.
func AssignIfCompatible(dst, src interface{}) error {
	orv := reflect.ValueOf(src) // save this value for error reporting
	result := orv

	// src can be a pointer or a slice, and the code will slightly change
	// depending on this
	var srcIsPtr bool
	var srcIsSlice bool
	switch result.Kind() {
	case reflect.Ptr:
		srcIsPtr = true
	case reflect.Slice:
		srcIsSlice = true
	}

	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf(`destination argument to AssignIfCompatible() must be a pointer: %T`, dst)
	}

	actualDst := rv
	for {
		if !actualDst.IsValid() {
			return fmt.Errorf(`could not find a valid destination for AssignIfCompatible() (%T)`, dst)
		}
		if actualDst.CanSet() {
			break
		}
		actualDst = actualDst.Elem()
	}

	switch actualDst.Kind() {
	case reflect.Interface:
		// If it's an interface, we can just assign the pointer to the interface{}
	default:
		// If it's a pointer to the struct we're looking for, we need to set
		// the de-referenced struct
		if !srcIsSlice && srcIsPtr {
			result = result.Elem()
		}
	}

	if !result.IsValid() {
		// At this point there's nothing we can do. return an error
		return fmt.Errorf(`source value is invalid (%T): %w`, src, InvalidValueError())
	}

	if actualDst.Kind() == reflect.Ptr {
		actualDst.Set(result.Addr())
		return nil
	}

	if !result.Type().AssignableTo(actualDst.Type()) {
		return fmt.Errorf(`argument to AssignIfCompatible() must be compatible with %T (was %T)`, orv.Interface(), dst)
	}

	if !actualDst.CanSet() {
		return fmt.Errorf(`argument to AssignIfCompatible() must be settable`)
	}
	actualDst.Set(result)

	return nil
}
