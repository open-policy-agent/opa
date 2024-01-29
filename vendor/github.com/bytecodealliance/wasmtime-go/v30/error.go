package wasmtime

// #include <wasmtime.h>
import "C"
import "runtime"

type Error struct {
	_ptr *C.wasmtime_error_t
}

func mkError(ptr *C.wasmtime_error_t) *Error {
	err := &Error{_ptr: ptr}
	runtime.SetFinalizer(err, func(err *Error) {
		err.Close()
	})
	return err
}

func (e *Error) ptr() *C.wasmtime_error_t {
	ret := e._ptr
	if ret == nil {
		panic("object has been closed already")
	}
	maybeGC()
	return ret
}

func (e *Error) Error() string {
	message := C.wasm_byte_vec_t{}
	C.wasmtime_error_message(e.ptr(), &message)
	ret := C.GoStringN(message.data, C.int(message.size))
	runtime.KeepAlive(e)
	C.wasm_byte_vec_delete(&message)
	return ret
}

// ExitStatus returns an `int32` exit status if this was a WASI-defined exit
// code. The `bool` returned indicates whether it was a WASI-defined exit or
// not.
func (e *Error) ExitStatus() (int32, bool) {
	status := C.int(0)
	ok := C.wasmtime_error_exit_status(e.ptr(), &status)
	runtime.KeepAlive(e)
	return int32(status), bool(ok)
}

// Close will deallocate this error's state explicitly.
//
// For more information see the documentation for engine.Close()
func (e *Error) Close() {
	if e._ptr == nil {
		return
	}
	runtime.SetFinalizer(e, nil)
	C.wasmtime_error_delete(e._ptr)
	e._ptr = nil

}
