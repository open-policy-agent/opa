package wazero

import (
	bs "bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type moduleOpts struct {
	policy     []byte
	ctx        context.Context
	minMemSize int
	maxMemSize int
	vm         *VM
}

//wrapper for wazero policy module and environment module
type Module struct {
	module, env            api.Module
	ctx, ctxt              context.Context
	tCTX                   *topdown.BuiltinContext
	vm                     *VM
	maxMemSize, minMemSize int
	builtinT               map[int32]topdown.BuiltinFunc
	entrypointT            map[string]int32
	seed                   io.Reader
	ns                     time.Time
	iqbCache               cache.InterQueryCache
	ph                     print.Hook
	capabilities           *ast.Capabilities
}

// Env is a wasm module that holds the shared memory buffer and the builtin bindings
func (m *Module) newEnv(opts moduleOpts, r wazero.Runtime) (api.Module, error) {
	if opts.maxMemSize == (moduleOpts{}).maxMemSize {

		return r.NewModuleBuilder("env").
			ExportFunction("opa_abort", m.opaAbort).
			ExportFunction("opa_builtin0", m.C0).
			ExportFunction("opa_builtin1", m.C1).
			ExportFunction("opa_builtin2", m.C2).
			ExportFunction("opa_builtin3", m.C3).
			ExportFunction("opa_builtin4", m.C4).
			ExportFunction("opa_println", m.opaPrintln).
			ExportMemory("memory", uint32(opts.minMemSize)).
			Instantiate(opts.ctx, r)
	}
	return r.NewModuleBuilder("env").
		ExportFunction("opa_abort", m.opaAbort).
		ExportFunction("opa_builtin0", m.C0).
		ExportFunction("opa_builtin1", m.C1).
		ExportFunction("opa_builtin2", m.C2).
		ExportFunction("opa_builtin3", m.C3).
		ExportFunction("opa_builtin4", m.C4).
		ExportFunction("opa_println", m.opaPrintln).
		ExportMemoryWithMax("memory", uint32(opts.minMemSize), uint32(opts.maxMemSize)).
		Instantiate(opts.ctx, r)

}
func (m *Module) GetEntrypoints() map[string]int32 {
	eLoc := m.entrypoints(m.ctx)
	return parseJSONString(m.fromRegoJSON(eLoc))
}

// Internal error
func (m *Module) opaAbort(ptr int32) {
	data := m.readFrom(ptr)
	n := bs.IndexByte(data, 0)
	if n < 0 {
		panic("invalid abort argument")
	}
	panic(abortError{message: string(data[:n])})
}

// calls the built-in functions
func (m *Module) Call(id, ctx int32, args ...int32) int32 {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-m.tCTX.Context.Done():
			m.tCTX.Cancel.Cancel()
		}
	}()
	var output *ast.Term
	pArgs := []*ast.Term{}
	for _, ter := range args {
		serialized, err := m.valueDump(m.ctx, (ter))
		if err != nil {
			panic(builtinError{err: err})
		}
		data := m.readStr(uint32(serialized))
		pTer, err := ast.ParseTerm(string(data))
		if err != nil {
			panic(builtinError{err: err})
		}
		pArgs = append(pArgs, pTer)
	}
	err := m.builtinT[id](*m.tCTX, pArgs, func(t *ast.Term) error {
		output = t
		return nil
	})
	if err != nil {
		if errors.As(err, &topdown.Halt{}) {
			var e *topdown.Error
			if errors.As(err, &e) && e.Code == topdown.CancelErr {

				panic(cancelledError{message: e.Message})
			}
			panic(builtinError{err: err})
		}
		// non-halt errors are treated as undefined ("non-strict eval" is the only
		// mode in wasm), the `output == nil` case below will return NULL
	}
	if output == nil {
		return 0
	}
	outB := []byte(output.String())
	loc := m.writeMem(outB)
	addr, err := m.valueParse(m.ctx, int32(loc), int32(len(outB)))
	if err != nil {
		panic(err)
	}
	return int32(addr)
}

// Exported to wasm and executes Call with the given builtin_id and arguments
func (m *Module) C0(id, ctx int32) int32 {
	return m.Call(id, ctx)
}
func (m *Module) C1(id, ctx, a1 int32) int32 {
	return m.Call(id, ctx, a1)
}
func (m *Module) C2(id, ctx, a1, a2 int32) int32 {
	return m.Call(id, ctx, a1, a2)
}
func (m *Module) C3(id, ctx, a1, a2, a3 int32) int32 {
	return m.Call(id, ctx, a1, a2, a3)
}
func (m *Module) C4(id, ctx, a1, a2, a3, a4 int32) int32 {
	return m.Call(id, ctx, a1, a2, a3, a4)
}

