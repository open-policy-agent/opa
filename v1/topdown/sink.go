package topdown

import (
	"bytes"
	"io"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	_ io.Writer = (*sinkW)(nil)

	sinkWriterPool = util.NewSyncPool[sinkW]()
)

type (
	sinkWriter interface {
		io.Writer
		io.StringWriter
		io.ByteWriter

		String() string
		Grow(int)
	}
	sinkW struct {
		bytes.Buffer

		cancel Cancel
		name   string
	}
)

func (sw *sinkW) Write(bs []byte) (int, error) {
	if sw.isCancelled() {
		return 0, sw.halt()
	}
	return sw.Buffer.Write(bs)
}

func (sw *sinkW) WriteByte(b byte) error {
	if sw.isCancelled() {
		return sw.halt()
	}
	return sw.Buffer.WriteByte(b)
}

func (sw *sinkW) WriteString(s string) (int, error) {
	if sw.isCancelled() {
		return 0, sw.halt()
	}
	return sw.Buffer.WriteString(s)
}

func (sw *sinkW) String() string {
	return sw.Buffer.String()
}

// reset used as Reset() would overwrite the method on the buffer
func (sw *sinkW) reset(name string, c Cancel) sinkWriter {
	sw.Buffer.Reset()
	sw.name = name
	sw.cancel = c

	return sw
}

func (sw *sinkW) isCancelled() bool {
	return sw.cancel != nil && sw.cancel.Cancelled()
}

func (sw *sinkW) halt() error {
	return Halt{Err: &Error{Code: CancelErr, Message: sw.name + ": timed out before finishing"}}
}
