// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build slow

package topdown

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTTPSendTimeout(t *testing.T) {

	// Each test can tweak the response delay, default is 0 with no delay
	var responseDelay time.Duration

	tsMtx := sync.Mutex{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tsMtx.Lock()
		defer tsMtx.Unlock()
		time.Sleep(responseDelay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`hello`))
	}))
	// Note: We don't Close() the test server as it will block waiting for the
	// timed out clients connections to shut down gracefully (they wont).
	// We don't need to clean it up nicely for the unit test.

	tests := []struct {
		note           string
		rule           string
		input          string
		defaultTimeout time.Duration
		evalTimeout    time.Duration
		serverDelay    time.Duration
		expected       interface{}
	}{
		{
			note:     "no timeout",
			rule:     `p = x { http.send({"method": "get", "url": "%URL%" }, resp); x := remove_headers(resp) }`,
			expected: `{"body": null, "raw_body": "hello", "status": "200 OK", "status_code": 200}`,
		},
		{
			note:           "default timeout",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%" }, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 500 * time.Millisecond,
			expected:       &Error{Code: BuiltinErr, Message: "request timed out"},
		},
		{
			note:           "eval timeout",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%" }, x) }`,
			evalTimeout:    500 * time.Millisecond,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: CancelErr, Message: "timed out (context deadline exceeded)"},
		},
		{
			note:           "param timeout less than default",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "500ms"}, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: BuiltinErr, Message: "request timed out"},
		},
		{
			note:           "param timeout greater than default",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "500ms"}, x) }`,
			evalTimeout:    1 * time.Minute,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Millisecond,
			expected:       &Error{Code: BuiltinErr, Message: "request timed out"},
		},
		{
			note:           "eval timeout less than param",
			rule:           `p = x { http.send({"method": "get", "url": "%URL%", "timeout": "1m" }, x) }`,
			evalTimeout:    500 * time.Millisecond,
			serverDelay:    5 * time.Second,
			defaultTimeout: 1 * time.Minute,
			expected:       &Error{Code: CancelErr, Message: "timed out (context deadline exceeded)"},
		},
	}

	for _, tc := range tests {
		tsMtx.Lock()
		responseDelay = tc.serverDelay
		tsMtx.Unlock()

		ctx := context.Background()
		if tc.evalTimeout > 0 {
			ctx, _ = context.WithTimeout(ctx, tc.evalTimeout)
		}

		// TODO(patrick-east): Remove this along with the environment variable so that the "default" can't change
		originalDefaultTimeout := defaultHTTPRequestTimeout
		if tc.defaultTimeout > 0 {
			defaultHTTPRequestTimeout = tc.defaultTimeout
		}

		rule := strings.ReplaceAll(tc.rule, "%URL%", ts.URL)
		if e, ok := tc.expected.(*Error); ok {
			e.Message = strings.ReplaceAll(e.Message, "%URL%", ts.URL)
		}

		runTopDownTestCaseWithContext(ctx, t, map[string]interface{}{}, tc.note, append(httpSendHelperRules, rule), nil, tc.input, tc.expected)

		// Put back the default (may not have changed)
		defaultHTTPRequestTimeout = originalDefaultTimeout
	}
}
