package pool

var bytesSlicePool = New(allocByteSlice, destroyByteSlice)

func ByteSlice() *Pool[*[]byte] {
	return bytesSlicePool
}

func allocByteSlice() interface{} {
	buf := make([]byte, 0, 1024) // Preallocate a slice with a capacity of 1024 bytes
	return &buf
}

func destroyByteSlice(b *[]byte) {
	// Reset the slice to its zero value
	*b = (*b)[:0]
}
