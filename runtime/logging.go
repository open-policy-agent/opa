// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"io/ioutil"

	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/server/types"
)

func loggingEnabled(level logrus.Level) bool {
	return level <= logrus.GetLevel()
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
	requestID := atomic.AddUint64(&h.requestID, uint64(1))

	recorder := newRecorder(w, r, requestID, loggingEnabled(logrus.DebugLevel))
	t0 := time.Now()

	if loggingEnabled(logrus.InfoLevel) {

		fields := logrus.Fields{
			"client_addr": r.RemoteAddr,
			"req_id":      requestID,
			"req_method":  r.Method,
			"req_path":    r.URL.EscapedPath(),
		}

		var err error

		if loggingEnabled(logrus.DebugLevel) {
			var bs []byte
			var err error
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
			logrus.WithFields(fields).Info("Received request.")
		} else {
			logrus.WithFields(fields).Error("Failed to read body.")
		}
	}

	params := r.URL.Query()

	if _, ok := params["watch"]; ok {
		logrus.Warn("Deprecated 'watch' parameter specified in request. See https://github.com/open-policy-agent/opa/releases/tag/v0.23.0 for details.")
	}

	if _, ok := params["partial"]; ok {
		logrus.Warn("Deprecated 'partial' parameter specified in request. See https://github.com/open-policy-agent/opa/releases/tag/v0.23.0 for details.")
	}

	h.inner.ServeHTTP(recorder, r)

	dt := time.Since(t0)
	statusCode := 200
	if recorder.statusCode != 0 {
		statusCode = recorder.statusCode
	}

	if loggingEnabled(logrus.InfoLevel) {
		fields := logrus.Fields{
			"client_addr":   r.RemoteAddr,
			"req_id":        requestID,
			"req_method":    r.Method,
			"req_path":      r.URL.EscapedPath(),
			"resp_status":   statusCode,
			"resp_bytes":    recorder.bytesWritten,
			"resp_duration": float64(dt.Nanoseconds()) / 1e6,
		}

		if loggingEnabled(logrus.DebugLevel) {
			fields["resp_body"] = recorder.buf.String()
		}

		logrus.WithFields(fields).Info("Sent response.")
	}
}

type recorder struct {
	inner http.ResponseWriter
	req   *http.Request
	id    uint64

	buf          *bytes.Buffer
	bytesWritten int
	statusCode   int
}

func newRecorder(w http.ResponseWriter, r *http.Request, id uint64, buffer bool) *recorder {
	var buf *bytes.Buffer
	if buffer {
		buf = new(bytes.Buffer)
	}
	return &recorder{
		buf:   buf,
		inner: w,
		req:   r,
		id:    id,
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

	fields := logrus.Fields{
		"client_addr": r.req.RemoteAddr,
		"req_id":      r.id,
		"req_method":  r.req.Method,
		"req_path":    r.req.URL.EscapedPath(),
	}

	queries := r.req.URL.Query()[types.ParamQueryV1]
	if len(queries) > 0 {
		fields["req_query"] = queries[len(queries)-1]
	}
	logrus.WithFields(fields).Info("Started watch.")

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

// prettyFormatter implements the Logrus Formatter interface
// and provides a more simple, but easier to read, text formatter
// option than the default logrus.TextFormatter.
type prettyFormatter struct {
}

func isJSON(buf []byte) bool {
	var tmp interface{}
	err := json.Unmarshal(buf, &tmp)
	return err == nil
}

func spaces(num int) string {
	sb := strings.Builder{}
	for i := 0; i < num; i++ {
		sb.WriteByte(' ')
	}
	return sb.String()
}

func (p *prettyFormatter) Format(e *logrus.Entry) ([]byte, error) {
	b := new(bytes.Buffer)

	level := strings.ToUpper(e.Level.String())
	b.WriteString(fmt.Sprintf("[%s] %s\n", level, e.Message))

	// Format each key for optimal ease of human reading
	fieldIndent := 2
	multiLineIndent := 6
	for k, v := range e.Data {
		// Special case for multi-line strings, keep them as-is
		// but indent them. Everything else gets json'd
		stringVal, ok := v.(string)
		if ok && strings.Contains(stringVal, "\n") {
			sb := strings.Builder{}
			for i, line := range strings.Split(stringVal, "\n") {
				// match the json indent helper by not indenting the first value
				if i != 0 {
					sb.WriteString(spaces(multiLineIndent))
				}
				sb.WriteString(line)
				sb.WriteByte('\n')
				stringVal = sb.String()
			}
		} else if ok && isJSON([]byte(stringVal)) {
			var tmp bytes.Buffer
			err := json.Indent(&tmp, []byte(stringVal), spaces(multiLineIndent), spaces(2))
			if err != nil {
				return nil, err
			}
			stringVal = tmp.String()
		} else {
			jsonVal, err := json.MarshalIndent(v, spaces(multiLineIndent), spaces(2))
			if err != nil {
				return nil, err
			}
			stringVal = string(jsonVal)
		}

		b.WriteString(spaces(fieldIndent))
		b.WriteString(k)
		if strings.Contains(stringVal, "\n") {
			b.WriteString(" = |\n")
			b.WriteString(spaces(multiLineIndent))
		} else {
			b.WriteString(" = ")
		}
		b.WriteString(stringVal)
		b.WriteString("\n")
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
