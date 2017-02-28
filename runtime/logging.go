// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"net/http/httputil"

	"github.com/golang/glog"
	"github.com/open-policy-agent/opa/server/types"
)

// LoggingHandler returns an http.Handler that will print log messages to glog
// containing the request information as well as response status and latency.
type LoggingHandler struct {
	inner http.Handler
	rid   uint64
}

// NewLoggingHandler returns a new http.Handler.
func NewLoggingHandler(inner http.Handler) http.Handler {
	return &LoggingHandler{inner, uint64(0)}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorder := newRecorder(w)
	t0 := time.Now()
	rid := atomic.AddUint64(&h.rid, uint64(1))
	if glog.V(3) {
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			glog.Infof("rid=%v: %v", rid, err)
		} else {
			for _, line := range strings.Split(string(dump), "\n") {
				glog.Infof("rid=%v: %v", rid, line)
			}
		}
	}
	h.inner.ServeHTTP(recorder, r)
	if glog.V(2) {
		dt := time.Since(t0)
		statusCode := 200
		if recorder.statusCode != 0 {
			statusCode = recorder.statusCode
		}
		glog.Infof("rid=%v: %v %v %v %v %v %vms",
			rid,
			r.RemoteAddr,
			r.Method,
			dropInputParam(r.URL),
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
