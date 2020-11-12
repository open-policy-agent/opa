// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"github.com/bytecodealliance/wasmtime-go"
)

func opaFunctions(vm *VM, store *wasmtime.Store) []*wasmtime.Extern {
	return []*wasmtime.Extern{
		wasmtime.WrapFunc(store, func(addr int32) {
			vm.Abort(addr)
		}).AsExtern(),
		wasmtime.WrapFunc(store, func(builtinID int32, context int32) int32 {
			return vm.Builtin(builtinID, context)
		}).AsExtern(),
		wasmtime.WrapFunc(store, func(builtinID int32, context int32, arg0 int32) int32 {
			return vm.Builtin(builtinID, context, arg0)
		}).AsExtern(),
		wasmtime.WrapFunc(store, func(builtinID int32, context int32, arg0 int32, arg1 int32) int32 {
			return vm.Builtin(builtinID, context, arg0, arg1)
		}).AsExtern(),
		wasmtime.WrapFunc(store, func(builtinID int32, context int32, arg0 int32, arg1 int32, arg2 int32) int32 {
			return vm.Builtin(builtinID, context, arg0, arg1, arg2)
		}).AsExtern(),
		wasmtime.WrapFunc(store, func(builtinID int32, context int32, arg0 int32, arg1 int32, arg2 int32, arg3 int32) int32 {
			return vm.Builtin(builtinID, context, arg0, arg1, arg2, arg3)
		}).AsExtern(),
	}
}