// resets the Builtin Context
func (m *Module) Reset(ctx context.Context,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) {
	if ns.IsZero() {
		ns = time.Now()
	}
	if seed == nil {
		seed = rand.Reader
	}
	m.ctxt = ctx
	m.seed = seed
	m.capabilities = capabilities
	m.ns = ns
	m.ph = ph
	m.iqbCache = iqbCache
	m.tCTX = &topdown.BuiltinContext{
		Context:                ctx,
		Metrics:                metrics.New(),
		Seed:                   seed,
		Time:                   ast.NumberTerm(json.Number(strconv.FormatInt(ns.UnixNano(), 10))),
		Cancel:                 topdown.NewCancel(),
		Runtime:                nil,
		Cache:                  make(builtins.Cache),
		Location:               nil,
		Tracers:                nil,
		QueryTracers:           nil,
		QueryID:                0,
		ParentID:               0,
		InterQueryBuiltinCache: iqbCache,
		PrintHook:              ph,
		Capabilities:           capabilities,
	}
}

// Used by the policy to print values
func (m *Module) opaPrintln(ptr int32) {

	bytes := []byte{}
	var index uint32
	for ok := true; ok; {
		b := m.readMemByte(uint32(ptr) + index)
		if b == 0b0 {
			ok = false
		} else {
			bytes = append(bytes, b)
		}
		index++
	}
	out := ""
	for _, b := range bytes {
		out += string(b)
	}
	fmt.Println(out)

}
func newModule(opts moduleOpts, r wazero.Runtime) Module {
	m := Module{}
	m.vm = opts.vm
	m.ctx = opts.ctx
	var err error

	m.env, err = m.newEnv(opts, r)
	m.minMemSize, m.maxMemSize = opts.minMemSize, opts.maxMemSize
	if err != nil {
		panic(err)
	}
	m.module, err = r.InstantiateModuleFromBinary(opts.ctx, opts.policy)
	if err != nil {
		panic(err)
	}
	m.tCTX = &topdown.BuiltinContext{Context: context.Background(), Metrics: metrics.New(),
		Time:  ast.NumberTerm(json.Number(strconv.FormatInt(time.Now().UnixNano(), 10))),
		Cache: make(builtins.Cache)}
	m.builtinT = newBuiltinTable(m)
	m.entrypointT = m.GetEntrypoints()
	return m
}

//reads a single byte from the shared memory buffer
func (m *Module) readMemByte(offset uint32) byte {
	data, _ := m.env.Memory().ReadByte(m.ctx, offset)
	return data
}

//writes data to a given point in memory, grows if necessary
func (m *Module) writeMemPlus(wAddr uint32, wData []byte, caller string) error {
	dataLeft := (m.env.Memory().Size(m.ctx)) - wAddr
	finPtrLoc := wAddr + uint32(len(wData))
	if (m.env.Memory().Size(m.ctx)) < finPtrLoc { // need to grow memory

		delta := uint32(len(wData)) - dataLeft
		_, success := m.env.Memory().Grow(m.ctx, Pages(uint32(delta)))
		if !success {
			return fmt.Errorf("%s: failed to grow memory by `%d` (max pages %d)", caller, Pages(delta), m.maxMemSize)
		}
	}
	m.env.Memory().Write(m.ctx, wAddr, wData)
	return nil
}

//allocates and writes data to the shared memory buffer
func (m *Module) writeMem(data []byte) uint32 {
	addr, err := m.malloc(m.ctx, int32(len(data)))
	if err != nil {
		panic("internal_error: opa_malloc: failed")
	}
	m.env.Memory().Write(m.ctx, uint32(addr), data)

	return uint32(addr)
}

//reads a null terminated string starting at the given address in the shared memory buffer
func (m *Module) readStr(loc uint32) string {
	bytes := []byte{}
	var index uint32
	for ok := true; ok; {
		b := m.readMemByte(loc + index)
		if b == 0b0 {
			ok = false
		} else {
			bytes = append(bytes, b)
		}
		index++
	}
	out := ""
	for _, b := range bytes {
		out += string(b)
	}
	return out
}

// returns a string representation of the JSON object stored att addr
func (m *Module) fromRegoJSON(addr int32) string {
	dumpAddr, err := m.jsonDump(m.ctx, addr)
	if err != nil {
		panic(err)
	}
	str := m.readStr(uint32(dumpAddr))
	return str
}

