package wasmtime

// #include <wasmtime.h>
import "C"
import "runtime"

// MemoryType is one of Memory types which classify linear memories and their size range.
// The limits constrain the minimum and optionally the maximum size of a memory. The limits are given in units of page size.
type MemoryType struct {
	_ptr   *C.wasm_memorytype_t
	_owner interface{}
}

// NewMemoryType creates a new `MemoryType` with the limits on size provided
//
// The `min` value is the minimum size, in WebAssembly pages, of this memory.
// The `has_max` boolean indicates whether a maximum size is present, and if so
// `max` is used as the maximum size of memory, in wasm pages.
//
// Note that this will create a 32-bit memory type, the default outside of the
// memory64 proposal.
func NewMemoryType(min uint32, has_max bool, max uint32, shared bool) *MemoryType {
	if min > (1<<16) || max > (1<<16) {
		panic("provided sizes are too large")
	}
	ptr := C.wasmtime_memorytype_new(C.uint64_t(min), C._Bool(has_max), C.uint64_t(max), false, C._Bool(shared))
	return mkMemoryType(ptr, nil)
}

// NewMemoryType64 creates a new 64-bit `MemoryType` with the provided limits
//
// The `min` value is the minimum size, in WebAssembly pages, of this memory.
// The `has_max` boolean indicates whether a maximum size is present, and if so
// `max` is used as the maximum size of memory, in wasm pages.
//
// Note that 64-bit memories are part of the memory64 WebAssembly proposal.
func NewMemoryType64(min uint64, has_max bool, max uint64, shared bool) *MemoryType {
	if min > (1<<48) || max > (1<<48) {
		panic("provided sizes are too large")
	}
	ptr := C.wasmtime_memorytype_new(C.uint64_t(min), C._Bool(has_max), C.uint64_t(max), true, C._Bool(shared))
	return mkMemoryType(ptr, nil)
}

func mkMemoryType(ptr *C.wasm_memorytype_t, owner interface{}) *MemoryType {
	memorytype := &MemoryType{_ptr: ptr, _owner: owner}
	if owner == nil {
		runtime.SetFinalizer(memorytype, func(memorytype *MemoryType) {
			memorytype.Close()
		})
	}
	return memorytype
}

func (ty *MemoryType) ptr() *C.wasm_memorytype_t {
	ret := ty._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

func (ty *MemoryType) owner() interface{} {
	if ty._owner != nil {
		return ty._owner
	}
	return ty
}

// Close will deallocate this type's state explicitly.
//
// For more information see the documentation for engine.Close()
func (ty *MemoryType) Close() {
	if ty._ptr == nil || ty._owner != nil {
		return
	}
	runtime.SetFinalizer(ty, nil)
	C.wasm_memorytype_delete(ty._ptr)
	ty._ptr = nil
}

// Minimum returns the minimum size of this memory, in WebAssembly pages
func (ty *MemoryType) Minimum() uint64 {
	ret := C.wasmtime_memorytype_minimum(ty.ptr())
	runtime.KeepAlive(ty)
	return uint64(ret)
}

// Maximum returns the maximum size of this memory, in WebAssembly pages, if
// specified.
//
// If the maximum size is not specified then `(false, 0)` is returned, otherwise
// `(true, N)` is returned where `N` is the listed maximum size of this memory.
func (ty *MemoryType) Maximum() (bool, uint64) {
	size := C.uint64_t(0)
	present := C.wasmtime_memorytype_maximum(ty.ptr(), &size)
	runtime.KeepAlive(ty)
	return bool(present), uint64(size)
}

// Is64 returns whether this is a 64-bit memory or not.
func (ty *MemoryType) Is64() bool {
	ok := C.wasmtime_memorytype_is64(ty.ptr())
	runtime.KeepAlive(ty)
	return bool(ok)
}

// IsShared returns whether this is a shared memory or not.
func (ty *MemoryType) IsShared() bool {
	ok := C.wasmtime_memorytype_isshared(ty.ptr())
	runtime.KeepAlive(ty)
	return bool(ok)
}

// AsExternType converts this type to an instance of `ExternType`
func (ty *MemoryType) AsExternType() *ExternType {
	ptr := C.wasm_memorytype_as_externtype_const(ty.ptr())
	return mkExternType(ptr, ty.owner())
}
