package wazero

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"log"
)

type environment struct {
	module                 *Module
	env                    api.Module
	maxMemSize, minMemSize int
	builtins               *builtinContainer
	ctx                    context.Context
}

// reads the shared memory buffer
func (e *environment) readMem(offset, length uint32) []byte {
	data, _ := e.env.Memory().Read(e.ctx, offset, length)
	return data
}

//reads a single byte from the shared memory buffer
func (e *environment) readMemByte(offset uint32) byte {
	data, _ := e.env.Memory().ReadByte(e.ctx, offset)
	return data
}

//writes data to a given point in memory, grows if necessary
func (e *environment) writeMemPlus(wAddr uint32, wData []byte, caller string) error {
	dataLeft := (e.env.Memory().Size(e.ctx)) - wAddr
	finPtrLoc := wAddr + uint32(len(wData))
	if (e.env.Memory().Size(e.ctx)) < finPtrLoc { // need to grow memory

		delta := uint32(len(wData)) - dataLeft
		_, success := e.env.Memory().Grow(e.ctx, Pages(uint32(delta)))
		if !success {
			return fmt.Errorf("%s: failed to grow memory by `%d` (max pages %d)", caller, Pages(delta), e.maxMemSize)
		}
	}
	e.env.Memory().Write(e.ctx, wAddr, wData)
	return nil
}

//allocates and writes data to the shared memory buffer
func (e *environment) writeMem(data []byte) uint32 {
	addr, err := e.module.malloc(e.ctx, uint64(len(data)))
	if err != nil {
		log.Panic("internal_error: opa_malloc: failed")
	}
	e.env.Memory().Write(e.ctx, uint32(addr[0]), data)

	return uint32(addr[0])
}
func (e *environment) readFrom(addr int32) []byte {
	out := []byte{}
	for i, j := addr, true; j; i++ {
		_, j = e.env.Memory().Read(e.ctx, 0, uint32(i))
		if j {
			out = append(out, e.readMemByte(uint32(i)))
		}
	}
	return out
}
func (e *environment) readUntil(addr int32, terminator byte) []byte {
	out := []byte{}
	for i, j := addr, true; j; i++ {
		_, j = e.env.Memory().Read(e.ctx, uint32(i), 1)
		if e.readMemByte(uint32(i)) == terminator {
			return out
		}
		out = append(out, e.readMemByte(uint32(i)))
	}
	return out
}
func (e *environment) opaPrintln(ptr int32) {

	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b := e.readMemByte(uint32(ptr) + index)
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
func (e *environment) opaAbort(ptr int32) {
	data := e.readFrom(ptr)
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		panic("invalid abort argument")
	}
	panic(abortError{message: string(data[:n])})
}
func (e *environment) setBuiltins() {
	e.builtins = newBuiltinContainer(*e.module)
}
func newEnv(m *Module, opts moduleOpts, r wazero.Runtime) (environment, error) {
	e := environment{}
	e.module = m
	e.ctx = opts.ctx
	e.minMemSize, e.maxMemSize = opts.minMemSize, opts.maxMemSize
	var envMod api.Module
	var err error
	if opts.maxMemSize == (moduleOpts{}).maxMemSize {

		envMod, err = r.NewModuleBuilder("env").
			ExportFunction("opa_abort", e.opaAbort).
			ExportFunction("opa_builtin0", e.builtins.C0).
			ExportFunction("opa_builtin1", e.builtins.C1).
			ExportFunction("opa_builtin2", e.builtins.C2).
			ExportFunction("opa_builtin3", e.builtins.C3).
			ExportFunction("opa_builtin4", e.builtins.C4).
			ExportFunction("opa_println", e.opaPrintln).
			ExportMemory("memory", uint32(opts.minMemSize)).
			Instantiate(opts.ctx, r)
	} else {

		envMod, err = r.NewModuleBuilder("env").
			ExportFunction("opa_abort", e.opaAbort).
			ExportFunction("opa_builtin0", e.builtins.C0).
			ExportFunction("opa_builtin1", e.builtins.C1).
			ExportFunction("opa_builtin2", e.builtins.C2).
			ExportFunction("opa_builtin3", e.builtins.C3).
			ExportFunction("opa_builtin4", e.builtins.C4).
			ExportFunction("opa_println", e.opaPrintln).
			ExportMemoryWithMax("memory", uint32(opts.minMemSize), uint32(opts.maxMemSize)).
			Instantiate(opts.ctx, r)
	}
	if err != nil {
		return environment{}, err
	}
	e.env = envMod
	return e, nil
}
func (e *environment) setMod(m *Module) {
	e.module = m
}
