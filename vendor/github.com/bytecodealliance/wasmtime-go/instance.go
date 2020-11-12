package wasmtime

// #include <wasmtime.h>
import "C"
import (
	"runtime"
	"unsafe"
)

// Instance is an instantiated module instance.
// Once a module has been instantiated as an Instance, any exported function can be invoked externally via its function address funcaddr in the store S and an appropriate list val∗ of argument values.
type Instance struct {
	_ptr     *C.wasm_instance_t
	exports  map[string]*Extern
	freelist *freeList
}

// NewInstance instantiates a WebAssembly `module` with the `imports` provided.
//
// This function will attempt to create a new wasm instance given the provided
// imports. This can fail if the wrong number of imports are specified, the
// imports aren't of the right type, or for other resource-related issues.
//
// This will also run the `start` function of the instance, returning an error
// if it traps.
func NewInstance(store *Store, module *Module, imports []*Extern) (*Instance, error) {
	importsRaw := make([]*C.wasm_extern_t, len(imports))
	for i, imp := range imports {
		importsRaw[i] = imp.ptr()
	}
	var importsRawPtr **C.wasm_extern_t
	if len(imports) > 0 {
		importsRawPtr = &importsRaw[0]
	}
	var trap *C.wasm_trap_t
	var ptr *C.wasm_instance_t
	err := C.wasmtime_instance_new(
		store.ptr(),
		module.ptr(),
		importsRawPtr,
		C.size_t(len(imports)),
		&ptr,
		&trap,
	)
	runtime.KeepAlive(store)
	runtime.KeepAlive(module)
	runtime.KeepAlive(imports)
	runtime.KeepAlive(importsRaw)
	if err != nil {
		return nil, mkError(err)
	}
	if trap != nil {
		return nil, mkTrap(trap)
	}
	return mkInstance(ptr, store, module), nil
}

func mkInstance(ptr *C.wasm_instance_t, store *Store, module *Module) *Instance {
	instance := &Instance{
		_ptr:     ptr,
		exports:  make(map[string]*Extern),
		freelist: store.freelist,
	}
	runtime.SetFinalizer(instance, func(instance *Instance) {
		freelist := instance.freelist
		freelist.lock.Lock()
		defer freelist.lock.Unlock()
		freelist.instances = append(freelist.instances, instance._ptr)
	})
	exports := instance.Exports()
	for i, ty := range module.Exports() {
		instance.exports[ty.Name()] = exports[i]
	}
	return instance
}

func (i *Instance) ptr() *C.wasm_instance_t {
	ret := i._ptr
	maybeGC()
	return ret
}

type externList struct {
	vec C.wasm_extern_vec_t
}

// Exports returns a list of exports from this instance.
//
// Each export is returned as a `*Extern` and lines up with the exports list of
// the associated `Module`.
func (i *Instance) Exports() []*Extern {
	externs := &externList{}
	C.wasm_instance_exports(i.ptr(), &externs.vec)
	runtime.KeepAlive(i)
	freelist := i.freelist
	runtime.SetFinalizer(externs, func(externs *externList) {
		freelist.lock.Lock()
		defer freelist.lock.Unlock()
		freelist.externVecs = append(freelist.externVecs, &externs.vec)
	})

	ret := make([]*Extern, int(externs.vec.size))
	base := unsafe.Pointer(externs.vec.data)
	var ptr *C.wasm_extern_t
	for i := 0; i < int(externs.vec.size); i++ {
		ptr := *(**C.wasm_extern_t)(unsafe.Pointer(uintptr(base) + unsafe.Sizeof(ptr)*uintptr(i)))
		ty := mkExtern(ptr, freelist, externs)
		ret[i] = ty
	}
	return ret
}

// GetExport attempts to find an export on this instance by `name`
//
// May return `nil` if this instance has no export named `name`
func (i *Instance) GetExport(name string) *Extern {
	return i.exports[name]
}
