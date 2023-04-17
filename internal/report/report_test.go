// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package report

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestNewReportDefaultURL(t *testing.T) {

	reporter, err := New("", Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	actual := reporter.client.Config().URL
	if actual != ExternalServiceURL {
		t.Fatalf("Expected server URL %v but got %v", ExternalServiceURL, actual)
	}
}

func TestSendReportBadRespStatus(t *testing.T) {

	// test server
	baseURL, teardown := getTestServer(nil, http.StatusBadRequest)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	reporter, err := New("", Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = reporter.SendReport(context.Background())

	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expectedErrMsg := "server replied with HTTP 400"
	if expectedErrMsg != err.Error() {
		t.Fatalf("Expected error: %v but got: %v", expectedErrMsg, err.Error())
	}
}

func TestSendReportDecodeError(t *testing.T) {

	// test server
	baseURL, teardown := getTestServer("foo", http.StatusOK)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	reporter, err := New("", Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = reporter.SendReport(context.Background())

	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestSendReportWithOPAUpdate(t *testing.T) {
	exp := &DataResponse{Latest: ReleaseDetails{
		Download:      "https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64",
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: "v100.0.0",
		OPAUpToDate:   false,
	}}

	// test server
	baseURL, teardown := getTestServer(exp, http.StatusOK)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	reporter, err := New("", Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	resp, err := reporter.SendReport(context.Background())

	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	if !reflect.DeepEqual(resp, exp) {
		t.Fatalf("Expected response: %+v but got: %+v", exp, resp)
	}
}

func TestReportWithHeapStats(t *testing.T) {
	// test server
	baseURL, teardown := getTestServer(nil, http.StatusOK)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	reporter, err := New("", Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = reporter.SendReport(context.Background())

	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	if _, ok := reporter.body["heap_usage_bytes"]; !ok {
		t.Fatal("Expected key \"heap_usage_bytes\" in the report")
	}
}

func TestPretty(t *testing.T) {
	dr := DataResponse{}
	resp := dr.Pretty()

	if resp != "" {
		t.Fatalf("Expected empty response but got %v", resp)
	}

	dr.Latest.Download = "https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64"
	resp = dr.Pretty()

	if resp != "" {
		t.Fatalf("Expected empty response but got %v", resp)
	}

	dr.Latest.ReleaseNotes = "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0"
	resp = dr.Pretty()

	if resp != "" {
		t.Fatalf("Expected empty response but got %v", resp)
	}

	dr.Latest.LatestRelease = "v100.0.0"
	resp = dr.Pretty()

	exp := "Latest Upstream Version: 100.0.0\n" +
		"Download: https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64\n" +
		"Release Notes: https://github.com/open-policy-agent/opa/releases/tag/v100.0.0"

	if resp != exp {
		t.Fatalf("Expected response:\n\n%v\n\nGot:\n\n%v\n\n", exp, resp)
	}
}

func TestSlice(t *testing.T) {

	var dr *DataResponse

	if len(dr.Slice()) != 0 {
		t.Fatal("expected empty slice")
	}

	dr = &DataResponse{}

	if len(dr.Slice()) != 0 {
		t.Fatal("expected empty slice since fields are unset")
	}

	dr.Latest.Download = "https://example.com"
	dr.Latest.LatestRelease = "v0.100.0"
	dr.Latest.ReleaseNotes = "https://example2.com"

	exp := [][2]string{
		{"Latest Upstream Version", "0.100.0"},
		{"Download", "https://example.com"},
		{"Release Notes", "https://example2.com"},
	}

	if !reflect.DeepEqual(exp, dr.Slice()) {
		t.Fatalf("expected %v but got %v", exp, dr.Slice())
	}
}

func getTestServer(update interface{}, statusCode int) (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(statusCode)
		bs, _ := json.Marshal(update)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bs)
	})
	return ts.URL, ts.Close
}
