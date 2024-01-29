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
		C.wasmtime_error_delete(err._ptr)
	})
	return err
}

func (e *Error) ptr() *C.wasmtime_error_t {
	ret := e._ptr
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
