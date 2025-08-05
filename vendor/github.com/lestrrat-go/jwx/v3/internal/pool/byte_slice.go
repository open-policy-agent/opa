package pool

var byteSlicePool = SlicePool[byte]{
	pool: New[[]byte](allocByteSlice, freeByteSlice),
}

func allocByteSlice() []byte {
	return make([]byte, 0, 64) // Default capacity of 64 bytes
}

func freeByteSlice(b []byte) []byte {
	clear(b)
	b = b[:0] // Reset the slice to zero length
	return b
}

func ByteSlice() SlicePool[byte] {
	return byteSlicePool
}
