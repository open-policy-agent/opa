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
	"io"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"

	sdk_errors "github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
	"github.com/open-policy-agent/opa/internal/wasm/util"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/topdown/print"
)

// VM is a wrapper around a Wasm VM instance
type VM struct {
	dispatcher      *builtinDispatcher
	runtime         wazero.Runtime
	mod             api.Module
	mem             api.Memory
	policy          []byte
	abiMinorVersion int32
	memoryMin       uint32
	memoryMax       uint32
	entrypointIDs   map[string]int32
	baseHeapPtr     int32
	dataAddr        int32
	evalHeapPtr     int32
	dead            bool // set when the runtime is closed mid-eval due to context cancellation
}

type vmOpts struct {
	policy         []byte
	data           []byte
	parsedData     []byte
	parsedDataAddr int32
	memoryMin      uint32
	memoryMax      uint32
	cache          wazero.CompilationCache
}

func newVM(ctx context.Context, opts vmOpts) (*VM, error) {
	cfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true) // interrupts tight wasm loops when ctx is cancelled
	if opts.cache != nil {
		cfg = cfg.WithCompilationCache(opts.cache)
	}
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	v := &VM{
		runtime:   r,
		policy:    opts.policy,
		memoryMin: opts.memoryMin,
		memoryMax: opts.memoryMax,
	}

	v.dispatcher = newBuiltinDispatcher()

	if err := instantiateOPAModule(ctx, r, v.dispatcher); err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("instantiate opa host module: %w", err)
	}

	if _, err := r.InstantiateWithConfig(ctx, buildEnvModule(opts.memoryMin, opts.memoryMax),
		wazero.NewModuleConfig().WithName("env")); err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("instantiate env glue module: %w", err)
	}

	mod, err := r.InstantiateWithConfig(ctx, opts.policy,
		wazero.NewModuleConfig().WithName("policy"))
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("instantiate policy module: %w", err)
	}

	v.mod = mod
	// The policy module gets its memory from the "env" glue module; after
	// instantiation wazero surfaces it as an export of the policy module.
	v.mem = mod.Memory()
	if v.mem == nil {
		r.Close(ctx)
		return nil, errors.New("policy module exports no memory")
	}

	// Wire the dispatcher to the policy module's memory and exported functions.
	v.dispatcher.mem = v.mem
	v.dispatcher.malloc = mod.ExportedFunction("opa_malloc")
	v.dispatcher.valueDumpFn = mod.ExportedFunction("opa_value_dump")
	v.dispatcher.valueParseFn = mod.ExportedFunction("opa_value_parse")

	major, minor, err := getABIVersion(mod)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("invalid module: %w", err)
	}
	if major != 1 || minor < 1 {
		r.Close(ctx)
		return nil, fmt.Errorf("invalid module: unsupported ABI version: %d.%d", major, minor)
	}
	v.abiMinorVersion = minor

	v.entrypointIDs = make(map[string]int32)

	// Initialize the heap.
	if _, err = call(ctx, v, "opa_malloc", 0); err != nil {
		r.Close(ctx)
		return nil, err
	}

	if v.baseHeapPtr, err = v.getHeapState(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	// Fast path: inject pre-parsed data directly into the new VM's memory.
	if opts.parsedData != nil {
		avail := v.mem.Size() - uint32(v.baseHeapPtr)
		if avail < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - avail
			neededPages := util.Pages(delta)
			if _, ok := v.mem.Grow(neededPages); !ok {
				r.Close(ctx)
				return nil, fmt.Errorf("failed to grow memory by `%d` for parsed data (max pages %d)", neededPages, opts.memoryMax)
			}
		}
		if !v.mem.Write(uint32(v.baseHeapPtr), opts.parsedData) {
			r.Close(ctx)
			return nil, errors.New("write parsed data into memory")
		}
		v.dataAddr = opts.parsedDataAddr
		v.evalHeapPtr = v.baseHeapPtr + int32(len(opts.parsedData))
		if err = v.setHeapState(ctx, v.evalHeapPtr); err != nil {
			r.Close(ctx)
			return nil, err
		}
	} else if opts.data != nil {
		if v.dataAddr, err = v.toRegoJSON(ctx, opts.data, true); err != nil {
			r.Close(ctx)
			return nil, err
		}
	}

	if v.evalHeapPtr, err = v.getHeapState(ctx); err != nil {
		r.Close(ctx)
		return nil, err
	}

	// Build the builtin id -> topdown.BuiltinFunc map.
	builtinsAddr, err := call(ctx, v, "builtins")
	if err != nil {
		r.Close(ctx)
		return nil, err
	}
	builtinsVal, err := v.fromRegoJSON(ctx, builtinsAddr, true)
	if err != nil {
		r.Close(ctx)
		return nil, err
	}

	builtinMap := map[int32]topdown.BuiltinFunc{}
	for name, id := range builtinsVal.(map[string]any) {
		f := topdown.GetBuiltin(name)
		if f == nil {
			r.Close(ctx)
			return nil, fmt.Errorf("builtin '%s' not found", name)
		}
		n, err := id.(json.Number).Int64()
		if err != nil {
			r.Close(ctx)
			panic(err)
		}
		builtinMap[int32(n)] = f
	}
	v.dispatcher.SetMap(builtinMap)

	// Extract entrypoint IDs.
	epAddr, err := call(ctx, v, "entrypoints")
	if err != nil {
		r.Close(ctx)
		return nil, err
	}
	epMap, err := v.fromRegoJSON(ctx, epAddr, true)
	if err != nil {
		r.Close(ctx)
		return nil, err
	}
	for ep, value := range epMap.(map[string]any) {
		id, err := value.(json.Number).Int64()
		if err != nil {
			r.Close(ctx)
			return nil, err
		}
		v.entrypointIDs[ep] = int32(id)
	}

	return v, nil
}

