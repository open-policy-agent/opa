package util

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/open-policy-agent/opa/v1/util/decoding"
)

var gzipReaderPool = NewSyncPool[gzip.Reader]()

// Note(philipc): Originally taken from server/server.go
// The DecodingLimitHandler handles setting the max size limits in the context.
// This function enforces those limits. For gzip payloads, we use a LimitReader
// to ensure we don't decompress more than the allowed maximum, preventing
// memory exhaustion from forged gzip trailers.
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

		gzReader := gzipReaderPool.Get()
		defer func() {
			gzReader.Close()
			gzipReaderPool.Put(gzReader)
		}()

		if err := gzReader.Reset(bytes.NewReader(content)); err != nil {
			return nil, err
		}

		decompressed := bytes.NewBuffer(make([]byte, 0, len(content)))
		limitReader := io.LimitReader(gzReader, gzipMaxLength+1)
		if _, err := decompressed.ReadFrom(limitReader); err != nil {
			return nil, err
		}

		if int64(decompressed.Len()) > gzipMaxLength {
			return nil, errors.New("gzip payload too large")
		}

		return decompressed.Bytes(), nil
	}

	// Request was not compressed; return the content bytes.
	return content, nil
}
