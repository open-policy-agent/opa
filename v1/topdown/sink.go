package topdown

import (
	"bytes"
	"io"
)

var _ io.Writer = (*sinkW)(nil)

type sinkWriter interface {
	io.Writer
	String() string
	Grow(int)
	WriteByte(byte) error
	WriteString(string) (int, error)
}

type sinkW struct {
	buf    *bytes.Buffer
	cancel Cancel
	err    error
}

func newSink(name string, hint int, c Cancel) sinkWriter {
	b := &bytes.Buffer{}
	if hint > 0 {
		b.Grow(hint)
	}

	if c == nil {
		return b
	}

	return &sinkW{
		cancel: c,
		buf:    b,
		err: Halt{
			Err: &Error{
				Code:    CancelErr,
				Message: name + ": timed out before finishing",
			},
		},
	}
}

func (sw *sinkW) Grow(n int) {
	sw.buf.Grow(n)
}

func (sw *sinkW) Write(bs []byte) (int, error) {
	if sw.cancel.Cancelled() {
		return 0, sw.err
	}
	return sw.buf.Write(bs)
}

func (sw *sinkW) WriteByte(b byte) error {
	if sw.cancel.Cancelled() {
		return sw.err
	}
	return sw.buf.WriteByte(b)
}

func (sw *sinkW) WriteString(s string) (int, error) {
	if sw.cancel.Cancelled() {
		return 0, sw.err
	}
	return sw.buf.WriteString(s)
}

func (sw *sinkW) String() string {
	return sw.buf.String()
}
