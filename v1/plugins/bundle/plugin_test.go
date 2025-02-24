// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package bundle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/disk"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	inmemtst "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

const (
	deltaBundleSize    = 128
	snapshotBundleSize = 1024
)

func TestPluginOneShot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
		Etag: "foo",
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if status.Type != bundle.SnapshotBundleType {
		t.Fatalf("expected snapshot bundle but got %v", status.Type)
	} else if status.Size != snapshotBundleSize {
		t.Fatalf("expected snapshot bundle size %d but got %d", snapshotBundleSize, status.Size)
	}

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{
		"foo": {"bar": 1, "baz": "qux"},
		"system": {
			"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestPluginOneShotWithAstStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false), inmem.OptReturnASTValuesOnRead(true))
	manager := getTestManagerWithOpts(nil, store)
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Etag:     "foo",
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if status.Type != bundle.SnapshotBundleType {
		t.Fatalf("expected snapshot bundle but got %v", status.Type)
	} else if status.Size != snapshotBundleSize {
		t.Fatalf("expected snapshot bundle size %d but got %d", snapshotBundleSize, status.Size)
	}

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := ast.MustParseTerm(`{"foo": {"bar": 1, "baz": "qux"}, "system": {"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}}}`)
	if err != nil {
		t.Fatal(err)
	} else if ast.Compare(data, expData) != 0 {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestPluginOneShotV1Compatible(t *testing.T) {
	t.Parallel()

	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x",
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, shadowed import (no error)",
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0",
			v1Compatible: true,
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, shadowed import",
			v1Compatible: true,
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := ast.RegoV0
			if tc.v1Compatible {
				regoVersion = ast.RegoV1
			}
			popts := ast.ParserOptions{RegoVersion: regoVersion}

			ctx := context.Background()
			manager := getTestManager()
			plugin := New(&Config{}, manager)
			bundleName := "test-bundle"
			plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

			ensurePluginState(t, plugin, plugins.StateNotReady)

			b := bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
				Data:     map[string]interface{}{},
				Modules: []bundle.ModuleFile{
					{
						Path:   "/foo/bar",
						Parsed: ast.MustParseModuleWithOpts(tc.module, popts),
						Raw:    []byte(tc.module),
					},
				},
				Etag: "foo",
			}

			b.Manifest.Init()

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

			if tc.expErrs != nil {
				ensurePluginState(t, plugin, plugins.StateNotReady)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if errs := status.Errors; len(errs) != len(tc.expErrs) {
					t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", tc.expErrs, errs)
				} else {
					for _, expErr := range tc.expErrs {
						found := false
						for _, err := range errs {
							if strings.Contains(err.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
						}
					}
				}
			} else {
				ensurePluginState(t, plugin, plugins.StateOK)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if status.Size != snapshotBundleSize {
					t.Fatalf("expected snapshot bundle size %d but got %d", snapshotBundleSize, status.Size)
				}

				txn := storage.NewTransactionOrDie(ctx, manager.Store)
				defer manager.Store.Abort(ctx, txn)

				ids, err := manager.Store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				} else if len(ids) != 1 {
					t.Fatal("Expected 1 policy")
				}

				bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
				exp := []byte(tc.module)
				if err != nil {
					t.Fatal(err)
				} else if !bytes.Equal(bs, exp) {
					t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
				}
			}
		})
	}
}

func TestPluginOneShotWithBundleRegoVersion(t *testing.T) {
	t.Parallel()

	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note               string
		managerRegoVersion ast.RegoVersion
		bundleRegoVersion  *ast.RegoVersion
		module             string
		expErrs            []string
	}{
		{
			note:               "v0.x manager, no bundle version",
			managerRegoVersion: ast.RegoV0,
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, no bundle version, shadowed import (no error)",
			managerRegoVersion: ast.RegoV0,
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},

		{
			note:               "v0.x manager, v0.x bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, shadowed import (no error)",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},

		{
			note:               "v0.x manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, shadowed import (error)",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},

		{
			note:               "v1.0 manager, no bundle version",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, no bundle version, shadowed import (error)",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},

		{
			note:               "v1.0 manager, v0.x bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, shadowed import (no error)",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},

		{
			note:               "v1.0 manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, shadowed import (error)",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			managerPopts := ast.ParserOptions{RegoVersion: tc.managerRegoVersion}
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
				plugins.WithParserOptions(managerPopts))
			if err != nil {
				t.Fatal(err)
			}
			plugin := New(&Config{}, manager)
			bundleName := "test-bundle"
			plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

			ensurePluginState(t, plugin, plugins.StateNotReady)

			var bundlePopts ast.ParserOptions
			m := bundle.Manifest{Revision: "quickbrownfaux"}
			if tc.bundleRegoVersion != nil {
				m.SetRegoVersion(*tc.bundleRegoVersion)
				bundlePopts = ast.ParserOptions{RegoVersion: *tc.bundleRegoVersion}
			} else {
				bundlePopts = managerPopts
			}
			b := bundle.Bundle{
				Manifest: m,
				Data:     map[string]interface{}{},
				Modules: []bundle.ModuleFile{
					{
						Path:   "/foo/bar",
						Parsed: ast.MustParseModuleWithOpts(tc.module, bundlePopts),
						Raw:    []byte(tc.module),
					},
				},
				Etag: "foo",
			}

			b.Manifest.Init()

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

			if tc.expErrs != nil {
				ensurePluginState(t, plugin, plugins.StateNotReady)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if errs := status.Errors; len(errs) != len(tc.expErrs) {
					t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", tc.expErrs, errs)
				} else {
					for _, expErr := range tc.expErrs {
						found := false
						for _, err := range errs {
							if strings.Contains(err.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
						}
					}
				}
			} else {
				ensurePluginState(t, plugin, plugins.StateOK)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if status.Size != snapshotBundleSize {
					t.Fatalf("expected snapshot bundle size %d but got %d", snapshotBundleSize, status.Size)
				}

				txn := storage.NewTransactionOrDie(ctx, manager.Store)
				defer manager.Store.Abort(ctx, txn)

				ids, err := manager.Store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				} else if len(ids) != 1 {
					t.Fatal("Expected 1 policy")
				}

				bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
				exp := []byte(tc.module)
				if err != nil {
					t.Fatal(err)
				} else if !bytes.Equal(bs, exp) {
					t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
				}
			}
		})
	}
}

