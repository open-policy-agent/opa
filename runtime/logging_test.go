// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package runtime

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log"
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

func TestValidateReceivedGzipHeader(t *testing.T) {

	httpHeader := http.Header{}
	httpHeader.Set("Content-Encoding", "*/*")
	if result, expected := gzipReceived(httpHeader), false; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Content-Encoding", "gzip")
	if result, expected := gzipReceived(httpHeader), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Content-Encoding", "gzip, deflate, br")
	if result, expected := gzipReceived(httpHeader), true; result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	httpHeader.Set("Content-Encoding", "br;q=1.0, gzip;q=0.8, *;q=0.1")
	if result, expected := gzipReceived(httpHeader), true; result != expected {
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

	// set the threshold to a low value so the server compresses the response when asked to
	gzipMinLength := "server.encoding.gzip.min_length=5"
	shutdownSeconds := 1
	params := NewParams()
	params.Addrs = &[]string{"localhost:0"}
	params.Logger = logger
	params.PprofEnabled = true
	params.GracefulShutdownPeriod = shutdownSeconds // arbitrary, must be non-zero
	params.ConfigOverrides = []string{gzipMinLength}

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

	// prepare the request bodies to be used
	var dataEndpointBody = []byte(`{"input": {"data": "checkForMe"}}`)
	var compileEndpointBody = []byte(`{"unknowns": ["input"], "query": "data.checkForMe = true"}`)
	var dataEndpointCompressedBody = zipString(`{"input": {"data": "checkForMe"}}`)
	var compileEndpointCompressedBody = zipString(`{"unknowns": ["input"], "query": "data.checkForMe = true"}`)

	tests := []struct {
		path             string
		acceptEncoding   string
		expected         string
		expectedEncoding string
		contentEncoding  string
		requestBody      *[]byte
	}{
		{
			path:             "/metrics",
			acceptEncoding:   "gzip",
			expected:         "[compressed payload]",
			expectedEncoding: "gzip",
			contentEncoding:  "",
			requestBody:      nil,
		},
		{
			path:             "/metrics",
			acceptEncoding:   "*/*",
			expected:         "HELP go_gc_duration_seconds A summary of the pause duration of garbage collection cycles.",
			expectedEncoding: "",
			contentEncoding:  "",
			requestBody:      nil,
		},
		{ // the data handler on GET will compress the response if response is above server.encoding.gzip.min_length in size
			path:             "/v1/data",
			acceptEncoding:   "gzip",
			expected:         "{\"result\":{}}",
			expectedEncoding: "gzip",
			contentEncoding:  "",
			requestBody:      nil,
		},
		{ // the data handler on POST can compress the response if response is above server.encoding.gzip.min_length in size
			path:             "/v1/data",
			acceptEncoding:   "gzip",
			expected:         "{\"result\":{}}",
			expectedEncoding: "gzip",
			contentEncoding:  "",
			requestBody:      &dataEndpointBody,
		},
		{ // the data handler on POST can consume compressed request
			path:             "/v1/data",
			acceptEncoding:   "gzip",
			expected:         "{\"result\":{}}",
			expectedEncoding: "gzip",
			contentEncoding:  "gzip",
			requestBody:      &dataEndpointCompressedBody,
		},
		{ // the compile handler will compress the response if response is above server.encoding.gzip.min_length in size
			path:             "/v1/compile",
			acceptEncoding:   "gzip",
			expected:         "{\"result\":{}}",
			expectedEncoding: "gzip",
			contentEncoding:  "",
			requestBody:      &compileEndpointBody,
		},
		{ // the compile handler can consume compressed request
			path:             "/v1/compile",
			acceptEncoding:   "gzip",
			expected:         "{\"result\":{}}",
			expectedEncoding: "gzip",
			contentEncoding:  "gzip",
			requestBody:      &compileEndpointCompressedBody,
		},
		{ // the handlers return plain data
			path:             "/v1/data",
			acceptEncoding:   "*/*",
			expected:         "{\"result\":{}}",
			expectedEncoding: "",
			contentEncoding:  "",
			requestBody:      nil,
		},
		{ // accept-encoding does not matter for pprof: it's always protobuf
			path:             "/debug/pprof/cmdline",
			acceptEncoding:   "*/*",
			expected:         "[binary payload]",
			expectedEncoding: "",
			contentEncoding:  "",
			requestBody:      nil,
		},
	}

	// execute all the requests
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		method := "GET"
		var body io.Reader
		if tc.requestBody != nil {
			method = "POST"
			body = bytes.NewReader(*tc.requestBody)
		}
		req, err := http.NewRequest(method, tc.path, body)

		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", tc.acceptEncoding)
		if tc.contentEncoding != "" {
			req.Header.Set("Content-Encoding", tc.contentEncoding)
		}
		rt.server.Handler.ServeHTTP(rec, req)
		if exp, act := http.StatusOK, rec.Result().StatusCode; exp != act {
			t.Errorf("%s %s: expected HTTP %d, got %d", method, tc.path, exp, act)
		}
		contentEncoding := rec.Result().Header.Get("Content-Encoding")
		if contentEncoding != tc.expectedEncoding {
			t.Errorf("%s %s: expected content encoding %s, got %s", method, tc.path, tc.expectedEncoding, contentEncoding)
		}
	}

	cancel()

	// check the logs
	ents := logger.Entries()
	for j, tc := range tests {
		i := uint64(j + 1)
		foundResponse := false
		foundRequest := false
		for _, ent := range entriesForReq(ents, i) {
			if ent.Message == "Sent response." {
				act := ent.Fields["resp_body"].(string)
				if !strings.Contains(act, tc.expected) {
					t.Errorf("expected %q in resp_body field, got %q", tc.expected, act)
				}
				foundResponse = true
			}
			if tc.requestBody != nil && ent.Message == "Received request." {
				if tc.requestBody != nil {
					// the req_body is always uncompressed
					act := ent.Fields["req_body"].(string)
					if !strings.Contains(act, "checkForMe") {
						t.Errorf("expected string %q in req_body field, got %q", "checkForMe", act)
					}
					foundRequest = true
				}
			}
		}
		if !foundResponse {
			t.Errorf("Expected \"Sent response.\" log for request %d (path %s)", j, tc.path)
		}
		if tc.requestBody != nil && !foundRequest {
			t.Errorf("Expected \"Received request.\" log for request %d (path %s)", j, tc.path)
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
func zipString(input string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(input)); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}
