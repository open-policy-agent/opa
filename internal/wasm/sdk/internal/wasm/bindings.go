// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

// #include <stdlib.h>
//
// extern void opa_abort(void *context, int32_t addr);
// extern int32_t opa_builtin0(void *context, int32_t builtin_id, int32_t ctx);
// extern int32_t opa_builtin1(void *context, int32_t builtin_id, int32_t ctx, int32_t arg0);
// extern int32_t opa_builtin2(void *context, int32_t builtin_id, int32_t ctx, int32_t arg0, int32_t arg1);
// extern int32_t opa_builtin3(void *context, int32_t builtin_id, int32_t ctx, int32_t arg0, int32_t arg1, int32_t arg2);
// extern int32_t opa_builtin4(void *context, int32_t builtin_id, int32_t ctx, int32_t arg0, int32_t arg1, int32_t arg2, int32_t arg3);
import "C"

import (
	"unsafe"

	wasm "github.com/wasmerio/go-ext-wasm/wasmer"
)

func opaFunctions(imports *wasm.Imports) (*wasm.Imports, error) {
	imports, err := imports.AppendFunction("opa_abort", opa_abort, C.opa_abort)
	if err != nil {
		return nil, err
	}

	imports, err = imports.AppendFunction("opa_builtin0", opa_builtin0, C.opa_builtin0)
	if err != nil {
		return nil, err
	}

	imports, err = imports.AppendFunction("opa_builtin1", opa_builtin1, C.opa_builtin1)
	if err != nil {
		return nil, err
	}

	imports, err = imports.AppendFunction("opa_builtin2", opa_builtin2, C.opa_builtin2)
	if err != nil {
		return nil, err
	}

	imports, err = imports.AppendFunction("opa_builtin3", opa_builtin3, C.opa_builtin3)
	if err != nil {
		return nil, err
	}

	return imports.AppendFunction("opa_builtin4", opa_builtin4, C.opa_builtin4)
}

//export opa_abort
func opa_abort(ctx unsafe.Pointer, addr int32) {
	getVM(ctx).Abort(addr)
}

//export opa_builtin0
func opa_builtin0(ctx unsafe.Pointer, builtinID, context int32) int32 {
	return getVM(ctx).Builtin(builtinID, context)
}

//export opa_builtin1
func opa_builtin1(ctx unsafe.Pointer, builtinID, context, arg0 int32) int32 {
	return getVM(ctx).Builtin(builtinID, context, arg0)
}

//export opa_builtin2
func opa_builtin2(ctx unsafe.Pointer, builtinID, context, arg0, arg1 int32) int32 {
	return getVM(ctx).Builtin(builtinID, context, arg0, arg1)
}

//export opa_builtin3
func opa_builtin3(ctx unsafe.Pointer, builtinID, context, arg0, arg1, arg2 int32) int32 {
	return getVM(ctx).Builtin(builtinID, context, arg0, arg1, arg2)
}

//export opa_builtin4
func opa_builtin4(ctx unsafe.Pointer, builtinID, context, arg0, arg1, arg2, arg3 int32) int32 {
	return getVM(ctx).Builtin(builtinID, context, arg0, arg1, arg2, arg3)
}

func getVM(ctx unsafe.Pointer) *VM {
	ictx := wasm.IntoInstanceContext(ctx)
	return ictx.Data().(*VM)
}
