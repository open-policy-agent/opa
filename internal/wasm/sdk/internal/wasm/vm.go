// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	_ "github.com/bytecodealliance/wasmtime-go/build/include"        // to include the C headers.
	_ "github.com/bytecodealliance/wasmtime-go/build/linux-x86_64"   // to include the static lib for linking.
	_ "github.com/bytecodealliance/wasmtime-go/build/macos-x86_64"   // to include the static lib for linking.
	_ "github.com/bytecodealliance/wasmtime-go/build/windows-x86_64" // to include the static lib for linking.

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
)

// VM is a wrapper around a Wasm VM instance
type VM struct {
	dispatcher           *builtinDispatcher
	store                *wasmtime.Store
	instance             *wasmtime.Instance // Pointer to avoid unintented destruction (triggering finalizers within).
	intHandle            *wasmtime.InterruptHandle
	policy               []byte
	data                 []byte
	memory               *wasmtime.Memory
	memoryMin            uint32
	memoryMax            uint32
	entrypointIDs        map[string]int32
	baseHeapPtr          int32
	dataAddr             int32
	evalHeapPtr          int32
	eval                 func(int32) error
	evalCtxGetResult     func(int32) (int32, error)
	evalCtxNew           func() (int32, error)
	evalCtxSetData       func(int32, int32) error
	evalCtxSetInput      func(int32, int32) error
	evalCtxSetEntrypoint func(int32, int32) error
	heapPtrGet           func() (int32, error)
	heapPtrSet           func(int32) error
	jsonDump             func(int32) (int32, error)
	jsonParse            func(int32, int32) (int32, error)
	valueDump            func(int32) (int32, error)
	valueParse           func(int32, int32) (int32, error)
	malloc               func(int32) (int32, error)
	free                 func(int32) error
	valueAddPath         func(int32, int32, int32) (int32, error)
	valueRemovePath      func(int32, int32) (int32, error)
}

type vmOpts struct {
	policy         []byte
	data           []byte
	parsedData     []byte
	parsedDataAddr int32
	memoryMin      uint32
	memoryMax      uint32
}

