// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"io/ioutil"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/opa/server/types"
)

// DebugLogging returns true if log verbosity is high enough to emit debug
// messages.
func DebugLogging() bool {
	return logrus.DebugLevel >= logrus.GetLevel()
}

// LoggingHandler returns an http.Handler that will print log messages
// containing the request information as well as response status and latency.
type LoggingHandler struct {
	inner     http.Handler
	requestID uint64
}

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(inner http.Handler) http.Handler {
	return &LoggingHandler{inner, uint64(0)}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorder := newRecorder(w)
	t0 := time.Now()
	requestID := atomic.AddUint64(&h.requestID, uint64(1))

	if DebugLogging() {

		var bs []byte
		var err error

		if r.Body != nil {
			bs, r.Body, err = readBody(r.Body)
		}

		if err == nil {
			logrus.WithFields(logrus.Fields{
				"client_addr": r.RemoteAddr,
				"req_id":      requestID,
				"req_method":  r.Method,
				"req_path":    r.URL.Path,
				"req_params":  r.URL.Query(),
				"req_body":    string(bs),
			}).Debug("Received request.")
		} else {
			logrus.WithFields(logrus.Fields{
				"client_addr": r.RemoteAddr,
				"req_id":      requestID,
				"req_method":  r.Method,
				"req_path":    r.URL.Path,
				"req_params":  r.URL.Query(),
				"err":         err,
			}).Error("Failed to read body.")
		}
	}

	h.inner.ServeHTTP(recorder, r)

	dt := time.Since(t0)
	statusCode := 200
	if recorder.statusCode != 0 {
		statusCode = recorder.statusCode
	}

	if DebugLogging() {
		logrus.WithFields(logrus.Fields{
			"client_addr":   r.RemoteAddr,
			"req_id":        requestID,
			"req_method":    r.Method,
			"req_path":      r.URL.Path,
			"resp_status":   statusCode,
			"resp_bytes":    recorder.bytesWritten,
			"resp_duration": float64(dt.Nanoseconds()) / 1e6,
		}).Debug("Sent response.")
	}
}

type recorder struct {
	inner        http.ResponseWriter
	bytesWritten int
	statusCode   int
}

func newRecorder(w http.ResponseWriter) *recorder {
	return &recorder{
		inner: w,
	}
}

func (r *recorder) Header() http.Header {
	return r.inner.Header()
}

func (r *recorder) Write(bs []byte) (int, error) {
	r.bytesWritten += len(bs)
	return r.inner.Write(bs)
}

func (r *recorder) WriteHeader(s int) {
	r.statusCode = s
	r.inner.WriteHeader(s)
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
