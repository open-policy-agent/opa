package util

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/open-policy-agent/opa/v1/util/decoding"
)

// BodyTooLargeError is returned by ReadMaybeCompressedBody when a request body
// exceeds server.decoding.max_length, or when a gzip body decompresses to more
// than server.decoding.gzip.max_length.
type BodyTooLargeError struct {
	// Limit is the exceeded limit in bytes.
	Limit int64
	// Size is the size of the body in bytes, or 0 when it isn't known.
	Size int64
	// Decompressed is true when the limit applies to the decompressed size.
	Decompressed bool
}

func (e *BodyTooLargeError) Error() string {
	// The limit stays in bytes to match how it is configured.
	size := ""
	if e.Size > 0 {
		size = " " + formatBytes(e.Size)
	}
	if e.Decompressed {
		return fmt.Sprintf("request body too large: decompressed size%s exceeds limit of %d bytes (server.decoding.gzip.max_length)", size, e.Limit)
	}
	return fmt.Sprintf("request body too large:%s exceeds limit of %d bytes (server.decoding.max_length)", size, e.Limit)
}

// formatBytes renders n in the largest binary unit it reaches, such as "256 MiB"
// or "286.1 MiB". Values under 1 KiB are rendered as bytes.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d bytes", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	suffix := "KMGTPE"[exp : exp+1]
	v := strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/float64(div)), ".0")
	return v + " " + suffix + "iB"
}

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

	// Read one byte past the limit so an oversized body is reported instead of
	// being silently truncated into a confusing parse or gzip error.
	readLimit := length
	if length < math.MaxInt64 {
		readLimit++
	}
	content, err := io.ReadAll(io.LimitReader(r.Body, readLimit))
	if err != nil {
		// The server wraps the body in an http.MaxBytesReader, which errors on
		// the extra byte.
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, &BodyTooLargeError{Limit: maxErr.Limit}
		}
		return nil, err
	}
	if length >= 0 && int64(len(content)) > length {
		return nil, &BodyTooLargeError{Limit: length}
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
			return nil, &BodyTooLargeError{Limit: gzipMaxLength, Decompressed: true}
		}

		return decompressed.Bytes(), nil
	}

	// Request was not compressed; return the content bytes.
	return content, nil
}
