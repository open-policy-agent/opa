package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

const (
	acceptEncodingHeader  = "Accept-Encoding"
	contentEncodingHeader = "Content-Encoding"
	contentLengthHeader   = "Content-Length"
	gzipEncodingValue     = "gzip"
)

type compressResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *compressResponseWriter) WriteHeader(statusCode int) {
	w.Header().Del(contentLengthHeader)
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func CompressHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// this handler applies only for data and compile endpoints
		shouldApplyCompression := isDataEndpoint(request) || isCompileEndpoint(request)
		if !shouldApplyCompression {
			handler.ServeHTTP(responseWriter, request)
			return
		}
		gzipHeaderDetected := strings.Contains(request.Header.Get(acceptEncodingHeader), gzipEncodingValue)
		if !gzipHeaderDetected {
			handler.ServeHTTP(responseWriter, request)
			return
		}
		responseWriter.Header().Set(contentEncodingHeader, gzipEncodingValue)
		gzipWriter, _ := gzip.NewWriterLevel(responseWriter, gzip.BestCompression)
		defer gzipWriter.Close()
		crw := &compressResponseWriter{Writer: gzipWriter, ResponseWriter: responseWriter}
		handler.ServeHTTP(crw, request)
	})
}

func isDataEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/v1/data") || strings.HasPrefix(req.URL.Path, "/v0/data")
}

func isCompileEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/v1/compile")
}