func getABIVersion(mod api.Module) (int32, int32, error) {
	major := mod.ExportedGlobal("opa_wasm_abi_version")
	minor := mod.ExportedGlobal("opa_wasm_abi_minor_version")
	if major == nil || minor == nil {
		return 0, 0, errors.New("failed to read ABI version")
	}
	return int32(major.Get()), int32(minor.Get()), nil
}

// close releases the wazero Runtime and all resources associated with this VM.
func (v *VM) close() {
	if v.runtime != nil {
		v.runtime.Close(context.Background())
	}
}

// Eval performs an evaluation of the specified entrypoint, with any provided
// input, and returns the resulting value dumped to a string.
func (i *VM) Eval(ctx context.Context,
	entrypoint int32,
	input *any,
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ndbCache builtins.NDBCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	if i.abiMinorVersion < int32(2) {
		return i.evalCompat(ctx, entrypoint, input, metrics, seed, ns, iqbCache, ndbCache, ph, capabilities)
	}
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	inputAddr, inputLen := int32(0), int32(0)

	// NOTE: we'll never free the memory used for the input string during
	// the one evaluation, but we'll overwrite it on the next evaluation.
	heapPtr := i.evalHeapPtr

	if input != nil {
		metrics.Timer("wasm_vm_eval_prepare_input").Start()
		var raw []byte
		switch v := (*input).(type) {
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
				return nil, err
			}
		}
		inputLen = int32(len(raw))
		inputAddr = i.evalHeapPtr

		end := uint32(inputAddr) + uint32(inputLen)
		if end > i.mem.Size() {
			delta := end - i.mem.Size()
			neededPages := util.Pages(delta)
			if _, ok := i.mem.Grow(neededPages); !ok {
				return nil, fmt.Errorf("input: failed to grow memory by `%d` (max pages %d)", neededPages, i.memoryMax)
			}
		}
		if !i.mem.Write(uint32(inputAddr), raw) {
			return nil, errors.New("write input to memory")
		}
		heapPtr += inputLen

		metrics.Timer("wasm_vm_eval_prepare_input").Stop()
	}

	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	stop := i.dispatcher.Reset(ctx, seed, ns, iqbCache, ndbCache, ph, capabilities)
	defer stop()

	metrics.Timer("wasm_vm_eval_call").Start()
	resultAddr, err := call(ctx, i, "opa_eval", 0 /* reserved */, entrypoint, i.dataAddr, inputAddr, inputLen, heapPtr, 1 /* value output */)
	if err != nil {
		return nil, err
	}
	metrics.Timer("wasm_vm_eval_call").Stop()

	data, ok := i.mem.Read(uint32(resultAddr), i.mem.Size()-uint32(resultAddr))
	if !ok {
		return nil, fmt.Errorf("read result from memory at %d", resultAddr)
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	// Skip free'ing input and result JSON as the heap will be reset next round anyway.
	return data[:n], nil
}

