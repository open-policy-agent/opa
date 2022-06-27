package wazero

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/tetratelabs/wazero"
)

type vmOpts struct {
	policy         []byte
	data           []byte
	parsedData     []byte
	parsedDataAddr int32
	memoryMin      uint32
	memoryMax      uint32
}
type VM struct {
	runtime              *wazero.Runtime
	ctx                  context.Context
	module               Module
	policy               []byte
	memoryMin            int
	memoryMax            int
	abiMajorVersion      int32
	abiMinorVersion      int32
	entrypointIDs        map[string]int32
	baseHeapPtr          int32
	dataAddr             int32
	dataLen              int32
	evalHeapPtr          int32
	evalOneOff           func(context.Context, int32, int32, int32, int32, int32) (int32, error)
	eval                 func(context.Context, int32) error
	evalCtxGetResult     func(context.Context, int32) (int32, error)
	evalCtxNew           func(context.Context) (int32, error)
	evalCtxSetData       func(context.Context, int32, int32) error
	evalCtxSetInput      func(context.Context, int32, int32) error
	evalCtxSetEntrypoint func(context.Context, int32, int32) error
	heapPtrGet           func(context.Context) (int32, error)
	heapPtrSet           func(context.Context, int32) error
	jsonDump             func(context.Context, int32) (int32, error)
	jsonParse            func(context.Context, int32, int32) (int32, error)
	valueDump            func(context.Context, int32) (int32, error)
	valueParse           func(context.Context, int32, int32) (int32, error)
	malloc               func(context.Context, int32) (int32, error)
	free                 func(context.Context, int32) error
	valueAddPath         func(context.Context, int32, int32, int32) (int32, error)
	valueRemovePath      func(context.Context, int32, int32) (int32, error)
}

func newVM(opts vmOpts, runtime *wazero.Runtime) (*VM, error) {
	vm := VM{}
	vm.ctx = context.Background()
	vm.runtime = runtime
	vm.policy = opts.policy
	vm.memoryMin = int(opts.memoryMin)
	vm.memoryMax = int(opts.memoryMax)
	modOpts := moduleOpts{policy: opts.policy, ctx: vm.ctx, minMemSize: int(opts.memoryMin), maxMemSize: int(opts.memoryMax), vm: &vm}
	vm.module = newModule(modOpts, *runtime)
	vm.abiMajorVersion = vm.module.wasmAbiVersion()
	vm.abiMinorVersion = vm.module.wasmAbiMinorVersion()
	vm.entrypointIDs = vm.GetEntrypoints()
	vm.dataAddr = opts.parsedDataAddr
	vm.evalOneOff = vm.module.opaEval
	vm.eval = vm.module.eval
	vm.evalCtxGetResult = vm.module.evalCtxGetResult
	vm.evalCtxNew = vm.module.evalCtxNew
	vm.evalCtxSetData = vm.module.evalCtxSetData
	vm.evalCtxSetInput = vm.module.evalCtxSetInput
	vm.evalCtxSetEntrypoint = vm.module.evalCtxSetEntrypoint
	vm.heapPtrGet = vm.module.heapPtrGet
	vm.heapPtrSet = vm.module.heapPtrSet
	vm.jsonDump = vm.module.jsonDump
	vm.jsonParse = vm.module.jsonParse
	vm.valueDump = vm.module.valueDump
	vm.valueParse = vm.module.valueParse
	vm.malloc = vm.module.malloc
	vm.free = vm.module.free
	vm.valueAddPath = vm.module.valueAddPath
	vm.valueRemovePath = vm.module.valueRemovePath
	err := vm.setData(vm.ctx, opts, "newVM")
	if err != nil {
		return nil, err
	}
	return &vm, nil
}
func (i *VM) SetPolicyData(ctx context.Context, opts vmOpts) error {

	if !bytes.Equal(opts.policy, i.policy) {
		// Swap the instance to a new one, with new policy.
		i.module.module.Close(i.ctx)
		i.module.env.Close(i.ctx)
		n, err := newVM(opts, i.runtime)
		if err != nil {
			return err
		}

		*i = *n
		return nil
	}

	i.dataAddr = opts.parsedDataAddr

	if err := i.setHeapState(ctx, i.baseHeapPtr); err != nil {
		return err
	}

	return i.setData(i.ctx, opts, "setPolicyData")
}

