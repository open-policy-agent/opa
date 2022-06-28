package wasm

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
)

const (
	// MemoryPageSize is the unit of memory length in WebAssembly,
	// and is defined as 2^16 = 65536.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-instances%E2%91%A0
	MemoryPageSize = uint32(65536)
	// MemoryLimitPages is maximum number of pages defined (2^16).
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#grow-mem
	MemoryLimitPages = uint32(65536)
	// MemoryPageSizeInBits satisfies the relation: "1 << MemoryPageSizeInBits == MemoryPageSize".
	MemoryPageSizeInBits = 16
)

// MemorySizer is the default function that derives min, capacity and max pages from decoded wasm. The capacity
// returned is set to minPages and max defaults to MemoryLimitPages when maxPages is nil.
var MemorySizer api.MemorySizer = func(minPages uint32, maxPages *uint32) (min, capacity, max uint32) {
	if maxPages != nil {
		return minPages, minPages, *maxPages
	}
	return minPages, minPages, MemoryLimitPages
}

// compile-time check to ensure MemoryInstance implements api.Memory
var _ api.Memory = &MemoryInstance{}

// MemoryInstance represents a memory instance in a store, and implements api.Memory.
//
// Note: In WebAssembly 1.0 (20191205), there may be up to one Memory per store, which means the precise memory is always
// wasm.Store Memories index zero: `store.Memories[0]`
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-instances%E2%91%A0.
type MemoryInstance struct {
	Buffer        []byte
	Min, Cap, Max uint32
	// mux is used to prevent overlapping calls to Grow.
	mux sync.RWMutex
}

// NewMemoryInstance creates a new instance based on the parameters in the SectionIDMemory.
func NewMemoryInstance(memSec *Memory) *MemoryInstance {
	min := MemoryPagesToBytesNum(memSec.Min)
	capacity := MemoryPagesToBytesNum(memSec.Cap)
	return &MemoryInstance{
		Buffer: make([]byte, min, capacity),
		Min:    memSec.Min,
		Cap:    memSec.Cap,
		Max:    memSec.Max,
	}
}

// Size implements the same method as documented on api.Memory.
func (m *MemoryInstance) Size(_ context.Context) uint32 {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.size()
}

// ReadByte implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadByte(_ context.Context, offset uint32) (byte, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if offset >= m.size() {
		return 0, false
	}
	return m.Buffer[offset], true
}

// ReadUint16Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadUint16Le(_ context.Context, offset uint32) (uint16, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if !m.hasSize(offset, 2) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(m.Buffer[offset : offset+2]), true
}

// ReadUint32Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadUint32Le(_ context.Context, offset uint32) (uint32, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.readUint32Le(offset)
}

// ReadFloat32Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadFloat32Le(_ context.Context, offset uint32) (float32, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	v, ok := m.readUint32Le(offset)
	if !ok {
		return 0, false
	}
	return math.Float32frombits(v), true
}

// ReadUint64Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadUint64Le(_ context.Context, offset uint32) (uint64, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.readUint64Le(offset)
}

// ReadFloat64Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) ReadFloat64Le(_ context.Context, offset uint32) (float64, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	v, ok := m.readUint64Le(offset)
	if !ok {
		return 0, false
	}
	return math.Float64frombits(v), true
}

// Read implements the same method as documented on api.Memory.
func (m *MemoryInstance) Read(_ context.Context, offset, byteCount uint32) ([]byte, bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if !m.hasSize(offset, byteCount) {
		return nil, false
	}
	return m.Buffer[offset : offset+byteCount : offset+byteCount], true
}

// WriteByte implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteByte(_ context.Context, offset uint32, v byte) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if offset >= m.size() {
		return false
	}
	m.Buffer[offset] = v
	return true
}

// WriteUint16Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteUint16Le(_ context.Context, offset uint32, v uint16) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if !m.hasSize(offset, 2) {
		return false
	}
	binary.LittleEndian.PutUint16(m.Buffer[offset:], v)
	return true
}

// WriteUint32Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteUint32Le(_ context.Context, offset, v uint32) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.writeUint32Le(offset, v)
}

// WriteFloat32Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteFloat32Le(_ context.Context, offset uint32, v float32) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.writeUint32Le(offset, math.Float32bits(v))
}

// WriteUint64Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteUint64Le(_ context.Context, offset uint32, v uint64) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.writeUint64Le(offset, v)
}

// WriteFloat64Le implements the same method as documented on api.Memory.
func (m *MemoryInstance) WriteFloat64Le(_ context.Context, offset uint32, v float64) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return m.writeUint64Le(offset, math.Float64bits(v))
}

