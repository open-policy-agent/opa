// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	wasm "github.com/wasmerio/go-ext-wasm/wasmer"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// VM is a wrapper around a Wasm VM instance
type VM struct {
	instance             *wasm.Instance // Pointer to avoid unintented destruction (triggering finalizers within).
	policy               []byte
	data                 []byte
	memory               *wasm.Memory
	memoryMin            uint32
	memoryMax            uint32
	bctx                 *topdown.BuiltinContext
	builtins             map[int32]topdown.BuiltinFunc
	builtinResult        *ast.Term
	entrypointIDs        map[string]int32
	baseHeapPtr          int32
	dataAddr             int32
	evalHeapPtr          int32
	eval                 func(...interface{}) (wasm.Value, error)
	evalCtxGetResult     func(...interface{}) (wasm.Value, error)
	evalCtxNew           func(...interface{}) (wasm.Value, error)
	evalCtxSetData       func(...interface{}) (wasm.Value, error)
	evalCtxSetInput      func(...interface{}) (wasm.Value, error)
	evalCtxSetEntrypoint func(...interface{}) (wasm.Value, error)
	heapPtrGet           func(...interface{}) (wasm.Value, error)
	heapPtrSet           func(...interface{}) (wasm.Value, error)
	heapTopGet           func(...interface{}) (wasm.Value, error)
	heapTopSet           func(...interface{}) (wasm.Value, error)
	jsonDump             func(...interface{}) (wasm.Value, error)
	jsonParse            func(...interface{}) (wasm.Value, error)
	valueDump            func(...interface{}) (wasm.Value, error)
	valueParse           func(...interface{}) (wasm.Value, error)
	malloc               func(...interface{}) (wasm.Value, error)
	free                 func(...interface{}) (wasm.Value, error)
	valueAddPath         func(...interface{}) (wasm.Value, error)
	valueRemovePath      func(...interface{}) (wasm.Value, error)
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
	memory, err := wasm.NewMemory(opts.memoryMin, opts.memoryMax)
	if err != nil {
		return nil, err
	}

	imports, err := opaFunctions(wasm.NewImports())
	if err != nil {
		return nil, err
	}

	imports, err = imports.AppendMemory("memory", memory)
	if err != nil {
		panic(err)
	}

	i, err := wasm.NewInstanceWithImports(opts.policy, imports)
	if err != nil {
		return nil, err
	}

	v := &VM{
		instance:             &i,
		policy:               opts.policy,
		memory:               memory,
		memoryMin:            opts.memoryMin,
		memoryMax:            opts.memoryMax,
		builtins:             make(map[int32]topdown.BuiltinFunc),
		entrypointIDs:        make(map[string]int32),
		dataAddr:             0,
		eval:                 i.Exports["eval"],
		evalCtxGetResult:     i.Exports["opa_eval_ctx_get_result"],
		evalCtxNew:           i.Exports["opa_eval_ctx_new"],
		evalCtxSetData:       i.Exports["opa_eval_ctx_set_data"],
		evalCtxSetInput:      i.Exports["opa_eval_ctx_set_input"],
		evalCtxSetEntrypoint: i.Exports["opa_eval_ctx_set_entrypoint"],
		free:                 i.Exports["opa_free"],
		heapPtrGet:           i.Exports["opa_heap_ptr_get"],
		heapPtrSet:           i.Exports["opa_heap_ptr_set"],
		heapTopGet:           i.Exports["opa_heap_top_get"],
		heapTopSet:           i.Exports["opa_heap_top_set"],
		jsonDump:             i.Exports["opa_json_dump"],
		jsonParse:            i.Exports["opa_json_parse"],
		valueDump:            i.Exports["opa_value_dump"],
		valueParse:           i.Exports["opa_value_parse"],
		malloc:               i.Exports["opa_malloc"],
		valueAddPath:         i.Exports["opa_value_add_path"],
		valueRemovePath:      i.Exports["opa_value_remove_path"],
	}

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
		if memory.Length()-uint32(v.baseHeapPtr) < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - (memory.Length() - uint32(v.baseHeapPtr))
			err := memory.Grow(Pages(delta))
			if err != nil {
				return nil, err
			}
		}
		mem := memory.Data()
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

	// For the opa builtin functions to access the instance.
	i.SetContextData(v)

	// Construct the builtin id to name mappings.

	val, err := i.Exports["builtins"]()
	if err != nil {
		return nil, err
	}

	builtins, err := v.fromRegoJSON(val.ToI32(), true)
	if err != nil {
		return nil, err
	}

	for name, id := range builtins.(map[string]interface{}) {
		f := topdown.GetBuiltin(name)
		if f == nil {
			return nil, fmt.Errorf("builtin '%s' not found", name)
		}

		n, err := id.(json.Number).Int64()
		if err != nil {
			panic(err)
		}

		v.builtins[int32(n)] = f
	}

	// Extract the entrypoint ID's
	val, err = i.Exports["entrypoints"]()
	if err != nil {
		return nil, err
	}

	epMap, err := v.fromRegoJSON(val.ToI32(), true)
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
func (i *VM) Eval(ctx context.Context, entrypoint int32, input *interface{}, metrics metrics.Metrics) ([]byte, error) {
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	metrics.Timer("wasm_vm_eval_prepare_input").Start()
	err := i.setHeapState(i.evalHeapPtr)
	if err != nil {
		return nil, err
	}

	defer func() {
		i.bctx = nil
	}()

	// Parse the input JSON and activate it with the data.

	addr, err := i.evalCtxNew()
	if err != nil {
		return nil, err
	}

	ctxAddr := addr.ToI32()

	if i.dataAddr != 0 {
		if _, err := i.evalCtxSetData(ctxAddr, i.dataAddr); err != nil {
			return nil, err
		}
	}

	_, err = i.evalCtxSetEntrypoint(ctxAddr, int32(entrypoint))
	if err != nil {
		return nil, err
	}

	if input != nil {
		inputAddr, err := i.toRegoJSON(*input, false)
		if err != nil {
			return nil, err
		}

		if _, err := i.evalCtxSetInput(ctxAddr, inputAddr); err != nil {
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
					err = errors.New(e.message)
				case builtinError:
					err = e.err
				default:
					panic(e)
				}

			}
		}()
		_, err = i.eval(ctxAddr)
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

	data := i.memory.Data()[serialized.ToI32():]
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
	if !bytes.Equal(opts.policy, i.policy) {
		// Swap the instance to a new one, with new policy.
		n, err := newVM(opts)
		if err != nil {
			return err
		}

		i.Close()

		*i = *n
		return nil
	}

	i.dataAddr = 0

	var err error
	if err = i.setHeapState(i.baseHeapPtr); err != nil {
		return err
	}

	if opts.parsedData != nil {
		if i.memory.Length()-uint32(i.baseHeapPtr) < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - (i.memory.Length() - uint32(i.baseHeapPtr))
			err := i.memory.Grow(Pages(delta))
			if err != nil {
				return err
			}
		}
		mem := i.memory.Data()
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

