package wasmtime

// #include <wasmtime.h>
//
// wasmtime_error_t *go_module_new(wasm_engine_t *engine, uint8_t *bytes, size_t len, wasm_module_t **ret) {
//    wasm_byte_vec_t vec;
//    vec.data = (wasm_byte_t*) bytes;
//    vec.size = len;
//    return wasmtime_module_new(engine, &vec, ret);
// }
//
// wasmtime_error_t *go_module_validate(wasm_store_t *store, uint8_t *bytes, size_t len) {
//    wasm_byte_vec_t vec;
//    vec.data = (wasm_byte_t*) bytes;
//    vec.size = len;
//    return wasmtime_module_validate(store, &vec);
// }
//
// wasmtime_error_t *go_module_deserialize(wasm_engine_t *engine, uint8_t *bytes, size_t len, wasm_module_t **ret) {
//    wasm_byte_vec_t vec;
//    vec.data = (wasm_byte_t*) bytes;
//    vec.size = len;
//    return wasmtime_module_deserialize(engine, &vec, ret);
// }
import "C"
import (
	"io/ioutil"
	"runtime"
	"unsafe"
)

// Module is a module which collects definitions for types, functions, tables, memories, and globals.
// In addition, it can declare imports and exports and provide initialization logic in the form of data and element segments or a start function.
// Modules organized WebAssembly programs as the unit of deployment, loading, and compilation.
type Module struct {
	_ptr   *C.wasm_module_t
	Engine *Engine
}

// NewModule compiles a new `Module` from the `wasm` provided with the given configuration
// in `engine`.
func NewModule(engine *Engine, wasm []byte) (*Module, error) {
	// We can't create the `wasm_byte_vec_t` here and pass it in because
	// that runs into the error of "passed a pointer to a pointer" because
	// the vec itself is passed by pointer and it contains a pointer to
	// `wasm`. To work around this we insert some C shims above and call
	// them.
	var wasmPtr *C.uint8_t
	if len(wasm) > 0 {
		wasmPtr = (*C.uint8_t)(unsafe.Pointer(&wasm[0]))
	}
	var ptr *C.wasm_module_t
	err := C.go_module_new(engine.ptr(), wasmPtr, C.size_t(len(wasm)), &ptr)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(wasm)

	if err != nil {
		return nil, mkError(err)
	}

	return mkModule(ptr, engine), nil
}

// NewModuleFromFile reads the contents of the `file` provided and interprets them as either the
// text format or the binary format for WebAssembly.
//
// Afterwards delegates to the `NewModule` constructor with the contents read.
func NewModuleFromFile(engine *Engine, file string) (*Module, error) {
	wasm, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	// If this wasm isn't actually wasm, treat it as the text format and
	// parse it as such.
	if len(wasm) > 0 && wasm[0] != 0 {
		wasm, err = Wat2Wasm(string(wasm))
		if err != nil {
			return nil, err
		}
	}
	return NewModule(engine, wasm)

}

// ModuleValidate validates whether `wasm` would be a valid wasm module according to the
// configuration in `store`
func ModuleValidate(store *Store, wasm []byte) error {
	var wasmPtr *C.uint8_t
	if len(wasm) > 0 {
		wasmPtr = (*C.uint8_t)(unsafe.Pointer(&wasm[0]))
	}
	err := C.go_module_validate(store.ptr(), wasmPtr, C.size_t(len(wasm)))
	runtime.KeepAlive(store)
	runtime.KeepAlive(wasm)
	if err == nil {
		return nil
	}

	return mkError(err)
}

func mkModule(ptr *C.wasm_module_t, engine *Engine) *Module {
	module := &Module{_ptr: ptr, Engine: engine}
	runtime.SetFinalizer(module, func(module *Module) {
		C.wasm_module_delete(module._ptr)
	})
	return module
}

func (m *Module) ptr() *C.wasm_module_t {
	ret := m._ptr
	maybeGC()
	return ret
}