func TestPluginOneShotWithAuthzSchemaVerification(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	manager := getTestManager()

	info, err := runtime.Term(runtime.Params{Config: nil, IsAuthorizationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	manager.Info = info

	plugin := New(&Config{}, manager)

	bundleName := "test-bundle"

	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// authz rules with no error
	authzModule := `package system.authz
		import rego.v1

		default allow := false

		allow if {
          input.identity == "foo"
		}`

	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/authz.rego",
				Path:   "/authz.rego",
				Parsed: ast.MustParseModule(authzModule),
				Raw:    []byte(authzModule),
			},
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	// authz rules with errors
	authzModule = `package system.authz
		import rego.v1

		default allow := false

		allow if {
          input.identty == "foo"            # type error 1
		}

        allow if {
          helper1
        }

        helper1 if {
          helper2
        }

        helper2 if {
          input.method == 123               # type error 2
		}

        dont_type_check_me if {
          input.methd == "GET"               # type error 3
		}`

	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/authz.rego",
				Path:   "/authz.rego",
				Parsed: ast.MustParseModule(authzModule),
				Raw:    []byte(authzModule),
			},
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if len(status.Errors) != 2 {
		t.Fatalf("expected 2 errors but got %v", len(status.Errors))
	}

	// disable authorization to ensure bundle activates with bad authz policy
	info, err = runtime.Term(runtime.Params{Config: nil, IsAuthorizationEnabled: false})
	if err != nil {
		t.Fatal(err)
	}
	plugin.manager.Info = info

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if len(status.Errors) != 0 {
		t.Fatalf("expected 0 errors but got %v", len(status.Errors))
	}

	// enable authorization but skip type checking of known input schemas
	info, err = runtime.Term(runtime.Params{Config: nil, IsAuthorizationEnabled: true, SkipKnownSchemaCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	plugin.manager.Info = info

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if len(status.Errors) != 0 {
		t.Fatalf("expected 0 errors but got %v", len(status.Errors))
	}
}

func TestPluginOneShotWithAuthzSchemaVerificationNonDefaultAuthzPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	manager := getTestManager()

	s := "/foo/authz/allow"
	manager.Config.DefaultAuthorizationDecision = &s

	info, err := runtime.Term(runtime.Params{Config: nil, IsAuthorizationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	manager.Info = info

	plugin := New(&Config{}, manager)

	bundleName := "test-bundle"

	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module := "package foo\n\ncorge=1"

	authzModule := `package foo.authz
		import rego.v1

		default allow := false

		allow if {
          input.identty == "foo"            # type error 1
		}

        allow if {
          helper
        }

        helper if {
          input.method == 123               # type error 2
		}

        dont_type_check_me if {
          input.methd == "GET"               # type error 3
		}`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/authz.rego",
				Path:   "/authz.rego",
				Parsed: ast.MustParseModule(authzModule),
				Raw:    []byte(authzModule),
			},
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if len(status.Errors) != 2 {
		t.Fatalf("expected 2 errors but got %v", len(status.Errors))
	}

	// no authz policy
	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if len(status.Errors) != 0 {
		t.Fatalf("expected 0 errors but got %v", len(status.Errors))
	}
}

func TestPluginStartLazyLoadInMem(t *testing.T) {
	t.Parallel()

	readMode := []struct {
		note    string
		readAst bool
	}{
		{
			note:    "read raw",
			readAst: false,
		},
		{
			note:    "read ast",
			readAst: true,
		},
	}

	for _, rm := range readMode {
		t.Run(rm.note, func(t *testing.T) {
			ctx := context.Background()

			module := "package authz\n\ncorge=1"

			// setup fake http server with mock bundle
			mockBundle1 := bundle.Bundle{
				Data: map[string]interface{}{"p": "x1"},
				Modules: []bundle.ModuleFile{
					{
						URL:    "/bar/policy.rego",
						Path:   "/bar/policy.rego",
						Parsed: ast.MustParseModule(module),
						Raw:    []byte(module),
					},
				},
				Manifest: bundle.Manifest{
					Roots: &[]string{"p", "authz"},
				},
			}

			s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				err := bundle.NewWriter(w).Write(mockBundle1)
				if err != nil {
					t.Fatal(err)
				}
			}))

			mockBundle2 := bundle.Bundle{
				Data:    map[string]interface{}{"q": "x2"},
				Modules: []bundle.ModuleFile{},
				Manifest: bundle.Manifest{
					Roots: &[]string{"q"},
				},
			}

			s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				err := bundle.NewWriter(w).Write(mockBundle2)
				if err != nil {
					t.Fatal(err)
				}
			}))

			config := []byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			},
			"acmecorp": {
				"url": %q
			}
		}
	}`, s1.URL, s2.URL))

			manager := getTestManagerWithOpts(config, inmem.NewWithOpts(inmem.OptReturnASTValuesOnRead(rm.readAst)))
			defer manager.Stop(ctx)

			var mode plugins.TriggerMode = "manual"

			plugin := New(&Config{
				Bundles: map[string]*Source{
					"test-1": {
						Service:        "default",
						SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
						Config:         download.Config{Trigger: &mode},
					},
					"test-2": {
						Service:        "acmecorp",
						SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
						Config:         download.Config{Trigger: &mode},
					},
				},
			}, manager)

			statusCh := make(chan map[string]*Status)

			// register for bundle updates to observe changes and start the plugin
			plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
				statusCh <- st
			})

			err := plugin.Start(ctx)
			if err != nil {
				t.Fatal(err)
			}

			// manually trigger bundle download on all configured bundles
			go func() {
				_ = plugin.Trigger(ctx)
			}()

			// wait for bundle update and then assert on data content
			<-statusCh
			<-statusCh

			result, err := storage.ReadOne(ctx, manager.Store, storage.Path{"p"})
			if err != nil {
				t.Fatal(err)
			}

			if rm.readAst {
				expected, _ := ast.InterfaceToValue(mockBundle1.Data["p"])
				if ast.Compare(result, expected) != 0 {
					t.Fatalf("expected data to be %v but got %v", expected, result)
				}
			} else if !reflect.DeepEqual(result, mockBundle1.Data["p"]) {
				t.Fatalf("expected data to be %v but got %v", mockBundle1.Data, result)
			}

			result, err = storage.ReadOne(ctx, manager.Store, storage.Path{"q"})
			if err != nil {
				t.Fatal(err)
			}

			if rm.readAst {
				expected, _ := ast.InterfaceToValue(mockBundle2.Data["q"])
				if ast.Compare(result, expected) != 0 {
					t.Fatalf("expected data to be %v but got %v", expected, result)
				}
			} else if !reflect.DeepEqual(result, mockBundle2.Data["q"]) {
				t.Fatalf("expected data to be %v but got %v", mockBundle2.Data, result)
			}

			txn := storage.NewTransactionOrDie(ctx, manager.Store)
			defer manager.Store.Abort(ctx, txn)

			ids, err := manager.Store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatal(err)
			} else if len(ids) != 1 {
				t.Fatal("Expected 1 policy")
			}

			bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
			exp := []byte("package authz\n\ncorge=1")
			if err != nil {
				t.Fatal(err)
			} else if !bytes.Equal(bs, exp) {
				t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
			}

			data, err := manager.Store.Read(ctx, txn, storage.Path{})
			if err != nil {
				t.Fatal(err)
			}

			expected := `{
				"p": "x1", "q": "x2",
				"system": {
					"bundles": {"test-1": {"etag": "", "manifest": {"revision": "", "roots": ["p", "authz"]}}, "test-2": {"etag": "", "manifest": {"revision": "", "roots": ["q"]}}}
				}
			}`
			if rm.readAst {
				expData := ast.MustParseTerm(expected)
				if ast.Compare(data, expData) != 0 {
					t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
				}
			} else {
				expData := util.MustUnmarshalJSON([]byte(expected))
				if !reflect.DeepEqual(data, expData) {
					t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
				}
			}
		})
	}
}

func TestPluginOneShotDiskStorageMetrics(t *testing.T) {
	t.Parallel()

	test.WithTempFS(nil, func(dir string) {
		ctx := context.Background()
		met := metrics.New()
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
			Partitions: []storage.Path{
				storage.MustParsePath("/foo"),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		manager := getTestManagerWithOpts(nil, store)
		defer manager.Stop(ctx)
		plugin := New(&Config{}, manager)
		bundleName := "test-bundle"
		plugin.status[bundleName] = &Status{Name: bundleName, Metrics: met}
		plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

		ensurePluginState(t, plugin, plugins.StateNotReady)

		module := "package foo\n\ncorge=1"

		b := bundle.Bundle{
			Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
			Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
			Modules: []bundle.ModuleFile{
				{
					Path:   "/foo/bar",
					Parsed: ast.MustParseModule(module),
					Raw:    []byte(module),
				},
			},
		}

		b.Manifest.Init()

		met = metrics.New()
		plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: met})

		ensurePluginState(t, plugin, plugins.StateOK)

		// NOTE(sr): These assertions reflect the current behaviour only! Not prescriptive.
		name := "disk_deleted_keys"
		if exp, act := 3, met.Counter(name).Value(); act.(uint64) != uint64(exp) {
			t.Errorf("%s: expected %v, got %v", name, exp, act)
		}
		name = "disk_written_keys"
		if exp, act := 6, met.Counter(name).Value(); act.(uint64) != uint64(exp) {
			t.Errorf("%s: expected %v, got %v", name, exp, act)
		}
		name = "disk_read_keys"
		if exp, act := 13, met.Counter(name).Value(); act.(uint64) != uint64(exp) {
			t.Errorf("%s: expected %v, got %v", name, exp, act)
		}
		name = "disk_read_bytes"
		if exp, act := 269, met.Counter(name).Value(); act.(uint64) != uint64(exp) {
			t.Errorf("%s: expected %v, got %v", name, exp, act)
		}
		for _, timer := range []string{
			"disk_commit",
			"disk_write",
			"disk_read",
		} {
			if act := met.Timer(timer).Int64(); act <= 0 {
				t.Errorf("%s: expected non-zero timer, got %v", timer, act)
			}
		}
		if t.Failed() {
			t.Logf("all metrics: %v", met.All())
		}

		// Ensure we can read it all back -- this is the only bundle plugin test using disk storage,
		// so some duplicating with TestPluginOneShot is OK:

		txn := storage.NewTransactionOrDie(ctx, manager.Store)
		defer manager.Store.Abort(ctx, txn)

		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			t.Fatal(err)
		} else if len(ids) != 1 {
			t.Fatal("Expected 1 policy")
		}

		bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
		exp := []byte("package foo\n\ncorge=1")
		if err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(bs, exp) {
			t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
		}

		data, err := manager.Store.Read(ctx, txn, storage.Path{})
		expData := util.MustUnmarshalJSON([]byte(`{
			"foo": {"bar": 1, "baz": "qux"},
			"system": {
				"bundles": {"test-bundle": {"etag": "", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
			}
		}`))
		if err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(data, expData) {
			t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
		}
	})
}

func TestPluginOneShotDeltaBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module := "package a\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"baz": "qux",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "a/policy.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	// simulate a delta bundle download

	// replace a value
	p1 := bundle.PatchOperation{
		Op:    "replace",
		Path:  "a/baz",
		Value: "bux",
	}

	// add a new object member
	p2 := bundle.PatchOperation{
		Op:    "upsert",
		Path:  "/a/foo",
		Value: []interface{}{"hello", "world"},
	}

	b2 := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "delta", Roots: &[]string{"a"}},
		Patch:    bundle.Patch{Data: []bundle.PatchOperation{p1, p2}},
		Etag:     "foo",
	}

	plugin.process(ctx, bundleName, download.Update{Bundle: &b2, Metrics: metrics.New(), Size: deltaBundleSize})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if status.Type != bundle.DeltaBundleType {
		t.Fatalf("expected delta bundle but got %v", status.Type)
	} else if status.Size != deltaBundleSize {
		t.Fatalf("expected delta bundle size %d but got %d", deltaBundleSize, status.Size)
	}

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("Expected 1 policy, got %d", len(ids))
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	exp := []byte("package a\n\ncorge=1")
	if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	if err != nil {
		t.Fatal(err)
	}
	expData := util.MustUnmarshalJSON([]byte(`{
		"a": {"baz": "bux", "foo": ["hello", "world"]},
		"system": {
			"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "delta", "roots": ["a"]}}}
		}
	}`))
	if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%#v\n\nGot:\n\n%#v", expData, data)
	}
}

func TestPluginOneShotDeltaBundleWithAstStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false), inmem.OptReturnASTValuesOnRead(true))
	manager := getTestManagerWithOpts(nil, store)
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module := "package a\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"baz": "qux",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "a/policy.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	// simulate a delta bundle download

	// replace a value
	p1 := bundle.PatchOperation{
		Op:    "replace",
		Path:  "a/baz",
		Value: "bux",
	}

	// add a new object member
	p2 := bundle.PatchOperation{
		Op:    "upsert",
		Path:  "/a/foo",
		Value: []interface{}{"hello", "world"},
	}

	b2 := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "delta", Roots: &[]string{"a"}},
		Patch:    bundle.Patch{Data: []bundle.PatchOperation{p1, p2}},
		Etag:     "foo",
	}

	plugin.process(ctx, bundleName, download.Update{Bundle: &b2, Metrics: metrics.New(), Size: deltaBundleSize})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if status.Type != bundle.DeltaBundleType {
		t.Fatalf("expected delta bundle but got %v", status.Type)
	} else if status.Size != deltaBundleSize {
		t.Fatalf("expected delta bundle size %d but got %d", deltaBundleSize, status.Size)
	}

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("Expected 1 policy, got %d", len(ids))
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	exp := []byte("package a\n\ncorge=1")
	if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	if err != nil {
		t.Fatal(err)
	}
	expData := ast.MustParseTerm(`{
		"a": {"baz": "bux", "foo": ["hello", "world"]},
		"system": {
			"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "delta", "roots": ["a"]}}}
		}
	}`)
	if ast.Compare(data, expData) != 0 {
		t.Fatalf("Bad data content. Exp:\n%#v\n\nGot:\n\n%#v", expData, data)
	}
}

func TestPluginStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	bundles := map[string]*Source{}

	plugin := New(&Config{Bundles: bundles}, manager)
	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
}

func TestStop(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	var longPollTimeout int64 = 3
	done := make(chan struct{})
	tsURLBase := "/opa-test/"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, tsURLBase) {
			t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
		}

		close(done)

		// simulate long operation
		time.Sleep(time.Duration(longPollTimeout) * time.Second)
		fmt.Fprintln(w) // Note: this is an invalid bundle and will fail the download
	}))
	defer ts.Close()

	ctx := context.Background()
	manager := getTestManager()

	serviceName := "test-svc"
	err := manager.Reconfigure(&config.Config{
		Services: []byte(fmt.Sprintf("{%q:{ \"url\": %q}}", serviceName, ts.URL+tsURLBase)),
	})
	if err != nil {
		t.Fatalf("Error configuring plugin manager: %s", err)
	}

	triggerPolling := plugins.TriggerPeriodic
	baseConf := download.Config{Polling: download.PollingConfig{LongPollingTimeoutSeconds: &longPollTimeout}, Trigger: &triggerPolling}

	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}

	callback := func(ctx context.Context, u download.Update) {
		plugin.oneShot(ctx, bundleName, u)
	}
	plugin.downloaders[bundleName] = download.New(baseConf, plugin.manager.Client(serviceName), bundleName).WithCallback(callback)

	err = plugin.Start(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	// Give time for a long poll request to be initiated
	<-done

	plugin.Stop(ctx)

	if plugin.status[bundleName].Code != errCode {
		t.Fatalf("expected error code %v but got %v", errCode, plugin.status[bundleName].Code)
	}

	if !strings.Contains(plugin.status[bundleName].Message, "context canceled") {
		t.Fatalf("unexpected error message %v", plugin.status[bundleName].Message)
	}
}

func TestPluginOneShotBundlePersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	bundleName := "test-bundle"
	bundleSource := Source{
		Persist: true,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource

	plugin := New(&Config{Bundles: bundles}, manager)

	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// simulate a bundle download error with no bundle on disk
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

	if plugin.status[bundleName].Message == "" {
		t.Fatal("expected error but got none")
	}

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// download a bundle and persist to disk. Then verify the bundle persisted to disk
	module := "package foo\n\ncorge=1"
	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo/bar.rego",
				Path:   "/foo/bar.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
		Etag: "foo",
	}

	b.Manifest.Init()
	expBndl := b.Copy() // We're opting out of roundtripping in storage/inmem, so we copy ourselves.

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatal("unexpected error:", err)
	}

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Raw: &buf})

	ensurePluginState(t, plugin, plugins.StateOK)

	result, err := plugin.loadBundleFromDisk(plugin.bundlePersistPath, bundleName, nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(expBndl) {
		t.Fatalf("expected the downloaded bundle to be equal to the one loaded from disk: result=%v, exp=%v", result, expBndl)
	}

	// simulate a bundle download error and verify that the bundle on disk is activated
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{
		"foo": {"bar": 1, "baz": "qux"},
		"system": {
			"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestPluginOneShotBundlePersistenceV1Compatible(t *testing.T) {
	t.Parallel()

	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x",
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, shadowed import (no error)",
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0",
			v1Compatible: true,
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, shadowed import",
			v1Compatible: true,
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := ast.RegoV0
			if tc.v1Compatible {
				regoVersion = ast.RegoV1
			}
			popts := ast.ParserOptions{RegoVersion: regoVersion}

			ctx := context.Background()
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(), plugins.WithParserOptions(popts))
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			dir := t.TempDir()

			bundleName := "test-bundle"
			bundleSource := Source{
				Persist: true,
			}

			bundles := map[string]*Source{}
			bundles[bundleName] = &bundleSource

			plugin := New(&Config{Bundles: bundles}, manager)

			plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
			plugin.bundlePersistPath = filepath.Join(dir, ".opa")

			ensurePluginState(t, plugin, plugins.StateNotReady)

			// simulate a bundle download error with no bundle on disk
			plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

			if plugin.status[bundleName].Message == "" {
				t.Fatal("expected error but got none")
			}

			ensurePluginState(t, plugin, plugins.StateNotReady)

			// download a bundle and persist to disk. Then verify the bundle persisted to disk
			b := bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
				Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
				Modules: []bundle.ModuleFile{
					{
						URL:    "/foo/bar.rego",
						Path:   "/foo/bar.rego",
						Parsed: ast.MustParseModuleWithOpts(tc.module, popts),
						Raw:    []byte(tc.module),
					},
				},
				Etag: "foo",
			}

			b.Manifest.Init()
			expBndl := b.Copy() // We're opting out of roundtripping in storage/inmem, so we copy ourselves.

			var buf bytes.Buffer
			if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
				t.Fatal("unexpected error:", err)
			}

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Raw: &buf})

			if tc.expErrs != nil {
				ensurePluginState(t, plugin, plugins.StateNotReady)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if errs := status.Errors; len(errs) != len(tc.expErrs) {
					t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", tc.expErrs, errs)
				} else {
					for _, expErr := range tc.expErrs {
						found := false
						for _, err := range errs {
							if strings.Contains(err.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
						}
					}
				}
			} else {
				ensurePluginState(t, plugin, plugins.StateOK)

				result, err := plugin.loadBundleFromDisk(plugin.bundlePersistPath, bundleName, nil)
				if err != nil {
					t.Fatal("unexpected error:", err)
				}

				if !result.Equal(expBndl) {
					t.Fatalf("expected the downloaded bundle to be equal to the one loaded from disk: result=%v, exp=%v", result, expBndl)
				}

				// simulate a bundle download error and verify that the bundle on disk is activated
				plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

				ensurePluginState(t, plugin, plugins.StateOK)

				txn := storage.NewTransactionOrDie(ctx, manager.Store)
				defer manager.Store.Abort(ctx, txn)

				ids, err := manager.Store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				} else if len(ids) != 1 {
					t.Fatal("Expected 1 policy")
				}

				bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
				exp := []byte(tc.module)
				if err != nil {
					t.Fatal(err)
				} else if !bytes.Equal(bs, exp) {
					t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
				}

				expData := util.MustUnmarshalJSON([]byte(`{
					"foo": {"bar": 1, "baz": "qux"},
					"system": {
						"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
					}
				}`))

				data, err := manager.Store.Read(ctx, txn, storage.Path{})
				if err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(data, expData) {
					t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
				}
			}
		})
	}
}

func TestPluginOneShotBundlePersistenceWithBundleRegoVersion(t *testing.T) {
	t.Parallel()

	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note               string
		managerRegoVersion ast.RegoVersion
		bundleRegoVersion  *ast.RegoVersion
		module             string
		expErrs            []string
	}{
		{
			note:               "v0.x manager, no bundle rego version",
			managerRegoVersion: ast.RegoV0,
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, no bundle rego version, shadowed import (no error)",
			managerRegoVersion: ast.RegoV0,
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, shadowed import (no error)",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},

		{
			note:               "v1.0 manager, no bundle rego version",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, no bundle rego version, shadowed import (no error)",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v1.0 manager, v0.x bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, shadowed import (no error)",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			managerPopts := ast.ParserOptions{RegoVersion: tc.managerRegoVersion}
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
				plugins.WithParserOptions(managerPopts))
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			dir := t.TempDir()

			bundleName := "test-bundle"
			bundleSource := Source{
				Persist: true,
			}

			bundles := map[string]*Source{}
			bundles[bundleName] = &bundleSource

			plugin := New(&Config{Bundles: bundles}, manager)

			plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
			plugin.bundlePersistPath = filepath.Join(dir, ".opa")

			ensurePluginState(t, plugin, plugins.StateNotReady)

			// simulate a bundle download error with no bundle on disk
			plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

			if plugin.status[bundleName].Message == "" {
				t.Fatal("expected error but got none")
			}

			ensurePluginState(t, plugin, plugins.StateNotReady)

			// download a bundle and persist to disk. Then verify the bundle persisted to disk
			var bundlePopts ast.ParserOptions
			m := bundle.Manifest{Revision: "quickbrownfaux"}
			if tc.bundleRegoVersion != nil {
				m.SetRegoVersion(*tc.bundleRegoVersion)
				bundlePopts = ast.ParserOptions{RegoVersion: *tc.bundleRegoVersion}
			} else {
				bundlePopts = managerPopts
			}
			b := bundle.Bundle{
				Manifest: m,
				Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
				Modules: []bundle.ModuleFile{
					{
						URL:    "/foo/bar.rego",
						Path:   "/foo/bar.rego",
						Parsed: ast.MustParseModuleWithOpts(tc.module, bundlePopts),
						Raw:    []byte(tc.module),
					},
				},
				Etag: "foo",
			}

			b.Manifest.Init()
			expBndl := b.Copy() // We're opting out of roundtripping in storage/inmem, so we copy ourselves.

			var buf bytes.Buffer
			if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
				t.Fatal("unexpected error:", err)
			}

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Raw: &buf})

			if tc.expErrs != nil {
				ensurePluginState(t, plugin, plugins.StateNotReady)

				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if errs := status.Errors; len(errs) != len(tc.expErrs) {
					t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", tc.expErrs, errs)
				} else {
					for _, expErr := range tc.expErrs {
						found := false
						for _, err := range errs {
							if strings.Contains(err.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
						}
					}
				}
			} else {
				ensurePluginState(t, plugin, plugins.StateOK)

				result, err := plugin.loadBundleFromDisk(plugin.bundlePersistPath, bundleName, nil)
				if err != nil {
					t.Fatal("unexpected error:", err)
				}

				if !result.Equal(expBndl) {
					t.Fatalf("expected the downloaded bundle to be equal to the one loaded from disk: result=%v, exp=%v", result, expBndl)
				}

				// simulate a bundle download error and verify that the bundle on disk is activated
				plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

				ensurePluginState(t, plugin, plugins.StateOK)

				txn := storage.NewTransactionOrDie(ctx, manager.Store)
				defer manager.Store.Abort(ctx, txn)

				ids, err := manager.Store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				} else if len(ids) != 1 {
					t.Fatal("Expected 1 policy")
				}

				bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
				exp := []byte(tc.module)
				if err != nil {
					t.Fatal(err)
				} else if !bytes.Equal(bs, exp) {
					t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
				}

				data, err := manager.Store.Read(ctx, txn, storage.Path{})

				var manifestRegoVersion string
				var moduleRegoVersion string
				if tc.bundleRegoVersion != nil {
					manifestRegoVersion = fmt.Sprintf(`, "rego_version": %d`, bundleRegoVersion(*tc.bundleRegoVersion))

					if *tc.bundleRegoVersion != tc.managerRegoVersion {
						moduleRegoVersion = fmt.Sprintf(`,"modules": {"test-bundle/foo/bar.rego": {"rego_version": %d}}`, tc.bundleRegoVersion.Int())
					}
				}

				expData := util.MustUnmarshalJSON([]byte(fmt.Sprintf(`{
					"foo": {"bar": 1, "baz": "qux"},
					"system": {
						"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux"%s, "roots": [""]}}}%s
					}
				}`,
					manifestRegoVersion, moduleRegoVersion)))

				if err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(data, expData) {
					t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
				}
			}
		})
	}
}

func TestPluginOneShotSignedBundlePersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	bundleName := "test-bundle"
	vc := bundle.NewVerificationConfig(map[string]*bundle.KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil)
	bundleSource := Source{
		Persist: true,
		Signing: vc,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource

	plugin := New(&Config{Bundles: bundles}, manager)

	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// simulate a bundle download error with no bundle on disk
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

	if plugin.status[bundleName].Message == "" {
		t.Fatal("expected error but got none")
	}

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// download a signed bundle and persist to disk. Then verify the bundle persisted to disk
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiI1MDdhMmMzOGExNDQxZGI1OGQyY2I4Nzk4MmM0MmFhOTFhNDM0MmVmNDIyYTZiNTQyZWRkZWJlZWY2ZjA0MTJmIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImV4YW1wbGUxL2RhdGEuanNvbiIsImhhc2giOiI3YTM4YmY4MWYzODNmNjk0MzNhZDZlOTAwZDM1YjNlMjM4NTU5M2Y3NmE3YjdhYjVkNDM1NWI4YmE0MWVlMjRiIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImV4YW1wbGUyL2RhdGEuanNvbiIsImhhc2giOiI5ZTRmMTg5YmY0MDc5ZDFiNmViNjQ0Njg3OTg2NmNkNWYzOWMyNjg4MGQ0ZmI1MThmNGUwMWNkMWJiZmU1MTNlIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9XX0.jCLRMyys5u8S2sTS2pWWY82IAeKDpLh3S641_BskCtY`

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
		{"/example1/data.json", `{"foo": "bar"}`},
		{"/example2/data.json", `{"x": true}`},
	}

	buf := archive.MustWriteTarGz(files)

	var dup bytes.Buffer
	tee := io.TeeReader(buf, &dup)
	reader := bundle.NewReader(tee).WithBundleVerificationConfig(vc).WithBundleEtag("foo")
	b, err := reader.Read()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	// We've opted out of having storage/inmem roundtrip our data, so we need to copy ourselves.
	expBndl := b.Copy()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Raw: &dup})

	ensurePluginState(t, plugin, plugins.StateOK)

	// load signed bundle from disk
	result, err := plugin.loadBundleFromDisk(plugin.bundlePersistPath, bundleName, bundles[bundleName])
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(expBndl) {
		t.Fatal("expected the downloaded bundle to be equal to the one loaded from disk")
	}

	// simulate a bundle download error and verify that the bundle on disk is activated
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("unknown error")})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 0 {
		t.Fatal("Expected no policy")
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	if err != nil {
		t.Fatal(err)
	}

	expData := util.MustUnmarshalJSON([]byte(`{"example1": {"foo": "bar"}, "example2": {"x": true}, "system": {"bundles": {"test-bundle": {"etag": "foo", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}}}`))
	if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestLoadAndActivateBundlesFromDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	bundleName := "test-bundle"
	bundleSource := Source{
		Persist: true,
	}

	bundleNameOther := "test-bundle-other"
	bundleSourceOther := Source{}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource
	bundles[bundleNameOther] = &bundleSourceOther

	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	plugin.loadAndActivateBundlesFromDisk(ctx)

	// persist a bundle to disk and then load it
	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo/bar.rego",
				Path:   "/foo/bar.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err := plugin.saveBundleToDisk(bundleName, &buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	plugin.loadAndActivateBundlesFromDisk(ctx)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{
		"foo": {"bar": 1, "baz": "qux"},
		"system": {
			"bundles": {"test-bundle": {"etag": "", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

// Warning: This test modifies package variables, and as
// a result, cannot be run in parallel with other tests.
func TestLoadAndActivateBundlesFromDiskReservedChars(t *testing.T) {
	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	goos = "windows"

	bundleName := "test?bundle=opa" // bundle name contains reserved characters
	bundleSource := Source{
		Persist: true,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource

	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	plugin.loadAndActivateBundlesFromDisk(ctx)

	// persist a bundle to disk and then load it
	module := "package foo\n\ncorge=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo/bar.rego",
				Path:   "/foo/bar.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err := plugin.saveBundleToDisk(bundleName, &buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	plugin.loadAndActivateBundlesFromDisk(ctx)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 1 {
		t.Fatal("Expected 1 policy")
	}

	bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
	exp := []byte("package foo\n\ncorge=1")
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(bs, exp) {
		t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{})
	expData := util.MustUnmarshalJSON([]byte(`{
		"foo": {"bar": 1, "baz": "qux"},
		"system": {
			"bundles": {"test?bundle=opa": {"etag": "", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(data, expData) {
		t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
	}
}

func TestLoadAndActivateBundlesFromDiskV1Compatible(t *testing.T) {
	t.Parallel()

	type update struct {
		modules map[string]string
		expErrs []string
	}
	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note         string
		v1Compatible bool
		updates      []update
	}{
		{
			note: "v0.x",
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
import future.keywords
corge contains 1 if {
	input.x == 2
}`,
					},
				},
			},
		},
		{
			note: "v0.x, shadowed import (no error)",
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
import future.keywords
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
					},
				},
			},
		},
		{
			note:         "v1.0",
			v1Compatible: true,
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
corge contains 1 if {
	input.x == 2
}`,
					},
				},
			},
		},
		{
			note:         "v1.0, shadowed import",
			v1Compatible: true,
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
					},
					expErrs: []string{
						"rego_compile_error: import must not shadow import data.foo",
					},
				},
			},
		},
		{
			note:         "v1.0, module updated",
			v1Compatible: true,
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
corge contains 1 if {
	input.x == 2
}`,
					},
				},
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
corge contains 2 if {
	input.x == 3
}`,
					},
				},
			},
		},
		{
			note:         "v1.0, module updated, shadowed import",
			v1Compatible: true,
			updates: []update{
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
corge contains 1 if {
	input.x == 2
}`,
					},
				},
				{
					modules: map[string]string{
						"/foo/bar.rego": `package foo
import data.foo
import data.bar as foo
corge contains 2 if {
	input.x == 3
}`,
					},
					expErrs: []string{
						"rego_compile_error: import must not shadow import data.foo",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := ast.RegoV0
			if tc.v1Compatible {
				regoVersion = ast.RegoV1
			}
			popts := ast.ParserOptions{RegoVersion: regoVersion}

			ctx := context.Background()
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
				plugins.WithParserOptions(popts))
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			dir := t.TempDir()

			bundleName := "test-bundle"
			bundleSource := Source{
				Persist: true,
			}

			bundleNameOther := "test-bundle-other"
			bundleSourceOther := Source{}

			bundles := map[string]*Source{}
			bundles[bundleName] = &bundleSource
			bundles[bundleNameOther] = &bundleSourceOther

			plugin := New(&Config{Bundles: bundles}, manager)
			plugin.bundlePersistPath = filepath.Join(dir, ".opa")

			plugin.loadAndActivateBundlesFromDisk(ctx)

			for _, update := range tc.updates {
				// persist a bundle to disk and then load it
				b := bundle.Bundle{
					Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
					Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
				}
				for url, module := range update.modules {
					b.Modules = append(b.Modules, bundle.ModuleFile{
						URL:    url,
						Path:   url,
						Parsed: ast.MustParseModuleWithOpts(module, popts),
						Raw:    []byte(module),
					})
				}

				b.Manifest.Init()

				var buf bytes.Buffer
				if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
					t.Fatal("unexpected error:", err)
				}

				err = plugin.saveBundleToDisk(bundleName, &buf)
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}

				plugin.loadAndActivateBundlesFromDisk(ctx)

				if update.expErrs != nil {
					if status, ok := plugin.status[bundleName]; !ok {
						t.Fatalf("Expected to find status for %s, found nil", bundleName)
					} else if status.Type != bundle.SnapshotBundleType {
						t.Fatalf("expected snapshot bundle but got %v", status.Type)
					} else if errs := status.Errors; len(errs) != len(update.expErrs) {
						t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", update.expErrs, errs)
					} else {
						for _, expErr := range update.expErrs {
							found := false
							for _, err := range errs {
								if strings.Contains(err.Error(), expErr) {
									found = true
									break
								}
							}
							if !found {
								t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
							}
						}
					}
				} else {
					txn := storage.NewTransactionOrDie(ctx, manager.Store)
					fatal := func(args ...any) {
						t.Helper()
						manager.Store.Abort(ctx, txn)
						t.Fatal(args...)
					}

					ids, err := manager.Store.ListPolicies(ctx, txn)
					if err != nil {
						fatal(err)
					}
					for _, id := range ids {
						bs, err := manager.Store.GetPolicy(ctx, txn, id)
						p, _ := strings.CutPrefix(id, bundleName)
						module := update.modules[p]
						exp := []byte(module)
						if err != nil {
							fatal(err)
						} else if !bytes.Equal(bs, exp) {
							fatal("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
						}
					}

					expData := util.MustUnmarshalJSON([]byte(`{
						"foo": {"bar": 1, "baz": "qux"},
						"system": {
							"bundles": {"test-bundle": {"etag": "", "manifest": {"revision": "quickbrownfaux", "roots": [""]}}}
						}
					}`))

					data, err := manager.Store.Read(ctx, txn, storage.Path{})
					if err != nil {
						fatal(err)
					} else if !reflect.DeepEqual(data, expData) {
						fatal("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
					}

					manager.Store.Abort(ctx, txn)
				}
			}
		})
	}
}

