package wasmtime

// #include "shims.h"
import "C"
import (
	"errors"
	"runtime"
)

// Table is a table instance, which is the runtime representation of a table.
//
// It holds a vector of reference types and an optional maximum size, if one was
// specified in the table type at the table’s definition site.
// Read more in [spec](https://webassembly.github.io/spec/core/exec/runtime.html#table-instances)
type Table struct {
	val C.wasmtime_table_t
}

// NewTable creates a new `Table` in the given `Store` with the specified `ty`.
//
// The `ty` must be a reference type (`funref` or `externref`) and `init`
// is the initial value for all table slots and must have the type specified by
// `ty`.
func NewTable(store Storelike, ty *TableType, init Val) (*Table, error) {
	var ret C.wasmtime_table_t
	var raw_val C.wasmtime_val_t
	init.initialize(store, &raw_val)
	err := C.wasmtime_table_new(store.Context(), ty.ptr(), &raw_val, &ret)
	C.wasmtime_val_unroot(store.Context(), &raw_val)
	runtime.KeepAlive(store)
	runtime.KeepAlive(ty)
	if err != nil {
		return nil, mkError(err)
	}
	return mkTable(ret), nil
}

func mkTable(val C.wasmtime_table_t) *Table {
	return &Table{val}
}

// Size returns the size of this table in units of elements.
func (t *Table) Size(store Storelike) uint64 {
	ret := C.wasmtime_table_size(store.Context(), &t.val)
	runtime.KeepAlive(store)
	return uint64(ret)
}

// Grow grows this table by the number of units specified, using the
// specified initializer value for new slots.
//
// Returns an error if the table failed to grow, or the previous size of the
// table if growth was successful.
func (t *Table) Grow(store Storelike, delta uint64, init Val) (uint64, error) {
	var prev C.uint64_t
	var raw_val C.wasmtime_val_t
	init.initialize(store, &raw_val)
	err := C.wasmtime_table_grow(store.Context(), &t.val, C.uint64_t(delta), &raw_val, &prev)
	C.wasmtime_val_unroot(store.Context(), &raw_val)
	runtime.KeepAlive(store)
	if err != nil {
		return 0, mkError(err)
	}

	return uint64(prev), nil
}

// Get gets an item from this table from the specified index.
//
// Returns an error if the index is out of bounds, or returns a value (which
// may be internally null) if the index is in bounds corresponding to the entry
// at the specified index.
func (t *Table) Get(store Storelike, idx uint64) (Val, error) {
	var val C.wasmtime_val_t
	ok := C.wasmtime_table_get(store.Context(), &t.val, C.uint64_t(idx), &val)
	runtime.KeepAlive(store)
	if !ok {
		return Val{}, errors.New("index out of bounds")
	}
	return takeVal(store, &val), nil
}

// Set sets an item in this table at the specified index.
//
// Returns an error if the index is out of bounds.
func (t *Table) Set(store Storelike, idx uint64, val Val) error {
	var raw_val C.wasmtime_val_t
	val.initialize(store, &raw_val)
	err := C.wasmtime_table_set(store.Context(), &t.val, C.uint64_t(idx), &raw_val)
	C.wasmtime_val_unroot(store.Context(), &raw_val)
	runtime.KeepAlive(store)
	if err != nil {
		return mkError(err)
	}
	return nil
}

// Type returns the underlying type of this table
func (t *Table) Type(store Storelike) *TableType {
	ptr := C.wasmtime_table_type(store.Context(), &t.val)
	runtime.KeepAlive(store)
	return mkTableType(ptr, nil)
}

func (t *Table) AsExtern() C.wasmtime_extern_t {
	ret := C.wasmtime_extern_t{kind: C.WASMTIME_EXTERN_TABLE}
	C.go_wasmtime_extern_table_set(&ret, t.val)
	return ret
}