func newVM(opts vmOpts) (*VM, error) {
	v := &VM{}
	cfg := wasmtime.NewConfig()
	cfg.SetInterruptable(true)
	store := wasmtime.NewStore(wasmtime.NewEngineWithConfig(cfg))
	memorytype := wasmtime.NewMemoryType(wasmtime.Limits{Min: opts.memoryMin, Max: opts.memoryMax})
	memory := wasmtime.NewMemory(store, memorytype)
	imports := []*wasmtime.Extern{
		memory.AsExtern(),
	}

	v.dispatcher = newBuiltinDispatcher()
	imports = append(imports, opaFunctions(v.dispatcher, store)...)
	module, err := wasmtime.NewModule(store.Engine, opts.policy)
	if err != nil {
		return nil, err
	}

	i, err := wasmtime.NewInstance(store, module, imports)
	if err != nil {
		return nil, err
	}
	v.intHandle, err = store.InterruptHandle()
	if err != nil {
		return nil, fmt.Errorf("get interrupt handle: %w", err)
	}

	v.store = store
	v.instance = i
	v.policy = opts.policy
	v.memory = memory
	v.memoryMin = opts.memoryMin
	v.memoryMax = opts.memoryMax
	v.entrypointIDs = make(map[string]int32)
	v.dataAddr = 0
	v.eval = func(a int32) error { return callVoid(v, "eval", a) }
	v.evalCtxGetResult = func(a int32) (int32, error) { return call(v, "opa_eval_ctx_get_result", a) }
	v.evalCtxNew = func() (int32, error) { return call(v, "opa_eval_ctx_new") }
	v.evalCtxSetData = func(a int32, b int32) error { return callVoid(v, "opa_eval_ctx_set_data", a, b) }
	v.evalCtxSetInput = func(a int32, b int32) error { return callVoid(v, "opa_eval_ctx_set_input", a, b) }
	v.evalCtxSetEntrypoint = func(a int32, b int32) error { return callVoid(v, "opa_eval_ctx_set_entrypoint", a, b) }
	v.free = func(a int32) error { return callVoid(v, "opa_free", a) }
	v.heapPtrGet = func() (int32, error) { return call(v, "opa_heap_ptr_get") }
	v.heapPtrSet = func(a int32) error { return callVoid(v, "opa_heap_ptr_set", a) }
	v.jsonDump = func(a int32) (int32, error) { return call(v, "opa_json_dump", a) }
	v.jsonParse = func(a int32, b int32) (int32, error) { return call(v, "opa_json_parse", a, b) }
	v.valueDump = func(a int32) (int32, error) { return call(v, "opa_value_dump", a) }
	v.valueParse = func(a int32, b int32) (int32, error) { return call(v, "opa_value_parse", a, b) }
	v.malloc = func(a int32) (int32, error) { return call(v, "opa_malloc", a) }
	v.valueAddPath = func(a int32, b int32, c int32) (int32, error) { return call(v, "opa_value_add_path", a, b, c) }
	v.valueRemovePath = func(a int32, b int32) (int32, error) { return call(v, "opa_value_remove_path", a, b) }

	// Initialize the heap.

	if _, err := v.malloc(0); err != nil {
		return nil, err
	}

	if v.baseHeapPtr, err = v.getHeapState(); err != nil {
		return nil, err
	}

	// Optimization for cloning a vm, if provided a parsed data memory buffer
	// insert it directly into the new vm's buffer and set pointers accordingly.
	// This only works because the placement is deterministic (eg, for a given policy
	// the base heap pointer and parsed data layout will always be the same).
	if opts.parsedData != nil {
		if uint32(memory.DataSize())-uint32(v.baseHeapPtr) < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - (uint32(memory.DataSize()) - uint32(v.baseHeapPtr))
			memory.Grow(uint(Pages(delta))) // TODO: Check return value?
		}
		mem := memory.UnsafeData()
		for src, dest := 0, v.baseHeapPtr; src < len(opts.parsedData); src, dest = src+1, dest+1 {
			mem[dest] = opts.parsedData[src]
		}
		v.dataAddr = opts.parsedDataAddr
		v.evalHeapPtr = v.baseHeapPtr + int32(len(opts.parsedData))
		err := v.setHeapState(v.evalHeapPtr)
		if err != nil {
			return nil, err
		}
	} else if opts.data != nil {
		if v.dataAddr, err = v.toRegoJSON(opts.data, true); err != nil {
			return nil, err
		}
	}

	if v.evalHeapPtr, err = v.getHeapState(); err != nil {
		return nil, err
	}

	// Construct the builtin id to name mappings.

	val, err := i.GetExport("builtins").Func().Call()
	if err != nil {
		return nil, err
	}

	builtins, err := v.fromRegoJSON(val.(int32), true)
	if err != nil {
		return nil, err
	}

	builtinMap := map[int32]topdown.BuiltinFunc{}

	for name, id := range builtins.(map[string]interface{}) {
		f := topdown.GetBuiltin(name)
		if f == nil {
			return nil, fmt.Errorf("builtin '%s' not found", name)
		}

		n, err := id.(json.Number).Int64()
		if err != nil {
			panic(err)
		}

		builtinMap[int32(n)] = f
	}

	v.dispatcher.SetMap(builtinMap)

	// Extract the entrypoint ID's
	val, err = i.GetExport("entrypoints").Func().Call()
	if err != nil {
		return nil, err
	}

	epMap, err := v.fromRegoJSON(val.(int32), true)
	if err != nil {
		return nil, err
	}

	for ep, value := range epMap.(map[string]interface{}) {
		id, err := value.(json.Number).Int64()
		if err != nil {
			return nil, err
		}
		v.entrypointIDs[ep] = int32(id)
	}

	return v, nil
}

