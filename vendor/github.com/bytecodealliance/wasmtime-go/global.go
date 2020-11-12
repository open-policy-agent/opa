package wasmtime

// #include <wasmtime.h>
import "C"
import "runtime"

// Global is a global instance, which is the runtime representation of a global variable.
// It holds an individual value and a flag indicating whether it is mutable.
// Read more in [spec](https://webassembly.github.io/spec/core/exec/runtime.html#global-instances)
type Global struct {
	_ptr     *C.wasm_global_t
	_owner   interface{}
	freelist *freeList
}

// NewGlobal creates a new `Global` in the given `Store` with the specified `ty` and
// initial value `val`.
func NewGlobal(
	store *Store,
	ty *GlobalType,
	val Val,
) (*Global, error) {
	var ptr *C.wasm_global_t
	err := C.wasmtime_global_new(
		store.ptr(),
		ty.ptr(),
		val.ptr(),
		&ptr,
	)
	runtime.KeepAlive(store)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(val)
	if err != nil {
		return nil, mkError(err)
	}

	return mkGlobal(ptr, store.freelist, nil), nil
}

func mkGlobal(ptr *C.wasm_global_t, freelist *freeList, owner interface{}) *Global {
	f := &Global{_ptr: ptr, _owner: owner, freelist: freelist}
	if owner == nil {
		runtime.SetFinalizer(f, func(f *Global) {
			f.freelist.lock.Lock()
			defer f.freelist.lock.Unlock()
			f.freelist.globals = append(f.freelist.globals, f._ptr)
		})
	}
	return f
}

func (g *Global) ptr() *C.wasm_global_t {
	ret := g._ptr
	maybeGC()
	return ret
}

func (g *Global) owner() interface{} {
	if g._owner != nil {
		return g._owner
	}
	return g
}

// Type returns the type of this global
func (g *Global) Type() *GlobalType {
	ptr := C.wasm_global_type(g.ptr())
	runtime.KeepAlive(g)
	return mkGlobalType(ptr, nil)
}

// Get gets the value of this global
func (g *Global) Get() Val {
	ret := C.wasm_val_t{}
	C.wasm_global_get(g.ptr(), &ret)
	runtime.KeepAlive(g)
	return takeVal(&ret, g.freelist)
}

// Set sets the value of this global
func (g *Global) Set(val Val) error {
	err := C.wasmtime_global_set(g.ptr(), val.ptr())
	runtime.KeepAlive(g)
	runtime.KeepAlive(val)
	if err == nil {
		return nil
	}

	return mkError(err)
}

func (g *Global) AsExtern() *Extern {
	ptr := C.wasm_global_as_extern(g.ptr())
	return mkExtern(ptr, g.freelist, g.owner())
}
