// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/test/e2e"
	"github.com/open-policy-agent/opa/util/test"
	"github.com/spf13/cobra"
)

func TestRunServerBase(t *testing.T) {
	params := newTestRunParams()
	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testRuntime := e2e.WrapRuntime(ctx, cancel, rt)

	done := make(chan bool)
	go func() {
		err := rt.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()

	err = testRuntime.WaitForServer()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateBasicServe(t, testRuntime)

	cancel()
	<-done
}

func TestRunServerBaseListenOnLocalhost(t *testing.T) {
	params := newTestRunParams()
	params.rt.V1Compatible = true

	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testRuntime := e2e.WrapRuntime(ctx, cancel, rt)

	done := make(chan bool)
	go func() {
		err := rt.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()

	err = testRuntime.WaitForServer()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateBasicServe(t, testRuntime)

	if len(rt.Addrs()) != 1 {
		t.Fatalf("Expected 1 listening address but got %v", len(rt.Addrs()))
	}

	expected := "127.0.0.1:8181"
	if rt.Addrs()[0] != expected {
		t.Fatalf("Expected listening address %v but got %v", expected, rt.Addrs()[0])
	}

	cancel()
	<-done
}

func TestRunServerWithDiagnosticAddr(t *testing.T) {
	params := newTestRunParams()
	params.rt.DiagnosticAddrs = &[]string{"localhost:0"}
	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	testRuntime := e2e.WrapRuntime(ctx, cancel, rt)

	done := make(chan bool)
	go func() {
		err := rt.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()

	err = testRuntime.WaitForServer()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateBasicServe(t, testRuntime)

	diagURL, err := testRuntime.AddrToURL(rt.DiagnosticAddrs()[0])
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := testRuntime.HealthCheck(diagURL); err != nil {
		t.Error(err)
	}

	cancel()
	<-done
}

func TestInitRuntimeVerifyNonBundle(t *testing.T) {

	params := newTestRunParams()
	params.pubKey = "secret"
	params.serverMode = false

	_, err := initRuntime(context.Background(), params, nil, false)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := "enable bundle mode (ie. --bundle) to verify bundle files or directories"
	if err.Error() != exp {
		t.Fatalf("expected error message %v but got %v", exp, err.Error())
	}
}

func TestInitRuntimeCipherSuites(t *testing.T) {
	testCases := []struct {
		name            string
		cipherSuites    []string
		expErr          bool
		expCipherSuites []uint16
	}{
		{"no cipher suites", []string{}, false, []uint16{}},
		{"secure and insecure cipher suites", []string{"TLS_RSA_WITH_AES_128_CBC_SHA", "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", "TLS_RSA_WITH_RC4_128_SHA"}, false, []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA, tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, tls.TLS_RSA_WITH_RC4_128_SHA}},
		{"invalid cipher suites", []string{"foo"}, true, []uint16{}},
		{"tls 1.3 cipher suite", []string{"TLS_AES_128_GCM_SHA256"}, true, []uint16{}},
		{"tls 1.2-1.3 cipher suite", []string{"TLS_RSA_WITH_AES_128_GCM_SHA256", "TLS_AES_128_GCM_SHA256"}, true, []uint16{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			params := newTestRunParams()

			if len(tc.cipherSuites) != 0 {
				params.cipherSuites = tc.cipherSuites
			}

			rt, err := initRuntime(context.Background(), params, nil, false)
			fmt.Println(err)

			if !tc.expErr && err != nil {
				t.Fatal("Unexpected error occurred:", err)
			} else if tc.expErr && err == nil {
				t.Fatal("Expected error but got nil")
			} else if err == nil {
				if len(tc.expCipherSuites) > 0 {
					if !reflect.DeepEqual(*rt.Params.CipherSuites, tc.expCipherSuites) {
						t.Fatalf("expected cipher suites %v but got %v", tc.expCipherSuites, *rt.Params.CipherSuites)
					}
				} else {
					if rt.Params.CipherSuites != nil {
						t.Fatal("expected no value defined for cipher suites")
					}
				}
			}
		})
	}
}

func TestInitRuntimeSkipKnownSchemaCheck(t *testing.T) {

	fs := map[string]string{
		"test/authz.rego": `package system.authz

		default allow := false

		allow {
          input.identty = "foo"        # this is a typo
		}`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		rootDir = filepath.Join(rootDir, "test")

		params := newTestRunParams()
		err := params.authorization.Set("basic")
		if err != nil {
			t.Fatal(err)
		}

		_, err = initRuntime(context.Background(), params, []string{rootDir}, false)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		if !strings.Contains(err.Error(), "undefined ref: input.identty") {
			t.Errorf("Expected error \"%v\" not found", "undefined ref: input.identty")
		}

		// skip type checking for known input schemas
		params.skipKnownSchemaCheck = true
		_, err = initRuntime(context.Background(), params, []string{rootDir}, false)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunServerCheckLogTimestampFormat(t *testing.T) {
	for _, format := range []string{time.Kitchen, time.RFC3339Nano} {
		t.Run(format, func(t *testing.T) {
			t.Run("parameter", func(t *testing.T) {
				params := newTestRunParams()
				params.logTimestampFormat = format
				checkLogTimeStampFormat(t, params, format)
			})
			t.Run("environment variable", func(t *testing.T) {
				t.Setenv("OPA_LOG_TIMESTAMP_FORMAT", format)
				params := newTestRunParams()
				checkLogTimeStampFormat(t, params, format)
			})
		})
	}
}

func checkLogTimeStampFormat(t *testing.T, params runCmdParams, format string) {
	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	var buf bytes.Buffer
	logger := rt.Manager.Logger().(*logging.StandardLogger)
	logger.SetOutput(&buf)
	testRuntime := e2e.WrapRuntime(ctx, cancel, rt)

	done := make(chan bool)
	go func() {
		err := rt.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()

	err = testRuntime.WaitForServer()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	validateBasicServe(t, testRuntime)

	cancel()
	<-done

	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			Time string `json:"time"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("incorrect log message %s: %v", line, err)
		}
		if rec.Time == "" {
			t.Fatalf("the time field is empty in log message: %s", line)
		}
		if _, err := time.Parse(format, rec.Time); err != nil {
			t.Fatalf("incorrect timestamp format %q: %v", rec.Time, err)
		}
	}
}

func TestInitRuntimeAddrSetByUser(t *testing.T) {
	testCases := []struct {
		name        string
		addrValue   string
		addrFlagSet bool
	}{
		{"AddrSetByUser_True", "localhost:8181", true},
		{"AddrSetByUser_False", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("addr", "", "set address")
			if tc.addrFlagSet {
				if err := cmd.Flags().Set("addr", tc.addrValue); err != nil {
					t.Fatalf("Failed to set addr flag: %v", err)
				}
			}

			params := newTestRunParams()
			params.rt.Addrs = &[]string{"localhost:0"}
			ctx, cancel := context.WithCancel(context.Background())

			rt, err := initRuntime(ctx, params, []string{}, cmd.Flags().Changed("addr"))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if rt.Params.AddrSetByUser != tc.addrFlagSet {
				t.Errorf("Expected AddrSetByUser to be %v, but got %v", tc.addrFlagSet, rt.Params.AddrSetByUser)
			}

			cancel()
		})
	}
}

func newTestRunParams() runCmdParams {
	params := newRunParams()
	params.rt.GracefulShutdownPeriod = 1
	params.rt.Addrs = &[]string{"localhost:0"}
	params.rt.DiagnosticAddrs = &[]string{}
	params.serverMode = true
	return params
}

func validateBasicServe(t *testing.T, runtime *e2e.TestRuntime) {
	t.Helper()

	err := runtime.UploadData(bytes.NewBufferString(`{"x": 1}`))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	resp := struct {
		Result int `json:"result"`
	}{}
	err = runtime.GetDataWithInputTyped("x", nil, &resp)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if resp.Result != 1 {
		t.Fatalf("Expected x to be 1, got %v", resp)
	}
}