func TestLoadAndActivateBundlesFromDiskWithBundleRegoVersion(t *testing.T) {
	t.Parallel()

	// Note: modules are parsed before passed to plugin, so any expected errors must be triggered by the compiler stage.
	tests := []struct {
		note               string
		managerRegoVersion ast.RegoVersion
		bundleRegoVersion  *ast.RegoVersion
		module             string
		expErrs            []string
	}{
		{
			note:               "v0.x manager, no bundle rego version",
			managerRegoVersion: ast.RegoV0,
			module: `package foo
corge[1] {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
corge[1] {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, compiler err (shadowed import)",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v1.0 manager, no bundle rego version",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, no bundle rego version, compiler err (shadowed import)",
			managerRegoVersion: ast.RegoV1,
			module: `package foo
import data.foo
import data.bar as foo
corge contains 1 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v1.0 manager, v0.x bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV0),
			module: `package foo
corge[1] {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  pointTo(ast.RegoV1),
			module: `package foo
corge contains 1 if {
	input.x == 2
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			managerPopts := ast.ParserOptions{RegoVersion: tc.managerRegoVersion}
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
				plugins.WithParserOptions(managerPopts))
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			dir := t.TempDir()

			bundleName := "test-bundle"
			bundleSource := Source{
				Persist: true,
			}

			bundleNameOther := "test-bundle-other"
			bundleSourceOther := Source{}

			bundles := map[string]*Source{}
			bundles[bundleName] = &bundleSource
			bundles[bundleNameOther] = &bundleSourceOther

			plugin := New(&Config{Bundles: bundles}, manager)
			plugin.bundlePersistPath = filepath.Join(dir, ".opa")

			plugin.loadAndActivateBundlesFromDisk(ctx)

			// persist a bundle to disk and then load it
			m := bundle.Manifest{Revision: "quickbrownfaux"}
			var bundlePopts ast.ParserOptions
			if tc.bundleRegoVersion != nil {
				m.SetRegoVersion(*tc.bundleRegoVersion)
				bundlePopts = ast.ParserOptions{RegoVersion: *tc.bundleRegoVersion}
			} else {
				bundlePopts = managerPopts
			}
			b := bundle.Bundle{
				Manifest: m,
				Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
				Modules: []bundle.ModuleFile{
					{
						URL:    "/foo/bar.rego",
						Path:   "/foo/bar.rego",
						Parsed: ast.MustParseModuleWithOpts(tc.module, bundlePopts),
						Raw:    []byte(tc.module),
					},
				},
			}

			b.Manifest.Init()

			var buf bytes.Buffer
			if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
				t.Fatal("unexpected error:", err)
			}

			err = plugin.saveBundleToDisk(bundleName, &buf)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			plugin.loadAndActivateBundlesFromDisk(ctx)

			if tc.expErrs != nil {
				if status, ok := plugin.status[bundleName]; !ok {
					t.Fatalf("Expected to find status for %s, found nil", bundleName)
				} else if status.Type != bundle.SnapshotBundleType {
					t.Fatalf("expected snapshot bundle but got %v", status.Type)
				} else if errs := status.Errors; len(errs) != len(tc.expErrs) {
					t.Fatalf("expected errors:\n\n%v\n\nbut got:\n\n%v", tc.expErrs, errs)
				} else {
					for _, expErr := range tc.expErrs {
						found := false
						for _, err := range errs {
							if strings.Contains(err.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", expErr, errs)
						}
					}
				}
			} else {
				txn := storage.NewTransactionOrDie(ctx, manager.Store)
				defer manager.Store.Abort(ctx, txn)

				ids, err := manager.Store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				} else if len(ids) != 1 {
					t.Fatal("Expected 1 policy")
				}

				bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
				exp := []byte(tc.module)
				if err != nil {
					t.Fatal(err)
				} else if !bytes.Equal(bs, exp) {
					t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
				}

				data, err := manager.Store.Read(ctx, txn, storage.Path{})

				manifestRegoVersionStr := ""
				if tc.bundleRegoVersion != nil {
					manifestRegoVersionStr = fmt.Sprintf(`, "rego_version": %d`, bundleRegoVersion(*tc.bundleRegoVersion))
				}

				runtimeRegoVersion := manager.ParserOptions().RegoVersion.Int()
				var moduleRegoVersion int
				if tc.bundleRegoVersion != nil {
					moduleRegoVersion = tc.bundleRegoVersion.Int()
				} else {
					moduleRegoVersion = runtimeRegoVersion
				}

				var expData any
				if moduleRegoVersion != runtimeRegoVersion {
					expData = util.MustUnmarshalJSON([]byte(fmt.Sprintf(`{
						"foo": {"bar": 1, "baz": "qux"},
						"system": {
							"bundles": {"test-bundle": {"etag": "", "manifest": {"revision": "quickbrownfaux"%s, "roots": [""]}}},
							"modules": {"test-bundle/foo/bar.rego": {"rego_version": %d}}
						}
					}`,
						manifestRegoVersionStr, moduleRegoVersion)))
				} else {
					expData = util.MustUnmarshalJSON([]byte(fmt.Sprintf(`{
						"foo": {"bar": 1, "baz": "qux"},
						"system": {
							"bundles": {"test-bundle": {"etag": "", "manifest": {"revision": "quickbrownfaux"%s, "roots": [""]}}}
						}
					}`,
						manifestRegoVersionStr)))
				}

				if err != nil {
					t.Fatal(err)
				} else if !reflect.DeepEqual(data, expData) {
					t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
				}
			}
		})
	}
}

func pointTo[T any](v T) *T {
	return &v
}

func bundleRegoVersion(v ast.RegoVersion) int {
	switch v {
	case ast.RegoV0:
		return 0
	case ast.RegoV0CompatV1:
		return 0
	case ast.RegoV1:
		return 1
	}
	panic("unknown ast.RegoVersion")
}

func TestLoadAndActivateDepBundlesFromDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	bundleName := "test-bundle-main"
	bundleSource := Source{
		Persist: true,
	}

	bundleNameOther := "test-bundle-lib"
	bundleSourceOther := Source{
		Persist: true,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource
	bundles[bundleNameOther] = &bundleSourceOther

	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	module1 := `
package bar

import rego.v1
import data.foo

default allow = false

allow if {
	foo.is_one(1)
}`

	module2 := `
package foo
import rego.v1

is_one(x) if {
	x == 1
}`

	b1 := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfauxbar", Roots: &[]string{"bar"}},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/bar/policy.rego",
				Path:   "/bar/policy.rego",
				Parsed: ast.MustParseModule(module1),
				Raw:    []byte(module1),
			},
		},
	}

	b1.Manifest.Init()

	b2 := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfauxfoo", Roots: &[]string{"foo"}},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo/policy.rego",
				Path:   "/foo/policy.rego",
				Parsed: ast.MustParseModule(module2),
				Raw:    []byte(module2),
			},
		},
	}

	b2.Manifest.Init()

	var buf1 bytes.Buffer
	if err := bundle.NewWriter(&buf1).UseModulePath(true).Write(b1); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err := plugin.saveBundleToDisk(bundleName, &buf1)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	var buf2 bytes.Buffer
	if err := bundle.NewWriter(&buf2).UseModulePath(true).Write(b2); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = plugin.saveBundleToDisk(bundleNameOther, &buf2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	plugin.loadAndActivateBundlesFromDisk(ctx)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 2 {
		t.Fatal("Expected 2 policies")
	}
}

func TestLoadAndActivateDepBundlesFromDiskMaxAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	dir := t.TempDir()

	bundleName := "test-bundle-main"
	bundleSource := Source{
		Persist: true,
	}

	bundles := map[string]*Source{}
	bundles[bundleName] = &bundleSource

	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	module := `
package bar

import rego.v1
import data.foo

default allow = false

allow if {
	foo.is_one(1)
}`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"bar"}},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/bar/policy.rego",
				Path:   "/bar/policy.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err := plugin.saveBundleToDisk(bundleName, &buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	plugin.loadAndActivateBundlesFromDisk(ctx)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)
	defer manager.Store.Abort(ctx, txn)

	ids, err := manager.Store.ListPolicies(ctx, txn)
	if err != nil {
		t.Fatal(err)
	} else if len(ids) != 0 {
		t.Fatal("Expected 0 policies")
	}
}

