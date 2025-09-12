// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	prometheus_sdk "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/discovery"
	"github.com/open-policy-agent/opa/v1/server/authorizer"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/tracing"

	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	testLog "github.com/open-policy-agent/opa/v1/logging/test"
	sdktest "github.com/open-policy-agent/opa/v1/sdk/test"
	"github.com/open-policy-agent/opa/v1/server"
	"github.com/open-policy-agent/opa/v1/storage"
	topdown_cache "github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestRuntimeProcessWatchEvents(t *testing.T) {
	tests := []struct {
		note     string
		asBundle bool
		readAst  bool
	}{
		{
			note: "no bundle, read raw data",
		},
		{
			note:    "no bundle, read ast",
			readAst: true,
		},
		{
			note:     "bundle, read raw data",
			asBundle: true,
		},
		{
			note:     "bundle, read ast",
			asBundle: true,
			readAst:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			testRuntimeProcessWatchEvents(t, tc.asBundle, tc.readAst)
		})
	}
}

func testRuntimeProcessWatchEvents(t *testing.T, asBundle bool, readAst bool) {
	t.Helper()

	ctx := t.Context()
	fs := map[string]string{
		"test/some/data.json": `{
			"hello": "world"
		}`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		// Prefix the directory intended to be watched with at least one
		// directory to avoid permission issues on the local host. Otherwise we
		// cannot always watch the tmp directory's parent.
		rootDir = filepath.Join(rootDir, "test")

		params := NewParams()
		params.Paths = []string{rootDir}
		params.BundleMode = asBundle
		params.ReadAstValuesFromStore = readAst

		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		txn := storage.NewTransactionOrDie(ctx, rt.Store)
		_, err = rt.Store.Read(ctx, txn, storage.MustParsePath("/system/version"))
		if err != nil {
			t.Fatal(err)
		}
		rt.Store.Abort(ctx, txn)

		var buf bytes.Buffer

		if err := rt.startWatcher(ctx, params.Paths, onReloadPrinter(&buf)); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		expected := map[string]any{
			"hello": "world-2",
		}

		if err := os.WriteFile(path.Join(rootDir, "some/data.json"), util.MustMarshalJSON(expected), 0o644); err != nil {
			panic(err)
		}

		t0 := time.Now()
		path := storage.MustParsePath("/some")

		// In practice, reload takes ~100us on development machine.
		maxWaitTime := time.Second * 1
		var val any

		for time.Since(t0) < maxWaitTime {
			time.Sleep(1 * time.Millisecond)
			txn := storage.NewTransactionOrDie(ctx, rt.Store)
			var err error
			val, err = rt.Store.Read(ctx, txn, path)
			if err != nil {
				panic(err)
			}

			// Ensure the update didn't overwrite the system version information
			_, err = rt.Store.Read(ctx, txn, storage.MustParsePath("/system/version"))
			if err != nil {
				t.Fatal(err)
			}

			rt.Store.Abort(ctx, txn)

			if readAst {
				exp, _ := ast.InterfaceToValue(expected)
				if ast.Compare(val, exp) == 0 {
					return // success
				}
			} else if reflect.DeepEqual(val, expected) {
				return // success
			}

		}

		t.Fatalf("Did not see expected change in %v, last value: %v, buf: %v", maxWaitTime, val, buf.String())
	})
}

func TestRuntimeProcessWatchEventPolicyError(t *testing.T) {
	testRuntimeProcessWatchEventPolicyError(t, false)
}

func TestRuntimeProcessWatchEventPolicyErrorWithBundle(t *testing.T) {
	testRuntimeProcessWatchEventPolicyError(t, true)
}

func testRuntimeProcessWatchEventPolicyError(t *testing.T, asBundle bool) {
	ctx := t.Context()

	fs := map[string]string{
		"test/x.rego": `package test

		default x = 1
		`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		// Prefix the directory intended to be watched with at least one
		// directory to avoid permission issues on the local host. Otherwise we
		// cannot always watch the tmp directory's parent.
		rootDir = filepath.Join(rootDir, "test")

		params := NewParams()
		params.Paths = []string{rootDir}
		params.BundleMode = asBundle

		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		err = storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
			return rt.Store.UpsertPolicy(ctx, txn, "out-of-band.rego", []byte(`package foo`))
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ch := make(chan error)

		testFunc := func(_ time.Duration, err error) {
			ch <- err
		}

		if err := rt.startWatcher(ctx, params.Paths, testFunc); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		newModule := []byte(`package test

		default x = 2`)

		if err := os.WriteFile(path.Join(rootDir, "y.rego"), newModule, 0o644); err != nil {
			t.Fatal(err)
		}

		// Wait for up to 1 second before considering test failed. On Linux we
		// observe multiple events on write (e.g., create -> write) which
		// triggers two errors instead of one, whereas on Darwin only a single
		// event (e.g., create) is sent. Same as below.
		maxWait := time.Second
		timer := time.NewTimer(maxWait)

		// Expect type error.
		func() {
			for {
				select {
				case result := <-ch:
					if errs, ok := result.(ast.Errors); ok {
						if errs[0].Code == ast.TypeErr {
							err = nil
							return
						}
					}
					err = result
				case <-timer.C:
					return
				}
			}
		}()

		if err != nil {
			t.Fatalf("Expected specific failure before %v. Last error: %v", maxWait, err)
		}

		if err := os.Remove(path.Join(rootDir, "x.rego")); err != nil {
			t.Fatal(err)
		}

		timer = time.NewTimer(maxWait)

		// Expect no error.
		func() {
			for {
				select {
				case result := <-ch:
					if result == nil {
						err = nil
						return
					}
					err = result
				case <-timer.C:
					return
				}
			}
		}()

		if err != nil {
			t.Fatalf("Expected result to succeed before %v. Last error: %v", maxWait, err)
		}
	})
}

