package handlers

import (
	"compress/gzip"
	"fmt"
	"io"
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

// This handler applies only for data and compile endpoints, for selected HTTP methods
//
// If the client asked for a gzip response, this handler will buffer the response and
// wait until it reached a certain threshold. If the threshold is not hit, the uncompressed response is sent
//
// If a gzip response is not asked by the client, it'll send the uncompressed response
//
// The threshold and the gzip compression level can be modified from server's configuration

func CompressHandler(handler http.Handler, gzipMinLength int, gzipCompressionLevel int) http.Handler {
	initGzipPool(gzipCompressionLevel)

	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		enabledForEndpoint := isDataEndpoint(request) || isCompileEndpoint(request)
		if !enabledForEndpoint {
			handler.ServeHTTP(responseWriter, request)
			return
		}

		responseWriter.Header().Add("Vary", acceptEncodingHeader)

		if !gzipHeaderDetected(request.Header) {
			handler.ServeHTTP(responseWriter, request)
			return
		}

		crw := &compressResponseWriter{
			ResponseWriter: responseWriter,
			headerWritten:  false,
			minlength:      gzipMinLength,
		}
		defer crw.Close()
		handler.ServeHTTP(crw, request)
	})
}

type compressResponseWriter struct {
	gzipWriter *gzip.Writer
	http.ResponseWriter
	buffer        []byte
	statusCode    int
	headerWritten bool
	minlength     int
}

var gzipPool *sync.Pool

func initGzipPool(compressionLevel int) {
	if gzipPool == nil {
		gzipPool = &sync.Pool{
			New: func() interface{} {
				writer, _ := gzip.NewWriterLevel(io.Discard, compressionLevel)
				return writer
			},
		}
	}
}

func (w *compressResponseWriter) WriteHeader(statusCode int) {
	// save the status code for later use
	w.statusCode = statusCode
}

func (w *compressResponseWriter) Write(bytes []byte) (int, error) {
	if w.isGzipInitialized() {
		return w.gzipWriter.Write(bytes)
	}

	// accumulate the buffer
	w.buffer = append(w.buffer, bytes...)

	// if the buffer is above threshold, use compression
	if len(w.buffer) >= w.minlength {
		err := w.doCompressedResponse()
		if err != nil {
			return 0, err
		}
		return len(bytes), nil
	}

	// wait for more data
	return len(bytes), nil
}

func (w *compressResponseWriter) Flush() {
	if w.isGzipInitialized() {
		w.gzipWriter.Flush()
		flusher, canFlush := w.ResponseWriter.(http.Flusher)
		if canFlush {
			flusher.Flush()
		}
	}
}

func (w *compressResponseWriter) Close() error {
	if !w.isGzipInitialized() {
		// gzip didn't handle the response, send it plain
		err := w.doUncompressedResponse()
		if err != nil {
			err = fmt.Errorf("error writing uncompressed data: %v", err.Error())
		}
		return err
	}

	err := w.gzipWriter.Close()
	defer gzipPool.Put(w.gzipWriter)
	w.gzipWriter = nil
	return err
}

func (w *compressResponseWriter) doCompressedResponse() error {
	w.ResponseWriter.Header().Set(contentEncodingHeader, gzipEncodingValue)
	w.Header().Del(contentLengthHeader)
	w.writeHeader()
	// there's nothing to write
	if w.buffer == nil || len(w.buffer) <= 0 {
		return nil
	}
	gzipWriter := gzipPool.Get().(*gzip.Writer)
	gzipWriter.Reset(w.ResponseWriter)
	w.gzipWriter = gzipWriter
	_, err := w.gzipWriter.Write(w.buffer)
	return err
}

func (w *compressResponseWriter) doUncompressedResponse() error {
	w.writeHeader()
	// there's nothing to write
	if w.buffer == nil {
		return nil
	}
	_, err := w.ResponseWriter.Write(w.buffer)
	w.buffer = nil
	return err
}

func (w *compressResponseWriter) isGzipInitialized() bool {
	return w.gzipWriter != nil
}

func (w *compressResponseWriter) writeHeader() {
	if !w.headerWritten && w.statusCode != 0 {
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.headerWritten = true
	}
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

func gzipHeaderDetected(header http.Header) bool {
	a := header.Get("Accept-Encoding")
	parts := strings.Split(a, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == gzipEncodingValue || strings.HasPrefix(part, gzipEncodingValue+";") {
			return true
		}
	}
	return false
}
