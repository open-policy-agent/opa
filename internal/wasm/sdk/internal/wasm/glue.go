// Copyright 2025 The OPA Authors.  All rights reserved.
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
// from "env".

package wasm

import (
	"bytes"

	"github.com/open-policy-agent/opa/internal/wasm/encoding"
	"github.com/open-policy-agent/opa/internal/wasm/module"
	"github.com/open-policy-agent/opa/internal/wasm/types"
)

// hostModuleName is the wazero module that provides the real Go host functions,
// which the glue "env" module imports and re-exports.
const hostModuleName = "opa"

// hostFuncImports lists the host functions in import order (determines function
// indices in the glue module).
var hostFuncImports = []struct {
	name    string
	typeIdx int
}{
	{"opa_builtin0", 1},
	{"opa_builtin1", 2},
	{"opa_builtin2", 3},
	{"opa_builtin3", 4},
	{"opa_builtin4", 5},
	{"opa_abort", 0},
	{"opa_println", 0},
}

// buildEnvModule assembles the glue "env" Wasm module and returns its binary
// encoding. maxPages == 0 means no maximum (unbounded growth).
func buildEnvModule(minPages, maxPages uint32) []byte {
	i32 := types.I32

	// Function type signatures, indexed by position.
	//   0: (i32) -> void       — opa_abort, opa_println
	//   1: (i32, i32) -> i32   — opa_builtin0
	//   2..5: add one i32 arg per builtin arity
	funcTypes := []module.FunctionType{
		{Params: []types.ValueType{i32}},
		{Params: []types.ValueType{i32, i32}, Results: []types.ValueType{i32}},
		{Params: []types.ValueType{i32, i32, i32}, Results: []types.ValueType{i32}},
		{Params: []types.ValueType{i32, i32, i32, i32}, Results: []types.ValueType{i32}},
		{Params: []types.ValueType{i32, i32, i32, i32, i32}, Results: []types.ValueType{i32}},
		{Params: []types.ValueType{i32, i32, i32, i32, i32, i32}, Results: []types.ValueType{i32}},
	}

	imports := make([]module.Import, len(hostFuncImports))
	for i, f := range hostFuncImports {
		imports[i] = module.Import{
			Module:     hostModuleName,
			Name:       f.name,
			Descriptor: module.FunctionImport{Func: uint32(f.typeIdx)},
		}
	}

	var maxPtr *uint32
	if maxPages > 0 {
		maxPtr = &maxPages
	}

	exports := make([]module.Export, 1+len(hostFuncImports))
	exports[0] = module.Export{
		Name:       "memory",
		Descriptor: module.ExportDescriptor{Type: module.MemoryExportType, Index: 0},
	}
	for i, f := range hostFuncImports {
		exports[1+i] = module.Export{
			Name:       f.name,
			Descriptor: module.ExportDescriptor{Type: module.FunctionExportType, Index: uint32(i)},
		}
	}

	mod := module.Module{
		Type:   module.TypeSection{Functions: funcTypes},
		Import: module.ImportSection{Imports: imports},
		Memory: module.MemorySection{Memories: []module.Memory{{Lim: module.Limit{Min: minPages, Max: maxPtr}}}},
		Export: module.ExportSection{Exports: exports},
	}

	var buf bytes.Buffer
	if err := encoding.WriteModule(&buf, &mod); err != nil {
		panic(err) // module is statically correct; only panics on a bug
	}
	return buf.Bytes()
}