func (i *VM) setData(ctx context.Context, opts vmOpts, path string) error {
	var err error
	if i.baseHeapPtr, err = i.getHeapState(ctx); err != nil {
		return err
	}

	// Optimization for cloning a vm, if provided a parsed data memory buffer
	// insert it directly into the new vm's buffer and set pointers accordingly.
	// This only works because the placement is deterministic (eg, for a given policy
	// the base heap pointer and parsed data layout will always be the same).
	if opts.parsedData != nil {
		err = i.module.writeMemPlus(uint32(i.baseHeapPtr), opts.parsedData, "data")
		if err != nil {
			return err
		}
		i.dataAddr = opts.parsedDataAddr
		i.evalHeapPtr = i.baseHeapPtr + int32(len(opts.parsedData))
		err := i.setHeapState(ctx, i.evalHeapPtr)
		if err != nil {
			return err
		}
	} else if opts.data != nil {
		if err = i.toDRegoJSON(ctx, opts.data, true); err != nil {
			return err
		}
	}
	if i.evalHeapPtr, err = i.getHeapState(ctx); err != nil {
		return err
	}
	return nil
}

// Println is invoked if the policy WASM code calls opa_println().
func (i *VM) Println(arg int32) {
	data := i.module.readFrom(arg)
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		panic("invalid opa_println argument")
	}

	fmt.Printf("opa_println(): %s\n", string(data[:n]))
}

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval().
func (i *VM) Entrypoints() map[string]int32 {
	return i.entrypointIDs
}

// SetDataPath will update the current data on the VM by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) SetDataPath(ctx context.Context, path []string, value interface{}) error {
	// Reset the heap ptr before patching the vm to try and keep any
	// new allocations safe from subsequent heap resets on eval.
	err := i.setHeapState(ctx, i.evalHeapPtr)
	if err != nil {
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

	result, err := i.valueAddPath(ctx, i.dataAddr, pathAddr, valueAddr)
	if err != nil {
		return err
	}

	// We don't need to free the value, assume it is "owned" as part of the
	// overall data object now.
	// We do need to free the path

	if err := i.free(ctx, pathAddr); err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any new allocations done
	// while patching.
	i.evalHeapPtr, err = i.getHeapState(ctx)
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
func (i *VM) RemoveDataPath(ctx context.Context, path []string) error {
	pathAddr, err := i.toRegoJSON(ctx, path, true)
	if err != nil {
		return err
	}

	errc, err := i.valueRemovePath(ctx, i.dataAddr, pathAddr)
	if err != nil {
		return err
	}

	if err := i.free(ctx, pathAddr); err != nil {
		return err
	}

	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

type builtinError struct {
	err error
}
type abortError struct {
	message string
}

type cancelledError struct {
	message string
}

// toRegoJSON converts go native JSON to Rego JSON. If the value is
// an AST type it will be dumped using its stringer.
func (i *VM) toRegoJSON(ctx context.Context, v interface{}, free bool) (int32, error) {
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
	p := int32(i.module.writeMem(raw))

	addr, err := i.valueParse(ctx, p, n)
	if err != nil {
		return 0, err
	}

	if free {
		if err := i.free(ctx, p); err != nil {
			return 0, err
		}
	}

	return addr, nil
}

//
//Parses the json data, writes it to the shared memory buffer and updates the baseHeapPtr and evalHeapPtr values accordingly
//Is used when setting the policy data
//
func (i *VM) toDRegoJSON(ctx context.Context, v interface{}, free bool) error {
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
			return err
		}
	}

	n := int32(len(raw))
	p := int32(i.module.writeMem(raw))
	i.dataLen = int32(n)
	addr, err := i.valueParse(ctx, p, n)
	if err != nil {
		return err
	}
	i.dataAddr = addr
	cPtr, err := i.getHeapState(ctx)
	if err != nil {
		return err
	}
	i.dataLen = cPtr - addr
	if free {
		if err := i.free(ctx, p); err != nil {
			return err
		}
	}

	return nil
}
func (i *VM) getHeapState(ctx context.Context) (int32, error) {
	return i.heapPtrGet(ctx)
}

func (i *VM) setHeapState(ctx context.Context, ptr int32) error {
	return i.heapPtrSet(ctx, ptr)
}

