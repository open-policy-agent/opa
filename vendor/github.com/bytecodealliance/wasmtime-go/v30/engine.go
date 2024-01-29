package wasmtime

// #include <wasmtime.h>
import "C"
import (
	"runtime"
)

// Engine is an instance of a wasmtime engine which is used to create a `Store`.
//
// Engines are a form of global configuration for wasm compilations and modules
// and such.
type Engine struct {
	_ptr *C.wasm_engine_t
}

// NewEngine creates a new `Engine` with default configuration.
func NewEngine() *Engine {
	engine := &Engine{_ptr: C.wasm_engine_new()}
	runtime.SetFinalizer(engine, func(engine *Engine) {
		engine.Close()
	})
	return engine
}

// NewEngineWithConfig creates a new `Engine` with the `Config` provided
//
// Note that once a `Config` is passed to this method it cannot be used again.
func NewEngineWithConfig(config *Config) *Engine {
	if config.ptr() == nil {
		panic("config already used")
	}
	engine := &Engine{_ptr: C.wasm_engine_new_with_config(config.ptr())}
	runtime.SetFinalizer(config, nil)
	config._ptr = nil
	runtime.SetFinalizer(engine, func(engine *Engine) {
		engine.Close()
	})
	return engine
}

// Close will deallocate this engine's state explicitly.
//
// By default state is cleaned up automatically when an engine is garbage
// collected but the Go GC. The Go GC, however, does not provide strict
// guarantees about finalizers especially in terms of timing. Additionally the
// Go GC is not aware of the full weight of an engine because it holds onto
// allocations in Wasmtime not tracked by the Go GC. For these reasons, it's
// recommended to where possible explicitly call this method and deallocate an
// engine to avoid relying on the Go GC.
//
// This method will deallocate Wasmtime-owned state. Future use of the engine
// will panic because the Wasmtime state is no longer there.
//
// Close can be called multiple times without error. Only the first time will
// deallocate resources.
func (engine *Engine) Close() {
	if engine._ptr == nil {
		return
	}
	runtime.SetFinalizer(engine, nil)
	C.wasm_engine_delete(engine.ptr())
	engine._ptr = nil

}

func (engine *Engine) ptr() *C.wasm_engine_t {
	ret := engine._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

// IncrementEpoch will increase the current epoch number by 1 within the
// current engine which will cause any connected stores with their epoch
// deadline exceeded to now be interrupted.
//
// This method is safe to call from any goroutine.
func (engine *Engine) IncrementEpoch() {
	C.wasmtime_engine_increment_epoch(engine.ptr())
	runtime.KeepAlive(engine)
}

// IsPulley will return whether the current engine's execution is backed by
// the Pulley interpreter inside of Wasmtime. If this returns false then
// native execution is used instead.
func (engine *Engine) IsPulley() bool {
	ret := C.wasmtime_engine_is_pulley(engine.ptr())
	runtime.KeepAlive(engine)
	return bool(ret)
}
