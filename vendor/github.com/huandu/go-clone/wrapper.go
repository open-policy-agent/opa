// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package clone

import (
	"encoding/binary"
	"hash/crc64"
	"reflect"
	"sync"
	"unsafe"
)

var (
	sizeOfChecksum = unsafe.Sizeof(uint64(0))

	crc64Table = crc64.MakeTable(crc64.ECMA)

	cachedWrapperTypes sync.Map
)

// Wrap creates a wrapper of v, which must be a pointer.
// If v is not a pointer, Wrap simply returns v and do nothing.
//
// The wrapper is a deep clone of v's value. It holds a shadow copy to v internally.
//
//	t := &T{Foo: 123}
//	v := Wrap(t).(*T)               // v is a clone of t.
//	reflect.DeepEqual(t, v) == true // v equals t.
//	v.Foo = 456                     // v.Foo is changed, but t.Foo doesn't change.
//	orig := Unwrap(v)               // Use `Unwrap` to discard wrapper and return original value, which is t.
//	orig.(*T) == t                  // orig and t is exactly the same.
//	Undo(v)                         // Use `Undo` to discard any change on v.
//	v.Foo == t.Foo                  // Now, the value of v and t are the same again.
func Wrap(v interface{}) interface{} {
	if v == nil {
		return v
	}

	val := reflect.ValueOf(v)
	pt := val.Type()

	if val.Kind() != reflect.Ptr {
		return v
	}

	t := pt.Elem()
	elem := val.Elem()
	ptr := unsafe.Pointer(val.Pointer())
	cache, ok := cachedWrapperTypes.Load(t)

	if !ok {
		cache = reflect.StructOf([]reflect.StructField{
			{
				Name:      "T",
				Type:      t,
				Anonymous: true,
			},
			{
				Name: "Checksum",
				Type: reflect.TypeOf(uint64(0)),
			},
			{
				Name: "Origin",
				Type: pt,
			},
		})
		cachedWrapperTypes.Store(t, cache)
	}

	wrapperType := cache.(reflect.Type)
	pw := defaultAllocator.New(wrapperType)

	wrapperPtr := unsafe.Pointer(pw.Pointer())
	wrapper := pw.Elem()

	// Equivalent code: wrapper.T = Clone(v)
	field := wrapper.Field(0)
	field.Set(heapCloneState.clone(elem))

	// Equivalent code: wrapper.Checksum = makeChecksum(v)
	checksumPtr := unsafe.Pointer((uintptr(wrapperPtr) + t.Size()))
	*(*uint64)(checksumPtr) = makeChecksum(t, uintptr(wrapperPtr), uintptr(ptr))

	// Equivalent code: wrapper.Origin = v
	originPtr := unsafe.Pointer((uintptr(wrapperPtr) + t.Size() + sizeOfChecksum))
	*(*uintptr)(originPtr) = uintptr(ptr)

	return field.Addr().Interface()
}

func validateChecksum(t reflect.Type, ptr unsafe.Pointer) bool {
	pw := uintptr(ptr)
	orig := uintptr(getOrigin(t, ptr))
	checksum := *(*uint64)(unsafe.Pointer(uintptr(ptr) + t.Size()))
	expected := makeChecksum(t, pw, orig)

	return checksum == expected
}

func makeChecksum(t reflect.Type, pw uintptr, orig uintptr) uint64 {
	var data [binary.MaxVarintLen64 * 2]byte
	binary.PutUvarint(data[:binary.MaxVarintLen64], uint64(pw))
	binary.PutUvarint(data[binary.MaxVarintLen64:], uint64(orig))
	return crc64.Checksum(data[:], crc64Table)
}

func getOrigin(t reflect.Type, ptr unsafe.Pointer) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(uintptr(ptr) + t.Size() + sizeOfChecksum))
}

// Unwrap returns v's original value if v is a wrapped value.
// Otherwise, simply returns v itself.
func Unwrap(v interface{}) interface{} {
	if v == nil {
		return v
	}

	val := reflect.ValueOf(v)

	if !isWrapped(val) {
		return v
	}

	origVal := origin(val)
	return origVal.Interface()
}

func origin(val reflect.Value) reflect.Value {
	pt := val.Type()
	t := pt.Elem()
	ptr := unsafe.Pointer(val.Pointer())
	orig := getOrigin(t, ptr)
	origVal := reflect.NewAt(t, orig)
	return origVal
}

// Undo discards any change made in wrapped value.
// If v is not a wrapped value, nothing happens.
func Undo(v interface{}) {
	if v == nil {
		return
	}

	val := reflect.ValueOf(v)

	if !isWrapped(val) {
		return
	}

	origVal := origin(val)
	elem := val.Elem()
	elem.Set(heapCloneState.clone(origVal.Elem()))
}

func isWrapped(val reflect.Value) bool {
	pt := val.Type()

	if pt.Kind() != reflect.Ptr {
		return false
	}

	t := pt.Elem()
	ptr := unsafe.Pointer(val.Pointer())
	return validateChecksum(t, ptr)
}
