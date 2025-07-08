// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
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
		"Rego Version",
	})
}

func TestGenerateCmdOutputWithCheckFlagNoError(t *testing.T) {
	// to support testing on all supported platforms
	downloadLink := fmt.Sprintf("https://openpolicyagent.org/downloads/v100.0.0/opa_%v_%v",
		runtime.GOOS, runtime.GOARCH)

	if runtime.GOARCH == "arm64" {
		downloadLink = fmt.Sprintf("%v_static", downloadLink)
	}

	if strings.HasPrefix(runtime.GOOS, "win") {
		downloadLink = fmt.Sprintf("%v.exe", downloadLink)
	}

	resp := &report.GHResponse{
		TagName:      "v100.0.0",
		Download:     downloadLink,
		ReleaseNotes: "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(resp, http.StatusOK)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

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
		"Rego Version",
	})
}

func TestCheckOPAUpdateBadURL(t *testing.T) {
	url := "http://foo:8112"
	t.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

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