func TestRuntimeReplWithBundleBuiltWithV1Compatibility(t *testing.T) {
	ctx := t.Context()

	test.WithTempFS(nil, func(rootDir string) {
		p := filepath.Join(rootDir, "bundle.tar.gz")

		mod := `package test
			p := 7 if 3 < 4
		`

		files := [][2]string{
			{"/.manifest", `{"revision": "foo", "rego_version": 1}`},
			{"/x.rego", mod},
		}

		buf := archive.MustWriteTarGz(files)
		bf, err := os.Create(p)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		_, err = bf.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		output := test.BlockingWriter{}

		params := NewParams()
		params.Output = &output
		params.Paths = []string{p}
		params.BundleMode = true

		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		go func() { _ = rt.StartREPL(ctx) }()

		if !test.Eventually(t, 5*time.Second, func() bool {
			return strings.Contains(output.String(), "Run 'help' to see a list of commands and check for updates.")
		}) {
			t.Fatal("Timed out waiting for REPL to start")
		}
		output.Reset()

		if err := rt.repl.OneShot(ctx, "data.test.p"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		actual := strings.TrimSpace(output.String())
		expected := "7"

		if actual != expected {
			t.Fatalf("expected data.test.p to be %v, got %v", expected, actual)
		}
	})
}

func TestRuntimeReplProcessWatchV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		policy       string
		expErrs      []string
		expOutput    string
	}{
		{
			note:         "v0, keywords not used",
			v0Compatible: true,
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
		},
		{
			note:         "v0, keywords not imported",
			v0Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:         "v0, keywords imported",
			v0Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v0, rego.v1 imported",
			v0Compatible: true,
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},

		{
			note:         "v1, keywords not used",
			v1Compatible: true,
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, keywords not imported",
			v1Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1, keywords imported",
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1, rego.v1 imported",
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},
	}

	fs := map[string]string{
		"test/data.json": `{"foo": "bar"}`,
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			test.WithTempFS(fs, func(rootDir string) {
				// Prefix the directory intended to be watched with at least one
				// directory to avoid permission issues on the local host. Otherwise, we
				// cannot always watch the tmp directory's parent.
				rootDir = filepath.Join(rootDir, "test")

				output := test.BlockingWriter{}

				params := NewParams()
				params.Output = &output
				params.Paths = []string{rootDir}
				params.Watch = true
				params.V0Compatible = tc.v0Compatible
				params.V1Compatible = tc.v1Compatible

				rt, err := NewRuntime(ctx, params)
				if err != nil {
					t.Fatal(err)
				}

				go func() { _ = rt.StartREPL(ctx) }()

				if !test.Eventually(t, 5*time.Second, func() bool {
					return strings.Contains(output.String(), "Run 'help' to see a list of commands and check for updates.")
				}) {
					t.Fatal("Timed out waiting for REPL to start")
				}
				output.Reset()

				// write new policy to disk, to trigger the watcher
				if err := os.WriteFile(path.Join(rootDir, "authz.rego"), []byte(tc.policy), 0o644); err != nil {
					t.Fatal(err)
				}

				if tc.expErrs != nil {
					if !test.Eventually(t, 5*time.Second, func() bool {
						for _, expErr := range tc.expErrs {
							if !strings.Contains(output.String(), expErr) {
								return false
							}
						}
						return true
					}) {
						t.Fatalf("Expected error(s):\n\n%v\n\ngot output:\n\n%s", tc.expErrs, output.String())
					}
				} else {
					if !test.Eventually(t, 5*time.Second, func() bool {
						return strings.Contains(output.String(), "# reloaded files")
					}) {
						t.Fatal("Timed out waiting for watcher")
					}
				}
			})
		})
	}
}

func TestRuntimeServerProcessWatchV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		policy       string
		expErrs      []string
		expOutput    string
	}{
		{
			note:         "v0, keywords not used",
			v0Compatible: true,
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
		},
		{
			note:         "v0, keywords not imported",
			v0Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:         "v0, keywords imported",
			v0Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v0, rego.v1 imported",
			v0Compatible: true,
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1, keywords not used",
			v1Compatible: true,
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, keywords not imported",
			v1Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1, keywords imported",
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1, rego.v1 imported",
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},
	}

	fs := map[string]string{
		"test/data.json": `{"foo": "bar"}`,
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			test.WithTempFS(fs, func(rootDir string) {
				// Prefix the directory intended to be watched with at least one
				// directory to avoid permission issues on the local host. Otherwise, we
				// cannot always watch the tmp directory's parent.
				rootDir = filepath.Join(rootDir, "test")

				testLogger := testLog.New()

				params := NewParams()
				params.Logger = testLogger
				params.Addrs = &[]string{"localhost:0"}
				params.AddrSetByUser = true
				params.Paths = []string{rootDir}
				params.Watch = true
				params.V0Compatible = tc.v0Compatible
				params.V1Compatible = tc.v1Compatible

				rt, err := NewRuntime(ctx, params)
				if err != nil {
					t.Fatal(err)
				}

				go rt.StartServer(ctx)

				if !test.Eventually(t, 5*time.Second, func() bool {
					return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
				}) {
					t.Fatal("Timed out waiting for server to start")
				}

				// write new policy to disk, to trigger the watcher
				if err := os.WriteFile(path.Join(rootDir, "authz.rego"), []byte(tc.policy), 0o644); err != nil {
					t.Fatal(err)
				}

				if tc.expErrs != nil {
					// wait for errors
					if !test.Eventually(t, 5*time.Second, func() bool {
						for _, expErr := range tc.expErrs {
							found := false
							for _, e := range testLogger.Entries() {
								if errs, ok := e.Fields["err"].(loader.Errors); ok {
									for _, err := range errs {
										found = strings.Contains(err.Error(), expErr) || found
									}
								}
							}
							if !found {
								return false
							}
						}
						return true
					}) {
						t.Fatalf("Timed out waiting for watcher. Expected errors:\n\n%v\n\ngot output:\n\n%v",
							tc.expErrs, testLogger.Entries())
					}
				} else {
					// wait for successful reload
					if !test.Eventually(t, 5*time.Second, func() bool {
						found := false
						for _, e := range testLogger.Entries() {
							found = strings.Contains(e.Message, "Processed file watch event.") || found
						}
						return found
					}) {
						t.Fatal("Timed out waiting for watcher")
					}
				}
			})
		})
	}
}

