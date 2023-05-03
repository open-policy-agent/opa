// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build slow
// +build slow

package topdown

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
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

func TestHTTPSendRetryRequest(t *testing.T) {

	tests := []struct {
		note        string
		query       string
		response    string
		evalTimeout time.Duration
		cancel      Cancel
		wantErr     bool
		err         error
	}{
		{
			note:     "success",
			query:    `http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "max_retry_attempts": 100,  "timeout": "500ms"}, x)`,
			response: `{"x": 1}`,
		},
		{
			note:        "eval timeout",
			query:       `http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "max_retry_attempts": 100,  "timeout": "500ms"}, x)`,
			evalTimeout: 2 * time.Second,
			wantErr:     true,
			err:         fmt.Errorf("eval_cancel_error: http.send: timed out (context deadline exceeded)"),
		},
		{
			note:    "cancel query",
			query:   `http.send({"method": "get", "url": "%URL%", "force_json_decode": true, "max_retry_attempts": 100,  "timeout": "500ms"}, x)`,
			cancel:  NewCancel(),
			wantErr: true,
			err:     fmt.Errorf("eval_cancel_error: caller cancelled query execution"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(tc.response))
				if err != nil {
					t.Fatal(err)
				}
			}))

			defer ts.Close()

			// delay server start to exercise retry logic
			go func() {
				time.Sleep(time.Second * 5)
				ts.Start()
			}()

			ctx := context.Background()
			if tc.evalTimeout > 0 {
				ctx, _ = context.WithTimeout(ctx, tc.evalTimeout)
			}

			q := newQuery(strings.ReplaceAll(tc.query, "%URL%", "http://"+ts.Listener.Addr().String()), time.Now())

			if tc.cancel != nil {
				q.WithCancel(tc.cancel)

				go func() {
					time.Sleep(2 * time.Second)
					tc.cancel.Cancel()
				}()
			}

			res, err := q.Run(ctx)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}

				if len(res) != 1 {
					t.Fatalf("Expected one result but got %v", len(res))
				}

				resResponse := res[0]["x"].Value.(ast.Object).Get(ast.StringTerm("raw_body"))
				if ast.String(tc.response).Compare(resResponse.Value) != 0 {
					t.Fatalf("Expected response %v but got %v", tc.response, resResponse.String())
				}
			}
		})
	}
}
