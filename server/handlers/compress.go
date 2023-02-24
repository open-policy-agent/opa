package handlers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
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
		// this handler applies only for data and compile endpoints for selected HTTP methods
		enabledForEndpoint := isDataEndpoint(request) || isCompileEndpoint(request)
		if !enabledForEndpoint {
			handler.ServeHTTP(responseWriter, request)
			return
		}
		gzipHeaderDetected := strings.Contains(request.Header.Get(acceptEncodingHeader), gzipEncodingValue)
		if !gzipHeaderDetected {
			handler.ServeHTTP(responseWriter, request)
			return
		}
		responseWriter.Header().Set(contentEncodingHeader, gzipEncodingValue)

		gzipWriter := gzipPool.Get().(*gzip.Writer)
		defer gzipPool.Put(gzipWriter)

		var b bytes.Buffer
		gzipWriter.Reset(&b)
		defer func() {
			gzipWriter.Close()
			responseWriter.Header().Set(contentLengthHeader, fmt.Sprint(len(b.Bytes())))
			_, err := responseWriter.Write(b.Bytes())
			if err != nil {
				log.Fatalf("Error writing the compressed response: %s\n", err)
			}
		}()

		crw := &compressResponseWriter{Writer: gzipWriter, ResponseWriter: responseWriter}
		handler.ServeHTTP(crw, request)
	})
}

func isDataEndpoint(req *http.Request) bool {
	isPostOrGetMethod := isPostMethod(req) || isGetMethod(req)
	isV1rV0 := strings.HasPrefix(req.URL.Path, "/v1/data") || strings.HasPrefix(req.URL.Path, "/v0/data")
	return isPostOrGetMethod && isV1rV0
}

func isCompileEndpoint(req *http.Request) bool {
	return isPostMethod(req) && strings.HasPrefix(req.URL.Path, "/v1/compile")
}

func isPostMethod(req *http.Request) bool {
	return req.Method == "POST"
}

func isGetMethod(req *http.Request) bool {
	return req.Method == "GET"
}

var gzipPool = sync.Pool{
	New: func() interface{} {
		writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestCompression)
		return writer
	},
}