// evalCompat implements policy evaluation for ABI 1.1 modules which predate
// the one-shot opa_eval export (added in ABI 1.2).
func (i *VM) evalCompat(ctx context.Context,
	entrypoint int32,
	input *any,
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ndbCache builtins.NDBCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	metrics.Timer("wasm_vm_eval_prepare_input").Start()

	stop := i.dispatcher.Reset(ctx, seed, ns, iqbCache, ndbCache, ph, capabilities)
	defer stop()

	if err := i.setHeapState(ctx, i.evalHeapPtr); err != nil {
		return nil, err
	}

	ctxAddr, err := call(ctx, i, "opa_eval_ctx_new")
	if err != nil {
		return nil, err
	}

	if i.dataAddr != 0 {
		if err := callVoid(ctx, i, "opa_eval_ctx_set_data", ctxAddr, i.dataAddr); err != nil {
			return nil, err
		}
	}

	if err := callVoid(ctx, i, "opa_eval_ctx_set_entrypoint", ctxAddr, entrypoint); err != nil {
		return nil, err
	}

	if input != nil {
		inputAddr, err := i.toRegoJSON(ctx, *input, false)
		if err != nil {
			return nil, err
		}
		if err := callVoid(ctx, i, "opa_eval_ctx_set_input", ctxAddr, inputAddr); err != nil {
			return nil, err
		}
	}
	metrics.Timer("wasm_vm_eval_prepare_input").Stop()

	metrics.Timer("wasm_vm_eval_execute").Start()
	err = callVoid(ctx, i, "eval", ctxAddr)
	metrics.Timer("wasm_vm_eval_execute").Stop()
	if err != nil {
		return nil, err
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Start()
	resultAddr, err := call(ctx, i, "opa_eval_ctx_get_result", ctxAddr)
	if err != nil {
		return nil, err
	}

	serialized, err := call(ctx, i, "opa_value_dump", resultAddr)
	if err != nil {
		return nil, err
	}

	data, ok := i.mem.Read(uint32(serialized), i.mem.Size()-uint32(serialized))
	if !ok {
		return nil, fmt.Errorf("read result from memory at %d", serialized)
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Stop()
	return data[:n], nil
}

// SetPolicyData Will either update the VM's data or, if the policy changed,
// re-initialize the VM.
func (i *VM) SetPolicyData(ctx context.Context, opts vmOpts) error {

	if !bytes.Equal(opts.policy, i.policy) {
		// Swap the instance to a new one, with new policy.
		n, err := newVM(ctx, opts)
		if err != nil {
			return err
		}
		oldRuntime := i.runtime
		*i = *n
		oldRuntime.Close(context.Background())
		return nil
	}

	i.dataAddr = 0

	// Release any stashed heap blocks since they will be above the base heap pointer
	if err := callVoid(ctx, i, "opa_heap_stash_clear"); err != nil {
		return err
	}

	if err := i.setHeapState(ctx, i.baseHeapPtr); err != nil {
		return err
	}

	var err error
	if opts.parsedData != nil {
		avail := i.mem.Size() - uint32(i.baseHeapPtr)
		if avail < uint32(len(opts.parsedData)) {
			delta := uint32(len(opts.parsedData)) - avail
			neededPages := util.Pages(delta)
			if _, ok := i.mem.Grow(neededPages); !ok {
				return fmt.Errorf("failed to grow memory by `%d` for parsed data (max pages %d)", neededPages, i.memoryMax)
			}
		}
		length := int32(len(opts.parsedData))
		if !i.mem.Write(uint32(i.baseHeapPtr), opts.parsedData) {
			return errors.New("write parsed data into memory")
		}
		i.dataAddr = opts.parsedDataAddr
		i.evalHeapPtr = i.baseHeapPtr + length
		if err = i.setHeapState(ctx, i.evalHeapPtr); err != nil {
			return err
		}
	} else if opts.data != nil {
		if i.dataAddr, err = i.toRegoJSON(ctx, opts.data, true); err != nil {
			return err
		}
	}

	// Stash any free blocks so that eval()/setHeapState() won't leak them
	if err = callVoid(ctx, i, "opa_heap_blocks_stash"); err != nil {
		return err
	}

	i.evalHeapPtr, err = i.getHeapState(ctx)
	return err
}

type abortError struct {
	message string
}

func (e abortError) Error() string { return e.message }

type cancelledError struct {
	message string
}

func (e cancelledError) Error() string { return e.message }

type builtinError struct {
	err error
}

func (e builtinError) Error() string { return e.err.Error() }

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval().
func (i *VM) Entrypoints() map[string]int32 {
	return i.entrypointIDs
}

// SetDataPath will update the current data on the VM by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) SetDataPath(ctx context.Context, path []string, value any) error {
	// Reset the heap ptr before patching the vm to try and keep any
	// new allocations safe from subsequent heap resets on eval.
	if err := i.setHeapState(ctx, i.evalHeapPtr); err != nil {
		return err
	}

	// Restore saved blocks protected from leaking in eval()/setHeapState()
	if err := callVoid(ctx, i, "opa_heap_blocks_restore"); err != nil {
		return err
	}

	valueAddr, err := i.toRegoJSON(ctx, value, true)
	if err != nil {
		return err
	}

	pathAddr, err := i.toRegoJSON(ctx, path, true)
	if err != nil {
		return err
	}

	result, err := call(ctx, i, "opa_value_add_path", i.dataAddr, pathAddr, valueAddr)
	if err != nil {
		return err
	}

	// We don't need to free the value, assume it is "owned" as part of the
	// overall data object now.
	// We do need to free the path
	if err = callVoid(ctx, i, "opa_value_free", pathAddr); err != nil {
		return err
	}

	// Stash free blocks so eval() calls don't leak them when calling setHeapState()
	if err = callVoid(ctx, i, "opa_heap_blocks_stash"); err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any new allocations done
	// while patching.
	i.evalHeapPtr, err = i.getHeapState(ctx)
	if err != nil {
		return err
	}

	if result != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, result)
	}

	return nil
}

