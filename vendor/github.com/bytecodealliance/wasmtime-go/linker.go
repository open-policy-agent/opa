package wasmtime

// #include <wasmtime.h>
// #include "shims.h"
import "C"
import "runtime"

// Linker implements a wasmtime Linking module, which can link instantiated modules together.
// More details you can see [examples for C](https://bytecodealliance.github.io/wasmtime/examples-c-linking.html) or
// [examples for Rust](https://bytecodealliance.github.io/wasmtime/examples-rust-linking.html)
type Linker struct {
	_ptr  *C.wasmtime_linker_t
	Store *Store
}

func NewLinker(store *Store) *Linker {
	ptr := C.wasmtime_linker_new(store.ptr())
	runtime.KeepAlive(store)
	return mkLinker(ptr, store)
}

func mkLinker(ptr *C.wasmtime_linker_t, store *Store) *Linker {
	linker := &Linker{_ptr: ptr, Store: store}
	runtime.SetFinalizer(linker, func(linker *Linker) {
		freelist := linker.Store.freelist
		freelist.lock.Lock()
		defer freelist.lock.Unlock()
		freelist.linkers = append(freelist.linkers, linker._ptr)
	})
	return linker
}

func (l *Linker) ptr() *C.wasmtime_linker_t {
	ret := l._ptr
	maybeGC()
	return ret
}

// AllowShadowing configures whether names can be redefined after they've already been defined
// in this linker.
func (l *Linker) AllowShadowing(allow bool) {
	C.wasmtime_linker_allow_shadowing(l.ptr(), C.bool(allow))
	runtime.KeepAlive(l)
}

// Define defines a new item in this linker with the given module/name pair. Returns
// an error if shadowing is disallowed and the module/name is already defined.
func (l *Linker) Define(module, name string, item AsExtern) error {
	extern := item.AsExtern()
	err := C.go_linker_define(
		l.ptr(),
		C._GoStringPtr(module),
		C._GoStringLen(module),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		extern.ptr(),
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(module)
	runtime.KeepAlive(name)
	runtime.KeepAlive(extern)
	if err == nil {
		return nil
	}

	return mkError(err)
}

// DefineFunc acts as a convenience wrapper to calling Define and WrapFunc.
//
// Returns an error if shadowing is disabled and the name is already defined.
func (l *Linker) DefineFunc(module, name string, f interface{}) error {
	return l.Define(module, name, WrapFunc(l.Store, f))
}

// DefineInstance defines all exports of an instance provided under the module name provided.
//
// Returns an error if shadowing is disabled and names are already defined.
func (l *Linker) DefineInstance(module string, instance *Instance) error {
	err := C.go_linker_define_instance(
		l.ptr(),
		C._GoStringPtr(module),
		C._GoStringLen(module),
		instance.ptr(),
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(module)
	runtime.KeepAlive(instance)
	if err == nil {
		return nil
	}

	return mkError(err)
}

// DefineModule defines automatic instantiations of the module in this linker.
//
// The `name` of the module is the name within the linker, and the `module` is
// the one that's being instantiated. This function automatically handles
// WASI Commands and Reactors for instantiation and initialization. For more
// information see the Rust documentation --
// https://docs.wasmtime.dev/api/wasmtime/struct.Linker.html#method.module.
func (l *Linker) DefineModule(name string, module *Module) error {
	err := C.go_linker_define_module(
		l.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		module.ptr(),
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(name)
	runtime.KeepAlive(module)
	if err == nil {
		return nil
	}

	return mkError(err)
}

// DefineWasi links a WASI module into this linker, ensuring that all exported functions
// are available for linking.
//
// Returns an error if shadowing is disabled and names are already defined.
func (l *Linker) DefineWasi(instance *WasiInstance) error {
	err := C.wasmtime_linker_define_wasi(l.ptr(), instance.ptr())
	runtime.KeepAlive(l)
	runtime.KeepAlive(instance)
	if err == nil {
		return nil
	}

	return mkError(err)
}

// Instantiate instantates a module with all imports defined in this linker.
//
// Returns an error if the instance's imports couldn't be satisfied, had the
// wrong types, or if a trap happened executing the start function.
func (l *Linker) Instantiate(module *Module) (*Instance, error) {
	var ret *C.wasm_instance_t
	var err *C.wasmtime_error_t
	trap := enterWasm(l.Store.freelist, func(trap **C.wasm_trap_t) {
		err = C.wasmtime_linker_instantiate(l.ptr(), module.ptr(), &ret, trap)
	})
	runtime.KeepAlive(l)
	runtime.KeepAlive(module)
	if trap != nil {
		return nil, trap
	}
	if err != nil {
		return nil, mkError(err)
	}
	return mkInstance(ret, l.Store.freelist, nil), nil
}

// GetDefault acquires the "default export" of the named module in this linker.
//
// If there is no default item then an error is returned, otherwise the default
// function is returned.
//
// For more information see the Rust documentation --
// https://docs.wasmtime.dev/api/wasmtime/struct.Linker.html#method.get_default.
func (l *Linker) GetDefault(name string) (*Func, error) {
	var ret *C.wasm_func_t
	err := C.go_linker_get_default(
		l.ptr(),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&ret,
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(name)
	if err != nil {
		return nil, mkError(err)
	}
	return mkFunc(ret, l.Store.freelist, nil), nil

}

// GetOneByName loads an item by name from this linker.
//
// If the item isn't defined then an error is returned, otherwise the item is
// returned.
func (l *Linker) GetOneByName(module, name string) (*Extern, error) {
	var ret *C.wasm_extern_t
	err := C.go_linker_get_one_by_name(
		l.ptr(),
		C._GoStringPtr(module),
		C._GoStringLen(module),
		C._GoStringPtr(name),
		C._GoStringLen(name),
		&ret,
	)
	runtime.KeepAlive(l)
	runtime.KeepAlive(name)
	runtime.KeepAlive(module)
	if err != nil {
		return nil, mkError(err)
	}
	return mkExtern(ret, l.Store.freelist, nil), nil

}
