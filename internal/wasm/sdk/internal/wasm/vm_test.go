// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package wasm

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go"

	"github.com/open-policy-agent/opa/ast"
)

func vm(max uint32) *VM {
	memory := wasmtime.NewMemory(
		wasmtime.NewStore(wasmtime.NewEngine()),
		wasmtime.NewMemoryType(wasmtime.Limits{Min: 2, Max: max}),
	)
	return &VM{memory: memory}
}

func TestVMAbort(t *testing.T) {
	v := vm(0xffffffff)
	copy(v.memory.UnsafeData(), []byte("test"))

	var err interface{}
	func() {
		defer func() { err = recover() }()
		v.Abort(0)
	}()

	expected := abortError{message: "test"}
	if err != expected {
		t.Errorf("unexpected error")
	}
}

func TestVMFromRegoValue(t *testing.T) {
	v := vm(0xffffffff)
	v.valueDump = func(addr int32) (int32, error) { return addr, nil }
	v.free = func(addr int32) error { return nil }

	copy(v.memory.UnsafeData()[1:], []byte("[]"))

	a, err := v.fromRegoValue(1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ast.ArrayTerm().Equal(a) {
		t.Errorf("unexpected value: %v", a)
	}
}

func TestVMGrow(t *testing.T) {
	v := vm(3)

	size := v.memory.DataSize()
	if err := grow(v.memory, int32(size), 1); err != nil {
		t.Fatalf("unable to grow the memory")
	}

	if v.memory.DataSize() != size+wasmPageSize {
		t.Errorf("memory did not grow by a page")
	}

	size = v.memory.DataSize()
	if err := grow(v.memory, int32(size), 1); err == nil {
		t.Errorf("memory grew beyond the limits")
	}
}