func TestPluginOneShotCompileError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	raw1 := `package foo
import rego.v1

p contains x if { x = 1 }`

	b1 := &bundle.Bundle{
		Data: map[string]interface{}{"a": "b"},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Raw:    []byte(raw1),
				Parsed: ast.MustParseModule(raw1),
			},
		},
	}

	b1.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b1, Metrics: metrics.New()})

	ensurePluginState(t, plugin, plugins.StateOK)

	b2 := &bundle.Bundle{
		Data: map[string]interface{}{"a": "b"},
		Modules: []bundle.ModuleFile{
			{
				Path: "/example2.rego",
				Parsed: ast.MustParseModule(`package foo
import rego.v1

p contains x`),
			},
		},
	}

	b2.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b2})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn := storage.NewTransactionOrDie(ctx, manager.Store)

	_, err := manager.Store.GetPolicy(ctx, txn, filepath.Join(bundleName, "example.rego"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err := manager.Store.Read(ctx, txn, storage.Path{"a"})
	if err != nil || !reflect.DeepEqual("b", data) {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}

	manager.Store.Abort(ctx, txn)

	b3 := &bundle.Bundle{
		Data: map[string]interface{}{"foo": map[string]interface{}{"p": "a"}},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example3.rego",
				Parsed: ast.MustParseModule("package foo\np=1"),
			},
		},
	}

	b3.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b3})

	ensurePluginState(t, plugin, plugins.StateOK)

	txn = storage.NewTransactionOrDie(ctx, manager.Store)

	_, err = manager.Store.GetPolicy(ctx, txn, filepath.Join(bundleName, "example.rego"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	data, err = manager.Store.Read(ctx, txn, storage.Path{"a"})
	if err != nil || !reflect.DeepEqual("b", data) {
		t.Fatalf("Expected data to be intact but got: %v, err: %v", data, err)
	}
}

func TestPluginOneShotHTTPError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	ch := make(chan Status)
	listenerName := "test"
	plugin.Register(listenerName, func(status Status) {
		ch <- status
	})
	go plugin.oneShot(ctx, bundleName, download.Update{Error: download.HTTPError{StatusCode: 403}})
	s := <-ch
	if s.HTTPCode != "403" {
		t.Fatal("expected http_code to be 403 instead of ", s.HTTPCode)
	}

	module := "package foo\n\ncorge=1"
	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     util.MustUnmarshalJSON([]byte(`{"foo": {"bar": 1, "baz": "qux"}}`)).(map[string]interface{}),
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo/bar",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s = <-ch
	if s.HTTPCode != "" {
		t.Fatal("expected http_code to be empty instead of ", s.HTTPCode)
	}
}

func TestPluginOneShotActivationRemovesOld(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	module1 := `package example

		p = 1`

	b1 := bundle.Bundle{
		Data: map[string]interface{}{
			"foo": "bar",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example.rego",
				Raw:    []byte(module1),
				Parsed: ast.MustParseModule(module1),
			},
		},
	}

	b1.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b1})

	ensurePluginState(t, plugin, plugins.StateOK)

	module2 := `package example

		p = 2`

	b2 := bundle.Bundle{
		Data: map[string]interface{}{
			"baz": "qux",
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/example2.rego",
				Raw:    []byte(module2),
				Parsed: ast.MustParseModule(module2),
			},
		},
	}

	b2.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b2})

	ensurePluginState(t, plugin, plugins.StateOK)

	err := storage.Txn(ctx, manager.Store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		} else if !slices.Equal([]string{filepath.Join(bundleName, "example2.rego")}, ids) {
			return errors.New("expected updated policy ids")
		}
		data, err := manager.Store.Read(ctx, txn, storage.Path{})
		// remove system key to make comparison simpler
		delete(data.(map[string]interface{}), "system")
		if err != nil {
			return err
		} else if !reflect.DeepEqual(data, map[string]interface{}{"baz": "qux"}) {
			return errors.New("expected updated data")
		}
		return nil
	})
	if err != nil {
		t.Fatal("Unexpected:", err)
	}
}

func TestPluginOneShotActivationConflictingRoots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	bundleNames := []string{"test-bundle1", "test-bundle2", "test-bundle3"}

	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}

	// Start with non-conflicting updates
	plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/c"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// ensure that both bundles are *not* in error status
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, false})

	// Add a third bundle that conflicts with one
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/aa"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateNotReady)

	// ensure that both in the conflict go into error state
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, true})

	// Update to fix conflict
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"b"},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateOK)
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, false})

	// Ensure empty roots conflict with all roots
	plugin.oneShot(ctx, bundleNames[2], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{""},
		},
	}})

	ensurePluginState(t, plugin, plugins.StateOK)
	ensureBundleOverlapStatus(t, plugin, bundleNames, []bool{false, false, true})
}

func TestPluginOneShotActivationPrefixMatchingRoots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleNames := []string{"test-bundle1", "test-bundle2"}

	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}

	plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/c"},
		},
	}})

	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{"a/b/cat"},
		},
	}})

	ensureBundleOverlapStatus(t, &plugin, bundleNames, []bool{false, false})

	// Ensure that empty roots conflict
	plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &bundle.Bundle{
		Manifest: bundle.Manifest{
			Roots: &[]string{""},
		},
	}})

	ensureBundleOverlapStatus(t, &plugin, bundleNames, []bool{false, true})
}

func ensureBundleOverlapStatus(t *testing.T, p *Plugin, bundleNames []string, expectedErrs []bool) {
	t.Helper()
	for i, name := range bundleNames {
		hasErr := p.status[name].Message != ""
		if expectedErrs[i] && !hasErr {
			t.Fatalf("expected bundle %s to be in an error state", name)
		} else if !expectedErrs[i] && hasErr {
			t.Fatalf("unexpected error state for bundle %s", name)
		} else if hasErr && expectedErrs[i] && !strings.Contains(p.status[name].Message, "detected overlapping roots") {
			t.Fatalf("expected bundle overlap error for bundle %s, got: %s", name, p.status[name].Message)
		}
	}
}

func TestPluginListener(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := New(&Config{}, manager)
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	ch := make(chan Status)

	listenerName := "test"
	plugin.Register(listenerName, func(status Status) {
		ch <- status
	})

	if len(plugin.listeners) != 1 || plugin.listeners[listenerName] == nil {
		t.Fatal("Listener not properly registered")
	}

	module := `package gork
import rego.v1

p contains x if { x = 1 }`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s1 := <-ch

	validateStatus(t, s1, "quickbrownfaux", false)

	module = `package gork
import rego.v1

p contains x`

	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that next update is failed.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s2 := <-ch

	validateStatus(t, s2, "quickbrownfaux", true)

	module = `package gork
import rego.v1

p contains 1`
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that the new update is successful.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s3 := <-ch

	validateStatus(t, s3, "fancybluederg", false)

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, bundleName, download.Update{})
	s4 := <-ch

	// Nothing should have changed in the update
	validateStatus(t, s4, s3.ActiveRevision, false)

	plugin.Unregister(listenerName)
	if len(plugin.listeners) != 0 {
		t.Fatal("Listener not properly unregistered")
	}
}

func isErrStatus(s Status) bool {
	return s.Code != "" || len(s.Errors) != 0 || s.Message != ""
}

func validateStatus(t *testing.T, actual Status, expected string, expectStatusErr bool) {
	t.Helper()

	if expectStatusErr && !isErrStatus(actual) {
		t.Errorf("Expected status to be in an error state, but no error has occurred.")
	} else if !expectStatusErr && isErrStatus(actual) {
		t.Errorf("Unexpected error status %v", actual)
	}

	if actual.ActiveRevision != expected {
		t.Errorf("Expected status revision %s, got %s", expected, actual.ActiveRevision)
	}
}

func TestPluginListenerErrorClearedOn304(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)
	ch := make(chan Status)

	plugin.Register("test", func(status Status) {
		ch <- status
	})

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{"foo": "bar"},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok.
	go plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})
	s1 := <-ch

	if s1.ActiveRevision != "quickbrownfaux" || s1.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	// Test that service error triggers failure notification.
	go plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("some error")})
	s2 := <-ch

	if s2.ActiveRevision != "quickbrownfaux" || s2.Code == "" {
		t.Fatal("Unexpected status update, got:", s2)
	}

	// Test that service recovery triggers healthy notification.
	go plugin.oneShot(ctx, bundleName, download.Update{})
	s3 := <-ch

	if s3.ActiveRevision != "quickbrownfaux" || s3.Code != "" {
		t.Fatal("Unexpected status update, got:", s3)
	}
}

func TestPluginBulkListener(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleNames := []string{
		"b1",
		"b2",
		"b3",
	}
	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}
	bulkChan := make(chan map[string]*Status)

	listenerName := "bulk test"
	plugin.RegisterBulkListener(listenerName, func(status map[string]*Status) {
		bulkChan <- status
	})

	if len(plugin.bulkListeners) != 1 || plugin.bulkListeners[listenerName] == nil {
		t.Fatal("Bulk listener not properly registered")
	}

	module := `package gork
import rego.v1

p contains x if { x = 1 }`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
			Roots:    &[]string{"gork"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s1 := <-bulkChan

	s := s1[bundleNames[0]]
	if s.ActiveRevision != "quickbrownfaux" || s.Code != "" {
		t.Fatal("Unexpected status update, got:", s1)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s1[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s1)
		}
		// they should be defaults at this point
		if !s.Equal(&Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s1)
		}
	}

	module = `package gork
import rego.v1

p contains x`

	b.Manifest.Revision = "slowgreenburd"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that next update is failed.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s2 := <-bulkChan

	s = s2[bundleNames[0]]
	if s.ActiveRevision != "quickbrownfaux" || s.Code == "" || s.Message == "" || len(s.Errors) == 0 {
		t.Fatal("Unexpected status update, got:", s2)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s2[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s2)
		}
		// they should be still defaults
		if !s.Equal(&Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s2)
		}
	}

	module = `package gork
import rego.v1

p contains 1`
	b.Manifest.Revision = "fancybluederg"
	b.Modules[0] = bundle.ModuleFile{
		Path:   "/foo.rego",
		Raw:    []byte(module),
		Parsed: ast.MustParseModule(module),
	}

	// Test that new update is successful.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s3 := <-bulkChan

	s = s3[bundleNames[0]]
	if s.ActiveRevision != "fancybluederg" || s.Code != "" || s.Message != "" || len(s.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s3)
	}

	for i := 1; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s3[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s3)
		}
		// they should still be defaults
		if !s.Equal(&Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s3)
		}
	}

	// Test that empty download update results in status update.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{})
	s4 := <-bulkChan

	s = s4[bundleNames[0]]
	if s.ActiveRevision != "fancybluederg" || s.Code != "" || s.Message != "" || len(s.Errors) != 0 {
		t.Errorf("Unexpected same status update for bundle %q, got: %v", bundleNames[0], s)
	}

	// Test updates the other bundles
	module = `package p1
import rego.v1

p contains x if { x = 1 }`

	b1 := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "123",
			Roots:    &[]string{"p1"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo1.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b1.Manifest.Init()

	// Test that new update is successful.
	go plugin.oneShot(ctx, bundleNames[1], download.Update{Bundle: &b1})
	s5 := <-bulkChan

	s = s5[bundleNames[1]]
	if s.ActiveRevision != "123" || s.Code != "" || s.Message != "" || len(s.Errors) != 0 {
		t.Fatal("Unexpected status update, got:", s5)
	}

	if !s5[bundleNames[0]].Equal(s4[bundleNames[0]]) {
		t.Fatalf("Expected bundle %q to have the same status as before updating bundle %q, got: %+v", bundleNames[0], bundleNames[1], s5)
	}

	for i := 2; i < len(bundleNames); i++ {
		name := bundleNames[i]
		s, ok := s5[name]
		if !ok {
			t.Errorf("Expected to have bundle status for %q included in update, got: %+v", name, s5)
		}
		// they should still be defaults
		if !s.Equal(&Status{Name: name}) {
			t.Errorf("Expected bundle %q to have an empty status, got: %+v", name, s5)
		}
	}

	plugin.UnregisterBulkListener(listenerName)
	if len(plugin.bulkListeners) != 0 {
		t.Fatal("Bulk listener not properly unregistered")
	}
}

func TestPluginBulkListenerStatusCopyOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleNames := []string{
		"b1",
		"b2",
		"b3",
	}
	for _, name := range bundleNames {
		plugin.status[name] = &Status{Name: name}
		plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)
	}
	bulkChan := make(chan map[string]*Status)

	plugin.RegisterBulkListener("bulk test", func(status map[string]*Status) {
		bulkChan <- status
	})

	module := `package gork
import rego.v1

p contains x if { x = 1 }`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
			Roots:    &[]string{"gork"},
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	// Test that initial bundle is ok. Defer to separate goroutine so we can
	// check result with channel.
	go plugin.oneShot(ctx, bundleNames[0], download.Update{Bundle: &b})
	s1 := <-bulkChan

	// Modify the status map received and ensure it doesn't affect the one on the plugin
	delete(s1, "b1")

	if _, ok := plugin.status["b1"]; !ok {
		t.Fatalf("Expected status for 'b1' to still be in 'plugin.status'")
	}
}

func TestPluginActivateScopedBundle(t *testing.T) {
	t.Parallel()

	readMode := []struct {
		note    string
		readAst bool
	}{
		{
			note:    "read raw",
			readAst: false,
		},
		{
			note:    "read ast",
			readAst: true,
		},
	}

	for _, rm := range readMode {
		t.Run(rm.note, func(t *testing.T) {
			ctx := context.Background()
			manager := getTestManagerWithOpts(nil, inmem.NewWithOpts(inmem.OptReturnASTValuesOnRead(rm.readAst)))
			plugin := Plugin{
				manager:     manager,
				status:      map[string]*Status{},
				etags:       map[string]string{},
				downloaders: map[string]Loader{},
			}
			bundleName := "test-bundle"
			plugin.status[bundleName] = &Status{Name: bundleName}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

			// Transact test data and policies that represent data coming from
			// _outside_ the bundle. The test will verify that data _outside_
			// the bundle is both not erased and is overwritten appropriately.
			//
			// The test data claims a/{a1-6} where even paths are policy and
			// odd paths are raw JSON.
			if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {

				externalData := map[string]interface{}{"a": map[string]interface{}{"a1": "x1", "a3": "x2", "a5": "x3"}}

				if err := manager.Store.Write(ctx, txn, storage.AddOp, storage.Path{}, externalData); err != nil {
					return err
				}
				if err := manager.Store.UpsertPolicy(ctx, txn, "some/id1", []byte(`package a.a2`)); err != nil {
					return err
				}
				if err := manager.Store.UpsertPolicy(ctx, txn, "some/id2", []byte(`package a.a4`)); err != nil {
					return err
				}
				return manager.Store.UpsertPolicy(ctx, txn, "some/id3", []byte(`package a.a6`))
			}); err != nil {
				t.Fatal(err)
			}

			// Activate a bundle that is scoped to a/a1 and a/a2. This will
			// erase and overwrite the external data at these paths but leave
			// a3-6 untouched.
			module := "package a.a2\n\nbar=1"

			b := bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
				Data: map[string]interface{}{
					"a": map[string]interface{}{
						"a1": "foo",
					},
				},
				Modules: []bundle.ModuleFile{
					{
						Path:   "bundle/id1",
						Parsed: ast.MustParseModule(module),
						Raw:    []byte(module),
					},
				},
			}

			b.Manifest.Init()

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

			// Ensure a/a3-6 are intact. a1-2 are overwritten by bundle, and
			// that the manifest has been written to storage.
			exp := `{"a1": "foo", "a3": "x2", "a5": "x3"}`
			var expData interface{}
			if rm.readAst {
				expData = ast.MustParseTerm(exp).Value
			} else {
				expData = util.MustUnmarshalJSON([]byte(exp))
			}
			expIDs := []string{filepath.Join(bundleName, "bundle", "id1"), "some/id2", "some/id3"}
			validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux", nil)

			// Activate a bundle that is scoped to a/a3 ad a/a6. Include a function
			// inside package a.a4 that we can depend on outside of the bundle scope to
			// exercise the compile check with remaining modules.
			module = "package a.a4\n\nbar=1\n\nfunc(x) = x"

			b = bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux-2", Roots: &[]string{"a/a3", "a/a4"},
					Metadata: map[string]interface{}{
						"a": map[string]interface{}{
							"a1": "deadbeef",
						},
					},
				},
				Data: map[string]interface{}{
					"a": map[string]interface{}{
						"a3": "foo",
					},
				},
				Modules: []bundle.ModuleFile{
					{
						Path:   "bundle/id2",
						Parsed: ast.MustParseModule(module),
						Raw:    []byte(module),
					},
				},
			}

			b.Manifest.Init()
			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

			// Ensure a/a5-a6 are intact. a3 and a4 are overwritten by bundle.
			exp = `{"a3": "foo", "a5": "x3"}`
			if rm.readAst {
				expData = ast.MustParseTerm(exp).Value
			} else {
				expData = util.MustUnmarshalJSON([]byte(exp))
			}
			expIDs = []string{filepath.Join(bundleName, "bundle", "id2"), "some/id3"}
			validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux-2",
				map[string]interface{}{
					"a": map[string]interface{}{"a1": "deadbeef"},
				})

			// Upsert policy outside of bundle scope that depends on bundle.
			if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
				return manager.Store.UpsertPolicy(ctx, txn, "not_scoped", []byte("package not_scoped\np { data.a.a4.func(1) = 1 }"))
			}); err != nil {
				t.Fatal(err)
			}

			b = bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux-3", Roots: &[]string{"a/a3", "a/a4"}},
				Data:     map[string]interface{}{},
				Modules:  []bundle.ModuleFile{},
			}

			b.Manifest.Init()
			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

			// Ensure bundle activation failed by checking that previous revision is
			// still active.
			expIDs = []string{filepath.Join(bundleName, "bundle", "id2"), "not_scoped", "some/id3"}
			validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux-2",
				map[string]interface{}{
					"a": map[string]interface{}{"a1": "deadbeef"},
				})
		})
	}
}

func TestPluginSetCompilerOnContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	module := `
		package test

		p = 1
		`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux"},
		Data:     map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/test.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	events := []storage.TriggerEvent{}

	if err := storage.Txn(ctx, manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		_, err := manager.Store.Register(ctx, txn, storage.TriggerConfig{
			OnCommit: func(_ context.Context, _ storage.Transaction, event storage.TriggerEvent) {
				events = append(events, event)
			},
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	exp := ast.MustParseModule(module)

	// Expect two events. One for trigger registration, one for policy update.
	if len(events) != 2 {
		t.Fatalf("Expected 2 events but got: %+v", events)
	} else if compiler := plugins.GetCompilerOnContext(events[1].Context); compiler == nil {
		t.Fatalf("Expected compiler on 2nd event but got: %+v", events)
	} else if !compiler.Modules[filepath.Join(bundleName, "test.rego")].Equal(exp) {
		t.Fatalf("Expected module on compiler but got: %v", compiler.Modules)
	}
}

func getTestManager() *plugins.Manager {
	return getTestManagerWithOpts(nil)
}

func getTestManagerWithOpts(config []byte, stores ...storage.Store) *plugins.Manager {
	store := inmemtst.New()
	if len(stores) == 1 {
		store = stores[0]
	}

	manager, err := plugins.New(config, "test-instance-id", store)
	if err != nil {
		panic(err)
	}
	return manager
}

func TestPluginReconfigure(t *testing.T) {
	t.Parallel()

	tsURLBase := "/opa-test/"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, tsURLBase) {
			t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
		}
		fmt.Fprintln(w, "") // Note: this is an invalid bundle and will fail the download
	}))
	defer ts.Close()

	ctx := context.Background()
	manager := getTestManager()

	serviceName := "test-svc"
	err := manager.Reconfigure(&config.Config{
		Services: []byte(fmt.Sprintf("{\"%s\":{ \"url\": \"%s\"}}", serviceName, ts.URL+tsURLBase)),
	})
	if err != nil {
		t.Fatalf("Error configuring plugin manager: %s", err)
	}

	plugin := New(&Config{}, manager)

	var delay int64 = 10

	triggerPolling := plugins.TriggerPeriodic
	baseConf := download.Config{Polling: download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}, Trigger: &triggerPolling}

	// Expect the plugin to emit a "not ready" status update each time we change the configuration
	updateCount := 0
	manager.RegisterPluginStatusListener(t.Name(), func(status map[string]*plugins.Status) {
		updateCount++
		bStatus, ok := status[Name]
		if !ok {
			t.Errorf("Expected to find status for %s in plugin status update, got: %+v", Name, status)
		}

		if bStatus.State != plugins.StateNotReady {
			t.Errorf("Expected plugin status update to have state = %s, got %s", plugins.StateNotReady, bStatus.State)
		}
	})

	// Note: test stages are accumulating state with reconfigures between them, the order does matter!
	// Each stage defines the new config, side effects are validated.
	stages := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "start with single legacy bundle",
			cfg: &Config{
				Name:    "bundle.tar.gz",
				Service: serviceName,
				Config:  baseConf,
				// Note: the config validation and default injection will add an entry
				// to the Bundles map for the older style configuration.
				Bundles: map[string]*Source{
					"bundle.tar.gz": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle.tar.gz"},
				},
			},
		},
		{
			name: "switch to multi-bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle.tar.gz"},
				},
			},
		},
		{
			name: "add second bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle1.tar.gz"},
					"b2": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "remove initial bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "Update single bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/new/path/bundles/bundle2.tar.gz"},
				},
			},
		},
		{
			name: "Add multiple new bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b3": {Config: baseConf, Service: serviceName, Resource: "/bundle3.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle4.tar.gz"},
					"b5": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle5.tar.gz"},
				},
			},
		},
		{
			name: "Remove multiple bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/new/path/bundles/bundle2.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/bundles/bundle4.tar.gz"},
				},
			},
		},
		{
			name: "Update multiple bundles",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b2": {Config: baseConf, Service: serviceName, Resource: "/update2/bundle2.tar.gz"},
					"b4": {Config: baseConf, Service: serviceName, Resource: "/update2/bundle4.tar.gz"},
				},
			},
		},
		{
			name: "Remove and add bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "bundle6.tar.gz"},
				},
			},
		},
		{
			name: "Add and update bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "/update3/bundle6.tar.gz"},
					"b7": {Config: baseConf, Service: serviceName, Resource: "bundle7.tar.gz"},
					"b8": {Config: baseConf, Service: serviceName, Resource: "bundle8.tar.gz"},
				},
			},
		},
		{
			name: "Update and remove",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b6": {Config: baseConf, Service: serviceName, Resource: "/update4/bundle6.tar.gz"},
					"b8": {Config: baseConf, Service: serviceName, Resource: "bundle8.tar.gz"},
				},
			},
		},
		// Add, Update, and Remove
		{
			name: "Add update and remove",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b8": {Config: baseConf, Service: serviceName, Resource: "/update5/bundle8.tar.gz"},
					"b9": {Config: baseConf, Service: serviceName, Resource: "bundle9.tar.gz"},
				},
			},
		},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {

			plugin.Reconfigure(ctx, stage.cfg)

			var expectedNumBundles int
			if stage.cfg.Name != "" {
				expectedNumBundles = 1
			} else {
				expectedNumBundles = len(stage.cfg.Bundles)
			}

			if expectedNumBundles != len(plugin.downloaders) {
				t.Fatalf("Expected a downloader for each configured bundle, expected %d found %d", expectedNumBundles, len(plugin.downloaders))
			}

			if expectedNumBundles != len(plugin.status) {
				t.Fatalf("Expected a status entry for each configured bundle, expected %d found %d", expectedNumBundles, len(plugin.status))
			}

			for name := range stage.cfg.Bundles {
				if _, found := plugin.downloaders[name]; !found {
					t.Fatalf("bundle %q not found in downloaders map", name)
				}

				if _, found := plugin.status[name]; !found {
					t.Fatalf("bundle %q not found in status map", name)
				}
			}
		})
	}
	if len(stages) != updateCount {
		t.Fatalf("Expected to have received %d updates, got %d", len(stages), updateCount)
	}
}

func TestPluginRequestVsDownloadTimestamp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := &bundle.Bundle{}
	b.Manifest.Init()

	// simulate HTTP 200 response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	// The time resolution is 1ns so sleeping for 1ms should be more than enough.
	time.Sleep(time.Millisecond)

	// simulate HTTP 304 response from downloader.
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: nil})

	if plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to differ from download and request")
	}

	// simulate HTTP 200 response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: b})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download and request")
	}

	// simulate error response from downloader
	plugin.oneShot(ctx, bundleName, download.Update{Error: errors.New("xxx")})

	if plugin.status[bundleName].LastSuccessfulDownload != plugin.status[bundleName].LastSuccessfulRequest || plugin.status[bundleName].LastSuccessfulDownload == plugin.status[bundleName].LastRequest {
		t.Fatal("expected last successful request to be same as download but different from request")
	}
}

func TestReconfigurePlugin_OneShot_BundleDeactivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note               string
		runtimeRegoVersion ast.RegoVersion
		bundleRegoVersion  ast.RegoVersion
		moduleRegoVersion  ast.RegoVersion
		module             string
	}{
		{
			note:               "v0 runtime, v0 bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v0 runtime, v1 bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
		{
			note:               "v0 runtime, custom bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoUndefined,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v1 runtime, v0 bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v1 runtime, v1 bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
		{
			note:               "v1 runtime, custom bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoUndefined,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(), plugins.WithParserOptions(ast.ParserOptions{RegoVersion: tc.runtimeRegoVersion}))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			bundleName := "test-bundle"

			plugin := Plugin{
				manager:     manager,
				status:      map[string]*Status{},
				etags:       map[string]string{},
				downloaders: map[string]Loader{},
				config: Config{
					Bundles: map[string]*Source{
						bundleName: {
							Service: "s1",
						},
					},
				},
			}

			plugin.status[bundleName] = &Status{Name: bundleName}
			plugin.downloaders[bundleName] = download.New(download.Config{Trigger: pointTo(plugins.TriggerManual)}, plugin.manager.Client(""), bundleName)

			b := bundle.Bundle{
				Manifest: bundle.Manifest{
					Revision: "quickbrownfaux",
					Roots:    &[]string{"a"},
				},
				Modules: []bundle.ModuleFile{
					{
						Path:   "bundle/id1",
						Parsed: ast.MustParseModuleWithOpts(tc.module, ast.ParserOptions{RegoVersion: tc.moduleRegoVersion}),
						Raw:    []byte(tc.module),
					},
				},
			}

			if tc.bundleRegoVersion != ast.RegoUndefined {
				b.Manifest.RegoVersion = pointTo(tc.bundleRegoVersion.Int())
			}

			b.Manifest.Init()

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

			// Ensure it has been activated

			txn := storage.NewTransactionOrDie(ctx, manager.Store)

			ids, err := manager.Store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expIDs := []string{"test-bundle/bundle/id1"}

			sort.Strings(ids)
			sort.Strings(expIDs)

			if !slices.Equal(ids, expIDs) {
				t.Fatalf("expected ids %v but got %v", expIDs, ids)
			}

			manager.Store.Abort(ctx, txn)

			// reconfigure with dropped bundle

			plugin.Reconfigure(ctx, &Config{
				Bundles: map[string]*Source{},
			})

			// bundle was removed from store

			txn = storage.NewTransactionOrDie(ctx, manager.Store)

			ids, err = manager.Store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expIDs = []string{}

			sort.Strings(ids)

			if !slices.Equal(ids, expIDs) {
				t.Fatalf("expected ids %v but got %v", expIDs, ids)
			}

			manager.Store.Abort(ctx, txn)
		})
	}
}

