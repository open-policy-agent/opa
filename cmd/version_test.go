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
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/report"
)

func TestGenerateCmdOutputDisableCheckFlag(t *testing.T) {
	var stdout bytes.Buffer

	generateCmdOutput(&stdout, false)

	expectOutputKeys(t, stdout.String(), []string{
		"Version",
		"Build Commit",
		"Build Timestamp",
		"Build Hostname",
		"Go Version",
		"Platform",
		"WebAssembly",
	})
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

	os.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	var stdout bytes.Buffer

	generateCmdOutput(&stdout, true)

	expectOutputKeys(t, stdout.String(), []string{
		"Version",
		"Build Commit",
		"Build Timestamp",
		"Build Hostname",
		"Go Version",
		"Platform",
		"WebAssembly",
		"Latest Upstream Version",
		"Release Notes",
		"Download",
	})
}

func TestCheckOPAUpdateBadURL(t *testing.T) {
	url := "http://foo:8112"
	os.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	err := checkOPAUpdate(nil)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func expectOutputKeys(t *testing.T, stdout string, expectedKeys []string) {
	t.Helper()

	lines := strings.Split(strings.Trim(stdout, "\n"), "\n")
	gotKeys := make([]string, 0, len(lines))

	for _, line := range lines {
		gotKeys = append(gotKeys, strings.Split(line, ":")[0])
	}

	sort.Strings(expectedKeys)
	sort.Strings(gotKeys)

	if len(expectedKeys) != len(gotKeys) {
		t.Fatalf("expected %v but got %v", expectedKeys, gotKeys)
	}

	for i, got := range gotKeys {
		if expectedKeys[i] != got {
			t.Fatalf("expected %v but got %v", expectedKeys, gotKeys)
		}
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
