package wasmtime

// #include <wasm.h>
import "C"
import (
	"runtime"
	"unsafe"
)

// Memory instance is the runtime representation of a linear memory.
// It holds a vector of bytes and an optional maximum size, if one was specified at the definition site of the memory.
// Read more in [spec](https://webassembly.github.io/spec/core/exec/runtime.html#memory-instances)
// In wasmtime-go, you can get the vector of bytes by the unsafe pointer of memory from `Memory.Data()`, or go style byte slice from `Memory.UnsafeData()`
type Memory struct {
	_ptr     *C.wasm_memory_t
	freelist *freeList
	_owner   interface{}
}

// NewMemory creates a new `Memory` in the given `Store` with the specified `ty`.
func NewMemory(store *Store, ty *MemoryType) *Memory {
	ptr := C.wasm_memory_new(store.ptr(), ty.ptr())
	runtime.KeepAlive(store)
	runtime.KeepAlive(ty)
	return mkMemory(ptr, store.freelist, nil)
}

func mkMemory(ptr *C.wasm_memory_t, freelist *freeList, owner interface{}) *Memory {
	f := &Memory{_ptr: ptr, _owner: owner, freelist: freelist}
	if owner == nil {
		runtime.SetFinalizer(f, func(f *Memory) {
			f.freelist.lock.Lock()
			defer f.freelist.lock.Unlock()
			f.freelist.memories = append(f.freelist.memories, f._ptr)
		})
	}
	return f
}

func (mem *Memory) ptr() *C.wasm_memory_t {
	ret := mem._ptr
	maybeGC()
	return ret
}

func (mem *Memory) owner() interface{} {
	if mem._owner != nil {
		return mem._owner
	}
	return mem
}

// Type returns the type of this memory
func (mem *Memory) Type() *MemoryType {
	ptr := C.wasm_memory_type(mem.ptr())
	runtime.KeepAlive(mem)
	return mkMemoryType(ptr, nil)
}

// Data returns the raw pointer in memory of where this memory starts
func (mem *Memory) Data() unsafe.Pointer {
	ret := unsafe.Pointer(C.wasm_memory_data(mem.ptr()))
	runtime.KeepAlive(mem)
	return ret
}

// UnsafeData returns the raw memory backed by this `Memory` as a byte slice (`[]byte`).
//
// This is not a safe method to call, hence the "unsafe" in the name. The byte
// slice returned from this function is not managed by the Go garbage collector.
// You need to ensure that `m`, the original `Memory`, lives longer than the
// `[]byte` returned.
//
// Note that you may need to use `runtime.KeepAlive` to keep the original memory
// `m` alive for long enough while you're using the `[]byte` slice. If the
// `[]byte` slice is used after `m` is GC'd then that is undefined behavior.
func (mem *Memory) UnsafeData() []byte {
	// see https://github.com/golang/go/wiki/cgo#turning-c-arrays-into-go-slices
	const MaxLen = 1 << 32
	length := mem.DataSize()
	if length >= MaxLen {
		panic("memory is too big")
	}
	return (*[MaxLen]byte)(mem.Data())[:length:length]
}

// DataSize returns the size, in bytes, that `Data()` is valid for
func (mem *Memory) DataSize() uintptr {
	ret := uintptr(C.wasm_memory_data_size(mem.ptr()))
	runtime.KeepAlive(mem)
	return ret
}

// Size returns the size, in wasm pages, of this memory
func (mem *Memory) Size() uint32 {
	ret := uint32(C.wasm_memory_size(mem.ptr()))
	runtime.KeepAlive(mem)
	return ret
}

// Grow grows this memory by `delta` pages
func (mem *Memory) Grow(delta uint) bool {
	ret := C.wasm_memory_grow(mem.ptr(), C.wasm_memory_pages_t(delta))
	runtime.KeepAlive(mem)
	return bool(ret)
}

func (mem *Memory) AsExtern() *Extern {
	ptr := C.wasm_memory_as_extern(mem.ptr())
	return mkExtern(ptr, mem.freelist, mem.owner())
}
