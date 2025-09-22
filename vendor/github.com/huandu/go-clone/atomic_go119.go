// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

//go:build go1.19
// +build go1.19

package clone

import (
	"reflect"
	"sync/atomic"
)

func init() {
	SetCustomFunc(reflect.TypeOf(atomic.Bool{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Bool.
		oldValue := old.Addr().Interface().(*atomic.Bool)
		newValue := new.Addr().Interface().(*atomic.Bool)
		v := oldValue.Load()
		newValue.Store(v)
	})
	SetCustomFunc(reflect.TypeOf(atomic.Int32{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Int32.
		oldValue := old.Addr().Interface().(*atomic.Int32)
		newValue := new.Addr().Interface().(*atomic.Int32)
		v := oldValue.Load()
		newValue.Store(v)
	})
	SetCustomFunc(reflect.TypeOf(atomic.Int64{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Int64.
		oldValue := old.Addr().Interface().(*atomic.Int64)
		newValue := new.Addr().Interface().(*atomic.Int64)
		v := oldValue.Load()
		newValue.Store(v)
	})
	SetCustomFunc(reflect.TypeOf(atomic.Uint32{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Uint32.
		oldValue := old.Addr().Interface().(*atomic.Uint32)
		newValue := new.Addr().Interface().(*atomic.Uint32)
		v := oldValue.Load()
		newValue.Store(v)
	})
	SetCustomFunc(reflect.TypeOf(atomic.Uint64{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Uint64.
		oldValue := old.Addr().Interface().(*atomic.Uint64)
		newValue := new.Addr().Interface().(*atomic.Uint64)
		v := oldValue.Load()
		newValue.Store(v)
	})
	SetCustomFunc(reflect.TypeOf(atomic.Uintptr{}), func(allocator *Allocator, old, new reflect.Value) {
		if !old.CanAddr() {
			return
		}

		// Clone value inside atomic.Uintptr.
		oldValue := old.Addr().Interface().(*atomic.Uintptr)
		newValue := new.Addr().Interface().(*atomic.Uintptr)
		v := oldValue.Load()
		newValue.Store(v)
	})
}
