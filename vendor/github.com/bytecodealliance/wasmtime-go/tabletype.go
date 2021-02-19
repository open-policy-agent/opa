package wasmtime

// #include <wasm.h>
import "C"
import "runtime"

// TableType is one of table types which classify tables over elements of element types within a size range.
type TableType struct {
	_ptr   *C.wasm_tabletype_t
	_owner interface{}
}

// NewTableType creates a new `TableType` with the `element` type provided as well as
// `limits` on its size.
func NewTableType(element *ValType, limits Limits) *TableType {
	valptr := C.wasm_valtype_new(C.wasm_valtype_kind(element.ptr()))
	runtime.KeepAlive(element)
	limitsFFI := limits.ffi()
	ptr := C.wasm_tabletype_new(valptr, &limitsFFI)

	return mkTableType(ptr, nil)
}

func mkTableType(ptr *C.wasm_tabletype_t, owner interface{}) *TableType {
	tabletype := &TableType{_ptr: ptr, _owner: owner}
	if owner == nil {
		runtime.SetFinalizer(tabletype, func(tabletype *TableType) {
			C.wasm_tabletype_delete(tabletype._ptr)
		})
	}
	return tabletype
}

func (ty *TableType) ptr() *C.wasm_tabletype_t {
	ret := ty._ptr
	maybeGC()
	return ret
}

func (ty *TableType) owner() interface{} {
	if ty._owner != nil {
		return ty._owner
	}
	return ty
}

// Element returns the type of value stored in this table
func (ty *TableType) Element() *ValType {
	ptr := C.wasm_tabletype_element(ty.ptr())
	return mkValType(ptr, ty.owner())
}

// Limits returns limits on the size of this table type
func (ty *TableType) Limits() Limits {
	ptr := C.wasm_tabletype_limits(ty.ptr())
	return mkLimits(ptr, ty.owner())
}

// AsExternType converts this type to an instance of `ExternType`
func (ty *TableType) AsExternType() *ExternType {
	ptr := C.wasm_tabletype_as_externtype_const(ty.ptr())
	return mkExternType(ptr, ty.owner())
}
