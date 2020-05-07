// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ghodss/yaml"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestWatchPaths(t *testing.T) {

	fs := map[string]string{
		"/foo/bar/baz.json": "true",
	}

	expected := []string{
		"/foo", "/foo/bar", "/foo/bar/baz.json",
	}

	test.WithTempFS(fs, func(rootDir string) {
		paths, err := getWatchPaths([]string{"prefix:" + rootDir + "/foo"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result := []string{}
		for _, path := range paths {
			result = append(result, strings.TrimPrefix(path, rootDir))
		}
		if !reflect.DeepEqual(expected, result) {
			t.Fatalf("Expected %v but got: %v", expected, result)
		}
	})
}

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
		"/some/data.json": `{
			"hello": "world"
		}`,
	}

	test.WithTempFS(fs, func(rootDir string) {
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

		if err := ioutil.WriteFile(path.Join(rootDir, "some/data.json"), util.MustMarshalJSON(expected), 0644); err != nil {
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
		"/x.rego": `package test

		default x = 1
		`,
	}

	test.WithTempFS(fs, func(rootDir string) {
		params := NewParams()
		params.Paths = []string{rootDir}
		params.BundleMode = asBundle

		rt, err := NewRuntime(ctx, params)
		if err != nil {
			t.Fatal(err)
		}

		storage.Txn(ctx, rt.Store, storage.WriteParams, func(txn storage.Transaction) error {
			return rt.Store.UpsertPolicy(ctx, txn, "out-of-band.rego", []byte(`package foo`))
		})

		ch := make(chan error)

		testFunc := func(d time.Duration, err error) {
			ch <- err
		}

		if err := rt.startWatcher(ctx, params.Paths, testFunc); err != nil {
			t.Fatalf("Unexpected watcher init error: %v", err)
		}

		newModule := []byte(`package test

		default x = 2`)

		if err := ioutil.WriteFile(path.Join(rootDir, "y.rego"), newModule, 0644); err != nil {
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

func TestLoadConfigWithParamOverride(t *testing.T) {
	fs := map[string]string{"/some/config.yaml": `
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1

discovery:
  name: /example/discovery
  prefix: configuration
`}

	test.WithTempFS(fs, func(rootDir string) {
		params := NewParams()
		params.ConfigFile = filepath.Join(rootDir, "/some/config.yaml")
		params.ConfigOverrides = []string{"services.acmecorp.credentials.bearer.token=bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"}

		configBytes, err := loadConfig(params)
		if err != nil {
			t.Errorf("unexpected error loading config: " + err.Error())
		}

		config := map[string]interface{}{}
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			t.Errorf("unexpected error unmarshalling config")
		}

		expected := map[string]interface{}{
			"services": map[string]interface{}{
				"acmecorp": map[string]interface{}{
					"url": "https://example.com/control-plane-api/v1",
					"credentials": map[string]interface{}{
						"bearer": map[string]interface{}{
							"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
						},
					},
				},
			},
			"discovery": map[string]interface{}{
				"name":   "/example/discovery",
				"prefix": "configuration",
			},
		}

		if !reflect.DeepEqual(config, expected) {
			t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
		}
	})
}

func TestLoadConfigWithFileOverride(t *testing.T) {
	fs := map[string]string{
		"/some/config.yaml": `
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "XXXXXXXXXX"

discovery:
  name: /example/discovery
  prefix: configuration
`,
		"/some/secret.txt": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
	}

	test.WithTempFS(fs, func(rootDir string) {
		params := NewParams()
		params.ConfigFile = filepath.Join(rootDir, "/some/config.yaml")
		secretFile := filepath.Join(rootDir, "/some/secret.txt")
		params.ConfigOverrideFiles = []string{fmt.Sprintf("services.acmecorp.credentials.bearer.token=%s", secretFile)}

		configBytes, err := loadConfig(params)
		if err != nil {
			t.Errorf("unexpected error loading config: " + err.Error())
		}

		config := map[string]interface{}{}
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			t.Errorf("unexpected error unmarshalling config")
		}

		expected := map[string]interface{}{
			"services": map[string]interface{}{
				"acmecorp": map[string]interface{}{
					"url": "https://example.com/control-plane-api/v1",
					"credentials": map[string]interface{}{
						"bearer": map[string]interface{}{
							"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
						},
					},
				},
			},
			"discovery": map[string]interface{}{
				"name":   "/example/discovery",
				"prefix": "configuration",
			},
		}

		if !reflect.DeepEqual(config, expected) {
			t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
		}
	})
}

func TestLoadConfigWithParamOverrideNoConfigFile(t *testing.T) {
	params := NewParams()
	params.ConfigOverrides = []string{
		"services.acmecorp.url=https://example.com/control-plane-api/v1",
		"services.acmecorp.credentials.bearer.token=bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
		"discovery.name=/example/discovery",
		"discovery.prefix=configuration",
	}

	configBytes, err := loadConfig(params)
	if err != nil {
		t.Errorf("unexpected error loading config: " + err.Error())
	}

	config := map[string]interface{}{}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		t.Errorf("unexpected error unmarshalling config")
	}

	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"acmecorp": map[string]interface{}{
				"url": "https://example.com/control-plane-api/v1",
				"credentials": map[string]interface{}{
					"bearer": map[string]interface{}{
						"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
					},
				},
			},
		},
		"discovery": map[string]interface{}{
			"name":   "/example/discovery",
			"prefix": "configuration",
		},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
	}
}
