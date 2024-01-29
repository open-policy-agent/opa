package wasmtime

// #include <wasm.h>
// #include "shims.h"
import "C"
import (
	"runtime"
	"sync"
	"unsafe"
)

var gExternrefLock sync.Mutex
var gExternrefMap = make(map[int]interface{})
var gExternrefSlab slab

// Val is a primitive numeric value.
// Moreover, in the definition of programs, immutable sequences of values occur to represent more complex data, such as text strings or other vectors.
type Val struct {
	kind C.wasmtime_valkind_t
	val  interface{}
}

// ValI32 converts a go int32 to a i32 Val
func ValI32(val int32) Val {
	return Val{kind: C.WASMTIME_I32, val: val}
}

// ValI64 converts a go int64 to a i64 Val
func ValI64(val int64) Val {
	return Val{kind: C.WASMTIME_I64, val: val}
}

// ValF32 converts a go float32 to a f32 Val
func ValF32(val float32) Val {
	return Val{kind: C.WASMTIME_F32, val: val}
}

// ValF64 converts a go float64 to a f64 Val
func ValF64(val float64) Val {
	return Val{kind: C.WASMTIME_F64, val: val}
}

// ValFuncref converts a Func to a funcref Val
//
// Note that `f` can be `nil` to represent a null `funcref`.
func ValFuncref(f *Func) Val {
	return Val{kind: C.WASMTIME_FUNCREF, val: f}
}

// ValExternref converts a go value to a externref Val
//
// Using `externref` is a way to pass arbitrary Go data into a WebAssembly
// module for it to store. Later, when you get a `Val`, you can extract the type
// with the `Externref()` method.
func ValExternref(val interface{}) Val {
	return Val{kind: C.WASMTIME_EXTERNREF, val: val}
}

//export goFinalizeExternref
func goFinalizeExternref(env unsafe.Pointer) {
	idx := int(uintptr(env)) - 1
	gExternrefLock.Lock()
	defer gExternrefLock.Unlock()
	delete(gExternrefMap, idx)
	gExternrefSlab.deallocate(idx)
}

func mkVal(store Storelike, src *C.wasmtime_val_t) Val {
	switch src.kind {
	case C.WASMTIME_I32:
		return ValI32(int32(C.go_wasmtime_val_i32_get(src)))
	case C.WASMTIME_I64:
		return ValI64(int64(C.go_wasmtime_val_i64_get(src)))
	case C.WASMTIME_F32:
		return ValF32(float32(C.go_wasmtime_val_f32_get(src)))
	case C.WASMTIME_F64:
		return ValF64(float64(C.go_wasmtime_val_f64_get(src)))
	case C.WASMTIME_FUNCREF:
		val := C.go_wasmtime_val_funcref_get(src)
		if val.store_id == 0 {
			return ValFuncref(nil)
		} else {
			return ValFuncref(mkFunc(val))
		}
	case C.WASMTIME_EXTERNREF:
		val := C.go_wasmtime_val_externref_get(src)
		if val.store_id == 0 {
			return ValExternref(nil)
		}
		data := C.wasmtime_externref_data(store.Context(), &val)
		runtime.KeepAlive(store)

		gExternrefLock.Lock()
		defer gExternrefLock.Unlock()
		return ValExternref(gExternrefMap[int(uintptr(data))-1])
	}
	panic("failed to get kind of `Val`")
}

func takeVal(store Storelike, src *C.wasmtime_val_t) Val {
	ret := mkVal(store, src)
	C.wasmtime_val_unroot(store.Context(), src)
	runtime.KeepAlive(store)
	return ret
}

// Kind returns the kind of value that this `Val` contains.
func (v Val) Kind() ValKind {
	switch v.kind {
	case C.WASMTIME_I32:
		return KindI32
	case C.WASMTIME_I64:
		return KindI64
	case C.WASMTIME_F32:
		return KindF32
	case C.WASMTIME_F64:
		return KindF64
	case C.WASMTIME_FUNCREF:
		return KindFuncref
	case C.WASMTIME_EXTERNREF:
		return KindExternref
	}
	panic("failed to get kind of `Val`")
}

// I32 returns the underlying 32-bit integer if this is an `i32`, or panics.
func (v Val) I32() int32 {
	if v.Kind() != KindI32 {
		panic("not an i32")
	}
	return v.val.(int32)
}

// I64 returns the underlying 64-bit integer if this is an `i64`, or panics.
func (v Val) I64() int64 {
	if v.Kind() != KindI64 {
		panic("not an i64")
	}
	return v.val.(int64)
}

// F32 returns the underlying 32-bit float if this is an `f32`, or panics.
func (v Val) F32() float32 {
	if v.Kind() != KindF32 {
		panic("not an f32")
	}
	return v.val.(float32)
}

// F64 returns the underlying 64-bit float if this is an `f64`, or panics.
func (v Val) F64() float64 {
	if v.Kind() != KindF64 {
		panic("not an f64")
	}
	return v.val.(float64)
}

// Funcref returns the underlying function if this is a `funcref`, or panics.
//
// Note that a null `funcref` is returned as `nil`.
func (v Val) Funcref() *Func {
	if v.Kind() != KindFuncref {
		panic("not a funcref")
	}
	return v.val.(*Func)
}

// Externref returns the underlying value if this is an `externref`, or panics.
//
// Note that a null `externref` is returned as `nil`.
func (v Val) Externref() interface{} {
	if v.Kind() != KindExternref {
		panic("not an externref")
	}
	return v.val
}

// Get returns the underlying 64-bit float if this is an `f64`, or panics.
func (v Val) Get() interface{} {
	return v.val
}

func (v Val) initialize(store Storelike, ptr *C.wasmtime_val_t) {
	ptr.kind = v.kind
	switch v.kind {
	case C.WASMTIME_I32:
		C.go_wasmtime_val_i32_set(ptr, C.int32_t(v.val.(int32)))
	case C.WASMTIME_I64:
		C.go_wasmtime_val_i64_set(ptr, C.int64_t(v.val.(int64)))
	case C.WASMTIME_F32:
		C.go_wasmtime_val_f32_set(ptr, C.float(v.val.(float32)))
	case C.WASMTIME_F64:
		C.go_wasmtime_val_f64_set(ptr, C.double(v.val.(float64)))
	case C.WASMTIME_FUNCREF:
		val := v.val.(*Func)
		if val != nil {
			C.go_wasmtime_val_funcref_set(ptr, val.val)
		} else {
			empty := C.wasmtime_func_t{}
			C.go_wasmtime_val_funcref_set(ptr, empty)
		}
	case C.WASMTIME_EXTERNREF:
		// If we have a non-nil value then store it in our global map
		// of all externref values. Otherwise there's nothing for us to
		// do since the `ref` field will already be a nil pointer.
		//
		// Note that we add 1 so all non-null externref values are
		// created with non-null pointers.
		if v.val == nil {
			C.go_wasmtime_val_externref_set(ptr, C.wasmtime_externref_t{})
		} else {
			gExternrefLock.Lock()
			defer gExternrefLock.Unlock()
			index := gExternrefSlab.allocate()
			gExternrefMap[index] = v.val
			var ref C.wasmtime_externref_t
			ok := C.go_externref_new(store.Context(), C.size_t(index+1), &ref)
			runtime.KeepAlive(store)
			if ok {
				C.go_wasmtime_val_externref_set(ptr, ref)
			} else {
				panic("failed to create an externref")
			}
		}
	default:
		panic("failed to get kind of `Val`")
	}
}
