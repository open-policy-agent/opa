package pool

import "bytes"

var bytesBufferPool = New[*bytes.Buffer](allocBytesBuffer, freeBytesBuffer)

func allocBytesBuffer() *bytes.Buffer {
	return &bytes.Buffer{}
}

func freeBytesBuffer(b *bytes.Buffer) *bytes.Buffer {
	b.Reset()
	return b
}

func BytesBuffer() *Pool[*bytes.Buffer] {
	return bytesBufferPool
}