func TestReconfigurePlugin_ManagerInit_BundleDeactivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note               string
		runtimeRegoVersion ast.RegoVersion
		bundleRegoVersion  ast.RegoVersion
		moduleRegoVersion  ast.RegoVersion
		module             string
	}{
		{
			note:               "v0 runtime, v0 bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v0 runtime, v1 bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
		{
			note:               "v0 runtime, custom bundle",
			runtimeRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoUndefined,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v1 runtime, v0 bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			moduleRegoVersion:  ast.RegoV0,
			module: `package a
					p[42] { true }`,
		},
		{
			note:               "v1 runtime, v1 bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
		{
			note:               "v1 runtime, custom bundle",
			runtimeRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoUndefined,
			moduleRegoVersion:  ast.RegoV1,
			module: `package a
					p contains 42 if { true }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			bundleName := "test-bundle"

			b := bundle.Bundle{
				Manifest: bundle.Manifest{
					Revision: "quickbrownfaux",
					Roots:    &[]string{"a"},
				},
				Modules: []bundle.ModuleFile{
					{
						Path:   "bundle/id1",
						Parsed: ast.MustParseModuleWithOpts(tc.module, ast.ParserOptions{RegoVersion: tc.moduleRegoVersion}),
						Raw:    []byte(tc.module),
					},
				},
			}

			if tc.bundleRegoVersion != ast.RegoUndefined {
				b.Manifest.RegoVersion = pointTo(tc.bundleRegoVersion.Int())
			}

			b.Manifest.Init()

			bundles := map[string]*bundle.Bundle{
				bundleName: &b,
			}

			ctx := context.Background()
			manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
				plugins.WithParserOptions(ast.ParserOptions{RegoVersion: tc.runtimeRegoVersion}),
				plugins.InitBundles(bundles))

			if err := manager.Init(ctx); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			plugin := Plugin{
				manager:     manager,
				status:      map[string]*Status{},
				etags:       map[string]string{},
				downloaders: map[string]Loader{},
				config: Config{
					Bundles: map[string]*Source{
						bundleName: {
							Service: "s1",
						},
					},
				},
			}

			plugin.status[bundleName] = &Status{Name: bundleName}
			plugin.downloaders[bundleName] = download.New(download.Config{Trigger: pointTo(plugins.TriggerManual)}, plugin.manager.Client(""), bundleName)

			// Ensure it has been activated

			txn := storage.NewTransactionOrDie(ctx, manager.Store)

			ids, err := manager.Store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expIDs := []string{"test-bundle/bundle/id1"}

			sort.Strings(ids)
			sort.Strings(expIDs)

			if !slices.Equal(ids, expIDs) {
				t.Fatalf("expected ids %v but got %v", expIDs, ids)
			}

			manager.Store.Abort(ctx, txn)

			// reconfigure with dropped bundle

			plugin.Reconfigure(ctx, &Config{
				Bundles: map[string]*Source{},
			})

			// bundle was removed from store

			txn = storage.NewTransactionOrDie(ctx, manager.Store)

			ids, err = manager.Store.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expIDs = []string{}

			sort.Strings(ids)

			if !slices.Equal(ids, expIDs) {
				t.Fatalf("expected ids %v but got %v", expIDs, ids)
			}

			manager.Store.Abort(ctx, txn)
		})
	}
}

func TestUpgradeLegacyBundleToMuiltiBundleSameBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()
	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}
	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	// Start with a "legacy" style config for a single bundle
	plugin.config = Config{
		Bundles: map[string]*Source{
			bundleName: {
				Service: "s1",
			},
		},
		Name:    bundleName,
		Service: "s1",
		Prefix:  nil,
	}

	module := "package a.a1\n\nbar=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure it has been activated
	expData := util.MustUnmarshalJSON([]byte(`{"a2": "foo"}`))
	expIDs := []string{"bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux", nil)

	if plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in non-multi bundle config mode")
	}

	// Update to the newer style config with the same bundle
	multiBundleConf := &Config{
		Bundles: map[string]*Source{
			bundleName: {
				Service: "s1",
			},
		},
	}

	plugin.Reconfigure(ctx, multiBundleConf)
	b.Manifest.Revision = "quickbrownfaux-2"
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// The only thing that should have changed is the store id for the policy
	expIDs = []string{"test-bundle/bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux-2", nil)

	// Make sure the legacy path is gone now that we are in multi-bundle mode
	var actual string
	err := storage.Txn(ctx, plugin.manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		if actual, err = bundle.LegacyReadRevisionFromStore(ctx, plugin.manager.Store, txn); err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest revision from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}
	if actual != "" {
		t.Fatalf("Expected to not find manifest revision but got %s", actual)
	}

	if !plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in multi bundle config mode")
	}
}

func TestUpgradeLegacyBundleToMultiBundleNewBundles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := getTestManager()

	plugin := Plugin{
		manager:     manager,
		status:      map[string]*Status{},
		etags:       map[string]string{},
		downloaders: map[string]Loader{},
	}

	bundleName := "test-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	tsURLBase := "/opa-test/"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, tsURLBase) {
			t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
		}
		fmt.Fprintln(w, "") // Note: this is an invalid bundle and will fail the download
	}))
	defer ts.Close()

	serviceName := "test-svc"
	err := manager.Reconfigure(&config.Config{
		Services: []byte(fmt.Sprintf("{\"%s\":{ \"url\": \"%s\"}}", serviceName, ts.URL+tsURLBase)),
	})
	if err != nil {
		t.Fatalf("Error configuring plugin manager: %s", err)
	}

	var delay int64 = 10
	triggerPolling := plugins.TriggerPeriodic
	downloadConf := download.Config{Polling: download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}, Trigger: &triggerPolling}

	// Start with a "legacy" style config for a single bundle
	plugin.config = Config{
		Bundles: map[string]*Source{
			bundleName: {
				Config:  downloadConf,
				Service: serviceName,
			},
		},
		Name:    bundleName,
		Service: serviceName,
		Prefix:  nil,
	}

	module := "package a.a1\n\nbar=1"

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

	// Ensure it has been activated
	expData := util.MustUnmarshalJSON([]byte(`{"a2": "foo"}`))
	expIDs := []string{"bundle/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux", nil)

	if plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in non-multi bundle config mode")
	}

	// Update to the newer style config with a new bundle
	multiBundleConf := &Config{
		Bundles: map[string]*Source{
			"b2": {
				Config:  downloadConf,
				Service: serviceName,
			},
		},
	}

	delete(plugin.downloaders, bundleName)
	plugin.downloaders["b2"] = download.New(download.Config{}, plugin.manager.Client(""), "b2")
	plugin.Reconfigure(ctx, multiBundleConf)

	module = "package a.c\n\nbar=1"
	b = bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "b2-1", Roots: &[]string{"a/b2", "a/c"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}
	b.Manifest.Init()
	plugin.oneShot(ctx, "b2", download.Update{Bundle: &b})

	expData = util.MustUnmarshalJSON([]byte(`{"b2": "foo"}`))
	expIDs = []string{"b2/id1"}
	validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, "b2", "b2-1", nil)

	// Make sure the legacy path is gone now that we are in multi-bundle mode
	var actual string
	err = storage.Txn(ctx, plugin.manager.Store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		if actual, err = bundle.LegacyReadRevisionFromStore(ctx, plugin.manager.Store, txn); err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest revision from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}
	if actual != "" {
		t.Fatalf("Expected to not find manifest revision but got %s", actual)
	}

	if !plugin.config.IsMultiBundle() {
		t.Fatalf("Expected plugin to be in multi bundle config mode")
	}
}

func TestLegacyBundleDataRead(t *testing.T) {
	t.Parallel()

	readModes := []struct {
		note    string
		readAst bool
	}{
		{
			note:    "read raw",
			readAst: false,
		},
		{
			note:    "read ast",
			readAst: true,
		},
	}

	for _, rm := range readModes {
		t.Run(rm.note, func(t *testing.T) {
			ctx := context.Background()
			manager := getTestManagerWithOpts(nil, inmem.NewWithOpts(inmem.OptReturnASTValuesOnRead(rm.readAst)))

			plugin := Plugin{
				manager:     manager,
				status:      map[string]*Status{},
				etags:       map[string]string{},
				downloaders: map[string]Loader{},
			}

			bundleName := "test-bundle"
			plugin.status[bundleName] = &Status{Name: bundleName}
			plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

			tsURLBase := "/opa-test/"
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, tsURLBase) {
					t.Fatalf("Invalid request URL path: %s, expected prefix %s", r.URL.Path, tsURLBase)
				}
				fmt.Fprintln(w, "") // Note: this is an invalid bundle and will fail the download
			}))
			defer ts.Close()

			serviceName := "test-svc"
			err := manager.Reconfigure(&config.Config{
				Services: []byte(fmt.Sprintf("{\"%s\":{ \"url\": \"%s\"}}", serviceName, ts.URL+tsURLBase)),
			})
			if err != nil {
				t.Fatalf("Error configuring plugin manager: %s", err)
			}

			var delay int64 = 10
			triggerPolling := plugins.TriggerPeriodic
			downloadConf := download.Config{Polling: download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}, Trigger: &triggerPolling}

			// Start with a "legacy" style config for a single bundle
			plugin.config = Config{
				Bundles: map[string]*Source{
					bundleName: {
						Config:  downloadConf,
						Service: serviceName,
					},
				},
				Name:    bundleName,
				Service: serviceName,
				Prefix:  nil,
			}

			module := "package a.a1\n\nbar=1"

			b := bundle.Bundle{
				Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
				Data: map[string]interface{}{
					"a": map[string]interface{}{
						"a2": "foo",
					},
				},
				Modules: []bundle.ModuleFile{
					{
						Path:   "bundle/id1",
						Parsed: ast.MustParseModule(module),
						Raw:    []byte(module),
					},
				},
			}

			b.Manifest.Init()

			if plugin.config.IsMultiBundle() {
				t.Fatalf("Expected plugin to be in non-multi bundle config mode")
			}

			plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b})

			exp := `{"a2": "foo"}`
			var expData interface{}
			if rm.readAst {
				expData = ast.MustParseTerm(exp).Value
			} else {
				expData = util.MustUnmarshalJSON([]byte(exp))
			}

			expIDs := []string{"bundle/id1"}
			validateStoreState(ctx, t, manager.Store, "/a", expData, expIDs, bundleName, "quickbrownfaux", nil)
		})
	}
}

func TestSaveBundleToDiskNew(t *testing.T) {
	t.Parallel()

	manager := getTestManager()

	dir := t.TempDir()

	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	err := plugin.saveBundleToDisk("foo", getTestRawBundle(t))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestSaveBundleToDiskNewConfiguredPersistDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	manager := getTestManager()
	manager.Config.PersistenceDirectory = &dir
	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)

	err := plugin.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	err = plugin.saveBundleToDisk("foo", getTestRawBundle(t))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expectBundlePath := filepath.Join(dir, "bundles", "foo", "bundle.tar.gz")
	_, err = os.Stat(expectBundlePath)
	if err != nil {
		t.Errorf("expected bundle persisted at path %v, %v", expectBundlePath, err)
	}
}

func TestSaveBundleToDiskOverWrite(t *testing.T) {
	t.Parallel()

	manager := getTestManager()

	// test to check existing bundle is replaced
	dir := t.TempDir()

	bundles := map[string]*Source{}
	plugin := New(&Config{Bundles: bundles}, manager)
	plugin.bundlePersistPath = filepath.Join(dir, ".opa")

	bundleName := "foo"
	bundleDir := filepath.Join(plugin.bundlePersistPath, bundleName)

	err := os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b2 := writeTestBundleToDisk(t, bundleDir, false)

	module := "package a.a1\n\nbar=1"

	newBundle := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "quickbrownfaux", Roots: &[]string{"a/a1", "a/a2"}},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"a2": "foo",
			},
		},
		Modules: []bundle.ModuleFile{
			{
				Path:   "bundle/id1",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}
	newBundle.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(newBundle); err != nil {
		t.Fatal("unexpected error:", err)
	}

	err = plugin.saveBundleToDisk("foo", &buf)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	actual, err := plugin.loadBundleFromDisk(plugin.bundlePersistPath, "foo", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if actual.Equal(b2) {
		t.Fatal("expected existing bundle to be overwritten")
	}
}

func TestSaveCurrentBundleToDisk(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()

	bundlePath, err := saveCurrentBundleToDisk(srcDir, getTestRawBundle(t))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	_, err = saveCurrentBundleToDisk(srcDir, nil)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expErrMsg := "no raw bundle bytes to persist to disk"
	if err.Error() != expErrMsg {
		t.Fatalf("expected error: %v but got: %v", expErrMsg, err)
	}
}

func TestLoadBundleFromDisk(t *testing.T) {
	t.Parallel()

	manager := getTestManager()
	plugin := New(&Config{}, manager)

	// no bundle on disk
	_, err := plugin.loadBundleFromDisk("foo", "bar", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// create a test bundle and load it from disk
	dir := t.TempDir()

	bundleName := "foo"
	bundleDir := filepath.Join(dir, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b := writeTestBundleToDisk(t, bundleDir, false)

	result, err := plugin.loadBundleFromDisk(dir, bundleName, nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the test bundle to be equal to the one loaded from disk")
	}
}

func TestLoadBundleFromDiskV1Compatible(t *testing.T) {
	t.Parallel()

	popts := ast.ParserOptions{RegoVersion: ast.RegoV1}

	manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(), plugins.WithParserOptions(popts))
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	plugin := New(&Config{}, manager)

	// create a test bundle and load it from disk
	dir := t.TempDir()

	bundleName := "foo"
	bundleDir := filepath.Join(dir, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// v1.0 policy
	policy := `package test
p contains 1 if {
	input.x == 2
}`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{Revision: "test-revision"},
		Modules: []bundle.ModuleFile{
			{
				URL:    `policy.rego`,
				Path:   `/policy.rego`,
				Raw:    []byte(policy),
				Parsed: ast.MustParseModuleWithOpts(policy, popts),
			},
		},
		Data: map[string]interface{}{},
	}

	b.Manifest.Init()

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.tar.gz"), buf.Bytes(), 0644); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	result, err := plugin.loadBundleFromDisk(dir, bundleName, nil)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the test bundle to be equal to the one loaded from disk")
	}
}

func TestLoadSignedBundleFromDisk(t *testing.T) {
	t.Parallel()

	manager := getTestManager()
	plugin := New(&Config{}, manager)

	// no bundle on disk
	_, err := plugin.loadBundleFromDisk("foo", "bar", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// create a test signed bundle and load it from disk
	dir := t.TempDir()

	bundleName := "foo"
	bundleDir := filepath.Join(dir, bundleName)

	err = os.MkdirAll(bundleDir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	b := writeTestBundleToDisk(t, bundleDir, true)

	src := Source{
		Signing: bundle.NewVerificationConfig(map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil),
	}

	result, err := plugin.loadBundleFromDisk(dir, bundleName, &src)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !result.Equal(b) {
		t.Fatal("expected the test bundle to be equal to the one loaded from disk")
	}

	if !reflect.DeepEqual(result.Signatures, b.Signatures) {
		t.Fatal("Expected signatures to be same")
	}
}

func TestGetDefaultBundlePersistPath(t *testing.T) {
	t.Parallel()

	plugin := New(&Config{}, getTestManager())
	path, err := plugin.getBundlePersistPath()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !strings.HasSuffix(path, ".opa/bundles") {
		t.Fatal("expected default persist path to end with '.opa/bundles' dir")
	}
}

func TestConfiguredBundlePersistPath(t *testing.T) {
	t.Parallel()

	persistPath := "/var/opa"
	manager := getTestManager()
	manager.Config.PersistenceDirectory = &persistPath
	plugin := New(&Config{}, manager)

	path, err := plugin.getBundlePersistPath()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if path != "/var/opa/bundles" {
		t.Errorf("expected configured persist path '/var/opa/bundles'")
	}
}

func TestPluginUsingFileLoader(t *testing.T) {
	t.Parallel()

	test.WithTempFS(map[string]string{}, func(dir string) {

		b := bundle.Bundle{
			Data: map[string]interface{}{},
			Modules: []bundle.ModuleFile{
				{
					URL: "test.rego",
					Raw: []byte(`package test

					p = 7`),
				},
			},
		}

		name := path.Join(dir, "bundle.tar.gz")

		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}

		if err := bundle.NewWriter(f).Write(b); err != nil {
			t.Fatal(err)
		}

		f.Close()

		mgr := getTestManager()
		url := "file://" + name

		p := New(&Config{Bundles: map[string]*Source{
			"test": {
				SizeLimitBytes: 1e5,
				Resource:       url,
			},
		}}, mgr)

		ch := make(chan Status)

		p.Register("test", func(s Status) {
			ch <- s
		})

		if err := p.Start(context.Background()); err != nil {
			t.Fatal(err)
		}

		s := <-ch

		if s.LastSuccessfulActivation.IsZero() {
			t.Fatal("expected successful activation")
		}
	})
}

func TestPluginUsingFileLoaderV1Compatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x, keywords not used",
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note: "v0.x, shadowed import",
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, keywords not imported",
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note: "v0.x, keywords imported",
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, rego.ve imported",
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:         "v1.0, keywords not used",
			v1Compatible: true,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:         "v1.0, shadowed import",
			v1Compatible: true,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:         "v1.0, keywords not imported",
			v1Compatible: true,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, rego.ve imported",
			v1Compatible: true,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := ast.RegoV0
			if tc.v1Compatible {
				regoVersion = ast.RegoV1
			}
			popts := ast.ParserOptions{RegoVersion: regoVersion}

			test.WithTempFS(map[string]string{}, func(dir string) {

				b := bundle.Bundle{
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							URL: "test.rego",
							Raw: []byte(tc.module),
						},
					},
				}

				name := path.Join(dir, "bundle.tar.gz")

				f, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}

				if err := bundle.NewWriter(f).Write(b); err != nil {
					t.Fatal(err)
				}

				f.Close()

				manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(), plugins.WithParserOptions(popts))
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				url := "file://" + name

				p := New(&Config{Bundles: map[string]*Source{
					"test": {
						SizeLimitBytes: 1e5,
						Resource:       url,
					},
				}}, manager)

				ch := make(chan Status)

				p.Register("test", func(s Status) {
					ch <- s
				})

				if err := p.Start(context.Background()); err != nil {
					t.Fatal(err)
				}

				s := <-ch

				if tc.expErrs != nil {
					for _, expErr := range tc.expErrs {
						found := false
						for _, e := range s.Errors {
							if strings.Contains(e.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", expErr, s.Errors)
						}
					}
				} else if s.LastSuccessfulActivation.IsZero() {
					t.Fatal("expected successful activation")
				}
			})
		})
	}
}

func TestPluginUsingFileLoaderWithBundleRegoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note               string
		managerRegoVersion ast.RegoVersion
		bundleRegoVersion  ast.RegoVersion
		module             string
		expErrs            []string
	}{
		{
			note:               "v0.x manager, v0.x bundle, keywords not used",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, shadowed import",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, keywords not imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:               "v0.x manager, v0.x bundle, keywords imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, rego.v1 imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:               "v0.x manager, v1.0 bundle, keywords not used",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:               "v0.x manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v0.x manager, v1.0 bundle, keywords not imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, keywords imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, rego.ve imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},

		{
			note:               "v1.0 manager, v0.x bundle, keywords not used",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, shadowed import",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, keywords not imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:               "v1.0 manager, v0.x bundle, keywords imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, rego.v1 imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:               "v1.0 manager, v1.0 bundle, keywords not used",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:               "v1.0 manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v1.0 manager, v1.0 bundle, keywords not imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, keywords imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, rego.ve imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{}, func(dir string) {

				manifest := bundle.Manifest{}
				manifest.SetRegoVersion(tc.bundleRegoVersion)
				b := bundle.Bundle{
					Manifest: manifest,
					Data:     map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							URL: "test.rego",
							Raw: []byte(tc.module),
						},
					},
				}

				name := path.Join(dir, "bundle.tar.gz")

				f, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}

				if err := bundle.NewWriter(f).Write(b); err != nil {
					t.Fatal(err)
				}

				f.Close()

				managerPopts := ast.ParserOptions{RegoVersion: tc.managerRegoVersion}
				manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
					plugins.WithParserOptions(managerPopts))
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				url := "file://" + name

				p := New(&Config{Bundles: map[string]*Source{
					"test": {
						SizeLimitBytes: 1e5,
						Resource:       url,
					},
				}}, manager)

				ch := make(chan Status)

				p.Register("test", func(s Status) {
					ch <- s
				})

				if err := p.Start(context.Background()); err != nil {
					t.Fatal(err)
				}

				s := <-ch

				if tc.expErrs != nil {
					for _, expErr := range tc.expErrs {
						found := false
						for _, e := range s.Errors {
							if strings.Contains(e.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", expErr, s.Errors)
						}
					}
				} else if s.LastSuccessfulActivation.IsZero() {
					t.Fatal("expected successful activation")
				}
			})
		})
	}
}

func TestPluginUsingDirectoryLoader(t *testing.T) {
	t.Parallel()

	test.WithTempFS(map[string]string{
		"test.rego": `package test

		p := 7`,
	}, func(dir string) {

		mgr := getTestManager()
		url := "file://" + dir

		p := New(&Config{Bundles: map[string]*Source{
			"test": {
				SizeLimitBytes: 1e5,
				Resource:       url,
			},
		}}, mgr)

		ch := make(chan Status)

		p.Register("test", func(s Status) {
			ch <- s
		})

		if err := p.Start(context.Background()); err != nil {
			t.Fatal(err)
		}

		s := <-ch

		if s.LastSuccessfulActivation.IsZero() {
			t.Fatal("expected successful activation")
		}
	})
}

func TestPluginUsingDirectoryLoaderV1Compatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x, keywords not used",
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note: "v0.x, shadowed import",
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, keywords not imported",
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note: "v0.x, keywords imported",
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note: "v0.x, rego.ve imported",
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:         "v1.0, keywords not used",
			v1Compatible: true,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:         "v1.0, shadowed import",
			v1Compatible: true,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:         "v1.0, keywords not imported",
			v1Compatible: true,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:         "v1.0, rego.ve imported",
			v1Compatible: true,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := ast.RegoV0
			if tc.v1Compatible {
				regoVersion = ast.RegoV1
			}
			popts := ast.ParserOptions{RegoVersion: regoVersion}

			test.WithTempFS(map[string]string{
				"test.rego": tc.module,
			}, func(dir string) {

				manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(), plugins.WithParserOptions(popts))
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				url := "file://" + dir

				p := New(&Config{Bundles: map[string]*Source{
					"test": {
						SizeLimitBytes: 1e5,
						Resource:       url,
					},
				}}, manager)

				ch := make(chan Status)

				p.Register("test", func(s Status) {
					ch <- s
				})

				if err := p.Start(context.Background()); err != nil {
					t.Fatal(err)
				}

				s := <-ch

				if tc.expErrs != nil {
					for _, expErr := range tc.expErrs {
						found := false
						for _, e := range s.Errors {
							if strings.Contains(e.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", expErr, s.Errors)
						}
					}
				} else if s.LastSuccessfulActivation.IsZero() {
					t.Fatal("expected successful activation")
				}
			})
		})
	}
}

func TestPluginUsingDirectoryLoaderWithBundleRegoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note               string
		managerRegoVersion ast.RegoVersion
		bundleRegoVersion  ast.RegoVersion
		module             string
		expErrs            []string
	}{
		{
			note:               "v0.x manager, v0.x bundle, keywords not used",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, shadowed import",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, keywords not imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:               "v0.x manager, v0.x bundle, keywords imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v0.x bundle, rego.v1 imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:               "v0.x manager, v1.0 bundle, keywords not used",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:               "v0.x manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v0.x manager, v1.0 bundle, keywords not imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, keywords imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v0.x manager, v1.0 bundle, rego.ve imported",
			managerRegoVersion: ast.RegoV0,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},

		{
			note:               "v1.0 manager, v0.x bundle, keywords not used",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p[7] {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, shadowed import",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, keywords not imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:               "v1.0 manager, v0.x bundle, keywords imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v0.x bundle, rego.v1 imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV0,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
		// parse-time error
		{
			note:               "v1.0 manager, v1.0 bundle, keywords not used",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p[7] {
	input.x == 2
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		// compile-time error
		{
			note:               "v1.0 manager, v1.0 bundle, shadowed import",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import data.foo
import data.bar as foo
p contains 7 if {
	input.x == 2
}`,
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:               "v1.0 manager, v1.0 bundle, keywords not imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, keywords imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import future.keywords
p contains 7 if {
	input.x == 2
}`,
		},
		{
			note:               "v1.0 manager, v1.0 bundle, rego.ve imported",
			managerRegoVersion: ast.RegoV1,
			bundleRegoVersion:  ast.RegoV1,
			module: `package test
import rego.v1
p contains 7 if {
	input.x == 2
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(map[string]string{
				"test.rego": tc.module,
				".manifest": fmt.Sprintf(`{"rego_version": %d}`, bundleRegoVersion(tc.bundleRegoVersion)),
			}, func(dir string) {

				managerPopts := ast.ParserOptions{RegoVersion: tc.managerRegoVersion}
				manager, err := plugins.New(nil, "test-instance-id", inmemtst.New(),
					plugins.WithParserOptions(managerPopts))
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				url := "file://" + dir

				p := New(&Config{Bundles: map[string]*Source{
					"test": {
						SizeLimitBytes: 1e5,
						Resource:       url,
					},
				}}, manager)

				ch := make(chan Status)

				p.Register("test", func(s Status) {
					ch <- s
				})

				if err := p.Start(context.Background()); err != nil {
					t.Fatal(err)
				}

				s := <-ch

				if tc.expErrs != nil {
					for _, expErr := range tc.expErrs {
						found := false
						for _, e := range s.Errors {
							if strings.Contains(e.Error(), expErr) {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", expErr, s.Errors)
						}
					}
				} else if s.LastSuccessfulActivation.IsZero() {
					t.Fatal("expected successful activation")
				}
			})
		})
	}
}

func TestPluginReadBundleEtagFromDiskStore(t *testing.T) {
	t.Parallel()

	// setup fake http server with mock bundle
	mockBundle := bundle.Bundle{
		Data:    map[string]interface{}{"p": "x1"},
		Modules: []bundle.ModuleFile{},
	}

	notModifiedCount := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		etag := r.Header.Get("If-None-Match")
		if etag == "foo" {
			notModifiedCount++
			w.WriteHeader(304)
			return
		}

		w.Header().Add("Etag", "foo")
		w.WriteHeader(200)

		err := bundle.NewWriter(w).Write(mockBundle)
		if err != nil {
			t.Fatal(err)
		}
	}))

	test.WithTempFS(nil, func(dir string) {
		ctx := context.Background()

		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
			Partitions: []storage.Path{
				storage.MustParsePath("/foo"),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// setup plugin pointing at fake server
		manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
				"default": {
					"url": %q
				}
			}
		}`, s.URL)), store)

		var mode plugins.TriggerMode = "manual"

		plugin := New(&Config{
			Bundles: map[string]*Source{
				"test": {
					Service:        "default",
					SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
					Config:         download.Config{Trigger: &mode},
				},
			},
		}, manager)

		statusCh := make(chan map[string]*Status)

		// register for bundle updates to observe changes and start the plugin
		plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
			statusCh <- st
		})

		err = plugin.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// manually trigger bundle download
		go func() {
			_ = plugin.Loaders()["test"].Trigger(ctx)
		}()

		// wait for bundle update and then verify that activated bundle etag written to store
		<-statusCh

		txn := storage.NewTransactionOrDie(ctx, manager.Store)

		actual, err := manager.Store.Read(ctx, txn, storage.MustParsePath("/system/bundles/test/etag"))
		if err != nil {
			t.Fatal(err)
		}

		if actual != "foo" {
			t.Fatalf("Expected etag foo but got %v", actual)
		}

		// Stop the "read" transaction
		manager.Store.Abort(ctx, txn)

		// Stop the plugin and reinitialize it. Verify that etag is retrieved from store in the bundle request.
		// The server should respond with a 304 as OPA has the right bundle loaded.
		plugin.Stop(ctx)

		plugin = New(&Config{
			Bundles: map[string]*Source{
				"test": {
					Service:        "default",
					SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
					Config:         download.Config{Trigger: &mode},
				},
			},
		}, manager)

		statusCh = make(chan map[string]*Status)

		// register for bundle updates to observe changes and start the plugin
		plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
			statusCh <- st
		})

		err = plugin.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		val, ok := plugin.etags["test"]
		if !ok {
			t.Fatal("Expected etag entry for bundle \"test\"")
		}

		if val != "foo" {
			t.Fatalf("Expected etag foo but got %v", val)
		}

		// manually trigger bundle download
		go func() {
			_ = plugin.Loaders()["test"].Trigger(ctx)
		}()

		<-statusCh

		if notModifiedCount != 1 {
			t.Fatalf("Expected one bundle response with HTTP status 304 but got %v", notModifiedCount)
		}

		// reconfigure the plugin
		cfg := &Config{
			Bundles: map[string]*Source{
				"test": {
					Service:        "default",
					SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
					Config:         download.Config{Trigger: &mode},
					Resource:       "/new/path/bundles/bundle.tar.gz",
				},
			},
		}

		plugin.Reconfigure(ctx, cfg)

		// manually trigger bundle download
		go func() {
			_ = plugin.Loaders()["test"].Trigger(ctx)
		}()

		<-statusCh

		if notModifiedCount != 2 {
			t.Fatalf("Expected two bundle responses with HTTP status 304 but got %v", notModifiedCount)
		}

		val, ok = plugin.etags["test"]
		if !ok {
			t.Fatal("Expected etag entry for bundle \"test\"")
		}

		if val != "foo" {
			t.Fatalf("Expected etag foo but got %v", val)
		}
	})
}

func TestPluginStateReconciliationOnReconfigure(t *testing.T) {
	t.Parallel()

	// setup fake http server with mock bundle
	mockBundles := map[string]bundle.Bundle{
		"b1": {
			Data:    map[string]interface{}{"b1": "x1"},
			Modules: []bundle.ModuleFile{},
			Manifest: bundle.Manifest{
				Roots: &[]string{"b1"},
			},
		},
		"b2": {
			Data:    map[string]interface{}{"b2": "x1"},
			Modules: []bundle.ModuleFile{},
			Manifest: bundle.Manifest{
				Roots: &[]string{"b2"},
			},
		},
		"b3_frequently_changing": {
			Data:    map[string]interface{}{"b3": "x1"},
			Modules: []bundle.ModuleFile{},
			Manifest: bundle.Manifest{
				Roots: &[]string{"b3"},
			},
		},
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		etag := r.Header.Get("If-None-Match")
		if etag == name && name != "b3_frequently_changing" {
			w.WriteHeader(304)
			return
		}

		if name != "b3_frequently_changing" {
			w.Header().Add("Etag", name)
		}
		w.WriteHeader(200)

		err := bundle.NewWriter(w).Write(mockBundles[name])
		if err != nil {
			t.Fatal(err)
		}
	}))

	// setup plugin pointing at fake server
	manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
				"default": {
					"url": %q
				}
			}
		}`, s.URL)))

	// setup manual trigger mode to simulate the downloader
	var mode plugins.TriggerMode = "manual"
	var delay int64 = 10
	polling := download.PollingConfig{MinDelaySeconds: &delay, MaxDelaySeconds: &delay}
	serviceName := "default"
	plugin := New(&Config{
		Bundles: map[string]*Source{
			"b1": {
				Service:        serviceName,
				Config:         download.Config{Trigger: &mode},
				Resource:       "/b1",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
			},
		},
	}, manager)

	statusCh := make(chan map[string]*Status)

	// register for bundle updates to observe changes
	plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
		statusCh <- st
	})

	ctx := context.Background()
	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// manually trigger bundle download
	go func() { _ = plugin.Loaders()["b1"].Trigger(ctx) }()
	<-statusCh

	// validate plugin started as expected
	ensurePluginState(t, plugin, plugins.StateOK)

	// change the plugin state with multiple stages
	stages := []struct {
		name             string
		cfg              *Config
		noChangeDetected bool
	}{
		{
			name: "Add a bundle", // b1 is NOT Modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b2": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b2", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
		{
			name: "change download config", // both bundles are Not Modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b2": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b2", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
		{
			name: "pass the same config", // should be no change detected
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b2": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b2", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
			noChangeDetected: true,
		},
		{
			name: "revert download config for one bundle", // both bundles are Not Modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b2": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b2", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
		{
			name: "remove a bundle", // b1 is Not Modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
			noChangeDetected: true,
		},
		{
			name: "change download config again", // b1 is Not Modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1": {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
		{
			name: "add frequently changing bundle",
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1":                     {Service: serviceName, Config: download.Config{Trigger: &mode, Polling: polling}, Resource: "/b1", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b3_frequently_changing": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b3_frequently_changing", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
		{
			name: "revert download config for Not Modified bundle", // b1 is Not Modified while b3_frequently_changing is modified
			cfg: &Config{
				Bundles: map[string]*Source{
					"b1":                     {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b2", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
					"b3_frequently_changing": {Service: serviceName, Config: download.Config{Trigger: &mode}, Resource: "/b3_frequently_changing", SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes)},
				},
			},
		},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			plugin.Reconfigure(ctx, stage.cfg)

			if stage.noChangeDetected {
				ensurePluginState(t, plugin, plugins.StateOK)
				return
			}

			// if there is a change in config
			// Reconfigure sets the plugin state as StateNotReady
			ensurePluginState(t, plugin, plugins.StateNotReady)

			for name := range stage.cfg.Bundles {
				go func(name string) {
					_ = plugin.Loaders()[name].Trigger(ctx)
				}(name)
				<-statusCh
			}

			// after all downloaders are processed the state should
			// reconcile to StateOK, if there are no errors
			ensurePluginState(t, plugin, plugins.StateOK)
		})
	}
}

func TestPluginManualTrigger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// setup fake http server with mock bundle
	mockBundle := bundle.Bundle{
		Data:    map[string]interface{}{"p": "x1"},
		Modules: []bundle.ModuleFile{},
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := bundle.NewWriter(w).Write(mockBundle)
		if err != nil {
			t.Fatal(err)
		}
	}))

	// setup plugin pointing at fake server
	manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			}
		}
	}`, s.URL)))

	var mode plugins.TriggerMode = "manual"

	plugin := New(&Config{
		Bundles: map[string]*Source{
			"test": {
				Service:        "default",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
				Config:         download.Config{Trigger: &mode},
			},
		},
	}, manager)

	statusCh := make(chan map[string]*Status)

	// register for bundle updates to observe changes and start the plugin
	plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
		statusCh <- st
	})

	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// manually trigger bundle download
	go func() {
		_ = plugin.Loaders()["test"].Trigger(ctx)
	}()

	// wait for bundle update and then assert on data content
	<-statusCh

	result, err := storage.ReadOne(ctx, manager.Store, storage.Path{"p"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result, mockBundle.Data["p"]) {
		t.Fatalf("expected data to be %v but got %v", mockBundle.Data, result)
	}

	// update data and trigger another bundle download
	mockBundle.Data["p"] = "x2"

	// manually trigger bundle download
	go func() {
		_ = plugin.Loaders()["test"].Trigger(ctx)
	}()

	// wait for bundle update and then assert on data content
	<-statusCh

	result, err = storage.ReadOne(ctx, manager.Store, storage.Path{"p"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result, mockBundle.Data["p"]) {
		t.Fatalf("expected data to be %v but got %v", mockBundle.Data, result)
	}
}

func TestPluginManualTriggerMultipleDiskStorage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	module := "package authz\n\ncorge=1"

	// setup fake http server with mock bundle
	mockBundle1 := bundle.Bundle{
		Data: map[string]interface{}{"p": "x1"},
		Modules: []bundle.ModuleFile{
			{
				URL:    "/bar/policy.rego",
				Path:   "/bar/policy.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
		Manifest: bundle.Manifest{
			Roots: &[]string{"p", "authz"},
		},
	}

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := bundle.NewWriter(w).Write(mockBundle1)
		if err != nil {
			t.Fatal(err)
		}
	}))

	mockBundle2 := bundle.Bundle{
		Data:    map[string]interface{}{"q": "x2"},
		Modules: []bundle.ModuleFile{},
		Manifest: bundle.Manifest{
			Roots: &[]string{"q"},
		},
	}

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := bundle.NewWriter(w).Write(mockBundle2)
		if err != nil {
			t.Fatal(err)
		}
	}))

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}
		config := []byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			},
			"acmecorp": {
				"url": %q
			}
		}
	}`, s1.URL, s2.URL))

		manager := getTestManagerWithOpts(config, store)
		defer manager.Stop(ctx)

		var mode plugins.TriggerMode = "manual"

		plugin := New(&Config{
			Bundles: map[string]*Source{
				"test-1": {
					Service:        "default",
					SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
					Config:         download.Config{Trigger: &mode},
				},
				"test-2": {
					Service:        "acmecorp",
					SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
					Config:         download.Config{Trigger: &mode},
				},
			},
		}, manager)

		statusCh := make(chan map[string]*Status)

		// register for bundle updates to observe changes and start the plugin
		plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
			statusCh <- st
		})

		err = plugin.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// manually trigger bundle download on all configured bundles
		go func() {
			_ = plugin.Trigger(ctx)
		}()

		// wait for bundle update and then assert on data content
		<-statusCh
		<-statusCh

		result, err := storage.ReadOne(ctx, manager.Store, storage.Path{"p"})
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, mockBundle1.Data["p"]) {
			t.Fatalf("expected data to be %v but got %v", mockBundle1.Data, result)
		}

		result, err = storage.ReadOne(ctx, manager.Store, storage.Path{"q"})
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, mockBundle2.Data["q"]) {
			t.Fatalf("expected data to be %v but got %v", mockBundle2.Data, result)
		}

		txn := storage.NewTransactionOrDie(ctx, manager.Store)
		defer manager.Store.Abort(ctx, txn)

		ids, err := manager.Store.ListPolicies(ctx, txn)
		if err != nil {
			t.Fatal(err)
		} else if len(ids) != 1 {
			t.Fatal("Expected 1 policy")
		}

		bs, err := manager.Store.GetPolicy(ctx, txn, ids[0])
		exp := []byte("package authz\n\ncorge=1")
		if err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(bs, exp) {
			t.Fatalf("Bad policy content. Exp:\n%v\n\nGot:\n\n%v", string(exp), string(bs))
		}

		data, err := manager.Store.Read(ctx, txn, storage.Path{})
		expData := util.MustUnmarshalJSON([]byte(`{
			"p": "x1", "q": "x2",
			"system": {
				"bundles": {"test-1": {"etag": "", "manifest": {"revision": "", "roots": ["p", "authz"]}}, "test-2": {"etag": "", "manifest": {"revision": "", "roots": ["q"]}}}
			}
		}`))
		if err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(data, expData) {
			t.Fatalf("Bad data content. Exp:\n%v\n\nGot:\n\n%v", expData, data)
		}
	})
}

