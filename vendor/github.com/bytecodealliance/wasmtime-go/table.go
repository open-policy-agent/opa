package wasmtime

// #include <wasmtime.h>
//
// wasm_ref_t *go_get_ref(wasm_val_t *val) { return val->of.ref; }
// void go_init_ref(wasm_val_t *val, wasm_ref_t *i) { val->of.ref = i; }
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
	_ptr     *C.wasm_table_t
	_owner   interface{}
	freelist *freeList
}

// NewTable creates a new `Table` in the given `Store` with the specified `ty`.
//
// The `ty` must be a reference type (`funref` or `externref`) and `init`
// is the initial value for all table slots and must have the type specified by
// `ty`.
func NewTable(store *Store, ty *TableType, init Val) (*Table, error) {
	initPtr, err := getRefPtr(init)
	if err != nil {
		return nil, err
	}
	ptr := C.wasm_table_new(store.ptr(), ty.ptr(), initPtr)
	runtime.KeepAlive(store)
	runtime.KeepAlive(ty)
	runtime.KeepAlive(init)
	if ptr == nil {
		return nil, errors.New("failed to create table")
	}
	return mkTable(ptr, store.freelist, nil), nil
}

func mkTable(ptr *C.wasm_table_t, freelist *freeList, owner interface{}) *Table {
	f := &Table{_ptr: ptr, _owner: owner, freelist: freelist}
	if owner == nil {
		runtime.SetFinalizer(f, func(f *Table) {
			f.freelist.lock.Lock()
			defer f.freelist.lock.Unlock()
			f.freelist.tables = append(f.freelist.tables, f._ptr)
		})
	}
	return f
}

func (t *Table) ptr() *C.wasm_table_t {
	ret := t._ptr
	maybeGC()
	return ret
}

func (t *Table) owner() interface{} {
	if t._owner != nil {
		return t._owner
	}
	return t
}

// Size returns the size of this table in units of elements.
func (t *Table) Size() uint32 {
	ret := C.wasm_table_size(t.ptr())
	runtime.KeepAlive(t)
	return uint32(ret)
}

// Grow grows this table by the number of units specified, using the
// specified initializer value for new slots.
//
// Returns an error if the table failed to grow, or the previous size of the
// table if growth was successful.
func (t *Table) Grow(delta uint32, init Val) (uint32, error) {
	if t.Type().Element().Kind() != init.Kind() {
		return 0, errors.New("wrong type of initializer passed to `Grow`")
	}
	ptr, err := getRefPtr(init)
	if err != nil {
		return 0, err
	}
	ok := C.wasm_table_grow(t.ptr(), C.uint32_t(delta), ptr)
	runtime.KeepAlive(t)
	runtime.KeepAlive(init)
	if ok {
		return t.Size() - delta, nil
	}

	return 0, errors.New("failed to grow table")
}

func (t *Table) nullValue() Val {
	switch t.Type().Element().Kind() {
	case KindFuncref:
		return ValFuncref(nil)
	case KindExternref:
		return ValExternref(nil)
	default:
		panic("unsupported table type")
	}
}

// Get gets an item from this table from the specified index.
//
// Returns an error if the index is out of bounds, or returns a value (which
// may be internally null) if the index is in bounds corresponding to the entry
// at the specified index.
func (t *Table) Get(idx uint32) (Val, error) {
	null := t.nullValue()
	if idx >= t.Size() {
		return null, errors.New("index out of bounds")
	}
	valPtr := C.wasm_table_get(t.ptr(), C.uint32_t(idx))
	runtime.KeepAlive(t)
	if valPtr == nil {
		return null, nil
	}
	C.go_init_ref(null.ptr(), valPtr)
	ret := takeVal(null.ptr(), t.freelist)
	runtime.KeepAlive(null)
	return ret, nil
}

// Set sets an item in this table at the specified index.
//
// Returns an error if the index is out of bounds.
func (t *Table) Set(idx uint32, val Val) error {
	if t.Type().Element().Kind() != val.Kind() {
		return errors.New("wrong type of initializer passed to `Grow`")
	}
	ptr, err := getRefPtr(val)
	if err != nil {
		return err
	}
	ok := C.wasm_table_set(t.ptr(), C.uint32_t(idx), ptr)
	runtime.KeepAlive(t)
	runtime.KeepAlive(val)
	if !ok {
		return errors.New("failed to set table index")
	}
	return nil
}

// Type returns the underlying type of this table
func (t *Table) Type() *TableType {
	ptr := C.wasm_table_type(t.ptr())
	runtime.KeepAlive(t)
	return mkTableType(ptr, nil)
}

func (t *Table) AsExtern() *Extern {
	ptr := C.wasm_table_as_extern(t.ptr())
	return mkExtern(ptr, t.freelist, t.owner())
}

func getRefPtr(val Val) (*C.wasm_ref_t, error) {
	switch val.Kind() {
	case KindExternref, KindFuncref:
		return C.go_get_ref(val.ptr()), nil
	}
	return nil, errors.New("not a reference type")
}
