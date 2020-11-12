package wasmtime

// #include <wasmtime.h>
import "C"
import "runtime"

// Extern is an external value, which is the runtime representation of an entity that can be imported or exported.
// It is an address denoting either a function instance, table instance, memory instance, or global instances in the shared store.
// Read more in [spec](https://webassembly.github.io/spec/core/exec/runtime.html#external-values)
//
type Extern struct {
	_ptr     *C.wasm_extern_t
	_owner   interface{}
	freelist *freeList
}

// AsExtern is an interface for all types which can be imported or exported as an Extern
type AsExtern interface {
	AsExtern() *Extern
}

func mkExtern(ptr *C.wasm_extern_t, freelist *freeList, owner interface{}) *Extern {
	f := &Extern{_ptr: ptr, _owner: owner, freelist: freelist}
	if owner == nil {
		runtime.SetFinalizer(f, func(f *Extern) {
			f.freelist.lock.Lock()
			defer f.freelist.lock.Unlock()
			f.freelist.externs = append(f.freelist.externs, f._ptr)
		})
	}
	return f
}

func (e *Extern) ptr() *C.wasm_extern_t {
	ret := e._ptr
	maybeGC()
	return ret
}

func (e *Extern) owner() interface{} {
	if e._owner != nil {
		return e._owner
	}
	return e
}

// Type returns the type of this export
func (e *Extern) Type() *ExternType {
	ptr := C.wasm_extern_type(e.ptr())
	runtime.KeepAlive(e)
	return mkExternType(ptr, nil)
}

// Func returns a Func if this export is a function or nil otherwise
func (e *Extern) Func() *Func {
	ret := C.wasm_extern_as_func(e.ptr())
	if ret == nil {
		return nil
	}

	return mkFunc(ret, e.freelist, e.owner())
}

// Global returns a Global if this export is a global or nil otherwise
func (e *Extern) Global() *Global {
	ret := C.wasm_extern_as_global(e.ptr())
	if ret == nil {
		return nil
	}

	return mkGlobal(ret, e.freelist, e.owner())
}

// Memory returns a Memory if this export is a memory or nil otherwise
func (e *Extern) Memory() *Memory {
	ret := C.wasm_extern_as_memory(e.ptr())
	if ret == nil {
		return nil
	}

	return mkMemory(ret, e.freelist, e.owner())
}

// Table returns a Table if this export is a table or nil otherwise
func (e *Extern) Table() *Table {
	ret := C.wasm_extern_as_table(e.ptr())
	if ret == nil {
		return nil
	}

	return mkTable(ret, e.freelist, e.owner())
}

// Module returns a Module if this export is a module or nil otherwise
func (e *Extern) Module() *Module {
	ret := C.wasm_extern_as_module(e.ptr())
	if ret == nil {
		return nil
	}

	return mkModule(ret, e.owner())
}

// Instance returns a Instance if this export is a module or nil otherwise
func (e *Extern) Instance() *Instance {
	ret := C.wasm_extern_as_instance(e.ptr())
	if ret == nil {
		return nil
	}

	return mkInstance(ret, e.freelist, e.owner())
}

func (e *Extern) AsExtern() *Extern {
	return e
}