func TestPluginManualTriggerMultiple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// setup fake http server with mock bundle
	mockBundle1 := bundle.Bundle{
		Data:    map[string]interface{}{"p": "x1"},
		Modules: []bundle.ModuleFile{},
		Manifest: bundle.Manifest{
			Roots: &[]string{"p"},
		},
	}

	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := bundle.NewWriter(w).Write(mockBundle1)
		if err != nil {
			t.Fatal(err)
		}
	}))

	mockBundle2 := bundle.Bundle{
		Data:    map[string]interface{}{"q": "x2"},
		Modules: []bundle.ModuleFile{},
		Manifest: bundle.Manifest{
			Roots: &[]string{"q"},
		},
	}

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		err := bundle.NewWriter(w).Write(mockBundle2)
		if err != nil {
			t.Fatal(err)
		}
	}))

	// setup plugin pointing at fake server
	manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			},
			"acmecorp": {
				"url": %q
			}
		}
	}`, s1.URL, s2.URL)))

	var mode plugins.TriggerMode = "manual"

	plugin := New(&Config{
		Bundles: map[string]*Source{
			"test-1": {
				Service:        "default",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
				Config:         download.Config{Trigger: &mode},
			},
			"test-2": {
				Service:        "acmecorp",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
				Config:         download.Config{Trigger: &mode},
			},
		},
	}, manager)

	statusCh := make(chan map[string]*Status)

	// register for bundle updates to observe changes and start the plugin
	plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
		statusCh <- st
	})

	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// manually trigger bundle download on all configured bundles
	go func() {
		_ = plugin.Trigger(ctx)
	}()

	// wait for bundle update and then assert on data content
	<-statusCh
	<-statusCh

	result, err := storage.ReadOne(ctx, manager.Store, storage.Path{"p"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result, mockBundle1.Data["p"]) {
		t.Fatalf("expected data to be %v but got %v", mockBundle1.Data, result)
	}

	result, err = storage.ReadOne(ctx, manager.Store, storage.Path{"q"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result, mockBundle2.Data["q"]) {
		t.Fatalf("expected data to be %v but got %v", mockBundle2.Data, result)
	}
}

func TestPluginManualTriggerWithTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * time.Second) // this should cause the context deadline to exceed
	}))

	// setup plugin pointing at fake server
	manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			}
		}
	}`, s.URL)))

	var mode plugins.TriggerMode = "manual"

	bundleName := "test"
	plugin := New(&Config{
		Bundles: map[string]*Source{
			bundleName: {
				Service:        "default",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
				Config:         download.Config{Trigger: &mode},
			},
		},
	}, manager)

	statusCh := make(chan map[string]*Status)

	// register for bundle updates to observe changes and start the plugin
	plugin.RegisterBulkListener("test-case", func(st map[string]*Status) {
		statusCh <- st
	})

	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// manually trigger bundle download
	go func() {
		_ = plugin.Loaders()[bundleName].Trigger(ctx)
	}()

	// wait for bundle update
	u := <-statusCh

	if u[bundleName].Code != errCode {
		t.Fatalf("expected error code %v but got %v", errCode, u[bundleName].Code)
	}

	if !strings.Contains(u[bundleName].Message, "context deadline exceeded") {
		t.Fatalf("unexpected error message %v", u[bundleName].Message)
	}
}

