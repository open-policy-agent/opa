package util

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Note(philipc): Originally taken from server/server.go
func ReadMaybeCompressedBody(r *http.Request) (io.ReadCloser, error) {
	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		gzReader, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		bytesBody, err := io.ReadAll(gzReader)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(bytesBody)), err
	}
	return r.Body, nil
}
