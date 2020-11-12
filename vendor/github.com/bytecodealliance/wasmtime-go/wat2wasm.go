package wasmtime

// #include <wasmtime.h>
//
// wasmtime_error_t *go_wat2wasm(
//  char *wat_ptr,
//  size_t wat_len,
//  wasm_byte_vec_t *ret
// ) {
//   wasm_byte_vec_t wat;
//   wat.data = wat_ptr;
//   wat.size = wat_len;
//   return wasmtime_wat2wasm(&wat, ret);
// }
import "C"
import (
	"runtime"
	"unsafe"
)

// Wat2Wasm converts the text format of WebAssembly to the binary format.
//
// Takes the text format in-memory as input, and returns either the binary
// encoding of the text format or an error if parsing fails.
func Wat2Wasm(wat string) ([]byte, error) {
	retVec := C.wasm_byte_vec_t{}
	err := C.go_wat2wasm(
		C._GoStringPtr(wat),
		C._GoStringLen(wat),
		&retVec,
	)
	runtime.KeepAlive(wat)

	if err == nil {
		ret := C.GoBytes(unsafe.Pointer(retVec.data), C.int(retVec.size))
		C.wasm_byte_vec_delete(&retVec)
		return ret, nil
	}

	return nil, mkError(err)
}
