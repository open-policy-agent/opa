// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// This file builds the tiny "env" module that OPA's policy.wasm imports from.
//
// An OPA-compiled policy.wasm does not own its linear memory; it *imports* it,
// along with a handful of host functions, all from a module named "env":
//
//	(import "env" "memory"       (memory 2))
//	(import "env" "opa_abort"    (func (param i32)))
//	(import "env" "opa_builtin0" (func (param i32 i32) (result i32)))
//	... opa_builtin1 .. opa_builtin4 ...
//
// wazero resolves every "env.*" import from a single module registered under
// the name "env", and its HostModuleBuilder can only export *functions*, not a
// memory. So we cannot put the real Go host functions and the memory in the
// same "env" module directly.
//
// The trick is that a Wasm module may re-export functions it imports. So we run
// two modules:
//
//   - module "opa" (a wazero host module, see bindings.go) holds the real Go
//     implementations of opa_abort, opa_println and opa_builtin0..4.
//   - module "env" (this glue, built as raw Wasm bytes) defines and exports the
//     linear memory, imports the host functions from "opa", and re-exports them
//     under their "env.*" names.
//
// The policy then resolves everything — memory and the real host functions —
// from "env". Because the shared memory lives in "env" and the host functions
// hold a reference to the policy module (set after instantiation), the Go
// builtins can read arguments from and write results back into that memory.

package wasm

import (
	"bytes"
	"encoding/binary"
)

// hostModuleName is the wazero module that provides the real Go host functions,
// which the glue "env" module imports and re-exports.
const hostModuleName = "opa"

// envMemoryMinPages is the initial size of the linear memory handed to the
// policy. OPA's policy.wasm imports a memory with a minimum of 2 pages; we
// must provide at least that. The memory is growable (no maximum).
const envMemoryMinPages = 2

// i32Wasm is the WebAssembly value-type byte for a 32-bit integer.
const i32Wasm = 0x7F

type importedFunc struct {
	name    string
	typeIdx int
}

// hostFuncImports lists the host functions in import order (determines function
// indices in the glue module).
var hostFuncImports = []importedFunc{
	{"opa_builtin0", 1},
	{"opa_builtin1", 2},
	{"opa_builtin2", 3},
	{"opa_builtin3", 4},
	{"opa_builtin4", 5},
	{"opa_abort", 0},
	{"opa_println", 0},
}

// buildEnvModule assembles the glue "env" Wasm module and returns its binary
// encoding. The module:
//   - imports each host function from hostModuleName ("opa") and re-exports it,
//   - defines and exports a linear memory with the given min and optional max.
//
// maxPages == 0 means no maximum (unbounded growth).
func buildEnvModule(minPages, maxPages uint32) []byte {
	types := []funcType{
		{params: []byte{i32Wasm}, results: nil},                                                          // 0: opa_abort / opa_println
		{params: []byte{i32Wasm, i32Wasm}, results: []byte{i32Wasm}},                                     // 1: opa_builtin0
		{params: []byte{i32Wasm, i32Wasm, i32Wasm}, results: []byte{i32Wasm}},                            // 2: opa_builtin1
		{params: []byte{i32Wasm, i32Wasm, i32Wasm, i32Wasm}, results: []byte{i32Wasm}},                   // 3: opa_builtin2
		{params: []byte{i32Wasm, i32Wasm, i32Wasm, i32Wasm, i32Wasm}, results: []byte{i32Wasm}},          // 4: opa_builtin3
		{params: []byte{i32Wasm, i32Wasm, i32Wasm, i32Wasm, i32Wasm, i32Wasm}, results: []byte{i32Wasm}}, // 5: opa_builtin4
	}

	var m bytes.Buffer
	m.Write([]byte{0x00, 0x61, 0x73, 0x6D}) // "\0asm" magic
	m.Write([]byte{0x01, 0x00, 0x00, 0x00}) // version 1

	// Type section (id 1).
	var typeSec bytes.Buffer
	writeUvarint(&typeSec, uint64(len(types)))
	for _, t := range types {
		typeSec.WriteByte(0x60) // func type tag
		writeVec(&typeSec, t.params)
		writeVec(&typeSec, t.results)
	}
	writeSection(&m, 1, typeSec.Bytes())

	// Import section (id 2): host functions from hostModuleName.
	var impSec bytes.Buffer
	writeUvarint(&impSec, uint64(len(hostFuncImports)))
	for _, f := range hostFuncImports {
		writeName(&impSec, hostModuleName)
		writeName(&impSec, f.name)
		impSec.WriteByte(0x00) // import kind: function
		writeUvarint(&impSec, uint64(f.typeIdx))
	}
	writeSection(&m, 2, impSec.Bytes())

	// Memory section (id 5): one memory with min, and optional max.
	var memSec bytes.Buffer
	writeUvarint(&memSec, 1) // one memory
	if maxPages > 0 {
		memSec.WriteByte(0x01) // limits flag: min+max
		writeUvarint(&memSec, uint64(minPages))
		writeUvarint(&memSec, uint64(maxPages))
	} else {
		memSec.WriteByte(0x00) // limits flag: min only (no max, growable)
		writeUvarint(&memSec, uint64(minPages))
	}
	writeSection(&m, 5, memSec.Bytes())

	// Export section (id 7): memory + re-export of every imported function.
	// Imported functions occupy indices 0..len(hostFuncImports)-1.
	var expSec bytes.Buffer
	writeUvarint(&expSec, uint64(len(hostFuncImports)+1))
	writeName(&expSec, "memory")
	expSec.WriteByte(0x02) // export kind: memory
	writeUvarint(&expSec, 0)
	for i, f := range hostFuncImports {
		writeName(&expSec, f.name)
		expSec.WriteByte(0x00) // export kind: function
		writeUvarint(&expSec, uint64(i))
	}
	writeSection(&m, 7, expSec.Bytes())

	return m.Bytes()
}

type funcType struct {
	params  []byte
	results []byte
}

func writeSection(w *bytes.Buffer, id byte, payload []byte) {
	w.WriteByte(id)
	writeUvarint(w, uint64(len(payload)))
	w.Write(payload)
}

func writeVec(w *bytes.Buffer, b []byte) {
	writeUvarint(w, uint64(len(b)))
	w.Write(b)
}

func writeName(w *bytes.Buffer, s string) {
	writeUvarint(w, uint64(len(s)))
	w.WriteString(s)
}

func writeUvarint(w *bytes.Buffer, v uint64) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	w.Write(buf[:n])
}
