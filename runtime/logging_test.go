// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/logging/test"
)

func TestValidateGzipHeader(t *testing.T) {

	httpHeader := http.Header{}
	httpHeader.Add("Accept", "*/*")
	if result, expected := gzipAccepted(httpHeader), false; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Add("Accept-Encoding", "gzip")
	if result, expected := gzipAccepted(httpHeader), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Accept-Encoding", "gzip, deflate, br")
	if result, expected := gzipAccepted(httpHeader), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Accept-Encoding", "br;q=1.0, gzip;q=0.8, *;q=0.1")
	if result, expected := gzipAccepted(httpHeader), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestValidatePprofUrl(t *testing.T) {

	req := http.Request{}

	req.URL = &url.URL{Path: "/metrics"}
	if result, expected := isPprofEndpoint(&req), false; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	req.URL = &url.URL{Path: "/debug/pprof/"}
	if result, expected := isPprofEndpoint(&req), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestValidateMetricsUrl(t *testing.T) {

	req := http.Request{}

	req.URL = &url.URL{Path: "/metrics"}
	if result, expected := isMetricsEndpoint(&req), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	req.URL = &url.URL{Path: "/debug/pprof/"}
	if result, expected := isMetricsEndpoint(&req), false; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestRequestLogging(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := test.New()
	logger.SetLevel(logging.Debug)

	shutdownSeconds := 1
	params := NewParams()
	params.Addrs = &[]string{":0"}
	params.Logger = logger
	params.PprofEnabled = true
	params.GracefulShutdownPeriod = shutdownSeconds // arbitrary, must be non-zero

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	initChannel := rt.Manager.ServerInitializedChannel()
	go func() {
		if err := rt.Serve(ctx); err != nil {
			t.Error(err)
		}
	}()
	<-initChannel

	tests := []struct {
		path           string
		acceptEncoding string
		expected       string
	}{
		{
			"/metrics", "gzip", "[compressed payload]",
		},
		{
			"/metrics", "*/*", "HELP go_gc_duration_seconds A summary of the pause duration of garbage collection cycles.", // rest omitted
		},
		{ // accept-encoding does not matter for "our" handlers -- they don't compress
			"/v1/data", "gzip", "{\"result\":{}}",
		},
		{ // accept-encoding does not matter for pprof: it's always protobuf
			"/debug/pprof/cmdline", "*/*", "[binary payload]",
		},
	}

	// execute all the requests
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		req, err := http.NewRequest("GET", tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", tc.acceptEncoding)
		rt.server.Handler.ServeHTTP(rec, req)
		if exp, act := http.StatusOK, rec.Result().StatusCode; exp != act {
			t.Errorf("GET %s: expected HTTP %d, got %d", tc.path, exp, act)
		}
	}

	cancel()

	// check the logs
	ents := logger.Entries()
	for j, tc := range tests {
		i := uint64(j + 1)
		found := false
		for _, ent := range entriesForReq(ents, i) {
			if ent.Message == "Sent response." {
				act := ent.Fields["resp_body"].(string)
				if !strings.Contains(act, tc.expected) {
					t.Errorf("expected %q in resp_body field, got %q", tc.expected, act)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("Expected \"Sent response.\" log for request %d (path %s)", j, tc.path)
		}

	}
	if t.Failed() {
		t.Logf("logs: %v", ents)
	}
}

func entriesForReq(ents []test.LogEntry, n uint64) []test.LogEntry {
	var ret []test.LogEntry
	for _, e := range ents {
		if r, ok := e.Fields["req_id"]; ok {
			if i, ok := r.(uint64); ok {
				if i == n {
					ret = append(ret, e)
				}
			}
		}
	}
	return ret
}
