// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/version"

	"testing"
)

func TestGenerateCmdOutputDisableCheckFlag(t *testing.T) {
	var stdout bytes.Buffer
	setTestVersion()

	generateCmdOutput(&stdout, false)

	expected := getTestVersion()
	if stdout.String() != expected {
		t.Fatalf("Expected output:%v but got %v", expected, stdout.String())
	}
}

func TestGenerateCmdOutputWithCheckFlagNoError(t *testing.T) {
	exp := &report.DataResponse{Latest: report.ReleaseDetails{
		Download:      "https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64",
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: "v100.0.0",
	}}

	// test server
	baseURL, teardown := getTestServer(exp, http.StatusOK)
	defer teardown()

	banner := "Latest Upstream Version: 100.0.0\n" +
		"Download: https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64\n" +
		"Release Notes: https://github.com/open-policy-agent/opa/releases/tag/v100.0.0\n"

	expected := getTestVersion() + banner

	testGenerateCmdOutput(t, baseURL, expected)
}

func TestCheckOPAUpdateBadURL(t *testing.T) {
	url := "http://foo:8112"
	os.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	err := checkOPAUpdate(nil)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func testGenerateCmdOutput(t *testing.T, url, expected string) {
	t.Helper()

	os.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	var stdout bytes.Buffer
	setTestVersion()

	generateCmdOutput(&stdout, true)

	if stdout.String() != expected {
		t.Fatalf("Expected output:\"%v\" but got \"%v\"", expected, stdout.String())
	}
}

func setTestVersion() {
	version.Version = "v0.20.0"
	version.Vcs = "12345"
	version.Timestamp = "2020-05-14T06:22:38Z"
	version.Hostname = "foo"
	version.GoVersion = "1.14.7"
}

func getTestVersion() string {
	return "Version: v0.20.0\n" +
		"Build Commit: 12345\n" +
		"Build Timestamp: 2020-05-14T06:22:38Z\n" +
		"Build Hostname: foo\n" +
		"Go Version: 1.14.7\n"
}

func getTestServer(update interface{}, statusCode int) (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(statusCode)
		bs, _ := json.Marshal(update)
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
	})
	return ts.URL, ts.Close
}
