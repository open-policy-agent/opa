package wasmtime

// #include <wasm.h>
import "C"
import "runtime"

// MemoryType is one of Memory types which classify linear memories and their size range.
// The limits constrain the minimum and optionally the maximum size of a memory. The limits are given in units of page size.
type MemoryType struct {
	_ptr   *C.wasm_memorytype_t
	_owner interface{}
}

// NewMemoryType creates a new `MemoryType` with the `limits` on size provided
func NewMemoryType(limits Limits) *MemoryType {
	limitsFFI := limits.ffi()
	ptr := C.wasm_memorytype_new(&limitsFFI)
	return mkMemoryType(ptr, nil)
}

func mkMemoryType(ptr *C.wasm_memorytype_t, owner interface{}) *MemoryType {
	memorytype := &MemoryType{_ptr: ptr, _owner: owner}
	if owner == nil {
		runtime.SetFinalizer(memorytype, func(memorytype *MemoryType) {
			C.wasm_memorytype_delete(memorytype._ptr)
		})
	}
	return memorytype
}

func (ty *MemoryType) ptr() *C.wasm_memorytype_t {
	ret := ty._ptr
	maybeGC()
	return ret
}

func (ty *MemoryType) owner() interface{} {
	if ty._owner != nil {
		return ty._owner
	}
	return ty
}

// Limits returns the limits on the size of this memory type
func (ty *MemoryType) Limits() Limits {
	ptr := C.wasm_memorytype_limits(ty.ptr())
	return mkLimits(ptr, ty.owner())
}

// AsExternType converts this type to an instance of `ExternType`
func (ty *MemoryType) AsExternType() *ExternType {
	ptr := C.wasm_memorytype_as_externtype_const(ty.ptr())
	return mkExternType(ptr, ty.owner())
}
