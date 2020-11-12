package wasmtime

// #include <wasi.h>
// #include <wasmtime.h>
import "C"
import (
	"runtime"
	"sync"
)

// A structure used to defer deletion of C API objects to the main thread.
//
// The C API is not threadsafe and objects must be destroyed on the original
// thread that they came from. We also, however, want to use `SetFinalizer` to
// free objects because it's vastly more convenient than explicit free
// methods. The `SetFinalizer` routine will spin up a goroutine for finalizers
// that might run concurrently, however. To fix this we use this structure to
// collect pointers which need to be free'd.
//
// When a `SetFinalizer` finalizer runs it will enqueue a pointer inside of
// this freelist. This list is then periodically checked to clear out any
// pointers on the main thread with the store. Pointers contained here are
// basically those all connected to a `wasm_store_t`.
//
// This isn't really a great solution but at this time I can't really think
// of anything else unfortunately. I'm hoping that we can continue to optimize
// this over time if necessary, but otherwise this should at least fix crashes
// seen on CI and ensure that everything is free'd correctly.
type freeList struct {
	// The freelist can be modified from both the main thread with a store
	// and from finalizers, so because that can happen concurrently we
	// protect the arrays below with a lock.
	lock sync.Mutex

	// All the various kinds of pointers that we'll store to get deallocated
	// here.

	stores        []*C.wasm_store_t
	memories      []*C.wasm_memory_t
	funcs         []*C.wasm_func_t
	tables        []*C.wasm_table_t
	globals       []*C.wasm_global_t
	instances     []*C.wasm_instance_t
	externs       []*C.wasm_extern_t
	linkers       []*C.wasmtime_linker_t
	wasiInstances []*C.wasi_instance_t
	externVecs    []*C.wasm_extern_vec_t
	vals          []*C.wasm_val_t
}

func newFreeList() *freeList {
	// freelists have their own finalizer which clears out all the contents
	// once the freelist itself has gone away. If this happens that should
	// be safe to do because no other live objects have access to the
	// freelist, so whatever thread is running the freelist is "the thread
	// which own things" so it's safe to clear everything out, we know that
	// no other concurrent accesses will be happening.
	ret := &freeList{}
	runtime.SetFinalizer(ret, func(f *freeList) { f.clear() })
	return ret
}

// Clears out this freelist, actually deleting all pointers that are contained
// within it.
func (f *freeList) clear() {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, p := range f.memories {
		C.wasm_memory_delete(p)
	}
	f.memories = nil

	for _, p := range f.stores {
		C.wasm_store_delete(p)
	}
	f.stores = nil

	for _, p := range f.funcs {
		C.wasm_func_delete(p)
	}
	f.funcs = nil

	for _, p := range f.tables {
		C.wasm_table_delete(p)
	}
	f.tables = nil

	for _, p := range f.globals {
		C.wasm_global_delete(p)
	}
	f.globals = nil

	for _, p := range f.instances {
		C.wasm_instance_delete(p)
	}
	f.instances = nil

	for _, p := range f.externs {
		C.wasm_extern_delete(p)
	}
	f.externs = nil

	for _, p := range f.linkers {
		C.wasmtime_linker_delete(p)
	}
	f.linkers = nil

	for _, p := range f.wasiInstances {
		C.wasi_instance_delete(p)
	}
	f.wasiInstances = nil

	for _, p := range f.externVecs {
		C.wasm_extern_vec_delete(p)
	}
	f.externVecs = nil

	for _, p := range f.vals {
		C.wasm_val_delete(p)
	}
	f.vals = nil
}