// RemoveDataPath will update the current data on the VM by removing the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) RemoveDataPath(ctx context.Context, path []string) error {
	// Reset the heap ptr before patching the vm to try and keep any
	// new allocations safe from subsequent heap resets on eval.
	if err := i.setHeapState(ctx, i.evalHeapPtr); err != nil {
		return err
	}

	// Restore saved blocks protected from leaking in eval()/setHeapState()
	if err := callVoid(ctx, i, "opa_heap_blocks_restore"); err != nil {
		return err
	}

	pathAddr, err := i.toRegoJSON(ctx, path, true)
	if err != nil {
		return err
	}

	errc, err := call(ctx, i, "opa_value_remove_path", i.dataAddr, pathAddr)
	if err != nil {
		return err
	}

	if err = callVoid(ctx, i, "opa_value_free", pathAddr); err != nil {
		return err
	}

	// Stash free blocks so eval() calls don't leak them when calling setHeapState()
	if err = callVoid(ctx, i, "opa_heap_blocks_stash"); err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any newly available memory
	if i.evalHeapPtr, err = i.getHeapState(ctx); err != nil {
		return err
	}

	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// fromRegoJSON parses serialized JSON from the Wasm memory buffer into
// native go types.
func (i *VM) fromRegoJSON(ctx context.Context, addr int32, free bool) (any, error) {
	serialized, err := call(ctx, i, "opa_json_dump", addr)
	if err != nil {
		return nil, err
	}

	data, ok := i.mem.Read(uint32(serialized), i.mem.Size()-uint32(serialized))
	if !ok {
		return nil, fmt.Errorf("read memory at %d", serialized)
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = 0
	}

	// Parse the result into go types.
	decoder := json.NewDecoder(bytes.NewReader(data[:n]))
	decoder.UseNumber()

	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}

	if free {
		if err = callVoid(ctx, i, "opa_free", serialized); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// toRegoJSON converts go native JSON to Rego JSON. If the value is
// an AST type it will be dumped using its stringer.
func (i *VM) toRegoJSON(ctx context.Context, v any, free bool) (int32, error) {
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
	p, err := call(ctx, i, "opa_malloc", n)
	if err != nil {
		return 0, err
	}

	if !i.mem.Write(uint32(p), raw) {
		return 0, fmt.Errorf("write %d bytes at %d", n, p)
	}

	addr, err := call(ctx, i, "opa_value_parse", p, n)
	if err != nil {
		return 0, err
	}

	if free {
		if err = callVoid(ctx, i, "opa_free", p); err != nil {
			return 0, err
		}
	}

	return addr, nil
}

func (i *VM) getHeapState(ctx context.Context) (int32, error) {
	return call(ctx, i, "opa_heap_ptr_get")
}

func (i *VM) setHeapState(ctx context.Context, ptr int32) error {
	return callVoid(ctx, i, "opa_heap_ptr_set", ptr)
}

func (i *VM) cloneDataSegment() (int32, []byte) {
	// The parsed data values sit between the base heap address and end
	// at the eval heap pointer address.
	size := uint32(i.evalHeapPtr - i.baseHeapPtr)
	srcData, _ := i.mem.Read(uint32(i.baseHeapPtr), size)
	patchedData := make([]byte, len(srcData))
	copy(patchedData, srcData)
	return i.dataAddr, patchedData
}

func call(ctx context.Context, vm *VM, name string, args ...int32) (int32, error) {
	res, err := callOrCancel(ctx, vm, name, args...)
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, nil
	}
	return res.(int32), nil
}

