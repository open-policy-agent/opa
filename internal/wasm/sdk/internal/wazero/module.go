package wazero

import (
	"context"

	"errors"
	"fmt"
	"io"
	"log"

	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
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
	module      api.Module
	env         environment
	ctx         context.Context
	vm          *VM
	entrypointT map[string]int32
}

// Env is a wasm module that holds the shared memory buffer and the builtin bindings

func (m *Module) GetEntrypoints() map[string]int32 {
	eLoc := m.entrypoints(m.ctx)
	return parseJsonString(m.fromRegoJSON(eLoc))
}

// calls the built-in functions

// resets the Builtin Context
func (m *Module) Reset(ctx context.Context,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) {
	m.env.builtins.Reset(ctx, seed, ns, iqbCache, ph, capabilities)

}

func newModule(opts moduleOpts, r wazero.Runtime) Module {
	m := Module{}
	m.vm = opts.vm
	m.ctx = opts.ctx
	var err error

	m.env, err = newEnv(&m, opts, r)
	if err != nil {
		log.Panic(err)
	}
	m.module, err = r.InstantiateModuleFromBinary(opts.ctx, opts.policy)
	if err != nil {
		log.Panic(err)
	}
	m.entrypointT = m.GetEntrypoints()
	m.env.setBuiltins()
	return m
}

// reads the shared memory buffer
func (m *Module) readMem(offset, length uint32) []byte {
	return m.env.readMem(offset, length)
}

//reads a single byte from the shared memory buffer
func (m *Module) readMemByte(offset uint32) byte {
	return m.env.readMemByte(offset)
}

//writes data to a given point in memory, grows if necessary
func (m *Module) writeMemPlus(wAddr uint32, wData []byte, caller string) error {
	return m.env.writeMemPlus(wAddr, wData, caller)
}

//allocates and writes data to the shared memory buffer
func (m *Module) writeMem(data []byte) uint32 {
	return m.env.writeMem(data)
}

//reads a null terminated string starting at the given address in the shared memory buffer
func (m *Module) readStr(loc uint32) string {
	bytes := []byte{}
	var index uint32 = 0
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
func (m *Module) fromRegoJSON(addr int32) string {
	dump_addr, err := m.json_dump(m.ctx, addr)
	if err != nil {
		log.Panic(err)
	}
	str := m.readStr(uint32(dump_addr))
	return str
}

//Reads and returns the shared memory buffer from the given address and stops when it reaches the terminator byte or reaches the end of the buffer
func (m *Module) readUntil(addr int32, terminator byte) []byte {
	return m.env.readUntil(addr, terminator)
}

//reads the shared memory buffer from the given address to the end
func (m *Module) readFrom(addr int32) []byte {
	return m.env.readFrom(addr)
}

//
// Expose the exported wasm functions for ease of use
//
func (m Module) wasm_abi_version() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_version").Get(m.ctx))
}
func (m Module) wasm_abi_minor_version() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_minor_version").Get(m.ctx))
}
func (m Module) eval(ctx context.Context, ctx_addr int32) error {
	_, err := m.module.ExportedFunction("eval").Call(ctx, uint64(ctx_addr))
	return err
}
func (m Module) builtins(ctx context.Context) int32 {
	addr, _ := m.module.ExportedFunction("builtins").Call(ctx)
	return int32(addr[0])
}
func (m Module) entrypoints(ctx context.Context) int32 {
	addr, _ := m.module.ExportedFunction("entrypoints").Call(ctx)
	return int32(addr[0])
}
func (m Module) eval_ctx_new(ctx context.Context) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_new").Call(ctx)
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) eval_ctx_set_input(ctx context.Context, ctx_addr, value_addr int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_input").Call(ctx, uint64(ctx_addr), uint64(value_addr))
	return err
}
func (m Module) eval_ctx_set_data(ctx context.Context, ctx_addr, value_addr int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, uint64(ctx_addr), uint64(value_addr))
	return err
}
func (m Module) eval_ctx_set_entrypoint(ctx context.Context, ctx_addr, entrypoint_id int32) error {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, uint64(ctx_addr), uint64(entrypoint_id))
	return err
}
func (m Module) eval_ctx_get_result(ctx context.Context, ctx_addr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_get_result").Call(ctx, uint64(ctx_addr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) malloc(ctx context.Context, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_malloc").Call(ctx, uint64(size))
	if err != nil {
		return 0, errors.New("internal_error: opa_malloc: failed")
	}
	return int32(addr[0]), err
}
func (m Module) free(ctx context.Context, addr int32) error {
	_, err := m.module.ExportedFunction("opa_free").Call(ctx, uint64(addr))
	return err
}
func (m Module) json_parse(ctx context.Context, str_addr, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_json_parse").Call(ctx, uint64(str_addr), uint64(size))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) value_parse(ctx context.Context, str_addr, size int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_value_parse").Call(ctx, uint64(str_addr), uint64(size))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) json_dump(ctx context.Context, value_addr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_json_dump").Call(ctx, uint64(value_addr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) value_dump(ctx context.Context, value_addr int32) (int32, error) {
	log.Println(m.module.ExportedFunction("opa_value_dump"))
	log.Println("value_dump:")
	addr, err := m.module.ExportedFunction("opa_value_dump").Call(ctx, uint64(value_addr))
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) heap_ptr_set(ctx context.Context, addr int32) error {
	_, err := m.module.ExportedFunction("opa_heap_ptr_set").Call(ctx, uint64(addr))
	return err
}
func (m Module) heap_ptr_get(ctx context.Context) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_heap_ptr_get").Call(ctx)
	if err != nil {
		return 0, err
	}
	return int32(addr[0]), err
}
func (m Module) value_add_path(ctx context.Context, base_value_addr, path_value_addr, value_addr int32) (int32, error) {
	ret, err := m.module.ExportedFunction("opa_value_add_path").Call(ctx, uint64(base_value_addr), uint64(path_value_addr), uint64(value_addr))
	if err != nil {
		return 0, err
	}
	return int32(ret[0]), err
}
func (m Module) value_remove_path(ctx context.Context, base_value_addr, path_value_addr int32) (int32, error) {
	ret, err := m.module.ExportedFunction("opa_value_remove_path").Call(ctx, uint64(base_value_addr), uint64(path_value_addr))
	if err != nil {
		return 0, err
	}
	return int32(ret[0]), err
}
func (m Module) opa_eval(ctx context.Context, entrypoint_id, data, input, input_len, heap_ptr int32) (int32, error) {
	addr, err := m.module.ExportedFunction("opa_eval").Call(ctx, 0, uint64(entrypoint_id), uint64(data), uint64(input), uint64(input_len), uint64(heap_ptr), 0)
	if err != nil {
		str := err.Error()
		if str[0] == '{' {
			str = str[1:]
		}
		var end int
		end = strings.Index(str, " (recovered")
		if str[end-1] == '}' {
			end--
		}
		return 0, fmt.Errorf("internal_error: %s", str[:end])
	}
	return int32(addr[0]), err
}