//copies the parsed data to optimize cloning VMs
func (i *VM) cloneDataSegment() (int32, []byte) {
	srcData := i.module.readFrom(0)[i.baseHeapPtr:i.evalHeapPtr]
	patchedData := make([]byte, len(srcData))
	copy(patchedData, srcData)
	return i.dataAddr, patchedData
}
func (i *VM) GetEntrypoints() map[string]int32 {
	return i.module.GetEntrypoints()
}
func (i *VM) Eval(ctx context.Context,
	entrypoint int32,
	input *interface{},
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	i.module.Reset(ctx, seed, ns, iqbCache, ph, capabilities)
	if i.abiMinorVersion < int32(2) {
		return i.evalCompat(ctx, entrypoint, input, metrics, seed, ns, iqbCache, ph, capabilities)
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
		err := i.module.writeMemPlus(uint32(inputAddr), raw, "input")
		if err != nil {
			return nil, err
		}
		heapPtr += inputLen
		metrics.Timer("wasm_vm_eval_prepare_input").Stop()
	}
	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	i.module.Reset(ctx, seed, ns, iqbCache, ph, capabilities)
	metrics.Timer("wasm_vm_eval_call").Start()
	resultAddr, err := i.evalOneOff(ctx, int32(entrypoint), i.dataAddr, inputAddr, inputLen, heapPtr)
	if err != nil {
		return nil, err
	}
	metrics.Timer("wasm_vm_eval_call").Stop()

	data := i.module.readUntil(resultAddr, 0b0)
	dataC := make([]byte, len(data)-2)
	copy(dataC, data[1:len(data)-1])
	retVals := []byte{byte(123)}
	retVals = append(retVals, dataC...)
	retVals = append(retVals, byte(125))
	retVals = formatAsReturn(retVals)
	if string(retVals) == "{}" {
		return []byte("set()"), nil
	}

	return retVals, nil
}
func formatAsReturn(data []byte) []byte {
	outBytes := []byte{}
	inString := false
	bComma := byte(',')
	bColon := byte(':')
	prev := byte(' ')
	for _, b := range data {
		outBytes = append(outBytes, b)
		if (b == bComma || b == bColon) && !inString {
			outBytes = append(outBytes, byte(' '))
		} else if b == byte('"') && prev != byte('\\') {
			inString = !inString
		}
		prev = outBytes[len(outBytes)-1]
	}
	return outBytes
}
func (i *VM) evalCompat(ctx context.Context,
	entrypoint int32,
	input *interface{},
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	i.module.Reset(ctx, seed, ns, iqbCache, ph, capabilities)
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	metrics.Timer("wasm_vm_eval_prepare_input").Start()

	err := i.setHeapState(ctx, i.evalHeapPtr)
	if err != nil {
		return nil, err
	}

	// Parse the input JSON and activate it with the data.
	ctxAddr, err := i.evalCtxNew(ctx)
	if err != nil {
		return nil, err
	}

	if i.dataAddr != 0 {
		if err := i.evalCtxSetData(ctx, ctxAddr, i.dataAddr); err != nil {
			return nil, err
		}
	}

	if err := i.evalCtxSetEntrypoint(ctx, ctxAddr, int32(entrypoint)); err != nil {
		return nil, err
	}

	if input != nil {
		inputAddr, err := i.toRegoJSON(ctx, *input, false)
		if err != nil {
			return nil, err
		}

		if err := i.evalCtxSetInput(ctx, ctxAddr, inputAddr); err != nil {
			return nil, err
		}
	}
	metrics.Timer("wasm_vm_eval_prepare_input").Stop()

	// Evaluate the policy.
	metrics.Timer("wasm_vm_eval_execute").Start()
	err = i.eval(ctx, ctxAddr)
	metrics.Timer("wasm_vm_eval_execute").Stop()
	if err != nil {
		return nil, err
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Start()
	resultAddr, err := i.evalCtxGetResult(ctx, ctxAddr)
	if err != nil {
		return nil, err
	}

	serialized, err := i.valueDump(ctx, resultAddr)
	if err != nil {
		return nil, err
	}

	data := i.module.readUntil(serialized, 0b0)

	metrics.Timer("wasm_vm_eval_prepare_result").Stop()

	// Skip free'ing input and result JSON as the heap will be reset next round anyway.
	return data, nil
}
