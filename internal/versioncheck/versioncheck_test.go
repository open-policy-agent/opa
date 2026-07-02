// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package versioncheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestNewReportDefaultURL(t *testing.T) {

	checker, err := New(Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	actual := checker.(*GitHubVersionChecker).client.Config().URL
	if actual != ExternalServiceURL {
		t.Fatalf("Expected server URL %v but got %v", ExternalServiceURL, actual)
	}
}

func TestLatestVersionBadRespStatus(t *testing.T) {

	// test server
	baseURL, teardown := getTestServer(nil, http.StatusBadRequest)
	defer teardown()

	t.Setenv("OPA_VERSION_CHECK_SERVICE_URL", baseURL)

	checker, err := New(Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = checker.LatestVersion(t.Context())

	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expectedErrMsg := "server replied with HTTP 400"
	if expectedErrMsg != err.Error() {
		t.Fatalf("Expected error: %v but got: %v", expectedErrMsg, err.Error())
	}
}

func TestLatestVersionDecodeError(t *testing.T) {

	// test server
	baseURL, teardown := getTestServer("foo", http.StatusOK)
	defer teardown()

	t.Setenv("OPA_VERSION_CHECK_SERVICE_URL", baseURL)

	checker, err := New(Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = checker.LatestVersion(t.Context())

	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestLatestVersionWithOPAUpdate(t *testing.T) {
	// to support testing on all supported platforms
	downloadLink := fmt.Sprintf("https://openpolicyagent.org/downloads/v100.0.0/opa_%v_%v",
		runtime.GOOS, runtime.GOARCH)

	if runtime.GOARCH == "arm64" {
		downloadLink = fmt.Sprintf("%v_static", downloadLink)
	}

	if strings.HasPrefix(runtime.GOOS, "win") {
		downloadLink = fmt.Sprintf("%v.exe", downloadLink)
	}

	srvResp := &GitHubRelease{
		TagName:      "v100.0.0",
		Download:     downloadLink,
		ReleaseNotes: "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(srvResp, http.StatusOK)
	defer teardown()

	t.Setenv("OPA_VERSION_CHECK_SERVICE_URL", baseURL)

	checker, err := New(Options{})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	resp, err := checker.LatestVersion(t.Context())

	if err != nil {
		t.Fatalf("Expected no error but got %v", err)
	}

	exp := &DataResponse{Latest: ReleaseDetails{
		Download:      downloadLink,
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: "v100.0.0",
		OPAUpToDate:   false,
	}}

	if !reflect.DeepEqual(resp, exp) {
		t.Fatalf("Expected response: %+v but got: %+v", exp, resp)
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

func getTestServer(update any, statusCode int) (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/repos/open-policy-agent/opa/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		bs, _ := json.Marshal(update)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bs)
	})
	return ts.URL, ts.Close
}