// Close the VM instance.
func (i *VM) Close() {
	i.memory.Close()
	i.instance.Close()
}

type abortError struct {
	message string
}

// Abort is invoked by the policy if an internal error occurs during
// the policy execution.
func (i *VM) Abort(arg int32) {
	data := i.memory.Data()[arg:]
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		panic("invalid abort argument")
	}

	panic(abortError{message: string(data[0:n])})
}

type builtinError struct {
	err error
}

// Builtin executes a builtin for the policy.
func (i *VM) Builtin(builtinID, ctx int32, args ...int32) int32 {

	// TODO: Returning proper errors instead of panicing.
	// TODO: To avoid growing the heap with every built-in call, recycle the JSON buffers since the free implementation is no-op.

	convertedArgs := make([]*ast.Term, len(args))
	for j, arg := range args {
		x, err := i.fromRegoJSON(arg, true)
		if err != nil {
			panic(builtinError{err: err})
		}

		y, err := ast.InterfaceToValue(x)
		if err != nil {
			panic(builtinError{err: err})
		}

		convertedArgs[j] = ast.NewTerm(y)
	}

	if i.bctx == nil {
		i.bctx = &topdown.BuiltinContext{
			Context:  context.Background(),
			Cancel:   nil,
			Runtime:  nil,
			Time:     ast.NumberTerm(json.Number(strconv.FormatInt(time.Now().UnixNano(), 10))),
			Metrics:  metrics.New(),
			Cache:    make(builtins.Cache),
			Location: nil,
			Tracers:  nil,
			QueryID:  0,
			ParentID: 0,
		}
	}

	err := i.builtins[builtinID](*i.bctx, convertedArgs, i.iter)
	if err != nil {
		panic(builtinError{err: err})
	}

	result, err := ast.JSON(i.builtinResult.Value)
	if err != nil {
		panic(builtinError{err: err})
	}

	addr, err := i.toRegoJSON(result, true)
	if err != nil {
		panic(builtinError{err: err})
	}

	return addr
}

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval().
func (i *VM) Entrypoints() map[string]int32 {
	return i.entrypointIDs
}

// SetDataPath will update the current data on the VM by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) SetDataPath(path []string, value interface{}) error {

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

	_, err = i.free(pathAddr)
	if err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any new allocations done
	// while patching.
	i.evalHeapPtr, err = i.getHeapState()
	if err != nil {
		return err
	}

	errc := result.ToI32()
	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// RemoveDataPath will update the current data on the VM by removing the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) RemoveDataPath(path []string) error {
	pathAddr, err := i.toRegoJSON(path, true)
	if err != nil {
		return err
	}

	result, err := i.valueRemovePath(i.dataAddr, pathAddr)
	if err != nil {
		return err
	}

	errc := result.ToI32()
	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

func (i *VM) iter(result *ast.Term) error {
	i.builtinResult = result
	return nil
}

// fromRegoJSON converts Rego JSON to go native JSON.
func (i *VM) fromRegoJSON(addr int32, free bool) (interface{}, error) {
	serialized, err := i.jsonDump(addr)
	if err != nil {
		return nil, err
	}

	data := i.memory.Data()[serialized.ToI32():]
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
		if _, err := i.free(serialized.ToI32()); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// toRegoJSON converts go native JSON to Rego JSON.
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
	pos, err := i.malloc(n)
	if err != nil {
		return 0, err
	}

	p := pos.ToI32()
	copy(i.memory.Data()[p:p+n], raw)

	addr, err := i.valueParse(p, n)
	if err != nil {
		return 0, err
	}

	if free {
		if _, err := i.free(p); err != nil {
			return 0, err
		}
	}

	return addr.ToI32(), nil
}

func (i *VM) getHeapState() (int32, error) {
	ptr, err := i.heapPtrGet()
	if err != nil {
		return 0, err
	}

	return ptr.ToI32(), nil
}

func (i *VM) setHeapState(ptr int32) error {
	_, err := i.heapPtrSet(ptr)
	return err
}

func (i *VM) cloneDataSegment() (int32, []byte) {
	// The parsed data values sit between the base heap address and end
	// at the eval heap pointer address.
	srcData := i.memory.Data()[i.baseHeapPtr:i.evalHeapPtr]
	patchedData := make([]byte, len(srcData))
	copy(patchedData, srcData)
	return i.dataAddr, patchedData
}
