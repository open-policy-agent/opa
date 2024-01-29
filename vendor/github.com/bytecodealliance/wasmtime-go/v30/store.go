package wasmtime

// #include <wasmtime.h>
// #include "shims.h"
import "C"
import (
	"reflect"
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
	engine    *Engine
	funcNew   []funcNewEntry
	funcWrap  []funcWrapEntry
	lastPanic interface{}
}

type funcNewEntry struct {
	callback func(*Caller, []Val) ([]Val, *Trap)
	results  []*ValType
}

type funcWrapEntry struct {
	callback reflect.Value
}

// NewStore creates a new `Store` from the configuration provided in `engine`
func NewStore(engine *Engine) *Store {
	// Allocate an index for this store and allocate some internal data to go with
	// the store.
	gStoreLock.Lock()
	idx := gStoreSlab.allocate()
	gStoreMap[idx] = &storeData{engine: engine}
	gStoreLock.Unlock()

	ptr := C.go_store_new(engine.ptr(), C.size_t(idx))
	store := &Store{
		_ptr:   ptr,
		Engine: engine,
	}
	runtime.SetFinalizer(store, func(store *Store) {
		store.Close()
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

func (store *Store) ptr() *C.wasmtime_store_t {
	ret := store._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// Close will deallocate this store's state explicitly.
//
// For more information see the documentation for engine.Close()
func (store *Store) Close() {
	if store._ptr == nil {
		return
	}
	runtime.SetFinalizer(store, nil)
	C.wasmtime_store_delete(store._ptr)
	store._ptr = nil
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
	C.wasmtime_context_set_wasi(store.Context(), ptr)
	runtime.KeepAlive(store)
}

// Implementation of the `Storelike` interface
func (store *Store) Context() *C.wasmtime_context_t {
	ret := C.wasmtime_store_context(store.ptr())
	maybeGC()
	runtime.KeepAlive(store)
	return ret
}

// SetEpochDeadline will configure the relative deadline, from the current
// engine's epoch number, after which wasm code will be interrupted.
func (store *Store) SetEpochDeadline(deadline uint64) {
	C.wasmtime_context_set_epoch_deadline(store.Context(), C.uint64_t(deadline))
	runtime.KeepAlive(store)
}

// Returns the underlying `*storeData` that this store references in Go, used
// for inserting functions or storing panic data.
func getDataInStore(store Storelike) *storeData {
	data := uintptr(C.wasmtime_context_get_data(store.Context()))
	gStoreLock.Lock()
	defer gStoreLock.Unlock()
	return gStoreMap[int(data)]
}

var gEngineFuncLock sync.Mutex
var gEngineFuncNew = make(map[int]*funcNewEntry)
var gEngineFuncNewSlab slab
var gEngineFuncWrap = make(map[int]*funcWrapEntry)
var gEngineFuncWrapSlab slab

func insertFuncNew(data *storeData, ty *FuncType, callback func(*Caller, []Val) ([]Val, *Trap)) int {
	var idx int
	entry := funcNewEntry{
		callback: callback,
		results:  ty.Results(),
	}
	if data == nil {
		gEngineFuncLock.Lock()
		defer gEngineFuncLock.Unlock()
		idx = gEngineFuncNewSlab.allocate()
		gEngineFuncNew[idx] = &entry
		idx = (idx << 1)
	} else {
		idx = len(data.funcNew)
		data.funcNew = append(data.funcNew, entry)
		idx = (idx << 1) | 1
	}
	return idx
}

func (data *storeData) getFuncNew(idx int) *funcNewEntry {
	if idx&1 == 0 {
		gEngineFuncLock.Lock()
		defer gEngineFuncLock.Unlock()
		return gEngineFuncNew[idx>>1]
	} else {
		return &data.funcNew[idx>>1]
	}
}

func insertFuncWrap(data *storeData, callback reflect.Value) int {
	var idx int
	entry := funcWrapEntry{callback}
	if data == nil {
		gEngineFuncLock.Lock()
		defer gEngineFuncLock.Unlock()
		idx = gEngineFuncWrapSlab.allocate()
		gEngineFuncWrap[idx] = &entry
		idx = (idx << 1)
	} else {
		idx = len(data.funcWrap)
		data.funcWrap = append(data.funcWrap, entry)
		idx = (idx << 1) | 1
	}
	return idx

}

func (data *storeData) getFuncWrap(idx int) *funcWrapEntry {
	if idx&1 == 0 {
		gEngineFuncLock.Lock()
		defer gEngineFuncLock.Unlock()
		return gEngineFuncWrap[idx>>1]
	} else {
		return &data.funcWrap[idx>>1]
	}
}

//export goFinalizeFuncNew
func goFinalizeFuncNew(env unsafe.Pointer) {
	idx := int(uintptr(env))
	if idx&1 != 0 {
		panic("shouldn't finalize a store-local index")
	}
	idx = idx >> 1
	gEngineFuncLock.Lock()
	defer gEngineFuncLock.Unlock()
	delete(gEngineFuncNew, idx)
	gEngineFuncNewSlab.deallocate(idx)

}

//export goFinalizeFuncWrap
func goFinalizeFuncWrap(env unsafe.Pointer) {
	idx := int(uintptr(env))
	if idx&1 != 0 {
		panic("shouldn't finalize a store-local index")
	}
	idx = idx >> 1
	gEngineFuncLock.Lock()
	defer gEngineFuncLock.Unlock()
	delete(gEngineFuncWrap, idx)
	gEngineFuncWrapSlab.deallocate(idx)
}

// GetFuel returns the amount of fuel remaining in this store.
//
// If fuel consumption is not enabled via `Config.SetConsumeFuel` then
// this function will return an error. Otherwise this will retrieve the fuel
// remaining and return it.
//
// Also note that fuel, if enabled, must be originally configured via
// `Store.SetFuel`.
func (store *Store) GetFuel() (uint64, error) {
	var remaining uint64
	c_remaining := C.uint64_t(remaining)
	err := C.wasmtime_context_get_fuel(store.Context(), &c_remaining)
	runtime.KeepAlive(store)
	if err != nil {
		return 0, mkError(err)
	}

	return uint64(c_remaining), nil
}

// SetFuel sets this store's fuel to the specified value.
//
// For this method to work fuel consumption must be enabled via
// `Config.SetConsumeFuel`. By default a store starts with 0 fuel
// for wasm to execute with (meaning it will immediately trap).
// This function must be called for the store to have
// some fuel to allow WebAssembly to execute.
//
// Note that at this time when fuel is entirely consumed it will cause
// wasm to trap. More usages of fuel are planned for the future.
//
// If fuel is not enabled within this store then an error is returned.
func (store *Store) SetFuel(fuel uint64) error {
	err := C.wasmtime_context_set_fuel(store.Context(), C.uint64_t(fuel))
	runtime.KeepAlive(store)
	if err != nil {
		return mkError(err)
	}

	return nil
}

// Limiter provides limits for a store. Used by hosts to limit resource
// consumption of instances. Use negative value to keep the default value
// for the limit.
func (store *Store) Limiter(
	memorySize int64,
	tableElements int64,
	instances int64,
	tables int64,
	memories int64,
) {
	C.wasmtime_store_limiter(
		store.ptr(),
		C.int64_t(memorySize),
		C.int64_t(tableElements),
		C.int64_t(instances),
		C.int64_t(tables),
		C.int64_t(memories),
	)
	runtime.KeepAlive(store)
}
