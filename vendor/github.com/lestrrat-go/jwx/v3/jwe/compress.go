package jwe

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/lestrrat-go/jwx/v3/internal/pool"
)

func uncompress(src []byte, maxBufferSize int64) ([]byte, error) {
	var dst bytes.Buffer
	r := flate.NewReader(bytes.NewReader(src))
	defer r.Close()
	var buf [16384]byte
	var sofar int64
	for {
		n, readErr := r.Read(buf[:])
		sofar += int64(n)
		if sofar > maxBufferSize {
			return nil, fmt.Errorf(`compressed payload exceeds maximum allowed size`)
		}
		if readErr != nil {
			// if we have a read error, and it's not EOF, then we need to stop
			if readErr != io.EOF {
				return nil, fmt.Errorf(`failed to read inflated data: %w`, readErr)
			}
		}

		if _, err := dst.Write(buf[:n]); err != nil {
			return nil, fmt.Errorf(`failed to write inflated data: %w`, err)
		}

		if readErr != nil {
			// if it got here, then readErr == io.EOF, we're done
			return dst.Bytes(), nil
		}
	}
}

func compress(plaintext []byte) ([]byte, error) {
	buf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(buf)

	w, _ := flate.NewWriter(buf, 1)
	in := plaintext
	for len(in) > 0 {
		n, err := w.Write(in)
		if err != nil {
			return nil, fmt.Errorf(`failed to write to compression writer: %w`, err)
		}
		in = in[n:]
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf(`failed to close compression writer: %w`, err)
	}

	ret := make([]byte, buf.Len())
	copy(ret, buf.Bytes())
	return ret, nil
}