// Eval performs an evaluation of the specified entrypoint, with any provided
// input, and returns the resulting value dumped to a string.
func (i *VM) Eval(ctx context.Context, entrypoint int32, input *interface{}, metrics metrics.Metrics, ns time.Time) ([]byte, error) {
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	if err := i.clearInterrupts(ctx); err != nil {
		return nil, fmt.Errorf("clear interrupts: %w", err)
	}

	metrics.Timer("wasm_vm_eval_prepare_input").Start()

	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	i.dispatcher.Reset(ctx, ns)

	// Interrupt the VM if the context is cancelled.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			i.intHandle.Interrupt()
		}
	}()

	err := i.setHeapState(i.evalHeapPtr)
	if err != nil {
		return nil, err
	}

	// Parse the input JSON and activate it with the data.
	ctxAddr, err := i.evalCtxNew()
	if err != nil {
		return nil, err
	}

	if i.dataAddr != 0 {
		if err := i.evalCtxSetData(ctxAddr, i.dataAddr); err != nil {
			return nil, err
		}
	}

	if err := i.evalCtxSetEntrypoint(ctxAddr, int32(entrypoint)); err != nil {
		return nil, err
	}

	if input != nil {
		inputAddr, err := i.toRegoJSON(*input, false)
		if err != nil {
			return nil, err
		}

		if err := i.evalCtxSetInput(ctxAddr, inputAddr); err != nil {
			return nil, err
		}
	}
	metrics.Timer("wasm_vm_eval_prepare_input").Stop()

	// Evaluate the policy.
	metrics.Timer("wasm_vm_eval_execute").Start()
	func() {
		defer func() {
			if e := recover(); e != nil {
				switch e := e.(type) {
				case abortError:
					err = fmt.Errorf(e.message)
				case cancelledError:
					err = errors.ErrCancelled
				case builtinError:
					err = e.err
					if _, ok := err.(topdown.Halt); !ok {
						err = nil
					}
				default:
					panic(e)
				}

			}
		}()
		err = i.eval(ctxAddr)
	}()

	metrics.Timer("wasm_vm_eval_execute").Stop()

	if err != nil {
		return nil, err
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Start()
	resultAddr, err := i.evalCtxGetResult(ctxAddr)
	if err != nil {
		return nil, err
	}

	serialized, err := i.valueDump(resultAddr)
	if err != nil {
		return nil, err
	}

	data := i.memory.UnsafeData()[serialized:]
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Stop()

	// Skip free'ing input and result JSON as the heap will be reset next round anyway.

	return data[0:n], err
}

// SetPolicyData Will either update the VM's data or, if the policy changed,
// re-initialize the VM.
func (i *VM) SetPolicyData(opts vmOpts) error {
	if err := i.clearInterrupts(context.TODO()); err != nil {
		return fmt.Errorf("clear interrupts: %w", err)
	}

	if !bytes.Equal(opts.policy, i.policy) {
		// Swap the instance to a new one, with new policy.
		n, err := newVM(opts)
		if err != nil {
			return err
		}

		*i = *n
		return nil
	}

	i.dataAddr = 0

	var err error
	if err = i.setHeapState(i.baseHeapPtr); err != nil {
		return err
	}

	if opts.parsedData != nil {
		if uint32(i.memory.DataSize())-uint32(i.baseHeapPtr) < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - (uint32(i.memory.DataSize()) - uint32(i.baseHeapPtr))
			i.memory.Grow(uint(Pages(delta))) // TODO: Check return value
		}
		mem := i.memory.UnsafeData()
		for src, dest := 0, i.baseHeapPtr; src < len(opts.parsedData); src, dest = src+1, dest+1 {
			mem[dest] = opts.parsedData[src]
		}
		i.dataAddr = opts.parsedDataAddr
		i.evalHeapPtr = i.baseHeapPtr + int32(len(opts.parsedData))
		err := i.setHeapState(i.evalHeapPtr)
		if err != nil {
			return err
		}
	} else if opts.data != nil {
		if i.dataAddr, err = i.toRegoJSON(opts.data, true); err != nil {
			return err
		}
	}

	if i.evalHeapPtr, err = i.getHeapState(); err != nil {
		return err
	}

	return nil
}

type abortError struct {
	message string
}

type cancelledError struct {
	message string
}

// Println is invoked if the policy WASM code calls opa_println().
func (i *VM) Println(arg int32) {
	data := i.memory.UnsafeData()[arg:]
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		panic("invalid opa_println argument")
	}

	fmt.Printf("opa_println(): %s\n", string(data[:n]))
}

type builtinError struct {
	err error
}

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval().
func (i *VM) Entrypoints() map[string]int32 {
	return i.entrypointIDs
}

