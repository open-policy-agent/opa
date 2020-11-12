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
	_ptr             *C.wasm_instance_t
	exports          map[string]*Extern
	exportsPopulated bool
	freelist         *freeList
	_owner           interface{}
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
	importsRaw := C.wasm_extern_vec_t{}
	C.wasm_extern_vec_new_uninitialized(&importsRaw, C.size_t(len(imports)))
	base := unsafe.Pointer(importsRaw.data)
	for i, imp := range imports {
		ptr := C.wasm_extern_copy(imp.ptr())
		*(**C.wasm_extern_t)(unsafe.Pointer(uintptr(base) + unsafe.Sizeof(ptr)*uintptr(i))) = ptr
	}
	var ptr *C.wasm_instance_t
	var err *C.wasmtime_error_t
	trap := enterWasm(store.freelist, func(trap **C.wasm_trap_t) {
		err = C.wasmtime_instance_new(
			store.ptr(),
			module.ptr(),
			&importsRaw,
			&ptr,
			trap,
		)
	})
	runtime.KeepAlive(store)
	runtime.KeepAlive(module)
	C.wasm_extern_vec_delete(&importsRaw)
	if trap != nil {
		return nil, trap
	}
	if err != nil {
		return nil, mkError(err)
	}
	return mkInstance(ptr, store.freelist, nil), nil
}

func mkInstance(ptr *C.wasm_instance_t, freelist *freeList, owner interface{}) *Instance {
	instance := &Instance{
		_ptr:             ptr,
		exports:          make(map[string]*Extern),
		exportsPopulated: false,
		freelist:         freelist,
		_owner:           owner,
	}
	if owner == nil {
		runtime.SetFinalizer(instance, func(instance *Instance) {
			freelist := instance.freelist
			freelist.lock.Lock()
			defer freelist.lock.Unlock()
			freelist.instances = append(freelist.instances, instance._ptr)
		})
	}
	return instance
}

func (i *Instance) ptr() *C.wasm_instance_t {
	ret := i._ptr
	maybeGC()
	return ret
}

func (i *Instance) owner() interface{} {
	if i._owner != nil {
		return i._owner
	}
	return i
}

// Type returns an `InstanceType` that corresponds for this instance.
func (i *Instance) Type() *InstanceType {
	ptr := C.wasm_instance_type(i.ptr())
	runtime.KeepAlive(i)
	return mkInstanceType(ptr, nil)
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
	if !i.exportsPopulated {
		i.populateExports()
	}
	return i.exports[name]
}

func (i *Instance) populateExports() {
	exports := i.Exports()
	for j, ty := range i.Type().Exports() {
		i.exports[ty.Name()] = exports[j]
	}
}

func (i *Instance) AsExtern() *Extern {
	ptr := C.wasm_instance_as_extern(i.ptr())
	return mkExtern(ptr, i.freelist, i.owner())
}
