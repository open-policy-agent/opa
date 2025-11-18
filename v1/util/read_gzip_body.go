package util

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util/decoding"
)

var gzipReaderPool = sync.Pool{
	New: func() any {
		reader := new(gzip.Reader)
		return reader
	},
}

// Note(philipc): Originally taken from server/server.go
// The DecodingLimitHandler handles validating that the gzip payload is within the
// allowed max size limit. Thus, in the event of a forged payload size trailer,
// the worst that can happen is that we waste memory up to the allowed max gzip
// payload size, but not an unbounded amount of memory, as was potentially
// possible before.
func ReadMaybeCompressedBody(r *http.Request) ([]byte, error) {
	length := r.ContentLength
	if maxLenConf, ok := decoding.GetServerDecodingMaxLen(r.Context()); ok {
		length = maxLenConf
	}

	content, err := io.ReadAll(io.LimitReader(r.Body, length))
	if err != nil {
		return nil, err
	}

	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		gzipMaxLength, _ := decoding.GetServerDecodingGzipMaxLen(r.Context())

		// Note(philipc): The last 4 bytes of a well-formed gzip blob will
		// always be a little-endian uint32, representing the decompressed
		// content size, modulo 2^32. We validate that the size is safe,
		// earlier in DecodingLimitHandler.
		sizeDecompressed := int64(binary.LittleEndian.Uint32(content[len(content)-4:]))
		if sizeDecompressed > gzipMaxLength {
			return nil, errors.New("gzip payload too large")
		}

		gzReader := gzipReaderPool.Get().(*gzip.Reader)
		defer func() {
			gzReader.Close()
			gzipReaderPool.Put(gzReader)
		}()

		if err := gzReader.Reset(bytes.NewReader(content)); err != nil {
			return nil, err
		}

		decompressed := bytes.NewBuffer(make([]byte, 0, sizeDecompressed))
		if _, err = io.CopyN(decompressed, gzReader, sizeDecompressed); err != nil {
			return nil, err
		}

		return decompressed.Bytes(), nil
	}

	// Request was not compressed; return the content bytes.
	return content, nil
}
