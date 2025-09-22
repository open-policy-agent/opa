// Copyright 2023 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package clone

import (
	"reflect"
	"unsafe"
)

// AllocatorMethods defines all methods required by allocator.
// If any of these methods is nil, allocator will use default method which allocates memory from heap.
type AllocatorMethods struct {
	// Parent is the allocator which handles all unhandled methods.
	// If it's nil, it will be the default allocator.
	Parent *Allocator

	New       func(pool unsafe.Pointer, t reflect.Type) reflect.Value
	MakeSlice func(pool unsafe.Pointer, t reflect.Type, len, cap int) reflect.Value
	MakeMap   func(pool unsafe.Pointer, t reflect.Type, n int) reflect.Value
	MakeChan  func(pool unsafe.Pointer, t reflect.Type, buffer int) reflect.Value
	IsScalar  func(k reflect.Kind) bool
}

func (am *AllocatorMethods) parent() *Allocator {
	if am != nil && am.Parent != nil {
		return am.Parent
	}

	return nil
}

func (am *AllocatorMethods) new(parent *Allocator, pool unsafe.Pointer) func(pool unsafe.Pointer, t reflect.Type) reflect.Value {
	if am != nil && am.New != nil {
		return am.New
	}

	if parent != nil {
		if parent.pool == pool {
			return parent.new
		} else {
			return func(pool unsafe.Pointer, t reflect.Type) reflect.Value {
				return parent.New(t)
			}
		}
	}

	return defaultAllocator.new
}

func (am *AllocatorMethods) makeSlice(parent *Allocator, pool unsafe.Pointer) func(pool unsafe.Pointer, t reflect.Type, len, cap int) reflect.Value {
	if am != nil && am.MakeSlice != nil {
		return am.MakeSlice
	}

	if parent != nil {
		if parent.pool == pool {
			return parent.makeSlice
		} else {
			return func(pool unsafe.Pointer, t reflect.Type, len, cap int) reflect.Value {
				return parent.MakeSlice(t, len, cap)
			}
		}
	}

	return defaultAllocator.makeSlice
}

func (am *AllocatorMethods) makeMap(parent *Allocator, pool unsafe.Pointer) func(pool unsafe.Pointer, t reflect.Type, n int) reflect.Value {
	if am != nil && am.MakeMap != nil {
		return am.MakeMap
	}

	if parent != nil {
		if parent.pool == pool {
			return parent.makeMap
		} else {
			return func(pool unsafe.Pointer, t reflect.Type, n int) reflect.Value {
				return parent.MakeMap(t, n)
			}
		}
	}

	return defaultAllocator.makeMap
}

func (am *AllocatorMethods) makeChan(parent *Allocator, pool unsafe.Pointer) func(pool unsafe.Pointer, t reflect.Type, buffer int) reflect.Value {
	if am != nil && am.MakeChan != nil {
		return am.MakeChan
	}

	if parent != nil {
		if parent.pool == pool {
			return parent.makeChan
		} else {
			return func(pool unsafe.Pointer, t reflect.Type, buffer int) reflect.Value {
				return parent.MakeChan(t, buffer)
			}
		}
	}

	return defaultAllocator.makeChan
}

func (am *AllocatorMethods) isScalar(parent *Allocator) func(t reflect.Kind) bool {
	if am != nil && am.IsScalar != nil {
		return am.IsScalar
	}

	if parent != nil {
		return parent.isScalar
	}

	return defaultAllocator.isScalar
}
