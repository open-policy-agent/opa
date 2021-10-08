// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/sirupsen/logrus"
)

type loggingPrintHook struct {
	logger logging.Logger
}

func (h loggingPrintHook) Print(pctx print.Context, msg string) error {
	// NOTE(tsandall): if the request context is not present then do not panic,
	// just log the print message without the additional context.
	rctx, _ := pctx.Context.Value(reqCtxKey).(requestContext)
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

type requestContextKey string

const reqCtxKey = requestContextKey("request-context-key")

type requestContext struct {
	ClientAddr string
	ReqID      uint64
	ReqMethod  string
	ReqPath    string
}

func (rctx requestContext) Fields() logrus.Fields {
	return logrus.Fields{
		"client_addr": rctx.ClientAddr,
		"req_id":      rctx.ReqID,
		"req_method":  rctx.ReqMethod,
		"req_path":    rctx.ReqPath,
	}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rctx requestContext
	rctx.ReqID = atomic.AddUint64(&h.requestID, uint64(1))
	recorder := newRecorder(h.logger, w, r, rctx.ReqID, h.loggingEnabled(logging.Debug))
	t0 := time.Now()

	if h.loggingEnabled(logging.Info) {

		rctx.ClientAddr = r.RemoteAddr
		rctx.ReqMethod = r.Method
		rctx.ReqPath = r.URL.EscapedPath()
		r = r.WithContext(context.WithValue(r.Context(), reqCtxKey, rctx))

		var err error
		fields := rctx.Fields()

		if h.loggingEnabled(logging.Debug) {
			var bs []byte
			if r.Body != nil {
				bs, r.Body, err = readBody(r.Body)
			}
			if err == nil {
				fields["req_body"] = string(bs)
			} else {
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

	if _, ok := params["watch"]; ok {
		h.logger.Warn("Deprecated 'watch' parameter specified in request. See https://github.com/open-policy-agent/opa/releases/tag/v0.23.0 for details.")
	}

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
			fields["resp_body"] = recorder.buf.String()
		}

		h.logger.WithFields(fields).Info("Sent response.")
	}
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

func (r *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.inner.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer is not a http.Hijacker")
	}

	c, rw, err := h.Hijack()
	if err != nil {
		return nil, nil, err
	}

	fields := map[string]interface{}{
		"client_addr": r.req.RemoteAddr,
		"req_id":      r.id,
		"req_method":  r.req.Method,
		"req_path":    r.req.URL.EscapedPath(),
	}

	queries := r.req.URL.Query()[types.ParamQueryV1]
	if len(queries) > 0 {
		fields["req_query"] = queries[len(queries)-1]
	}
	r.logger.WithFields(fields).Info("Started watch.")

	return c, rw, nil
}

func dropInputParam(u *url.URL) string {
	cpy := url.Values{}
	for k, v := range u.Query() {
		if k != types.ParamInputV1 {
			cpy[k] = v
		}
	}
	if len(cpy) == 0 {
		return u.Path
	}
	return u.Path + "?" + cpy.Encode()
}

func readBody(r io.ReadCloser) ([]byte, io.ReadCloser, error) {
	if r == http.NoBody {
		return nil, r, nil
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, r, err
	}
	return buf.Bytes(), ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