func TestCheckOPAUpdateBadURL(t *testing.T) {
	testCheckOPAUpdate(t, "http://foo:8112", nil)
}

func TestCheckOPAUpdateWithNewUpdate(t *testing.T) {
	tag := "v100.0.0"
	downloadLink := createDownloadLink(tag)

	resp := &report.GHResponse{
		TagName:      tag,
		Download:     downloadLink,
		ReleaseNotes: "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(resp, http.StatusOK)
	defer teardown()

	exp := &report.DataResponse{Latest: report.ReleaseDetails{
		Download:      downloadLink,
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: tag,
	}}

	testCheckOPAUpdate(t, baseURL, exp)
}

func createDownloadLink(tag string) string {
	// to support testing on all supported platforms
	downloadLink := fmt.Sprintf("https://openpolicyagent.org/downloads/%s/opa_%v_%v",
		tag, runtime.GOOS, runtime.GOARCH)

	if runtime.GOARCH == "arm64" {
		downloadLink = fmt.Sprintf("%v_static", downloadLink)
	}

	if strings.HasPrefix(runtime.GOOS, "win") {
		downloadLink = fmt.Sprintf("%v.exe", downloadLink)
	}

	return downloadLink
}

func TestCheckOPAUpdateLoopBadURL(t *testing.T) {
	testCheckOPAUpdateLoop(t, "http://foo:8112", "Unable to send OPA version report")
}

func TestCheckOPAUpdateLoopNoUpdate(t *testing.T) {
	srvResp := &report.GHResponse{
		TagName: "v1.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(srvResp, http.StatusOK)
	defer teardown()

	testCheckOPAUpdateLoop(t, baseURL, "OPA is up to date.")
}

func TestCheckOPAUpdateLoopLaterRequests(t *testing.T) {
	resp := &report.GHResponse{
		TagName: "v1.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(resp, http.StatusOK)
	defer teardown()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", baseURL)

	ctx := t.Context()

	logger := logging.New()
	stdout := bytes.NewBuffer(nil)
	logger.SetOutput(stdout)
	logger.SetLevel(logging.Debug)

	rt := getTestRuntime(ctx, t, logger)

	done := make(chan struct{})
	go func() {
		initial := time.Millisecond
		later := 100 * time.Millisecond
		rt.checkOPAUpdateLoopDurations(ctx, done, initial, later)
	}()
	time.Sleep(150 * time.Millisecond)
	done <- struct{}{}

	// NOTE(sr): We'll assert that within 200ms, we have gotten less than
	// 10 requests. This is a little less strict than we could be, to not
	// make this test too sensitive to timing and noise test environments.
	// However, it's strict enbough: If the "later" duration wasn't
	// respected, we'd see a lot more requests.
	needle := "OPA is up to date."
	act := strings.Count(stdout.String(), needle)
	exp := 7
	if act > exp+1 || act < exp {
		t.Fatalf("Expected output to contain: %q >= 7 times, less than 8, got %d", needle, act)
	}
}

func TestCheckOPAUpdateLoopWithNewUpdate(t *testing.T) {
	tag := "v100.0.0"
	downloadLink := createDownloadLink(tag)

	resp := &report.GHResponse{
		TagName:      tag,
		Download:     downloadLink,
		ReleaseNotes: "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
	}

	// test server
	baseURL, teardown := getTestServer(resp, http.StatusOK)
	defer teardown()

	testCheckOPAUpdateLoop(t, baseURL, "OPA is out of date.")
}

func TestRuntimeWithAuthzSchemaVerification(t *testing.T) {
	ctx := t.Context()

	fs := map[string]string{
		"test/authz.rego": `package system.authz
		import rego.v1

		default allow := false

		allow if {
          input.identity = "foo"
		}`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		rootDir = filepath.Join(rootDir, "test")

		params := NewParams()
		params.Paths = []string{rootDir}
		params.Authorization = server.AuthorizationBasic

		_, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		badModule := []byte(`package system.authz
		import rego.v1

		default allow := false

		allow if {
           input.identty = "foo"
		}`)

		if err := os.WriteFile(path.Join(rootDir, "authz.rego"), badModule, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err = NewRuntime(ctx, params)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		if !strings.Contains(err.Error(), "undefined ref: input.identty") {
			t.Errorf("Expected error \"%v\" not found", "undefined ref: input.identty")
		}

		// no verification checks
		params.Authorization = server.AuthorizationOff
		_, err = NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestRuntimeWithAuthzSchemaVerificationTransitive(t *testing.T) {
	ctx := t.Context()

	fs := map[string]string{
		"test/authz.rego": `package system.authz
		import rego.v1

		default allow := false

        is_secret :=  input.identty == "secret"

        # even though "is_secret" is called via 2 paths, there should be only one resulting error
        # 1-step dependency
        allow if {
          is_secret
        }

        # 2-step dependency
        allow if {
          allow2
        }

        allow2 if {
          is_secret
        }`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		rootDir = filepath.Join(rootDir, "test")

		params := NewParams()
		params.Paths = []string{rootDir}
		params.Authorization = server.AuthorizationBasic

		_, err := NewRuntime(ctx, params)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		if !strings.Contains(err.Error(), "undefined ref: input.identty") {
			t.Errorf("Expected error \"%v\" not found", "undefined ref: input.identty")
		}
	})
}

func TestCheckAuthIneffective(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
	defer cancel() // NOTE(sr): The timeout will have been reached by the time `done` is closed.

	params := NewParams()
	params.Authentication = server.AuthenticationToken
	params.Authorization = server.AuthorizationOff

	logger := logging.New()
	stdout := bytes.NewBuffer(nil)
	logger.SetOutput(stdout)

	params.Logger = logger
	params.Addrs = &[]string{"localhost:0"}
	params.GracefulShutdownPeriod = 1
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	done := make(chan struct{})
	go func() {
		rt.StartServer(ctx)
		close(done)
	}()
	<-done

	expected := "Token authentication enabled without authorization. Authentication will be ineffective. See https://www.openpolicyagent.org/docs/latest/security/#authentication-and-authorization for more information."
	if !strings.Contains(stdout.String(), expected) {
		t.Fatalf("Expected output to contain: \"%v\" but got \"%v\"", expected, stdout.String())
	}
}

func TestServerInitialized(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
	defer cancel() // NOTE(sr): The timeout will have been reached by the time `done` is closed.
	var output bytes.Buffer

	params := NewParams()
	params.Output = &output
	params.Addrs = &[]string{"localhost:0"}
	params.GracefulShutdownPeriod = 1
	params.Logger = logging.NewNoOpLogger()

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	initChannel := rt.Manager.ServerInitializedChannel()
	done := make(chan struct{})
	go func() {
		rt.StartServer(ctx)
		close(done)
	}()
	<-done
	select {
	case <-initChannel:
		return
	default:
		t.Fatal("expected ServerInitializedChannel to be closed")
	}
}

func TestServerInitializedWithRegoV1(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		files        map[string]string
		expErr       string
	}{
		{
			note:         "Rego v0, keywords not imported",
			v0Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				p if {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error: var cannot be used for rule name",
		},
		{
			note:         "Rego v0, rego.v1 imported",
			v0Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				import rego.v1
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v0, future.keywords imported",
			v0Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				import future.keywords.if
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v0, no keywords used",
			v0Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				p {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v1, keywords not imported",
			v1Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v1, rego.v1 imported",
			v1Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				import rego.v1
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v1, future.keywords imported",
			v1Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				import future.keywords.if
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note:         "Rego v1, no keywords used",
			v1Compatible: true,
			files: map[string]string{
				"policy.rego": `package test
				p {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error: `if` keyword is required before rule body",
		},
	}

	bundle := []bool{false, true}

	for _, tc := range tests {
		for _, b := range bundle {
			t.Run(fmt.Sprintf("%s; bundle=%v", tc.note, b), func(t *testing.T) {
				test.WithTempFS(tc.files, func(root string) {
					ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
					defer cancel()
					var output bytes.Buffer

					params := NewParams()
					params.Output = &output
					params.Paths = []string{root}
					params.BundleMode = b
					params.Addrs = &[]string{"localhost:0"}
					params.GracefulShutdownPeriod = 1
					params.Logger = logging.NewNoOpLogger()
					params.V0Compatible = tc.v0Compatible
					params.V1Compatible = tc.v1Compatible

					rt, err := NewRuntime(ctx, params)

					if tc.expErr != "" {
						if err == nil {
							t.Fatal("Expected error but got nil")
						}
						if !strings.Contains(err.Error(), tc.expErr) {
							t.Fatalf("Expected error:\n\n%v\n\ngot:\n\n%v", tc.expErr, err.Error())
						}
					} else {
						if err != nil {
							t.Fatalf("Unexpected error %v", err)
						}

						initChannel := rt.Manager.ServerInitializedChannel()
						done := make(chan struct{})
						go func() {
							rt.StartServer(ctx)
							close(done)
						}()
						<-done
						select {
						case <-initChannel:
							return
						default:
							t.Fatal("expected ServerInitializedChannel to be closed")
						}
					}
				})
			})
		}
	}
}

func TestServerInitializedWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note   string
		files  map[string]string
		expErr string
	}{
		{
			note: "v0.x bundle, keywords not imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
				p if {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error: var cannot be used for rule name",
		},
		{
			note: "v0.x bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
				import rego.v1
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v0.x bundle, future.keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
				import future.keywords.if
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v0.x bundle, no keywords used",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
				p {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override",
			files: map[string]string{
				".manifest": `{
					"rego_version": 0,
					"file_rego_versions": {
						"/policy2.rego": 1
					}
				}`,
				"policy1.rego": `package test
				p[1] {
					input.x == 1
				}
				`,
				"policy2.rego": `package test
				q contains 2 if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
					"rego_version": 0,
					"file_rego_versions": {
						"/bar/*.rego": 1
					}
				}`,
				"foo/policy1.rego": `package test
				p[1] {
					input.x == 1
				}
				`,
				"bar/policy2.rego": `package test
				q contains 2 if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override, incompatible",
			files: map[string]string{
				".manifest": `{
					"rego_version": 0,
					"file_rego_versions": {
						"/policy2.rego": 1
					}
				}`,
				"policy1.rego": `package test
				p[1] {
					input.x == 1
				}
				`,
				"policy2.rego": `package test
				q[2] {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error",
		},

		{
			note: "v1.0 bundle, keywords not imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v1.0 bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
				import rego.v1
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v1.0 bundle, future.keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
				import future.keywords.if
				p if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v1.0 bundle, no keywords used",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
				p {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error: `if` keyword is required before rule body",
		},
		{
			note: "v1 bundle, v0 per-file override",
			files: map[string]string{
				".manifest": `{
					"rego_version": 1,
					"file_rego_versions": {
						"/policy1.rego": 0
					}
				}`,
				"policy1.rego": `package test
				p[1] {
					input.x == 1
				}
				`,
				"policy2.rego": `package test
				q contains 2 if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
					"rego_version": 1,
					"file_rego_versions": {
						"/foo/*.rego": 0
					}
				}`,
				"foo/policy1.rego": `package test
				p[1] {
					input.x == 1
				}
				`,
				"bar/policy2.rego": `package test
				q contains 2 if {
					input.x == 1
				}
				`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override, incompatible",
			files: map[string]string{
				".manifest": `{
					"rego_version": 1,
					"file_rego_versions": {
						"/policy1.rego": 0
					}
				}`,
				"policy1.rego": `package test
				p contains 1 if {
					input.x == 1
				}
				`,
				"policy2.rego": `package test
				q contains 2 if {
					input.x == 1
				}
				`,
			},
			expErr: "rego_parse_error",
		},
	}

	bundleTypeCases := []struct {
		note string
		tar  bool
	}{
		{
			"bundle dir", false,
		},
		{
			"bundle tar", true,
		},
	}

	for _, bundleType := range bundleTypeCases {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s, %s", bundleType.note, tc.note), func(t *testing.T) {
				files := map[string]string{}
				if bundleType.tar {
					files["bundle.tar.gz"] = ""
				} else {
					maps.Copy(files, tc.files)
				}

				test.WithTempFS(files, func(root string) {
					p := root
					if bundleType.tar {
						p = filepath.Join(root, "bundle.tar.gz")
						files := make([][2]string, 0, len(tc.files))
						for k, v := range tc.files {
							files = append(files, [2]string{k, v})
						}
						buf := archive.MustWriteTarGz(files)
						bf, err := os.Create(p)
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
						_, err = bf.Write(buf.Bytes())
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
					}

					ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
					defer cancel()
					var output bytes.Buffer

					params := NewParams()
					params.Output = &output
					params.Paths = []string{p}
					params.BundleMode = true
					params.Addrs = &[]string{"localhost:0"}
					params.GracefulShutdownPeriod = 1
					params.Logger = logging.NewNoOpLogger()

					rt, err := NewRuntime(ctx, params)

					if tc.expErr != "" {
						if err == nil {
							t.Fatal("Expected error but got nil")
						}
						if !strings.Contains(err.Error(), tc.expErr) {
							t.Fatalf("Expected error:\n\n%v\n\ngot:\n\n%v", tc.expErr, err.Error())
						}
					} else {
						if err != nil {
							t.Fatalf("Unexpected error %v", err)
						}

						initChannel := rt.Manager.ServerInitializedChannel()
						done := make(chan struct{})
						go func() {
							rt.StartServer(ctx)
							close(done)
						}()
						<-done
						select {
						case <-initChannel:
							return
						default:
							t.Fatal("expected ServerInitializedChannel to be closed")
						}
					}
				})
			})
		}
	}
}

func TestGracefulTracerShutdown(t *testing.T) {
	fs := map[string]string{
		"/config.yaml": `{"distributed_tracing": {"type": "grpc"}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
		defer cancel() // NOTE(sr): The timeout will have been reached by the time `done` is closed.

		logger := testLog.New()

		params := NewParams()
		params.ConfigFile = filepath.Join(testDirRoot, "config.yaml")
		params.Addrs = &[]string{"localhost:0"}
		params.GracefulShutdownPeriod = 1
		params.Logger = logger

		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		if rt.traceExporter == nil {
			t.Fatal("traceExporter should not be nil")
		}

		done := make(chan struct{})
		go func() {
			rt.StartServer(ctx)
			close(done)
		}()
		<-done

		expected := "Failed to shutdown OpenTelemetry trace exporter gracefully."
		if strings.Contains(logger.Entries()[0].Message, expected) {
			t.Fatalf("Expected no output containing: \"%v\"", expected)
		}
	})
}

func TestUrlPathToConfigOverride(t *testing.T) {
	params := NewParams()
	params.Paths = []string{"https://www.example.com/bundles/bundle.tar.gz"}
	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	cfg := rt.Manager.GetConfig()

	var servicesConfig map[string]map[string]any
	if len(cfg.Services) > 0 {
		if err := json.Unmarshal([]byte(cfg.Services), &servicesConfig); err != nil {
			t.Fatal(err)
		}
	}

	cliService, ok := servicesConfig["cli1"]
	if !ok {
		t.Fatal("expected service configuration for 'cli1' service")
	}

	if cliService["url"] != "https://www.example.com" {
		t.Error("expected cli1 service url value: 'https://www.example.com'")
	}

	bundleConf := make(map[string]map[string]any)
	if len(cfg.Bundles) > 0 {
		var bundleConfRaw map[string]any
		if err := json.Unmarshal([]byte(cfg.Bundles), &bundleConfRaw); err != nil {
			t.Fatal(err)
		}
		for k, v := range bundleConfRaw {
			if bundleMap, ok := v.(map[string]any); ok {
				bundleConf[k] = bundleMap
			}
		}
	}

	cliBundle, ok := bundleConf["cli1"]
	if !ok {
		t.Fatal("excpected bundle configuration for 'cli1' bundle")
	}

	if cliBundle["service"] != "cli1" {
		t.Error("expected cli1 bundle service value: 'cli1'")
	}

	if cliBundle["resource"] != "/bundles/bundle.tar.gz" {
		t.Error("expected cli1 bundle resource value: 'bundles/bundle.tar.gz'")
	}

	if cliBundle["persist"] != true {
		t.Error("expected cli1 bundle persist value: true")
	}
}

func getTestServer(update any, statusCode int) (string, func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/repos/open-policy-agent/opa/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		bs, _ := json.Marshal(update)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bs) // ignore error
	})
	return ts.URL, ts.Close
}

func testCheckOPAUpdate(t *testing.T, url string, expected *report.DataResponse) {
	t.Helper()
	t.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	ctx := t.Context()
	rt := getTestRuntime(ctx, t, logging.NewNoOpLogger())
	result := rt.checkOPAUpdate(ctx)

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected output:\"%v\" but got: \"%v\"", expected, result)
	}
}

func testCheckOPAUpdateLoop(t *testing.T, url, expected string) {
	t.Helper()
	t.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	ctx := t.Context()

	logger := logging.New()
	stdout := bytes.NewBuffer(nil)
	logger.SetOutput(stdout)
	logger.SetLevel(logging.Debug)

	rt := getTestRuntime(ctx, t, logger)

	done := make(chan struct{})
	go func() {
		initial := time.Millisecond
		later := initial
		rt.checkOPAUpdateLoopDurations(ctx, done, initial, later)
	}()
	time.Sleep(2 * time.Millisecond)
	done <- struct{}{}

	if !strings.Contains(stdout.String(), expected) {
		t.Fatalf("Expected output to contain: \"%v\" but got \"%v\"", expected, stdout.String())
	}
}

func getTestRuntime(ctx context.Context, t *testing.T, logger logging.Logger) *Runtime {
	t.Helper()

	params := NewParams()
	params.EnableVersionCheck = true
	params.Logger = logger
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	return rt
}

func TestAddrWarningMessage(t *testing.T) {
	testCases := []struct {
		name          string
		addrSetByUser bool
		containsMsg   bool
		v0Compatible  bool
	}{
		{"NoWarningMessage", true, false, false},
		{"WarningMessage", false, true, true},
		{"V0Compatible", false, true, true},
		{"V0InCompatible", false, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
			defer cancel()

			params := NewParams()

			logger := testLog.New()
			logLevel := logging.Info

			params.Logger = logger
			params.Addrs = &[]string{"localhost:8181"}
			params.AddrSetByUser = tc.addrSetByUser
			params.GracefulShutdownPeriod = 1
			params.V0Compatible = tc.v0Compatible
			rt, err := NewRuntime(ctx, params)
			if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}

			done := make(chan struct{})
			go func() {
				rt.StartServer(ctx)
				close(done)
			}()
			<-done

			warning := " OPA is running on a public (0.0.0.0) network interface. Unless you intend to expose OPA outside of the host, binding to the localhost interface (--addr localhost:8181) is recommended. See https://www.openpolicyagent.org/docs/latest/security/#interface-binding"
			containsWarning := strings.Contains(logger.Entries()[0].Message, warning)

			if containsWarning != tc.containsMsg {
				t.Fatal("Mismatch between OPA server displaying the interface warning message and user setting the server address")
			}

			if logger.GetLevel() != logLevel {
				t.Fatalf("Expected log level to be: \"%v\" but got \"%v\"", logLevel, logger.GetLevel())
			}
		})
	}
}

