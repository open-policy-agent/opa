// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"net/http"
	"time"

	"github.com/golang/glog"
)

// LoggingHandler returns an http.Handler that will print log messages to glog
// containing the request information as well as response status and latency.
type LoggingHandler struct {
	inner http.Handler
}

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(inner http.Handler) http.Handler {
	return &LoggingHandler{inner}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorder := newRecorder(w)
	t0 := time.Now()
	h.inner.ServeHTTP(recorder, r)
	if glog.V(2) {
		dt := time.Since(t0)
		statusCode := 200
		if recorder.statusCode != 0 {
			statusCode = recorder.statusCode
		}
		glog.Infof("%v %v %v %v %v %vms",
			r.RemoteAddr,
			r.Method,
			r.RequestURI,
			statusCode,
			recorder.bytesWritten,
			float64(dt.Nanoseconds())/1e6)
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
