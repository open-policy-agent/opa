// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/test/e2e"
)

func TestRunServerBase(t *testing.T) {
	params := newTestRunParams()
	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil)
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

func TestRunServerWithDiagnosticAddr(t *testing.T) {
	params := newTestRunParams()
	params.rt.DiagnosticAddrs = &[]string{":0"}
	ctx, cancel := context.WithCancel(context.Background())

	rt, err := initRuntime(ctx, params, nil)
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

	_, err := initRuntime(context.Background(), params, nil)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	exp := "enable bundle mode (ie. --bundle) to verify bundle files or directories"
	if err.Error() != exp {
		t.Fatalf("expected error message %v but got %v", exp, err.Error())
	}
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

	rt, err := initRuntime(ctx, params, nil)
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

func newTestRunParams() runCmdParams {
	params := newRunParams()
	params.rt.GracefulShutdownPeriod = 1
	params.rt.Addrs = &[]string{":0"}
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