func callVoid(ctx context.Context, vm *VM, name string, args ...int32) error {
	_, err := callOrCancel(ctx, vm, name, args...)
	return err
}

func callOrCancel(ctx context.Context, vm *VM, name string, args ...int32) (any, error) {
	ul := make([]uint64, len(args))
	for i, a := range args {
		ul[i] = uint64(uint32(a))
	}

	f := vm.mod.ExportedFunction(name)
	if f == nil {
		return nil, fmt.Errorf("function %q not found in policy module", name)
	}

	res, err := f.Call(ctx, ul...)
	if err != nil {
		var abort abortError
		if errors.As(err, &abort) {
			return 0, sdk_errors.New(sdk_errors.InternalErr, abort.message)
		}
		var cancelled cancelledError
		if errors.As(err, &cancelled) {
			return 0, sdk_errors.New(sdk_errors.CancelledErr, cancelled.message)
		}
		var be builtinError
		if errors.As(err, &be) {
			return 0, sdk_errors.New(sdk_errors.InternalErr, be.err.Error())
		}
		// WithCloseOnContextDone: wazero closes the module and returns sys.ExitError
		// when ctx is cancelled or times out mid-execution (tight wasm loops included).
		var exitErr *sys.ExitError
		if errors.As(err, &exitErr) {
			c := exitErr.ExitCode()
			if c == sys.ExitCodeContextCanceled || c == sys.ExitCodeDeadlineExceeded {
				vm.dead = true
				return 0, sdk_errors.New(sdk_errors.CancelledErr, "interrupted")
			}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, sdk_errors.New(sdk_errors.CancelledErr, "interrupted")
		}
		return 0, err
	}

	if len(res) == 0 {
		return nil, nil
	}
	return int32(res[0]), nil
}