func TestRuntimeWithExplicitMetricConfiguration(t *testing.T) {
	fs := map[string]string{
		"/config.yaml": `{"server": {"metrics": {"prom": {"http_request_duration_seconds": {"buckets": [0.1, 0.2, 0.3]}}}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {
		params := NewParams()
		params.ConfigFile = filepath.Join(testDirRoot, "config.yaml")

		_, err := NewRuntime(t.Context(), params)
		if err != nil {
			t.Fatal(err.Error())
		}
	})
}

func TestRuntimeWithExplicitBadMetricConfiguration(t *testing.T) {
	fs := map[string]string{
		"/config.yaml": `{"server": {"metrics": {"prom": {"http_request_duration_seconds": {"buckets": "would-not-work"}}}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {
		params := NewParams()
		params.ConfigFile = filepath.Join(testDirRoot, "config.yaml")

		_, err := NewRuntime(t.Context(), params)
		if err == nil {
			t.Fatalf("Expected error to be thrown on malformed metrics config")
		}

		if !strings.HasPrefix(err.Error(), "server metrics configuration parse error") {
			t.Fatalf("Expected specific error to be thrown on malformed metrics config")
		}
	})
}

func TestExtraDiscoveryOpts(t *testing.T) {
	ctx := t.Context()
	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/discovery.tar.gz", map[string]string{
			"main.rego": `
package config

plugins.foobar := {}
`,
		}),
	)
	defer server.Stop()

	config := fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"discovery": {
			"decision": "config",
			"resource": "/bundles/discovery.tar.gz"
		}
	}`, server.URL())
	cfg := filepath.Join(t.TempDir(), "opa.json")
	if err := os.WriteFile(cfg, []byte(config), 0x755); err != nil {
		t.Fatalf("write config %s: %v", cfg, err)
	}

	params := NewParams()
	params.ConfigFile = cfg
	params.Output = io.Discard
	params.Addrs = &[]string{"localhost:0"}
	params.GracefulShutdownPeriod = 1
	testLogger := testLog.New()
	params.Logger = testLogger
	params.ExtraDiscoveryOpts = []func(*discovery.Discovery){
		discovery.Factories(map[string]plugins.Factory{"foobar": &factory{}}),
	}

	// To check that the ExtraDiscoveryOpts have had an effect, we'll start the
	// runtime and trigger a discovery update. The config it'll receive has a
	// plugin called "foobar", which it'll only know if the factories have been
	// set properly.
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	disco := discovery.Lookup(rt.Manager)
	if err := disco.Trigger(ctx); err != nil {
		t.Errorf("trigger discovery: %v", err)
	}

	if !test.Eventually(t, 5*time.Second, func() bool {
		found := false
		for _, e := range testLogger.Entries() {
			t.Log(e.Message)
			if e.Message == "Discovery update processed successfully." {
				found = true
				break
			}
		}
		return found
	}) {
		t.Error("discovery failed, check logs")
	}
}

type factory struct{}

func (f *factory) New(m *plugins.Manager, _ any) plugins.Plugin {
	m.ExtraRoute("GET /v1/flusher", "v1/flusher", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hey\n"))
		w.(http.Flusher).Flush()
		time.Sleep(1 * time.Second)
		_, _ = w.Write([]byte("there\n"))
	})
	return f
}

func (*factory) Validate(*plugins.Manager, []byte) (any, error) {
	return nil, nil
}

func (*factory) Start(context.Context) error {
	return nil
}

func (*factory) Stop(context.Context) {
}

func (*factory) Reconfigure(context.Context, any) {
}

// TestCustomHandlerFlusher ensures that a handler defined through a plugin
// can call Flush(), and that all the middlewares in between work in the
// expected way -- passing the Flush() along. It needs to be tested through
// the runtime package to cover all the layers of middlwares typically used
// in OPA run as server.
func TestCustomHandlerFlusher(t *testing.T) {
	fact := &factory{}
	ctx := t.Context()
	spanExporter := tracetest.NewInMemoryExporter()
	options := tracing.NewOptions(
		otelhttp.WithTracerProvider(trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(spanExporter)))),
	)

	RegisterPlugin("test_custom_handler_flusher", fact)

	config := `plugins:
  test_custom_handler_flusher: {}
