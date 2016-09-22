// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"net/http"
	"net/url"
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
			dropGlobals(r.URL),
			statusCode,
			recorder.bytesWritten,
			float64(dt.Nanoseconds())/1e6)
		if glog.V(3) {
			for _, g := range getGlobals(r.URL) {
				glog.Infoln(g)
			}
		}
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

func dropGlobals(u *url.URL) string {
	cpy := url.Values{}
	for k, v := range u.Query() {
		if k != "global" {
			cpy[k] = v
		}
	}
	if len(cpy) == 0 {
		return u.Path
	}
	return u.Path + "?" + cpy.Encode()
}

func getGlobals(u *url.URL) (r []string) {
	for _, g := range u.Query()["global"] {
		s, err := url.QueryUnescape(g)
		if err == nil {
			r = append(r, s)
		}
	}
	return r
}