func TestPluginManualTriggerWithServerError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(500)
	}))

	// setup plugin pointing at fake server
	manager := getTestManagerWithOpts([]byte(fmt.Sprintf(`{
		"services": {
			"default": {
				"url": %q
			}
		}
	}`, s.URL)))

	var manual plugins.TriggerMode = "manual"

	plugin := New(&Config{
		Bundles: map[string]*Source{
			"test": {
				Service:        "default",
				SizeLimitBytes: int64(bundle.DefaultSizeLimitBytes),
				Config:         download.Config{Trigger: &manual},
			},
		},
	}, manager)

	err := plugin.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// manually trigger bundle download
	err = plugin.Trigger(ctx)

	plugin.Stop(ctx)

	var bundleErrors Errors
	if errors.As(err, &bundleErrors) {
		if len(bundleErrors) != 1 {
			t.Fatalf("expected exactly one error, got %d", len(bundleErrors))
		}
		for _, e := range bundleErrors {
			if e.BundleName != "test" {
				t.Fatalf("expected error for bundle 'test' but got '%s'", e.BundleName)
			}
		}
	} else {
		t.Fatalf("expected type of error to be %s but got %s", reflect.TypeOf(bundleErrors), reflect.TypeOf(err))
	}
}

// Warning: This test modifies package variables, and as
// a result, cannot be run in parallel with other tests.
func TestGetNormalizedBundleName(t *testing.T) {
	cases := []struct {
		input string
		goos  string
		exp   string
	}{
		{
			input: "foo",
			exp:   "foo",
		},
		{
			input: "foo=bar",
			exp:   "foo=bar",
			goos:  "windows",
		},
		{
			input: "c:/foo",
			exp:   "c:/foo",
		},
		{
			input: "c:/foo",
			exp:   "c\\:\\/foo",
			goos:  "windows",
		},
		{
			input: "file:\"<>c:/a",
			exp:   "file\\:\\\"\\<\\>c\\:\\/a",
			goos:  "windows",
		},
		{
			input: "|a?b*c",
			exp:   "\\|a\\?b\\*c",
			goos:  "windows",
		},
		{
			input: "a?b=c",
			exp:   "a\\?b=c",
			goos:  "windows",
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			goos = tc.goos
			actual := getNormalizedBundleName(tc.input)
			if actual != tc.exp {
				t.Fatalf("Want %v but got: %v", tc.exp, actual)
			}
		})
	}
}

func TestBundleActivationWithRootOverlap(t *testing.T) {
	ctx := context.Background()
	plugin := getPluginWithExistingLoadedBundle(
		t,
		"policy-bundle",
		[]string{"foo/bar"},
		nil,
		[]testModule{
			{
				Path: "foo/bar/bar.rego",
				Data: `package foo.bar
result := true`,
			},
		},
	)

	bundleName := "new-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := getTestBundleWithData(
		[]string{"foo/bar/baz"},
		[]byte(`{"foo": {"bar": 1, "baz": "qux"}}`),
		nil,
	)

	b.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	// "foo/bar" and "foo/bar/baz" overlap with each other; activation will fail
	status, ok := plugin.status[bundleName]
	if !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	}
	if status.Code != errCode {
		t.Fatalf("Expected status code to be %s, found %s", errCode, status.Code)
	}
	if exp := "detected overlapping roots"; !strings.Contains(status.Message, exp) {
		t.Fatalf(`Expected status message to contain "%s", found %s`, exp, status.Message)
	}
}

func TestBundleActivationWithNoManifestRootsButWithPathConflict(t *testing.T) {
	ctx := context.Background()
	plugin := getPluginWithExistingLoadedBundle(
		t,
		"policy-bundle",
		[]string{"foo/bar"},
		nil,
		[]testModule{
			{
				Path: "foo/bar/bar.rego",
				Data: `package foo.bar
result := true`,
			},
		},
	)

	bundleName := "new-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := getTestBundleWithData(
		nil,
		[]byte(`{"foo": {"bar": 1, "baz": "qux"}}`),
		nil,
	)

	b.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	// new bundle has path "foo/bar" which overlaps with existing bundle with path "foo/bar"; activation will fail
	status, ok := plugin.status[bundleName]
	if !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	}
	if status.Code != errCode {
		t.Fatalf("Expected status code to be %s, found %s", errCode, status.Code)
	}
	if !strings.Contains(status.Message, "detected overlapping") {
		t.Fatalf(`Expected status message to contain "detected overlapping roots", found %s`, status.Message)
	}
}

func TestBundleActivationWithNoManifestRootsOverlap(t *testing.T) {
	ctx := context.Background()
	plugin := getPluginWithExistingLoadedBundle(
		t,
		"policy-bundle",
		[]string{"foo/bar"},
		nil,
		[]testModule{
			{
				Path: "foo/bar/bar.rego",
				Data: `package foo.bar
result := true`,
			},
		},
	)

	bundleName := "new-bundle"
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	b := getTestBundleWithData(
		[]string{"foo/baz"},
		nil,
		[]testModule{
			{
				Path: "foo/bar/baz.rego",
				Data: `package foo.baz
result := true`,
			},
		},
	)

	b.Manifest.Init()
	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	status, ok := plugin.status[bundleName]
	if !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	}
	if status.Code != "" {
		t.Fatalf("Expected status code to be empty, found %s", status.Code)
	}
}

type testModule struct {
	Path string
	Data string
}

func getTestBundleWithData(roots []string, data []byte, modules []testModule) bundle.Bundle {
	b := bundle.Bundle{}

	if len(roots) > 0 {
		b.Manifest = bundle.Manifest{Roots: &roots}
	}

	if len(data) > 0 {
		b.Data = util.MustUnmarshalJSON(data).(map[string]interface{})
	}

	for _, m := range modules {
		if len(m.Data) > 0 {
			b.Modules = append(b.Modules,
				bundle.ModuleFile{
					Path:   m.Path,
					Parsed: ast.MustParseModule(m.Data),
					Raw:    []byte(m.Data),
				},
			)
		}
	}

	b.Manifest.Init()

	return b
}

func getPluginWithExistingLoadedBundle(t *testing.T, bundleName string, roots []string, data []byte, modules []testModule) *Plugin {
	ctx := context.Background()
	store := inmem.NewWithOpts(inmem.OptRoundTripOnWrite(false), inmem.OptReturnASTValuesOnRead(true))
	manager := getTestManagerWithOpts(nil, store)
	plugin := New(&Config{}, manager)
	plugin.status[bundleName] = &Status{Name: bundleName, Metrics: metrics.New()}
	plugin.downloaders[bundleName] = download.New(download.Config{}, plugin.manager.Client(""), bundleName)

	ensurePluginState(t, plugin, plugins.StateNotReady)

	b := getTestBundleWithData(roots, data, modules)

	plugin.oneShot(ctx, bundleName, download.Update{Bundle: &b, Metrics: metrics.New(), Size: snapshotBundleSize})

	ensurePluginState(t, plugin, plugins.StateOK)

	if status, ok := plugin.status[bundleName]; !ok {
		t.Fatalf("Expected to find status for %s, found nil", bundleName)
	} else if status.Type != bundle.SnapshotBundleType {
		t.Fatalf("Expected snapshot bundle but got %v", status.Type)
	} else if status.Size != snapshotBundleSize {
		t.Fatalf("Expected snapshot bundle size %d but got %d", snapshotBundleSize, status.Size)
	}

	return plugin
}

func writeTestBundleToDisk(t *testing.T, srcDir string, signed bool) bundle.Bundle {
	t.Helper()

	var b bundle.Bundle

	if signed {
		b = getTestSignedBundle(t)
	} else {
		b = getTestBundle(t)
	}

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "bundle.tar.gz"), buf.Bytes(), 0644); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	return b
}

func getTestBundle(t *testing.T) bundle.Bundle {
	t.Helper()

	module := `package gork
import rego.v1


p contains x if { x = 1 }`

	b := bundle.Bundle{
		Manifest: bundle.Manifest{
			Revision: "quickbrownfaux",
		},
		Data: map[string]interface{}{},
		Modules: []bundle.ModuleFile{
			{
				Path:   "/foo.rego",
				URL:    "/foo.rego",
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
	}

	b.Manifest.Init()
	return b
}

func getTestSignedBundle(t *testing.T) bundle.Bundle {
	t.Helper()

	b := getTestBundle(t)

	if err := b.GenerateSignature(bundle.NewSigningConfig("secret", "HS256", ""), "foo", false); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	return b
}

func getTestRawBundle(t *testing.T) io.Reader {
	t.Helper()

	b := getTestBundle(t)

	var buf bytes.Buffer
	if err := bundle.NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		t.Fatal("unexpected error:", err)
	}

	return &buf
}

func validateStoreState(ctx context.Context, t *testing.T, store storage.Store, root string, expData interface{}, expIDs []string, expBundleName string, expBundleRev string, expMetadata map[string]interface{}) {
	t.Helper()
	if err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
		value, err := store.Read(ctx, txn, storage.MustParsePath(root))
		if err != nil {
			return err
		}

		if expAst, ok := expData.(ast.Value); ok {
			if ast.Compare(value, expAst) != 0 {
				return fmt.Errorf("expected %v but got %v", expAst, value)
			}
		} else {
			if !reflect.DeepEqual(value, expData) {
				return fmt.Errorf("expected %v but got %v", expData, value)
			}
		}

		ids, err := store.ListPolicies(ctx, txn)
		if err != nil {
			return err
		}

		sort.Strings(ids)
		sort.Strings(expIDs)

		if !slices.Equal(ids, expIDs) {
			return fmt.Errorf("expected ids %v but got %v", expIDs, ids)
		}

		rev, err := bundle.ReadBundleRevisionFromStore(ctx, store, txn, expBundleName)
		if err != nil {
			return fmt.Errorf("unexpected error when reading bundle revision from store: %s", err)
		}

		if rev != expBundleRev {
			return fmt.Errorf("unexpected revision found on bundle: %s", rev)
		}

		metadata, err := bundle.ReadBundleMetadataFromStore(ctx, store, txn, expBundleName)
		if err != nil {
			return fmt.Errorf("unexpected error when reading bundle metadata from store: %s", err)
		}
		if !reflect.DeepEqual(expMetadata, metadata) {
			return fmt.Errorf("unexpected metadata found on bundle: %v", metadata)
		}

		return nil

	}); err != nil {
		t.Fatal(err)
	}
}

func ensurePluginState(t *testing.T, p *Plugin, state plugins.State) {
	t.Helper()
	status, ok := p.manager.PluginStatus()[Name]
	if !ok {
		t.Fatalf("Expected to find state for %s, found nil", Name)
		return
	}
	if status.State != state {
		t.Fatalf("Unexpected status state found in plugin manager for %s:\n\n\tFound:%+v\n\n\tExpected: %s", Name, status.State, state)
	}
}