// SetDataPath will update the current data on the VM by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) SetDataPath(path []string, value interface{}) error {
	if err := i.clearInterrupts(context.TODO()); err != nil {
		return fmt.Errorf("clear interrupts: %w", err)
	}

	// Reset the heap ptr before patching the vm to try and keep any
	// new allocations safe from subsequent heap resets on eval.
	err := i.setHeapState(i.evalHeapPtr)
	if err != nil {
		return err
	}

	valueAddr, err := i.toRegoJSON(value, true)
	if err != nil {
		return err
	}

	pathAddr, err := i.toRegoJSON(path, true)
	if err != nil {
		return err
	}

	result, err := i.valueAddPath(i.dataAddr, pathAddr, valueAddr)
	if err != nil {
		return err
	}

	// We don't need to free the value, assume it is "owned" as part of the
	// overall data object now.
	// We do need to free the path

	if err := i.free(pathAddr); err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any new allocations done
	// while patching.
	i.evalHeapPtr, err = i.getHeapState()
	if err != nil {
		return err
	}

	errc := result
	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// RemoveDataPath will update the current data on the VM by removing the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) RemoveDataPath(path []string) error {
	if err := i.clearInterrupts(context.TODO()); err != nil {
		return fmt.Errorf("clear interrupts: %w", err)
	}

	pathAddr, err := i.toRegoJSON(path, true)
	if err != nil {
		return err
	}

	errc, err := i.valueRemovePath(i.dataAddr, pathAddr)
	if err != nil {
		return err
	}

	if err := i.free(pathAddr); err != nil {
		return err
	}

	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// fromRegoJSON parses serialized JSON from the Wasm memory buffer into
// native go types.
func (i *VM) fromRegoJSON(addr int32, free bool) (interface{}, error) {
	serialized, err := i.jsonDump(addr)
	if err != nil {
		return nil, err
	}

	data := i.memory.UnsafeData()[serialized:]
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	// Parse the result into go types.

	decoder := json.NewDecoder(bytes.NewReader(data[0:n]))
	decoder.UseNumber()

	var result interface{}
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}

	if free {
		if err := i.free(serialized); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// toRegoJSON converts go native JSON to Rego JSON. If the value is
// an AST type it will be dumped using its stringer.
func (i *VM) toRegoJSON(v interface{}, free bool) (int32, error) {
	var raw []byte
	switch v := v.(type) {
	case []byte:
		raw = v
	case *ast.Term:
		raw = []byte(v.String())
	case ast.Value:
		raw = []byte(v.String())
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return 0, err
		}
	}

	n := int32(len(raw))
	p, err := i.malloc(n)
	if err != nil {
		return 0, err
	}

	copy(i.memory.UnsafeData()[p:p+n], raw)

	addr, err := i.valueParse(p, n)
	if err != nil {
		return 0, err
	}

	if free {
		if err := i.free(p); err != nil {
			return 0, err
		}
	}

	return addr, nil
}

// fromRegoValue parses serialized opa values from the Wasm memory buffer into
// Rego AST types.
func (i *VM) fromRegoValue(addr int32, free bool) (*ast.Term, error) {
	serialized, err := i.valueDump(addr)
	if err != nil {
		return nil, err
	}

	data := i.memory.UnsafeData()[serialized:]
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	// Parse the result into ast types.
	result, err := ast.ParseTerm(string(data[0:n]))

	if free {
		if err := i.free(serialized); err != nil {
			return nil, err
		}
	}

	return result, err
}

func (i *VM) getHeapState() (int32, error) {
	return i.heapPtrGet()
}

func (i *VM) setHeapState(ptr int32) error {
	return i.heapPtrSet(ptr)
}

func (i *VM) cloneDataSegment() (int32, []byte) {
	// The parsed data values sit between the base heap address and end
	// at the eval heap pointer address.
	srcData := i.memory.UnsafeData()[i.baseHeapPtr:i.evalHeapPtr]
	patchedData := make([]byte, len(srcData))
	copy(patchedData, srcData)
	return i.dataAddr, patchedData
}

func call(vm *VM, name string, args ...int32) (int32, error) {
	sl := make([]interface{}, len(args))
	for i := range sl {
		sl[i] = args[i]
	}

	x, err := vm.instance.GetExport(name).Func().Call(sl...)
	if err != nil {
		return 0, err
	}
	return x.(int32), nil
}

func callVoid(vm *VM, name string, args ...int32) error {
	sl := make([]interface{}, len(args))
	for i := range sl {
		sl[i] = args[i]
	}

	_, err := vm.instance.GetExport(name).Func().Call(sl...)
	return err
}

func (i *VM) clearInterrupts(ctx context.Context) error {
	// NOTE: It doesn't matter which exported function of the wasm module we call,
	// any set traps will trigger. Let's call a cheap one.
	_, err := i.heapPtrGet()
	if err == nil {
		return nil
	}
	if t, ok := err.(*wasmtime.Trap); ok && strings.HasPrefix(t.Message(), "wasm trap: interrupt") {
		// Check if OUR ctx is done. If it wasn't, we've triggered the
		// trap of a previous evaluation.
		select {
		case <-ctx.Done(): // chan closed
			return errors.ErrCancelled
		default: // don't block
		}
		return nil
	}
	return err
}