`
	cfg := filepath.Join(t.TempDir(), "opa.yml")
	if err := os.WriteFile(cfg, []byte(config), 0x755); err != nil {
		t.Fatalf("write config %s: %v", cfg, err)
	}

	for _, tc := range []struct {
		note string
		otel tracing.Options
	}{
		{
			note: "with otel",
			otel: options,
		},
		{
			note: "without otel",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			testLogger := testLog.New()
			params := NewParams()
			params.DistributedTracingOpts = tc.otel
			params.Logger = testLogger
			params.Addrs = &[]string{"localhost:0"}
			params.ConfigFile = cfg

			rt, err := NewRuntime(ctx, params)
			if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}
			go rt.StartServer(ctx)
			if !test.Eventually(t, 5*time.Second, func() bool {
				return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
			}) {
				t.Fatal("Timed out waiting for server to start")
			}
			host := rt.Addrs()[0]
			r, err := http.NewRequest(http.MethodGet, "http://"+host+"/v1/flusher", nil)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			start := time.Now()
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal("expected no error, got", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status %d (want 200)", resp.StatusCode)
			}

			defer resp.Body.Close()

			reader := bufio.NewReader(resp.Body)
			{
				line, err := reader.ReadString('\n')
				if err != nil && !strings.Contains(err.Error(), "timeout") {
					t.Fatalf("unexpected error reading: %v", err)
				}
				flushed := "hey\n"
				if line != flushed {
					t.Errorf("expected flushed line %q, got %q", flushed, line)
				}
				if dur := time.Since(start); dur > 100*time.Millisecond { // we're very gracious here, giving it 100ms leeway
					t.Error("first line came too late (flush hasn't happened)", dur.String())
				}
			}
			{
				line, err := reader.ReadString('\n')
				if err != nil && !strings.Contains(err.Error(), "timeout") {
					t.Fatalf("unexpected error reading: %v", err)
				}
				rest := "there\n"
				if line != rest {
					t.Errorf("expected flushed line %q, got %q", rest, line)
				}
			}

			t.Log(time.Since(start).String())
			for _, e := range testLogger.Entries() {
				t.Log(e.Message)
			}
		})
	}
}

type configHook struct {
	some string
}

func (ch *configHook) OnConfig(_ context.Context, c *config.Config) (*config.Config, error) {
	ch.some = string(c.Extra["some"])
	return c, nil
}

func TestConfigHookAndNonReplacedEnvVars(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
	defer cancel() // NOTE(sr): The timeout will have been reached by the time `done` is closed.
	testLogger := testLog.New()

	hk := configHook{}

	cf := filepath.Join(t.TempDir(), "opa.yaml")
	if err := os.WriteFile(cf, []byte("some: ${thing}\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	params := NewParams()
	params.Logger = testLogger
	params.Addrs = &[]string{"localhost:0"}
	params.Hooks = hooks.New(&hk)
	params.ConfigFile = cf

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	initChannel := rt.Manager.ServerInitializedChannel()
	done := make(chan struct{})
	go func() {
		rt.StartServer(ctx)
		close(done)
	}()
	<-done
	select {
	case <-initChannel:
		return
	default:
		t.Fatal("expected ServerInitializedChannel to be closed")
	}

	if act, exp := hk.some, "${thing}"; exp != act {
		t.Errorf("Expected %q, got %q", exp, act)
	}

	for _, e := range testLogger.Entries() {
		t.Log(e.Message)
	}
}

type iqcHook struct {
	c topdown_cache.InterQueryCache
}

func (j *iqcHook) OnInterQueryCache(_ context.Context, c topdown_cache.InterQueryCache) error {
	j.c = c
	return nil
}

type iqvcHook struct {
	c topdown_cache.InterQueryValueCache
}

func (j *iqvcHook) OnInterQueryValueCache(_ context.Context, c topdown_cache.InterQueryValueCache) error {
	j.c = c
	return nil
}

func TestCacheHooksOnServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
	defer cancel() // NOTE(sr): The timeout will have been reached by the time `done` is closed.
	testLogger := testLog.New()

	h1 := iqcHook{}
	h2 := iqvcHook{}

	params := NewParams()
	params.Logger = testLogger
	params.Addrs = &[]string{"localhost:0"}
	params.Hooks = hooks.New(&h1, &h2)

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	initChannel := rt.Manager.ServerInitializedChannel()
	done := make(chan struct{})
	go func() {
		rt.StartServer(ctx)
		close(done)
	}()
	<-done
	select {
	case <-initChannel:
		return
	default:
		t.Fatal("expected ServerInitializedChannel to be closed")
	}
	if h1.c == nil {
		t.Errorf("expected non-nil inter-query cache")
	}
	if h2.c == nil {
		t.Errorf("expected non-nil inter-query value cache")
	}

	for _, e := range testLogger.Entries() {
		t.Log(e.Message)
	}
}

type fakeStore struct {
	storage.Store
}

func (f *fakeStore) Read(ctx context.Context, txn storage.Transaction, p storage.Path) (any, error) {
	if slices.Contains(p, "foo") {
		return map[string]any{"fake": p}, nil
	}
	return f.Store.Read(ctx, txn, p)
}

func TestCustomStoreBuilder(t *testing.T) {
	ctx := t.Context()
	testLogger := testLog.New()
	params := NewParams()
	params.Logger = testLogger
	params.Addrs = &[]string{"localhost:0"}
	params.StoreBuilder = func(_ context.Context, logger logging.Logger, registerer prometheus_sdk.Registerer, config []byte, id string) (storage.Store, error) {
		switch {
		case logger == nil:
			t.Fatal("logger empty")
		case registerer == nil:
			t.Fatal("registerer empty")
		case config == nil:
			t.Fatal("config empty")
		case id == "":
			t.Fatal("id empty")
		}
		return &fakeStore{inmem.New()}, nil
	}

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	go rt.StartServer(ctx)
	if !test.Eventually(t, 5*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("Timed out waiting for server to start")
	}
	host := rt.Addrs()[0]
	r, err := http.NewRequest(http.MethodGet, "http://"+host+"/v1/data/foo/bar", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal("expected no error, got", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200)", resp.StatusCode)
	}

	defer resp.Body.Close()
	var payload struct {
		Result map[string]any
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(map[string]any{"fake": []any{"foo", "bar"}}, payload.Result) {
		t.Errorf("unexpected result: %v", payload.Result)
	}
}

func TestExtraMiddleware(t *testing.T) {
	ctx := t.Context()
	testLogger := testLog.New()
	params := NewParams()
	params.Logger = testLogger
	params.Addrs = &[]string{"localhost:0"}

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	rt.Manager.ExtraMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "foo", "bar") //nolint:staticcheck,SA1029 // this is a simple example
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	rt.Manager.ExtraRoute("GET /exp/foo", "exp/foo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Context().Value("foo"))
	}))
	go rt.StartServer(ctx)
	if !test.Eventually(t, 5*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("Timed out waiting for server to start")
	}
	host := rt.Addrs()[0]
	r, err := http.NewRequest(http.MethodGet, "http://"+host+"/exp/foo", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal("expected no error, got", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200)", resp.StatusCode)
	}

	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if act, exp := string(buf), "bar"; !reflect.DeepEqual(act, exp) {
		t.Errorf("got %v (%[1]T), want %v (%[2]T)", act, exp)
	}
}

func TestExtraAuthorizerRoutes(t *testing.T) {
	ctx := t.Context()
	testLogger := testLog.New()
	params := NewParams()
	params.Logger = testLogger
	params.Addrs = &[]string{"localhost:0"}
	params.Authorization = server.AuthorizationBasic
	authzPolicy := []byte(`