//Reads and returns the shared memory buffer from the given address and stops when it reaches the terminator byte or reaches the end of the buffer
func (m *Module) readUntil(addr int32, terminator byte) []byte {
	out := []byte{}
	for i, j := addr, true; j; i++ {
		_, j = m.env.Memory().Read(m.ctx, uint32(i), 1)
		if m.readMemByte(uint32(i)) == terminator {
			return out
		}
		out = append(out, m.readMemByte(uint32(i)))
	}
	return out
}

//reads the shared memory buffer from the given address to the end
func (m *Module) readFrom(addr int32) []byte {
	out := []byte{}
	for i, j := addr, true; j; i++ {
		_, j = m.env.Memory().Read(m.ctx, 0, uint32(i))
		if j {
			out = append(out, m.readMemByte(uint32(i)))
		}
	}
	return out
}

//
// Expose the exported wasm functions for ease of use
//
func (m *Module) wasmAbiVersion() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_version").Get(m.ctx))
}
func (m *Module) wasmAbiMinorVersion() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_minor_version").Get(m.ctx))
}
func (m *Module) eval(ctx context.Context, ctxAddr int32) error {
	_, err := m.module.ExportedFunction("eval").Call(ctx, uint64(ctxAddr))
	return err
}
func (m *Module) builtins(ctx context.Context) int32 {
	addr, _ := m.module.ExportedFunction("builtins").Call(ctx)
	return int32(addr[0])
}
func (m *Module) entrypoints(ctx context.Context) int32 {
	addr, _ := m.module.ExportedFunction("entrypoints").Call(ctx)
	return int32(addr[0])
}
func (m *Module) evalCtxNew(ctx context.Context) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_new").Call(ctx)
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) evalCtxSetInput(ctx context.Context, ctxAddr, valueAddr int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_input").Call(ctx, uint64(ctxAddr), uint64(valueAddr))
	return err
}
func (m *Module) evalCtxSetData(ctx context.Context, ctxAddr, valueAddr int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, uint64(ctxAddr), uint64(valueAddr))
	return err
}
func (m *Module) evalCtxSetEntrypoint(ctx context.Context, ctxAddr, entrypointID int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, uint64(ctxAddr), uint64(entrypointID))
	return err
}
func (m *Module) evalCtxGetResult(ctx context.Context, ctxAddr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_get_result").Call(ctx, uint64(ctxAddr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) malloc(ctx context.Context, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_malloc").Call(ctx, uint64(size))
	if err != nil {
		return 0, errors.New("internal_error: opa_malloc: failed")
	}
	return int32(addr[0]), err
}
func (m *Module) free(ctx context.Context, addr int32) error {
	_, err := m.module.ExportedFunction("opa_free").Call(ctx, uint64(addr))
	return err
}
func (m *Module) jsonParse(ctx context.Context, strAddr, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_json_parse").Call(ctx, uint64(strAddr), uint64(size))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) valueParse(ctx context.Context, strAddr, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_value_parse").Call(ctx, uint64(strAddr), uint64(size))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) jsonDump(ctx context.Context, valueAddr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_json_dump").Call(ctx, uint64(valueAddr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) valueDump(ctx context.Context, valueAddr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_value_dump").Call(ctx, uint64(valueAddr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) heapPtrSet(ctx context.Context, addr int32) error {
	_, err := m.module.ExportedFunction("opa_heap_ptr_set").Call(ctx, uint64(addr))
	return err
}
func (m *Module) heapPtrGet(ctx context.Context) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_heap_ptr_get").Call(ctx)
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m *Module) valueAddPath(ctx context.Context, baseValueAddr, pathValueAddr, valueAddr int32) (int32, error) {
	ret, err := m.module.ExportedFunction("opa_value_add_path").Call(ctx, uint64(baseValueAddr), uint64(pathValueAddr), uint64(valueAddr))
	if err != nil {
		return 0, err
	}
	return int32(ret[0]), err
}
func (m *Module) valueRemovePath(ctx context.Context, baseValueAddr, pathValueAddr int32) (int32, error) {
	ret, err := m.module.ExportedFunction("opa_value_remove_path").Call(ctx, uint64(baseValueAddr), uint64(pathValueAddr))
	if err != nil {
		return 0, err
	}
	return int32(ret[0]), err
}
func (m *Module) opaEval(ctx context.Context, entrypointID, data, input, inputLen, heapPtr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval").Call(ctx, 0, uint64(entrypointID), uint64(data), uint64(input), uint64(inputLen), uint64(heapPtr), 0)
	if err != nil {
		str := err.Error()[1:]
		end := strings.Index(str, "} (recovered")
		if end < 0 {
			return 0, err
		}
		return 0, fmt.Errorf("internal_error: %s", str[:end])
	}
	return int32(addr[0]), err
}
