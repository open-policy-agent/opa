// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

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

type vm struct {
	instance             *wasm.Instance // Pointer to avoid unintented destruction (triggering finalizers within).
	policy               []byte
	data                 []byte
	memory               *wasm.Memory
	memoryMin            uint32
	memoryMax            uint32
	bctx                 *topdown.BuiltinContext
	builtins             map[int32]topdown.BuiltinFunc
	builtinResult        *ast.Term
	entrypointIDs        map[string]EntrypointID
	baseHeapPtr          int32
	dataAddr             int32
	evalHeapPtr          int32
	evalHeapTop          int32
	eval                 func(...interface{}) (wasm.Value, error)
	evalCtxGetResult     func(...interface{}) (wasm.Value, error)
	evalCtxNew           func(...interface{}) (wasm.Value, error)
	evalCtxSetData       func(...interface{}) (wasm.Value, error)
	evalCtxSetInput      func(...interface{}) (wasm.Value, error)
	evalCtxSetEntrypoint func(...interface{}) (wasm.Value, error)
	free                 func(...interface{}) (wasm.Value, error)
	heapPtrGet           func(...interface{}) (wasm.Value, error)
	heapPtrSet           func(...interface{}) (wasm.Value, error)
	heapTopGet           func(...interface{}) (wasm.Value, error)
	heapTopSet           func(...interface{}) (wasm.Value, error)
	jsonDump             func(...interface{}) (wasm.Value, error)
	jsonParse            func(...interface{}) (wasm.Value, error)
	malloc               func(...interface{}) (wasm.Value, error)
}

func newVM(policy []byte, data []byte, memoryMin, memoryMax uint32) (*vm, error) {
	memory, err := wasm.NewMemory(memoryMin, memoryMax)
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

	i, err := wasm.NewInstanceWithImports(policy, imports)
	if err != nil {
		return nil, err
	}

	v := &vm{
		instance:             &i,
		policy:               policy,
		data:                 data,
		memory:               memory,
		memoryMin:            memoryMin,
		memoryMax:            memoryMax,
		builtins:             make(map[int32]topdown.BuiltinFunc),
		entrypointIDs:        make(map[string]EntrypointID),
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
		malloc:               i.Exports["opa_malloc"],
	}

	// Initialize the heap.

	if _, err := v.malloc(0); err != nil {
		return nil, err
	}

	if v.baseHeapPtr, err = v.getHeapState(); err != nil {
		return nil, err
	}

	if data != nil {
		if v.dataAddr, err = v.toRegoJSON(data, true); err != nil {
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
		v.entrypointIDs[ep] = EntrypointID(id)
	}

	return v, nil
}

func (i *vm) Eval(ctx context.Context, entrypoint EntrypointID, input *interface{}, metrics metrics.Metrics) (interface{}, error) {
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

	result, err := i.fromRegoJSON(resultAddr.ToI32(), false)
	metrics.Timer("wasm_vm_eval_prepare_result").Stop()

	// Skip free'ing input and result JSON as the heap will be reset next round anyway.

	return result, err
}

func (i *vm) SetPolicyData(policy []byte, data []byte) error {
	if !bytes.Equal(policy, i.policy) {
		// Swap the instance to a new one, with new policy.

		n, err := newVM(policy, data, i.memoryMin, i.memoryMax)
		if err != nil {
			return err
		}

		i.Close()

		*i = *n
		return nil
	}

	i.data = data
	i.dataAddr = 0

	var err error
	if err = i.setHeapState(i.baseHeapPtr); err != nil {
		return err
	}

	if data != nil {
		if i.dataAddr, err = i.toRegoJSON(data, true); err != nil {
			return err
		}
	}

	if i.evalHeapPtr, err = i.getHeapState(); err != nil {
		return err
	}

	return nil
}

func (i *vm) Close() {
	i.memory.Close()
	i.instance.Close()
}

type abortError struct {
	message string
}

// Abort is invoked by the policy if an internal error occurs during
// the policy execution.
func (i *vm) Abort(arg int32) {
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
func (i *vm) Builtin(builtinID, ctx int32, args ...int32) int32 {

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
func (i *vm) Entrypoints() map[string]EntrypointID {
	return i.entrypointIDs
}

func (i *vm) iter(result *ast.Term) error {
	i.builtinResult = result
	return nil
}

// fromRegoJSON converts Rego JSON to go native JSON.
func (i *vm) fromRegoJSON(addr int32, free bool) (interface{}, error) {
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
func (i *vm) toRegoJSON(v interface{}, free bool) (int32, error) {
	raw, ok := v.([]byte)
	if !ok {
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

	addr, err := i.jsonParse(p, n)
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

func (i *vm) getHeapState() (int32, error) {
	ptr, err := i.heapPtrGet()
	if err != nil {
		return 0, err
	}

	return ptr.ToI32(), nil
}

func (i *vm) setHeapState(ptr int32) error {
	_, err := i.heapPtrSet(ptr)
	return err
}
