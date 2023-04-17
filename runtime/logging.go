// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/topdown/print"
)

type loggingPrintHook struct {
	logger logging.Logger
}

func (h loggingPrintHook) Print(pctx print.Context, msg string) error {
	// NOTE(tsandall): if the request context is not present then do not panic,
	// just log the print message without the additional context.
	rctx, _ := logging.FromContext(pctx.Context)
	fields := rctx.Fields()
	fields["line"] = pctx.Location.String()
	h.logger.WithFields(fields).Info(msg)
	return nil
}

// LoggingHandler returns an http.Handler that will print log messages
// containing the request information as well as response status and latency.
type LoggingHandler struct {
	logger    logging.Logger
	inner     http.Handler
	requestID uint64
}

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(logger logging.Logger, inner http.Handler) http.Handler {
	return &LoggingHandler{
		logger:    logger,
		inner:     inner,
		requestID: uint64(0),
	}
}

func (h *LoggingHandler) loggingEnabled(level logging.Level) bool {
	return level <= h.logger.GetLevel()
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rctx logging.RequestContext
	rctx.ReqID = atomic.AddUint64(&h.requestID, uint64(1))
	recorder := newRecorder(h.logger, w, r, rctx.ReqID, h.loggingEnabled(logging.Debug))
	t0 := time.Now()

	if h.loggingEnabled(logging.Info) {

		rctx.ClientAddr = r.RemoteAddr
		rctx.ReqMethod = r.Method
		rctx.ReqPath = r.URL.EscapedPath()
		r = r.WithContext(logging.NewContext(r.Context(), &rctx))

		var err error
		fields := rctx.Fields()

		if h.loggingEnabled(logging.Debug) {
			var bs []byte
			if r.Body != nil {
				bs, r.Body, err = readBody(r.Body)
			}
			if err == nil {
				if gzipReceived(r.Header) {
					// the request is compressed
					var gzReader *gzip.Reader
					var plainOutput []byte
					reader := bytes.NewReader(bs)
					gzReader, err = gzip.NewReader(reader)
					if err == nil {
						plainOutput, err = io.ReadAll(gzReader)
						if err == nil {
							defer gzReader.Close()
							fields["req_body"] = string(plainOutput)
						}
					}
				} else {
					fields["req_body"] = string(bs)
				}
			}

			// err can be thrown on different statements
			if err != nil {
				fields["err"] = err
			}

			fields["req_params"] = r.URL.Query()
		}

		if err == nil {
			h.logger.WithFields(fields).Info("Received request.")
		} else {
			h.logger.WithFields(fields).Error("Failed to read body.")
		}
	}

	params := r.URL.Query()

	if _, ok := params["partial"]; ok {
		h.logger.Warn("Deprecated 'partial' parameter specified in request. See https://github.com/open-policy-agent/opa/releases/tag/v0.23.0 for details.")
	}

	h.inner.ServeHTTP(recorder, r)

	dt := time.Since(t0)
	statusCode := 200
	if recorder.statusCode != 0 {
		statusCode = recorder.statusCode
	}

	if h.loggingEnabled(logging.Info) {
		fields := map[string]interface{}{
			"client_addr":   rctx.ClientAddr,
			"req_id":        rctx.ReqID,
			"req_method":    rctx.ReqMethod,
			"req_path":      rctx.ReqPath,
			"resp_status":   statusCode,
			"resp_bytes":    recorder.bytesWritten,
			"resp_duration": float64(dt.Nanoseconds()) / 1e6,
		}

		if h.loggingEnabled(logging.Debug) {
			switch {
			case isPprofEndpoint(r):
				// pprof always sends binary data (protobuf)
				fields["resp_body"] = "[binary payload]"

			case gzipAccepted(r.Header) && isMetricsEndpoint(r):
				// metrics endpoint does so when the client accepts it (e.g. prometheus)
				fields["resp_body"] = "[compressed payload]"

			case gzipAccepted(r.Header) && gzipReceived(w.Header()) && (isDataEndpoint(r) || isCompileEndpoint(r)):
				// data and compile endpoints might compress the response
				gzReader, gzErr := gzip.NewReader(recorder.buf)
				if gzErr == nil {
					plainOutput, readErr := io.ReadAll(gzReader)
					if readErr == nil {
						defer gzReader.Close()
						fields["resp_body"] = string(plainOutput)
					} else {
						h.logger.Error("Failed to decompressed the payload: %v", readErr.Error())
					}
				} else {
					h.logger.Error("Failed to read the compressed payload: %v", gzErr.Error())
				}

			default:
				fields["resp_body"] = recorder.buf.String()
			}
		}

		h.logger.WithFields(fields).Info("Sent response.")
	}
}

func gzipAccepted(header http.Header) bool {
	a := header.Get("Accept-Encoding")
	parts := strings.Split(a, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "gzip" || strings.HasPrefix(part, "gzip;") {
			return true
		}
	}
	return false
}

func gzipReceived(header http.Header) bool {
	a := header.Get("Content-Encoding")
	parts := strings.Split(a, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "gzip" || strings.HasPrefix(part, "gzip;") {
			return true
		}
	}
	return false
}

func isPprofEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/debug/pprof/")
}

func isMetricsEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/metrics")
}

func isDataEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/v1/data") || strings.HasPrefix(req.URL.Path, "/v0/data")
}

func isCompileEndpoint(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/v1/compile")
}

type recorder struct {
	logger logging.Logger
	inner  http.ResponseWriter
	req    *http.Request
	id     uint64

	buf          *bytes.Buffer
	bytesWritten int
	statusCode   int
}

func newRecorder(logger logging.Logger, w http.ResponseWriter, r *http.Request, id uint64, buffer bool) *recorder {
	var buf *bytes.Buffer
	if buffer {
		buf = new(bytes.Buffer)
	}
	return &recorder{
		logger: logger,
		buf:    buf,
		inner:  w,
		req:    r,
		id:     id,
	}
}

func (r *recorder) Header() http.Header {
	return r.inner.Header()
}

func (r *recorder) Write(bs []byte) (int, error) {
	r.bytesWritten += len(bs)
	if r.buf != nil {
		r.buf.Write(bs)
	}
	return r.inner.Write(bs)
}

func (r *recorder) WriteHeader(s int) {
	r.statusCode = s
	r.inner.WriteHeader(s)
}

func readBody(r io.ReadCloser) ([]byte, io.ReadCloser, error) {
	if r == http.NoBody {
		return nil, r, nil
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, r, err
	}
	return buf.Bytes(), io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
