package pool

import (
	"bytes"
)

var bytesBufferPool = New(allocBytesBuffer, destroyBytesBuffer)

func BytesBuffer() *Pool[*bytes.Buffer] {
	return bytesBufferPool
}

func destroyBytesBuffer(b *bytes.Buffer) {
	b.Reset()
}

func allocBytesBuffer() interface{} {
	return &bytes.Buffer{}
}