package system.authz

default allow := false # Reject requests by default.

# Authorizer will deny request if it cannot see the parsed request body.
allow if {
	input.method == "POST"
	input.path == ["exp", "foo"]
	input.body.example == "A"
}`)

	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	// Add a simple authz policy for POST /exp/foo.
	err = storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
		return rt.Store.UpsertPolicy(ctx, txn, "authz.rego", authzPolicy)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Use a basic "echo" handler here that will reflect back the request body.
	// If the authorizer blocks the request body from being parsed, we won't see it here on the request context.
	rt.Manager.ExtraRoute("POST /exp/foo", "exp/foo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := authorizer.GetBodyOnContext(r.Context()); ok {
			bs := string(util.MustMarshalJSON(body))
			fmt.Fprint(w, bs)
		} else {
			t.Fatal("No request body found on request context.")
		}
	}))
	// Add POST /exp/foo to authorizer's list of routes that should have request bodies.
	rt.Manager.ExtraAuthorizerRoute(func(method string, path []any) bool {
		s0 := path[0].(string)
		s1 := path[1].(string)
		return method == "POST" && s0 == "exp" && s1 == "foo"
	})
	go rt.StartServer(ctx)
	if !test.Eventually(t, 5*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("Timed out waiting for server to start")
	}
	host := rt.Addrs()[0]
	r, err := http.NewRequest(http.MethodPost, "http://"+host+"/exp/foo", strings.NewReader(`{"example": "A"}`))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal("expected no error, got", err)
	}
	// If we get a 401 Not Authorized here, it means the authz policy could not
	// validate the contents of the parsed request body for some reason.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d (want 200)", resp.StatusCode)
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if act, exp := string(buf), `{"example":"A"}`; !reflect.DeepEqual(act, exp) {
		t.Errorf("got %v (%[1]T), want %v (%[2]T)", act, exp)
	}
}
