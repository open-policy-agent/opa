// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/loader"

	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/logging"
	testLog "github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/server"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestRuntimeProcessWatchEvents(t *testing.T) {
	testRuntimeProcessWatchEvents(t, false)
}

func TestRuntimeProcessWatchEventsWithBundle(t *testing.T) {
	testRuntimeProcessWatchEvents(t, true)
}

func testRuntimeProcessWatchEvents(t *testing.T, asBundle bool) {
	t.Helper()

	ctx := context.Background()
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

		expected := map[string]interface{}{
			"hello": "world-2",
		}

		if err := os.WriteFile(path.Join(rootDir, "some/data.json"), util.MustMarshalJSON(expected), 0644); err != nil {
			panic(err)
		}

		t0 := time.Now()
		path := storage.MustParsePath("/some")

		// In practice, reload takes ~100us on development machine.
		maxWaitTime := time.Second * 1
		var val interface{}

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
			if reflect.DeepEqual(val, expected) {
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
	ctx := context.Background()

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

		testFunc := func(d time.Duration, err error) {
			ch <- err
		}

		if err := rt.startWatcher(ctx, params.Paths, testFunc); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		newModule := []byte(`package test

		default x = 2`)

		if err := os.WriteFile(path.Join(rootDir, "y.rego"), newModule, 0644); err != nil {
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

func TestRuntimeReplProcessWatchV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		policy       string
		expErrs      []string
		expOutput    string
	}{
		{
			note: "v0.x, keywords not used",
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
		},
		{
			note: "v0.x, keywords not imported",
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
			note: "v0.x, keywords imported",
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note: "v0.x, rego.v1 imported",
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},

		{
			note:         "v1.0, keywords not used",
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
			note:         "v1.0, keywords not imported",
			v1Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1.0, rego.v1 imported",
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
			ctx, cancel := context.WithCancel(context.Background())
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
				params.V1Compatible = tc.v1Compatible

				rt, err := NewRuntime(ctx, params)
				if err != nil {
					t.Fatal(err)
				}

				go rt.StartREPL(ctx)

				if !test.Eventually(t, 5*time.Second, func() bool {
					return strings.Contains(output.String(), "Run 'help' to see a list of commands and check for updates.")
				}) {
					t.Fatal("Timed out waiting for REPL to start")
				}
				output.Reset()

				// write new policy to disk, to trigger the watcher
				if err := os.WriteFile(path.Join(rootDir, "authz.rego"), []byte(tc.policy), 0644); err != nil {
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
		v1Compatible bool
		policy       string
		expErrs      []string
		expOutput    string
	}{
		{
			note: "v0.x, keywords not used",
			policy: `package test
p[1] {
	data.foo == "bar"
}`,
		},
		{
			note: "v0.x, keywords not imported",
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
			note: "v0.x, keywords imported",
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note: "v0.x, rego.v1 imported",
			policy: `package test
import rego.v1
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1.0, keywords not used",
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
			note:         "v1.0, keywords not imported",
			v1Compatible: true,
			policy: `package test
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains 1 if {
	data.foo == "bar"
}`,
		},
		{
			note:         "v1.0, rego.v1 imported",
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
			ctx, cancel := context.WithCancel(context.Background())
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
				params.V1Compatible = tc.v1Compatible

				rt, err := NewRuntime(ctx, params)
				if err != nil {
					t.Fatal(err)
				}

				go rt.StartServer(ctx)

				if !test.Eventually(t, 5*time.Second, func() bool {
					found := false
					for _, e := range testLogger.Entries() {
						found = strings.Contains(e.Message, "Server initialized.") || found
					}
					return found
				}) {
					t.Fatal("Timed out waiting for server to start")
				}

				// write new policy to disk, to trigger the watcher
				if err := os.WriteFile(path.Join(rootDir, "authz.rego"), []byte(tc.policy), 0644); err != nil {
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
	exp := &report.DataResponse{Latest: report.ReleaseDetails{
		Download:      "https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64",
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: "v100.0.0",
	}}

	// test server
	baseURL, teardown := getTestServer(exp, http.StatusOK)
	defer teardown()

	testCheckOPAUpdate(t, baseURL, exp)
}

func TestCheckOPAUpdateLoopBadURL(t *testing.T) {
	testCheckOPAUpdateLoop(t, "http://foo:8112", "Unable to send OPA version report")
}

func TestCheckOPAUpdateLoopNoUpdate(t *testing.T) {
	exp := &report.DataResponse{Latest: report.ReleaseDetails{
		OPAUpToDate: true,
	}}

	// test server
	baseURL, teardown := getTestServer(exp, http.StatusOK)
	defer teardown()

	testCheckOPAUpdateLoop(t, baseURL, "OPA is up to date.")
}

func TestCheckOPAUpdateLoopWithNewUpdate(t *testing.T) {
	exp := &report.DataResponse{Latest: report.ReleaseDetails{
		Download:      "https://openpolicyagent.org/downloads/v100.0.0/opa_darwin_amd64",
		ReleaseNotes:  "https://github.com/open-policy-agent/opa/releases/tag/v100.0.0",
		LatestRelease: "v100.0.0",
		OPAUpToDate:   false,
	}}

	// test server
	baseURL, teardown := getTestServer(exp, http.StatusOK)
	defer teardown()

	testCheckOPAUpdateLoop(t, baseURL, "OPA is out of date.")
}

func TestRuntimeWithAuthzSchemaVerification(t *testing.T) {
	ctx := context.Background()

	fs := map[string]string{
		"test/authz.rego": `package system.authz

		default allow := false

		allow {
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

		default allow := false

		allow {
           input.identty = "foo"
		}`)

		if err := os.WriteFile(path.Join(rootDir, "authz.rego"), badModule, 0644); err != nil {
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
	ctx := context.Background()

	fs := map[string]string{
		"test/authz.rego": `package system.authz

		default allow := false

        is_secret :=  input.identty == "secret"

        # even though "is_secret" is called via 2 paths, there should be only one resulting error
        # 1-step dependency
        allow {
          is_secret
        }

        # 2-step dependency
        allow {
          allow2
        }

        allow2 {
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
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
		v1Compatible bool
		files        map[string]string
		expErr       string
	}{
		{
			note: "Rego v0, keywords not imported",
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
			note: "Rego v0, rego.v1 imported",
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
			note: "Rego v0, future.keywords imported",
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
			note: "Rego v0, no keywords used",
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
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
					defer cancel()
					var output bytes.Buffer

					params := NewParams()
					params.Output = &output
					params.Paths = []string{root}
					params.BundleMode = b
					params.Addrs = &[]string{"localhost:0"}
					params.GracefulShutdownPeriod = 1
					params.Logger = logging.NewNoOpLogger()
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

func TestUrlPathToConfigOverride(t *testing.T) {
	params := NewParams()
	params.Paths = []string{"https://www.example.com/bundles/bundle.tar.gz"}
	ctx := context.Background()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	var serviceConf map[string]interface{}
	if err = json.Unmarshal(rt.Manager.Config.Services, &serviceConf); err != nil {
		t.Fatal(err)
	}

	cliService, ok := serviceConf["cli1"].(map[string]interface{})
	if !ok {
		t.Fatal("excpected service configuration for 'cli1' service")
	}

	if cliService["url"] != "https://www.example.com" {
		t.Error("expected cli1 service url value: 'https://www.example.com'")
	}

	var bundleConf map[string]interface{}
	if err = json.Unmarshal(rt.Manager.Config.Bundles, &bundleConf); err != nil {
		t.Fatal(err)
	}

	cliBundle, ok := bundleConf["cli1"].(map[string]interface{})
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

func getTestServer(update interface{}, statusCode int) (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, req *http.Request) {
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

	ctx := context.Background()
	rt := getTestRuntime(ctx, t, logging.NewNoOpLogger())
	result := rt.checkOPAUpdate(ctx)

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected output:\"%v\" but got: \"%v\"", expected, result)
	}
}

func testCheckOPAUpdateLoop(t *testing.T, url, expected string) {
	t.Helper()
	t.Setenv("OPA_TELEMETRY_SERVICE_URL", url)

	ctx := context.Background()

	logger := logging.New()
	stdout := bytes.NewBuffer(nil)
	logger.SetOutput(stdout)
	logger.SetLevel(logging.Debug)

	rt := getTestRuntime(ctx, t, logger)

	done := make(chan struct{})
	go func() {
		d := time.Duration(int64(time.Millisecond) * 1)
		rt.checkOPAUpdateLoop(ctx, d, done)
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
		v1Compatible  bool
	}{
		{"WarningMessage", false, true, false},
		{"NoWarningMessage", true, false, false},
		{"V1Compatible", false, false, true},
		{"V1InCompatible", false, true, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
			defer cancel()

			params := NewParams()

			logger := testLog.New()
			logLevel := logging.Info

			params.Logger = logger
			params.Addrs = &[]string{"localhost:8181"}
			params.AddrSetByUser = tc.addrSetByUser
			params.GracefulShutdownPeriod = 1
			params.V1Compatible = tc.v1Compatible
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
		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		_, err := NewRuntime(context.Background(), params)
		if err != nil {
			t.Fatalf(err.Error())
		}
	})
}

func TestRuntimeWithExplicitBadMetricConfiguration(t *testing.T) {
	fs := map[string]string{
		"/config.yaml": `{"server": {"metrics": {"prom": {"http_request_duration_seconds": {"buckets": "would-not-work"}}}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {
		params := NewParams()
		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		_, err := NewRuntime(context.Background(), params)
		if err == nil {
			t.Fatalf("Expected error to be thrown on malformed metrics config")
		}

		if !strings.HasPrefix(err.Error(), "server metrics configuration parse error") {
			t.Fatalf("Expected specific error to be thrown on malformed metrics config")
		}
	})
}