type importTypeList struct {
	vec C.wasm_importtype_vec_t
}

// Imports returns a list of `ImportType` items which are the items imported by this
// module and are required for instantiation.
func (m *Module) Imports() []*ImportType {
	imports := &importTypeList{}
	C.wasm_module_imports(m.ptr(), &imports.vec)
	runtime.KeepAlive(m)
	runtime.SetFinalizer(imports, func(imports *importTypeList) {
		C.wasm_importtype_vec_delete(&imports.vec)
	})

	ret := make([]*ImportType, int(imports.vec.size))
	base := unsafe.Pointer(imports.vec.data)
	var ptr *C.wasm_importtype_t
	for i := 0; i < int(imports.vec.size); i++ {
		ptr := *(**C.wasm_importtype_t)(unsafe.Pointer(uintptr(base) + unsafe.Sizeof(ptr)*uintptr(i)))
		ty := mkImportType(ptr, imports)
		ret[i] = ty
	}
	return ret
}

type exportTypeList struct {
	vec C.wasm_exporttype_vec_t
}

// Exports returns a list of `ExportType` items which are the items that will
// be exported by this module after instantiation.
func (m *Module) Exports() []*ExportType {
	exports := &exportTypeList{}
	C.wasm_module_exports(m.ptr(), &exports.vec)
	runtime.KeepAlive(m)
	runtime.SetFinalizer(exports, func(exports *exportTypeList) {
		C.wasm_exporttype_vec_delete(&exports.vec)
	})

	ret := make([]*ExportType, int(exports.vec.size))
	base := unsafe.Pointer(exports.vec.data)
	var ptr *C.wasm_exporttype_t
	for i := 0; i < int(exports.vec.size); i++ {
		ptr := *(**C.wasm_exporttype_t)(unsafe.Pointer(uintptr(base) + unsafe.Sizeof(ptr)*uintptr(i)))
		ty := mkExportType(ptr, exports)
		ret[i] = ty
	}
	return ret
}

// NewModuleDeserialize decodes and deserializes in-memory bytes previously
// produced by `module.Serialize()`.
//
// This function does not take a WebAssembly binary as input. It takes
// as input the results of a previous call to `Serialize()`, and only takes
// that as input.
//
// If deserialization is successful then a compiled module is returned,
// otherwise nil and an error are returned.
//
// Note that to deserialize successfully the bytes provided must have beeen
// produced with an `Engine` that has the same commpilation options as the
// provided engine, and from the same version of this library.
func NewModuleDeserialize(engine *Engine, encoded []byte) (*Module, error) {
	var encodedPtr *C.uint8_t
	var ptr *C.wasm_module_t
	if len(encoded) > 0 {
		encodedPtr = (*C.uint8_t)(unsafe.Pointer(&encoded[0]))
	}
	err := C.go_module_deserialize(
		engine.ptr(),
		encodedPtr,
		C.size_t(len(encoded)),
		&ptr,
	)
	runtime.KeepAlive(engine)
	runtime.KeepAlive(encoded)

	if err != nil {
		return nil, mkError(err)
	}

	return mkModule(ptr, engine), nil
}

// Serialize will convert this in-memory compiled module into a list of bytes.
//
// The purpose of this method is to extract an artifact which can be stored
// elsewhere from this `Module`. The returned bytes can, for example, be stored
// on disk or in an object store. The `NewModuleDeserialize` function can be
// used to deserialize the returned bytes at a later date to get the module
// back.
func (m *Module) Serialize() ([]byte, error) {
	retVec := C.wasm_byte_vec_t{}
	err := C.wasmtime_module_serialize(m.ptr(), &retVec)
	runtime.KeepAlive(m)

	if err != nil {
		return nil, mkError(err)
	}
	ret := C.GoBytes(unsafe.Pointer(retVec.data), C.int(retVec.size))
	C.wasm_byte_vec_delete(&retVec)
	return ret, nil
}
