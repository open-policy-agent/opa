package wasmtime

// #include <wasmtime.h>
// #include "shims.h"
import "C"
import (
	"errors"
	"runtime"
	"sync"
	"unsafe"
)

// Store is a general group of wasm instances, and many objects
// must all be created with and reference the same `Store`
type Store struct {
	_ptr *C.wasmtime_store_t

	// The `Engine` that this store uses for compilation and environment
	// settings.
	Engine *Engine
}

// Storelike represents types that can be used to contextually reference a
// `Store`.
//
// This interface is implemented by `*Store` and `*Caller` and is pervasively
// used throughout this library. You'll want to pass one of those two objects
// into functions that take a `Storelike`.
type Storelike interface {
	// Returns the wasmtime context pointer this store is attached to.
	Context() *C.wasmtime_context_t
}

var gStoreLock sync.Mutex
var gStoreMap = make(map[int]*storeData)
var gStoreSlab slab

// State associated with a `Store`, currently used to propagate panic
// information through invocations as well as store Go closures that have been
// added to the store.
type storeData struct {
	funcNew   []funcNewEntry
	funcWrap  []funcWrapEntry
	lastPanic interface{}
}

// NewStore creates a new `Store` from the configuration provided in `engine`
func NewStore(engine *Engine) *Store {
	// Allocate an index for this store and allocate some internal data to go with
	// the store.
	gStoreLock.Lock()
	idx := gStoreSlab.allocate()
	gStoreMap[idx] = &storeData{}
	gStoreLock.Unlock()

	ptr := C.go_store_new(engine.ptr(), C.size_t(idx))
	store := &Store{
		_ptr:   ptr,
		Engine: engine,
	}
	runtime.SetFinalizer(store, func(store *Store) {
		C.wasmtime_store_delete(store._ptr)
	})
	return store
}

//export goFinalizeStore
func goFinalizeStore(env unsafe.Pointer) {
	// When a store is finalized this is used as the finalization callback for the
	// custom data within the store, and our finalization here will delete the
	// store's data from the global map and deallocate its index to get reused by
	// a future store.
	idx := int(uintptr(env))
	gStoreLock.Lock()
	defer gStoreLock.Unlock()
	delete(gStoreMap, idx)
	gStoreSlab.deallocate(idx)
}

// InterruptHandle returns a handle, if enabled, which can be used to interrupt
// execution of WebAssembly within this `Store` from any goroutine.
//
// This requires that `SetInterruptable` is set to `true` on the `Config`
// associated with this `Store`. Returns an error if interrupts aren't enabled.
func (store *Store) InterruptHandle() (*InterruptHandle, error) {
	ptr := C.wasmtime_interrupt_handle_new(store.Context())
	runtime.KeepAlive(store)
	if ptr == nil {
		return nil, errors.New("interrupts not enabled in `Config`")
	}

	handle := &InterruptHandle{_ptr: ptr}
	runtime.SetFinalizer(handle, func(handle *InterruptHandle) {
		C.wasmtime_interrupt_handle_delete(handle._ptr)
	})
	return handle, nil
}

// GC will clean up any `externref` values that are no longer actually
// referenced.
//
// This function is not required to be called for correctness, it's only an
// optimization if desired to clean out any extra `externref` values.
func (store *Store) GC() {
	C.wasmtime_context_gc(store.Context())
	runtime.KeepAlive(store)
}

// SetWasi will configure the WASI state to use for instances within this
// `Store`.
//
// The `wasi` argument cannot be reused for another `Store`, it's consumed by
// this function.
func (store *Store) SetWasi(wasi *WasiConfig) {
	runtime.SetFinalizer(wasi, nil)
	ptr := wasi.ptr()
	wasi._ptr = nil
	if ptr == nil {
		panic("reuse of already-consumed WasiConfig")
	}
	C.wasmtime_context_set_wasi(store.Context(), ptr)
	runtime.KeepAlive(store)
}

// Implementation of the `Storelike` interface
func (store *Store) Context() *C.wasmtime_context_t {
	ret := C.wasmtime_store_context(store._ptr)
	maybeGC()
	return ret
}

// Returns the underlying `*storeData` that this store references in Go, used
// for inserting functions or storing panic data.
func getDataInStore(store Storelike) *storeData {
	data := uintptr(C.wasmtime_context_get_data(store.Context()))
	gStoreLock.Lock()
	defer gStoreLock.Unlock()
	return gStoreMap[int(data)]
}

// InterruptHandle is used to interrupt the execution of currently running
// wasm code.
//
// For more information see
// https://bytecodealliance.github.io/wasmtime/api/wasmtime/struct.Store.html#method.interrupt_handle
type InterruptHandle struct {
	_ptr *C.wasmtime_interrupt_handle_t
}

// Interrupt interrupts currently executing WebAssembly code, if it's currently running,
// or interrupts wasm the next time it starts running.
//
// For more information see
// https://bytecodealliance.github.io/wasmtime/api/wasmtime/struct.Store.html#method.interrupt_handle
func (i *InterruptHandle) Interrupt() {
	C.wasmtime_interrupt_handle_interrupt(i.ptr())
	runtime.KeepAlive(i)
}

func (i *InterruptHandle) ptr() *C.wasmtime_interrupt_handle_t {
	ret := i._ptr
	maybeGC()
	return ret
}