// Write implements the same method as documented on api.Memory.
func (m *MemoryInstance) Write(_ context.Context, offset uint32, val []byte) bool {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	if !m.hasSize(offset, uint32(len(val))) {
		return false
	}
	copy(m.Buffer[offset:], val)
	return true
}

// MemoryPagesToBytesNum converts the given pages into the number of bytes contained in these pages.
func MemoryPagesToBytesNum(pages uint32) (bytesNum uint64) {
	return uint64(pages) << MemoryPageSizeInBits
}

// Grow implements the same method as documented on api.Memory.
func (m *MemoryInstance) Grow(_ context.Context, delta uint32) (result uint32, ok bool) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	// We take write-lock here as the following might result in a new slice
	m.mux.Lock()
	defer m.mux.Unlock()

	currentPages := memoryBytesNumToPages(uint64(len(m.Buffer)))
	if delta == 0 {
		return currentPages, true
	}

	// If exceeds the max of memory size, we push -1 according to the spec.
	newPages := currentPages + delta
	if newPages > m.Max {
		return 0, false
	} else if newPages > m.Cap { // grow the memory.
		m.Buffer = append(m.Buffer, make([]byte, MemoryPagesToBytesNum(delta))...)
		m.Cap = newPages
		return currentPages, true
	} else { // We already have the capacity we need.
		sp := (*reflect.SliceHeader)(unsafe.Pointer(&m.Buffer))
		sp.Len = int(MemoryPagesToBytesNum(newPages))
		return currentPages, true
	}
}

// PageSize returns the current memory buffer size in pages.
func (m *MemoryInstance) PageSize(_ context.Context) (result uint32) {
	// Note: If you use the context.Context param, don't forget to coerce nil to context.Background()!

	return memoryBytesNumToPages(uint64(len(m.Buffer)))
}

// PagesToUnitOfBytes converts the pages to a human-readable form similar to what's specified. Ex. 1 -> "64Ki"
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#memory-instances%E2%91%A0
func PagesToUnitOfBytes(pages uint32) string {
	k := pages * 64
	if k < 1024 {
		return fmt.Sprintf("%d Ki", k)
	}
	m := k / 1024
	if m < 1024 {
		return fmt.Sprintf("%d Mi", m)
	}
	g := m / 1024
	if g < 1024 {
		return fmt.Sprintf("%d Gi", g)
	}
	return fmt.Sprintf("%d Ti", g/1024)
}

// Below are raw functions used to implement the api.Memory API:

// memoryBytesNumToPages converts the given number of bytes into the number of pages.
func memoryBytesNumToPages(bytesNum uint64) (pages uint32) {
	return uint32(bytesNum >> MemoryPageSizeInBits)
}

// size returns the size in bytes of the buffer.
func (m *MemoryInstance) size() uint32 {
	return uint32(len(m.Buffer)) // We don't lock here because size can't become smaller.
}

// hasSize returns true if Len is sufficient for byteCount at the given offset.
//
// Note: This is always fine, because memory can grow, but never shrink.
func (m *MemoryInstance) hasSize(offset uint32, byteCount uint32) bool {
	return uint64(offset)+uint64(byteCount) <= uint64(len(m.Buffer)) // uint64 prevents overflow on add
}

// readUint32Le implements ReadUint32Le without using a context. This is extracted as both ints and floats are stored in
// memory as uint32le.
func (m *MemoryInstance) readUint32Le(offset uint32) (uint32, bool) {
	if !m.hasSize(offset, 4) {
		return 0, false
	}
	return binary.LittleEndian.Uint32(m.Buffer[offset : offset+4]), true
}

// readUint64Le implements ReadUint64Le without using a context. This is extracted as both ints and floats are stored in
// memory as uint64le.
func (m *MemoryInstance) readUint64Le(offset uint32) (uint64, bool) {
	if !m.hasSize(offset, 8) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(m.Buffer[offset : offset+8]), true
}

// writeUint32Le implements WriteUint32Le without using a context. This is extracted as both ints and floats are stored
// in memory as uint32le.
func (m *MemoryInstance) writeUint32Le(offset uint32, v uint32) bool {
	if !m.hasSize(offset, 4) {
		return false
	}
	binary.LittleEndian.PutUint32(m.Buffer[offset:], v)
	return true
}

// writeUint64Le implements WriteUint64Le without using a context. This is extracted as both ints and floats are stored
// in memory as uint64le.
func (m *MemoryInstance) writeUint64Le(offset uint32, v uint64) bool {
	if !m.hasSize(offset, 8) {
		return false
	}
	binary.LittleEndian.PutUint64(m.Buffer[offset:], v)
	return true
}
