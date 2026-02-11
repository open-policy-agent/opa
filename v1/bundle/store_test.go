package bundle

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/storage/mock"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"

	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/disk"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	inmemtst "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func TestManifestStoreLifecycleSingleBundle(t *testing.T) {
	store := inmemtst.New()
	tb := Manifest{Revision: "abc123", Roots: &[]string{"/a/b", "/a/c"}}

	verifyWriteManifests(t, store, map[string]Manifest{"test_bundle": tb}) // write one
	verifyReadBundleNames(t, store, nil, "test_bundle")                    // read one
	verifyDeleteManifest(t, store, "test_bundle")                          // delete it
	verifyReadBundleNames(t, store, nil)                                   // ensure it was removed
}

func TestManifestStoreLifecycleMultiBundle(t *testing.T) {
	store := inmemtst.New()
	bundles := map[string]Manifest{
		"bundle1": {
			Revision: "abc123",
			Roots:    &[]string{"/a/b", "/a/c"},
		},
		"bundle2": {
			Revision: "def123",
			Roots:    &[]string{"/x/y", "/z"},
		},
	}

	verifyWriteManifests(t, store, bundles)                    // write multiple
	verifyReadBundleNames(t, store, nil, "bundle1", "bundle2") // read them
	verifyDeleteManifest(t, store, "bundle1")                  // delete one
	verifyReadBundleNames(t, store, nil, "bundle2")            // ensure it was removed
	verifyDeleteManifest(t, store, "bundle2")                  // delete the last one
	verifyReadBundleNames(t, store, nil)                       // ensure it was removed
}

func TestLegacyManifestStoreLifecycle(t *testing.T) {
	store := inmemtst.New()
	tb := Manifest{Revision: "abc123", Roots: &[]string{"/a/b", "/a/c"}}

	// write a "legacy" manifest
	if err := storage.Txn(t.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
		return LegacyWriteManifestToStore(t.Context(), store, txn, tb)
	}); err != nil {
		t.Fatalf("Failed to write manifest to store: %s", err)
	}

	// make sure it can be retrieved
	verifyReadLegacyRevision(t, store, tb.Revision)

	// delete it
	if err := storage.Txn(t.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
		return LegacyEraseManifestFromStore(t.Context(), store, txn)
	}); err != nil {
		t.Fatalf("Failed to erase manifest from store: %s", err)
	}

	verifyReadLegacyRevision(t, store, "")
}

func TestMixedManifestStoreLifecycle(t *testing.T) {
	store := inmemtst.New()
	bundles := map[string]Manifest{
		"bundle1": {
			Revision: "abc123",
			Roots:    &[]string{"/a/b", "/a/c"},
		},
		"bundle2": {
			Revision: "def123",
			Roots:    &[]string{"/x/y", "/z"},
		},
	}

	// Write the legacy one first
	if err := storage.Txn(t.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
		return LegacyWriteManifestToStore(t.Context(), store, txn, bundles["bundle1"])
	}); err != nil {
		t.Fatalf("Failed to write manifest to store: %s", err)
	}

	verifyReadBundleNames(t, store, nil)

	// Write both new ones
	verifyWriteManifests(t, store, bundles)
	verifyReadBundleNames(t, store, nil, "bundle1", "bundle2")

	// Ensure the original legacy one is still there
	verifyReadLegacyRevision(t, store, bundles["bundle1"].Revision)
}

func TestBundleLazyModeNoPolicyOrData(t *testing.T) {
	mockStore := mock.New()
	bundles := map[string]*Bundle{"bundle1": {
		Manifest: Manifest{
			Roots:    &[]string{"a"},
			Revision: "foo",
		},
		Etag:            "foo",
		lazyLoadingMode: true,
	}}

	mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles})

	// Ensure the bundle was activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyResultRead(t, mockStore, `{
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "foo",
						"roots": ["a"]
					},
					"etag": "foo"
				}
			}
		}
	}`)
}

func TestBundleLifecycle_ModuleRegoVersions(t *testing.T) {
	type (
		files        [][2]string
		bundles      map[string]files
		deactivation struct {
			bundles map[string]struct{}
			expData string
		}
		activation struct {
			bundles            bundles
			lazy               bool
			readWithBundleName bool
			expData            string
		}
	)

	tests := []struct {
		note               string
		updates            []any
		runtimeRegoVersion ast.RegoVersion
	}{
		// single v0 bundle
		{
			note: "v0 bundle, lazy, read with bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
										"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v0 bundle, not lazy, read with bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					// Not lazy mode, bundle store decides that module name should be prefixed with bundle name.
					expData: `{
						"system":{
							"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
							"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
						}
					}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v0 bundle, lazy, read with NO bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader not initialized with bundle name, so prefix not expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
										"modules":{"a/policy.rego":{"rego_version":0}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v0 bundle, not lazy, read with NO bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					// Not lazy mode, bundle store decides that module name should be prefixed with bundle name.
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note:               "v0 bundle, not lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "v0 bundle, lazy, read with bundle name, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "v0 bundle, lazy, read with NO bundle name, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},

		// single v1 bundle
		{
			note: "v1 bundle, lazy, read with bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note: "v1 bundle, not lazy, read with bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					// Not lazy mode, bundle store decides that module name should be prefixed with bundle name.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note: "v1 bundle, lazy, read with NO bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader not initialized with bundle name, so prefix not expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note: "v1 bundle, not lazy, read with NO bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					// Not lazy mode, bundle store decides that module name should be prefixed with bundle name.
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},

		{
			note:               "v1 bundle, not lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
										"modules":{"bundle1/a/policy.rego":{"rego_version":1}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}, "modules":{}}}`,
				},
			},
		},
		{
			note:               "v1 bundle, lazy, read with bundle name, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
										"modules":{"bundle1/a/policy.rego":{"rego_version":1}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}, "modules":{}}}`,
				},
			},
		},
		{
			note:               "v1 bundle, lazy, read with NO bundle name, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy: true,
					// Lazy mode, bundle reader decides if module name should be prefixed with bundle name; reader initialized with bundle name, so prefix is expected.
					expData: `{
									"system":{
										"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
										"modules":{"a/policy.rego":{"rego_version":1}}
									}
								}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}, "modules":{}}}`,
				},
			},
		},

		{
			note: "custom bundle without rego-version, lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 1337 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "custom bundle without rego-version, lazy, v1 runtime (explicit)",
			runtimeRegoVersion: ast.RegoV1,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 1337 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "custom bundle without rego-version, lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p[1337] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},

		{
			note: "custom bundle without rego-version, not lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 1337 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "custom bundle without rego-version, not lazy, v1 runtime (explicit)",
			runtimeRegoVersion: ast.RegoV1,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p contains 1337 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},
		{
			note:               "custom bundle without rego-version, not lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`},
							{"a/policy.rego", `package a
								p[1337] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{}}}`,
				},
			},
		},

		{
			note: "v0, lazy replaced by non-lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
									"modules":{}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note: "v0 bundle replaced by v1 bundle, lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
									"modules":{}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v0 bundle replaced by v1 bundle, not lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}},
									"modules":{}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v0 bundle replaced by custom bundle, not lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`}, // no rego-version
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}},
									"modules":{}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note: "v1 bundle replaced by v0 bundle, lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "v1 bundle replaced by v0 bundle, not lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "custom bundle replaced by v0 bundle, lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"]}`}, // no rego-version
							{"a/policy.rego", `package a
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"revision":"","roots":["a"]}}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note: "multiple v0 bundles, all dropped",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["b"], "rego_version": 0}`},
							{"b/policy.rego", `package b
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}},
										"bundle2":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["b"]}}
									},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0},"bundle2/b/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note: "multiple v0 bundles, one dropped",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["b"], "rego_version": 0}`},
							{"b/policy.rego", `package b
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}},
										"bundle2":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["b"]}}
									},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0},"bundle2/b/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}},
					expData: `{
								"system":{
									"bundles":{
										"bundle2":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["b"]}}
									},
									"modules":{"bundle2/b/policy.rego":{"rego_version":0}}
								}
							}`,
				},
			},
		},

		{
			note: "v0 bundle with v1 bundle added",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a"], "rego_version": 0}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}}},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				activation{
					bundles: bundles{
						"bundle2": {
							{"/.manifest", `{"roots": ["b"], "rego_version": 1}`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"rego_version":0,"revision":"","roots":["a"]}},
										"bundle2":{"etag":"bar","manifest":{"rego_version":1,"revision":"","roots":["b"]}}
									},
									"modules":{"bundle1/a/policy.rego":{"rego_version":0}}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note: "mixed-version bundles, lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/a/policy.rego":{"rego_version":0},
										"bundle2/d/policy.rego":{"rego_version":0}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/a/policy2.rego":{"rego_version":0},
										"bundle2/d/policy2.rego":{"rego_version":0}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "mixed-version bundles, lazy, read with NO bundle name",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					lazy: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"a/policy.rego":{"rego_version":0},
										"d/policy.rego":{"rego_version":0}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					lazy: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"a/policy2.rego":{"rego_version":0},
										"d/policy2.rego":{"rego_version":0}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note: "mixed-version bundles, not lazy",
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/a/policy.rego":{"rego_version":0},
										"bundle2/d/policy.rego":{"rego_version":0}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/a/policy2.rego":{"rego_version":0},
										"bundle2/d/policy2.rego":{"rego_version":0}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},

		{
			note:               "mixed-version bundles, lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/b/policy.rego":{"rego_version":1},
										"bundle2/c/policy.rego":{"rego_version":1}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					lazy:               true,
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/b/policy2.rego":{"rego_version":1},
										"bundle2/c/policy2.rego":{"rego_version":1}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note:               "mixed-version bundles, lazy, read with NO bundle name, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					lazy: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"b/policy.rego":{"rego_version":1},
										"c/policy.rego":{"rego_version":1}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					lazy: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"b/policy2.rego":{"rego_version":1},
										"c/policy2.rego":{"rego_version":1}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
		{
			note:               "mixed-version bundles, not lazy, --v0-compatible",
			runtimeRegoVersion: ast.RegoV0,
			updates: []any{
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy.rego": 1}}`},
							{"a/policy.rego", `package a
								p[42] { true }`},
							{"b/policy.rego", `package b
								p contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy.rego": 0}}`},
							{"c/policy.rego", `package c
								p contains 42 if { true }`},
							{"d/policy.rego", `package d
								p[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/b/policy.rego":{"rego_version":1},
										"bundle2/c/policy.rego":{"rego_version":1}
									}
								}
							}`,
				},
				// replacing bundles
				activation{
					bundles: bundles{
						"bundle1": {
							{"/.manifest", `{"roots": ["a", "b"], "rego_version": 0, "file_rego_versions": {"/b/policy2.rego": 1}}`},
							{"a/policy2.rego", `package a
								q[42] { true }`},
							{"b/policy2.rego", `package b
								q contains 42 if { true }`},
						},
						"bundle2": {
							{"/.manifest", `{"roots": ["c", "d"], "rego_version": 1, "file_rego_versions": {"/d/policy2.rego": 0}}`},
							{"c/policy2.rego", `package c
								q contains 42 if { true }`},
							{"d/policy2.rego", `package d
								q[42] { true }`},
						},
					},
					readWithBundleName: true,
					expData: `{
								"system":{
									"bundles":{
										"bundle1":{"etag":"bar","manifest":{"file_rego_versions":{"/b/policy2.rego":1},"rego_version":0,"revision":"","roots":["a","b"]}},
										"bundle2":{"etag":"bar","manifest":{"file_rego_versions":{"/d/policy2.rego":0},"rego_version":1,"revision":"","roots":["c","d"]}}
									},
									"modules":{
										"bundle1/b/policy2.rego":{"rego_version":1},
										"bundle2/c/policy2.rego":{"rego_version":1}
									}
								}
							}`,
				},
				deactivation{
					bundles: map[string]struct{}{"bundle1": {}, "bundle2": {}},
					expData: `{"system":{"bundles":{},"modules":{}}}`,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			runtimeRegoVersion := cmp.Or(tc.runtimeRegoVersion, ast.DefaultRegoVersion)

			for _, update := range tc.updates {
				if act, ok := update.(activation); ok {
					bundles := map[string]*Bundle{}
					for bundleName, files := range act.bundles {
						br := NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
							WithBundleEtag("bar").
							WithLazyLoadingMode(act.lazy).
							WithRegoVersion(runtimeRegoVersion)

						if act.readWithBundleName {
							br = br.WithBundleName(bundleName)
						}

						bundle := must(br.Read())(t)
						bundles[bundleName] = &bundle
					}

					mustActivate(t, mockStore, &ActivateOpts{
						Bundles:       bundles,
						ParserOptions: ast.ParserOptions{RegoVersion: runtimeRegoVersion},
					})
					verifyResultRead(t, mockStore, act.expData)
				} else if deact, ok := update.(deactivation); ok {
					mustDeactivate(t, mockStore, &DeactivateOpts{
						BundleNames:   deact.bundles,
						ParserOptions: ast.ParserOptions{RegoVersion: runtimeRegoVersion},
					})
					verifyResultRead(t, mockStore, deact.expData)
				}
			}
		})
	}
}

func TestBundleLazyModeLifecycleRaw(t *testing.T) {
	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example
			p contains 42 if { true }
		`},
		{"/example/example_v0.rego", `package example
			q[42] { true }
		`},
		{"/authz/allow/policy.wasm", `wasm-module`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
		{"/.manifest", `{
			"revision": "foo", 
			"roots": ["a", "example", "x", "authz"],
			"wasm":[{"entrypoint": "authz/allow", "module": "/authz/allow/policy.wasm"}],
			"rego_version": 1,
			"file_rego_versions": {"/example/example_v0.rego": 0}
		}`},
	}

	bundle := must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
		WithBundleEtag("bar").
		WithLazyLoadingMode(true).
		Read())(t)

	mockStore := mock.New()
	compiler := ast.NewCompiler()
	bundles := map[string]*Bundle{"bundle1": &bundle}

	mustActivate(t, mockStore, &ActivateOpts{
		Compiler:     compiler,
		Bundles:      bundles,
		ExtraModules: map[string]*ast.Module{"mod1": ast.MustParseModule("package x\np = true")},
	})

	// Ensure the bundle was activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyBundleModulesCompiled(t, compiler, bundles)
	verifyResultRead(t, mockStore, `{
		"a": {
			"b": {
				"c": [1,2,3],
				"d": true,
				"y": {
					"foo": 1
				},
				"z": true
			}
		},
		"x": {
			"y": true
		},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "foo",
						"roots": ["a", "example", "x", "authz"],
						"wasm": [
							{
								"entrypoint": "authz/allow",
								"module": "/authz/allow/policy.wasm"
							}
						],
						"rego_version": 1,
						"file_rego_versions": {
							"/example/example_v0.rego": 0
						}
					},
					"etag": "bar",
					"wasm": {
						"/authz/allow/policy.wasm": "d2FzbS1tb2R1bGU="
					}
				}
			},
			"modules":{
				"example/example.rego":{
					"rego_version":1
				},
				"example/example_v0.rego":{
					"rego_version":0
				}
			}
		}
	}`)

	// Ensure that the extra module was included
	if _, ok := compiler.Modules["mod1"]; !ok {
		t.Fatalf("expected extra module to be compiled")
	}

	mustDeactivate(t, mockStore, &DeactivateOpts{BundleNames: map[string]struct{}{"bundle1": {}}})

	// Expect the store to have been cleared out after deactivating the bundle
	verifyReadBundleNames(t, mockStore, nil)
	verifyResultRead(t, mockStore, `{"system": {"bundles": {}, "modules": {}}}`)

	mockStore.AssertValid(t)
}

func TestBundleLazyModeLifecycleRawInvalidData(t *testing.T) {
	tests := map[string]struct {
		files [][2]string
		err   error
	}{
		"non-object root": {[][2]string{{"/data.json", `[1,2,3]`}}, errors.New("root value must be object")},
		"invalid yaml":    {[][2]string{{"/a/b/data.yaml", `"foo`}}, errors.New("yaml: found unexpected end of stream")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			bundle := must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(tc.files), "")).
				WithBundleEtag("bar").
				WithLazyLoadingMode(true).
				Read())(t)

			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

			err := Activate(&ActivateOpts{
				Ctx:      t.Context(),
				Store:    mockStore,
				Txn:      txn,
				Compiler: ast.NewCompiler(),
				Metrics:  metrics.NoOp(),
				Bundles:  map[string]*Bundle{"bundle1": &bundle},
			})

			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if tc.err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
			}
		})
	}
}

func TestBundleLazyModeLifecycle(t *testing.T) {
	mockStore := mock.New()
	compiler := ast.NewCompiler()
	extraMods := map[string]*ast.Module{"mod1": ast.MustParseModule("package x\np = true")}

	// v1 bundle
	b1Files := [][2]string{
		{"/.manifest", `{"roots": ["a"], "rego_version": 1}`},
		{"a/policy.rego", "package a\np contains 42 if { true }"},
		{"/data.json", `{"a": {"b": "foo"}}`},
	}

	bundle1 := must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(b1Files), "")).
		WithBundleEtag("foo").
		WithLazyLoadingMode(true).
		WithBundleName("bundle1").
		Read())(t)

	// v0 bundle
	bundle2 := bundleFromFiles(t, "bundle2", [][2]string{
		{"/.manifest", `{"roots": ["b", "c"], "rego_version": 0}`},
		{"b/policy.rego", `package b
			p[42] { true }
		`},
		{"/data.json", `{}`},
	})

	bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles, ExtraModules: extraMods})

	// Ensure the bundle was activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyBundleModulesCompiled(t, compiler, bundles)
	verifyResultRead(t, mockStore, `{
		"a": {
			"b": "foo"
		},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "",
						"roots": ["a"],
						"rego_version": 1
					},
					"etag": "foo"
				},
				"bundle2": {
					"manifest": {
						"revision": "",
						"roots": ["b", "c"],
						"rego_version": 0
					},
					"etag": ""
				}
			},
			"modules":{
				"bundle1/a/policy.rego":{
					"rego_version":1
				},
				"bundle2/b/policy.rego":{
					"rego_version":0
				}
			}
		}
	}`)

	// Ensure that the extra module was included
	if _, ok := compiler.Modules["mod1"]; !ok {
		t.Fatalf("expected extra module to be compiled")
	}

	mustDeactivate(t, mockStore, &DeactivateOpts{BundleNames: map[string]struct{}{"bundle1": {}, "bundle2": {}}})

	// Expect the store to have been cleared out after deactivating the bundles
	verifyReadBundleNames(t, mockStore, nil)
	verifyResultRead(t, mockStore, `{"system": {"bundles": {}, "modules": {}}}`)

	mockStore.AssertValid(t)
}

func TestBundleLazyModeLifecycleRawNoBundleRoots(t *testing.T) {
	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
		{"/.manifest", `{"revision": "rev-1"}`},
	}

	bundle := must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
		WithBundleEtag("foo").
		WithLazyLoadingMode(true).
		Read())(t)

	compiler := ast.NewCompiler()
	mockStore := mock.New()
	bundles := map[string]*Bundle{"bundle1": &bundle}

	mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles, Compiler: compiler})

	// Ensure the bundle was activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyBundleModulesCompiled(t, compiler, bundles)
	verifyResultRead(t, mockStore, `{
		"a": {
			"b": {
				"c": [1,2,3],
				"d": true,
				"y": {
					"foo": 1
				},
				"z": true
			}
		},
		"x": {
			"y": true
		},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "rev-1",
						"roots": [""]
					},
					"etag": "foo"
				}
			}
		}
	}`)

	files = [][2]string{
		{"/c/data.json", `{"hello": "world"}`},
		{"/.manifest", `{"revision": "rev-2"}`},
	}

	bundle = must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
		WithBundleEtag("bar").
		WithLazyLoadingMode(true).
		Read())(t)

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: map[string]*Bundle{"bundle1": &bundle}})
	verifyResultRead(t, mockStore, `{
		"c": {
			"hello": "world"
		},
		"system": {
		"bundles": {
			"bundle1": {
				"manifest": {
					"revision": "rev-2",
					"roots": [""]
				},
				"etag": "bar"
			}
		}
		}
	}`)
}

func TestBundleLazyModeLifecycleRawNoBundleRootsDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		files := [][2]string{
			{"/a/b/c/data.json", "[1,2,3]"},
			{"/a/b/d/data.json", "true"},
			{"/a/b/y/data.yaml", `foo: 1`},
			{"/example/example.rego", `package example`},
			{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
			{"/.manifest", `{"revision": "rev-1"}`},
		}

		bundle := must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
			WithBundleEtag("foo").
			WithLazyLoadingMode(true).
			Read())(t)

		bundles := map[string]*Bundle{"bundle1": &bundle}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the bundle was activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)
		verifyBundleModulesCompiled(t, compiler, bundles)
		verifyResultRead(t, store, `{
			"a": {
				"b": {
					"c": [1,2,3],
					"d": true,
					"y": {
						"foo": 1
					},
					"z": true
				}
			},
			"x": {
				"y": true
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "rev-1",
							"roots": [""]
						},
						"etag": "foo"
					}
				}
			}
		}`)

		files = [][2]string{
			{"/c/data.json", `{"hello": "world"}`},
			{"/.manifest", `{"revision": "rev-2"}`},
		}

		bundle = must(NewCustomReader(NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
			WithBundleEtag("bar").
			WithLazyLoadingMode(true).
			Read())(t)

		bundles = map[string]*Bundle{"bundle1": &bundle}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})
		verifyResultRead(t, store, `{
			"c": {
				"hello": "world"
			},
			"system": {
				"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "rev-2",
						"roots": [""]
					},
					"etag": "bar"
				}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleNoBundleRoots(t *testing.T) {
	mockStore := mock.New()
	compiler := ast.NewCompiler()

	mod1 := "package a\np = true"

	b := Bundle{
		Manifest: Manifest{Revision: "rev-1"},
		Data: unpack(map[string]any{
			"a.b":   "foo",
			"a.e.f": "bar",
			"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
		}),
		Modules: []ModuleFile{moduleFile("a/policy.rego", mod1)},
		Etag:    "foo",
	}

	bundle1 := bundleFromRoundtrip(t, "bundle1", b)
	bundles := map[string]*Bundle{"bundle1": &bundle1}

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles})
	verifyResultRead(t, mockStore, `{
         "a": {
            "b": "foo",
            "e": {
               "f": "bar"
            },
            "x": [{"name": "john"}, {"name": "jane"}]
         },
         "system": {
            "bundles": {
               "bundle1": {
                  "manifest": {
                     "revision": "rev-1",
                     "roots": [""]
                  },
                  "etag": ""
               }
            }
         }
    }`)

	// add a new bundle with no roots. this means all the data from the currently activated should be removed
	bundle2 := bundleFromRoundtrip(t, "bundle1", Bundle{
		Manifest: Manifest{Revision: "rev-2"},
		Data:     unpack(map[string]any{"c.hello": "world"}),
		Etag:     "bar",
	})

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: map[string]*Bundle{"bundle1": &bundle2}})
	verifyResultRead(t, mockStore, `{
		"c": {
			"hello": "world"
		},
		"system": {
			"bundles": {
			"bundle1": {
				"manifest": {
					"revision": "rev-2",
					"roots": [""]
				},
				"etag": ""
			}
			}
		}
	}`)
}

func TestBundleLazyModeLifecycleNoBundleRootsDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()
		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1"},
			Data: unpack(map[string]any{
				"a.b":   "foo",
				"a.e.f": "bar",
				"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
			}),
			Modules: []ModuleFile{moduleFile("a/policy.rego", mod1)},
			Etag:    "foo",
		}

		bundle1 := bundleFromRoundtrip(t, "bundle1", b)
		bundles := map[string]*Bundle{"bundle1": &bundle1}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundle was activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)
		verifyBundleModulesCompiled(t, compiler, bundles)
		verifyResultRead(t, store, `{
			"a": {
				"b": "foo",
				"e": {
				"f": "bar"
				},
				"x": [{"name": "john"}, {"name": "jane"}]
			},
			"system": {
				"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "rev-1",
						"roots": [""]
					},
					"etag": ""
				}
				}
			}
		}`)

		// add a new bundle with no roots. this means all the data from the currently activated should be removed
		bundle2 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Revision: "rev-2"},
			Data:     unpack(map[string]any{"c.hello": "world"}),
			Etag:     "bar",
		})

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: map[string]*Bundle{"bundle1": &bundle2}})
		verifyResultRead(t, store, `{
			"c": {
				"hello": "world"
			},
			"system": {
				"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "rev-2",
						"roots": [""]
					},
					"etag": ""
				}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleMixBundleTypeActivationDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		mod1 := "package a\np = true"

		bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Revision: "snap-1", Roots: &[]string{"a"}},
			Data: unpack(map[string]any{
				"a.b":   "foo",
				"a.e.f": "bar",
				"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
			}),
			Modules: []ModuleFile{moduleFile("a/policy.rego", mod1)},
			Etag:    "foo",
		})

		// create a delta bundle and activate it

		// add a new object member
		bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
			Manifest: Manifest{Revision: "delta-1", Roots: &[]string{"x"}},
			Patch:    Patch{Data: []PatchOperation{{Op: "upsert", Path: "/x/y", Value: []string{"foo", "bar"}}}},
			Etag:     "bar",
		})
		bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the patches were applied
		verifyResultRead(t, store, `{
			"a": {
				"b": "foo",
				"e": {
				"f": "bar"
				},
				"x": [{"name": "john"}, {"name": "jane"}]
			},
			"x": {
				"y": ["foo","bar"]
			},
			"system": {
				"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "snap-1",
						"roots": ["a"]
					},
					"etag": ""
				},
				"bundle2": {
					"manifest": {
						"revision": "delta-1",
						"roots": ["x"]
					},
					"etag": ""
				}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleOldBundleEraseDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)

		compiler := ast.NewCompiler()
		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1", Roots: &[]string{"a"}},
			Data: unpack(map[string]any{
				"a.b":   "foo",
				"a.e.f": "bar",
				"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
			}),
			Modules: []ModuleFile{moduleFile("a/policy.rego", mod1)},
			Etag:    "foo",
		}

		bundle1 := bundleFromRoundtrip(t, "bundle1", b)
		bundles := map[string]*Bundle{"bundle1": &bundle1}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundle was activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)
		verifyBundleModulesCompiled(t, compiler, bundles)
		verifyResultRead(t, store, `{
			"a": {
				"b": "foo",
				"e": {
					"f": "bar"
				},
				"x": [{"name": "john"}, {"name": "jane"}]
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "rev-1",
							"roots": ["a"]
						},
						"etag": ""
					}
				}
			}
		}`)

		// add a new bundle and verify data from the currently activated is removed
		bundle2 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Revision: "rev-2", Roots: &[]string{"c"}},
			Data:     unpack(map[string]any{"c.hello": "world"}),
			Etag:     "bar",
		})
		bundles = map[string]*Bundle{"bundle1": &bundle2}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundle was activated
		verifyResultRead(t, store, `{
			"c": {
				"hello": "world"
			},
			"system": {
				"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "rev-2",
						"roots": ["c"]
					},
					"etag": ""
				}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleRestoreBackupDB(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1", Roots: &[]string{"a"}},
			Data: unpack(map[string]any{
				"a.b":   "foo",
				"a.e.f": "bar",
				"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
			}),
			Modules: []ModuleFile{moduleFile("a/policy.rego", "package a\np = true")},
			Etag:    "foo",
		}

		bundle1 := bundleFromRoundtrip(t, "bundle1", b)
		bundles := map[string]*Bundle{"bundle1": &bundle1}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundle was activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)
		verifyBundleModulesCompiled(t, compiler, bundles)
		verifyResultRead(t, store, `{
			"a": {
				"b": "foo",
				"e": {
				"f": "bar"
				},
				"x": [{"name": "john"}, {"name": "jane"}]
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "rev-1",
							"roots": ["a"]
						},
						"etag": ""
					}
				}
			}
		}`)

		// add a new bundle but abort the transaction and verify only old the bundle data is kept in store
		bundle2 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Revision: "rev-2", Roots: &[]string{"c"}},
			Data:     unpack(map[string]any{"c.hello": "world"}),
			Etag:     "bar",
		})

		// can't use mustActivate here because we want to abort the txn!
		txn := storage.NewTransactionOrDie(t.Context(), store, storage.WriteParams)
		err := Activate(&ActivateOpts{
			Ctx:      t.Context(),
			Store:    store,
			Txn:      txn,
			Metrics:  metrics.NoOp(),
			Compiler: compiler,
			Bundles:  map[string]*Bundle{"bundle1": &bundle2},
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		store.Abort(t.Context(), txn)

		verifyResultRead(t, store, `{
			"a": {
				"b": "foo",
				"e": {
					"f": "bar"
				},
				"x": [{"name": "john"}, {"name": "jane"}]
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "rev-1",
							"roots": ["a"]
						},
						"etag": ""
					}
				}
			}
		}`)

		// check symlink is created
		symlink := filepath.Join(dir, "active")
		if _, err := os.Lstat(symlink); err != nil {
			t.Fatal(err)
		}

		// check symlink target
		if _, err := filepath.EvalSymlinks(symlink); err != nil {
			t.Fatalf("eval symlinks: %v", err)
		}
	})
}

func TestDeltaBundleLazyModeLifecycleDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		mod1 := "package a\np = true"
		mod2 := "package b\np = true"
		bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Roots: &[]string{"a"}},
			Data: unpack(map[string]any{
				"a.b":   "foo",
				"a.e.f": "bar",
				"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
			}),
			Modules: []ModuleFile{moduleFile("a/policy.rego", mod1)},
			Etag:    "foo",
		})

		bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
			Manifest: Manifest{Roots: &[]string{"b", "c"}},
			Modules:  []ModuleFile{moduleFile("b/policy.rego", mod2)},
			Etag:     "foo",
		})

		bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundles were activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)
		verifyBundleModulesCompiled(t, compiler, bundles)

		// create a delta bundle and activate it
		deltaBundles := map[string]*Bundle{
			"bundle1": {
				Manifest: Manifest{Revision: "delta-1", Roots: &[]string{"a"}},
				Patch: Patch{Data: []PatchOperation{
					{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}},
					{Op: "upsert", Path: "/a/c/d/-", Value: "baz"}, // append value to array
					{Op: "replace", Path: "a/b", Value: "bar"},     // replace a value
				}},
				Etag: "bar",
			},
			"bundle2": {
				Manifest: Manifest{Revision: "delta-2", Roots: &[]string{"b", "c"}},
				Patch: Patch{Data: []PatchOperation{
					{Op: "upsert", Path: "/c/d", Value: []string{"foo", "bar"}},
				}},
				Etag: "baz",
			},
			"bundle3": {
				Manifest: Manifest{Roots: &[]string{"d"}},
				Data:     unpack(map[string]any{"d.e": "foo"}),
			},
		}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: deltaBundles})

		// check the modules from the snapshot bundles are on the compiler
		verifyBundleModulesCompiled(t, compiler, bundles)
		verifyResultRead(t, store, `{
			"a": {
		     	"b": "bar",
		     	"c": {
					"d": ["foo", "bar", "baz"]
		     	},
				"e": {
					"f": "bar"
				},
			   "x": [{"name": "john"}, {"name": "jane"}]
			},
			"c": {"d": ["foo", "bar"]},
			"d": {"e": "foo"},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "delta-1",
							"roots": ["a"]
						},
						"etag": "bar"
					},
					"bundle2": {
						"manifest": {
							"revision": "delta-2",
							"roots": ["b", "c"]
						},
						"etag": "baz"
					},
					"bundle3": {
						"manifest": {
							"revision": "",
							"roots": ["d"]
						},
						"etag": ""
					}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleOverlappingBundleRoots(t *testing.T) {
	mockStore := mock.New()

	bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
		Manifest: Manifest{Revision: "foo", Roots: &[]string{"a/b", "a/c", "a/d"}},
		Data: unpack(map[string]any{
			"a.b":   "foo",
			"a.c.d": "bar",
			"a.d":   []map[string]string{{"name": "john"}, {"name": "jane"}},
		}),
	})

	bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
		Manifest: Manifest{Revision: "bar", Roots: &[]string{"a/e"}},
		Data:     unpack(map[string]any{"a.e.f": "bar"}),
	})

	bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

	mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles})

	// Ensure the snapshot bundles were activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyResultRead(t, mockStore, `{
		"a": {
			"b": "foo",
			"c": {
				"d": "bar"
			},
			"e": {
				"f": "bar"
			},
			"d": [{"name": "john"}, {"name": "jane"}]
		},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "foo",
						"roots": ["a/b", "a/c", "a/d"]
					},
					"etag": ""
				},
				"bundle2": {
					"manifest": {
						"revision": "bar",
						"roots": ["a/e"]
					},
					"etag": ""
				}
			}
		}
	}`)
}

func TestBundleLazyModeLifecycleOverlappingBundleRootsDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
			Manifest: Manifest{Revision: "foo", Roots: &[]string{"a/b/c", "a/b/d", "a/b/e"}},
			Data: unpack(map[string]any{
				"a.b.c": "bar",
				"a.b.d": []map[string]string{{"name": "john"}, {"name": "jane"}},
				"a.b.e": []string{"foo", "bar"},
			}),
		})

		bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
			Manifest: Manifest{Revision: "bar", Roots: &[]string{"a/b/f"}},
			Data:     unpack(map[string]any{"a.b.f.hello": "world"}),
		})

		bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundles were activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)

		// Ensure the patches were applied
		verifyResultRead(t, store, `{
			"a": {
				"b": {
					"c": "bar",
					"d": [{"name": "john"}, {"name": "jane"}],
					"e": ["foo", "bar"],
					"f": {"hello": "world"}
				}
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "foo",
							"roots": ["a/b/c", "a/b/d", "a/b/e"]
						},
						"etag": ""
					},
					"bundle2": {
						"manifest": {
							"revision": "bar",
							"roots": ["a/b/f"]
						},
						"etag": ""
					}
				}
			}
		}`)
	})
}

func TestBundleLazyModeLifecycleRawOverlappingBundleRoots(t *testing.T) {
	bundle1 := bundleFromFiles(t, "bundle1", [][2]string{
		{"/a/b/x/data.json", "[1,2,3]"},
		{"/a/c/y/data.json", "true"},
		{"/a/d/z/data.yaml", `foo: 1`},
		{"/data.json", `{"a": {"b": {"z": true}}}`},
		{"/.manifest", `{"revision": "foo", "roots": ["a/b", "a/c", "a/d"]}`},
	})

	bundle2 := bundleFromFiles(t, "bundle2", [][2]string{
		{"/a/e/x/data.json", "[4,5,6]"},
		{"/data.json", `{"a": {"e": {"f": true}}}`},
		{"/.manifest", `{"revision": "bar", "roots": ["a/e"]}`},
	})

	bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}
	mockStore := mock.New()

	mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles})

	// Ensure the snapshot bundles were activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)

	// Ensure the patches were applied
	verifyResultRead(t, mockStore, `{
		"a": {
			"b": {
				"x": [1,2,3],
				"z": true
			},
			"c": {
				"y": true
			},
			"d": {
				"z": {"foo": 1}
			},
			"e": {
				"x": [4,5,6],
				"f": true
			}
		},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "foo",
						"roots": ["a/b", "a/c", "a/d"]
					},
					"etag": ""
				},
				"bundle2": {
					"manifest": {
						"revision": "bar",
						"roots": ["a/e"]
					},
					"etag": ""
				}
			}
		}
	}`)
}

func TestBundleLazyModeLifecycleRawOverlappingBundleRootsDiskStorage(t *testing.T) {
	test.WithTempFS(nil, func(dir string) {
		store := must(disk.New(t.Context(), logging.NewNoOpLogger(), nil, disk.Options{Dir: dir}))(t)
		compiler := ast.NewCompiler()

		bundle1 := bundleFromFiles(t, "bundle1", [][2]string{
			{"/a/b/u/data.json", "[1,2,3]"},
			{"/a/b/v/data.json", "true"},
			{"/a/b/w/data.yaml", `foo: 1`},
			{"/data.json", `{"a": {"b": {"x": true}}}`},
			{"/.manifest", `{"revision": "foo", "roots": ["a/b"]}`},
		})

		bundle2 := bundleFromFiles(t, "bundle2", [][2]string{
			{"/a/c/x/data.json", "[4,5,6]"},
			{"/data.json", `{"a": {"c": {"y": true}}}`},
			{"/.manifest", `{"revision": "bar", "roots": ["a/c"]}`},
		})

		bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

		mustActivate(t, store, &ActivateOpts{Compiler: compiler, Bundles: bundles})

		// Ensure the snapshot bundles were activated
		verifyReadBundleNames(t, store, nil, util.Keys(bundles)...)

		// Ensure the patches were applied
		verifyResultRead(t, store, `{
			"a": {
				"b": {
					"u": [1,2,3],
					"v": true,
					"w": {"foo": 1},
					"x": true
				},
				"c": {
					"x": [4,5,6],
					"y": true
				}
			},
			"system": {
				"bundles": {
					"bundle1": {
						"manifest": {
							"revision": "foo",
							"roots": ["a/b"]
						},
						"etag": ""
					},
					"bundle2": {
						"manifest": {
							"revision": "bar",
							"roots": ["a/c"]
						},
						"etag": ""
					}
				}
			}
		}`)
	})
}

func TestDeltaBundleLazyModeLifecycle(t *testing.T) {
	mockStore := mock.New()
	compiler := ast.NewCompiler()

	mod1 := "package a\np = true"
	mod2 := "package b\np = true"

	bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
		Manifest: Manifest{Roots: &[]string{"a"}},
		Data: unpack(map[string]any{
			"a.b":   "foo",
			"a.e.f": "bar",
			"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
		}),
		Modules: []ModuleFile{moduleFile("policy.rego", mod1)},
		Etag:    "foo",
	})

	bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
		Manifest:        Manifest{Roots: &[]string{"b", "c"}},
		Modules:         []ModuleFile{moduleFile("policy.rego", mod2)},
		Etag:            "foo",
		lazyLoadingMode: true,
		sizeLimitBytes:  DefaultSizeLimitBytes + 1,
	})

	bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles})

	// Ensure the snapshot bundles were activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyBundleModulesCompiled(t, compiler, bundles)

	// create a delta bundle and activate it
	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{Revision: "delta-1", Roots: &[]string{"a"}},
			Patch: Patch{Data: []PatchOperation{
				{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}},
				{Op: "upsert", Path: "/a/c/d/-", Value: "baz"},
				{Op: "upsert", Path: "/a/x/1", Value: map[string]string{"name": "alice"}},
				{Op: "replace", Path: "a/b", Value: "bar"},
				{Op: "remove", Path: "a/e"},
				{Op: "upsert", Path: "a/y/~0z", Value: []int{1, 2, 3}},
			}},
			Etag: "bar",
		},
		"bundle2": {
			Manifest: Manifest{Revision: "delta-2", Roots: &[]string{"b", "c"}},
			Patch:    Patch{Data: []PatchOperation{{Op: "upsert", Path: "/c/d", Value: []string{"foo", "bar"}}}},
			Etag:     "baz",
		},
		"bundle3": {
			Manifest: Manifest{Roots: &[]string{"d"}},
			Data:     unpack(map[string]any{"d.e": "foo"}),
		},
	}

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: deltaBundles})

	// check the modules from the snapshot bundles are on the compiler
	verifyBundleModulesCompiled(t, compiler, bundles)

	// Ensure the patches were applied
	verifyResultRead(t, mockStore, `{
		"a": {
          "b": "bar",
	       "c": {
				"d": ["foo", "bar", "baz"]
          },
		   "x": [{"name": "john"}, {"name": "alice"}, {"name": "jane"}],
		   "y": {"~z": [1, 2, 3]}
		},
		"c": {"d": ["foo", "bar"]},
		"d": {"e": "foo"},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "delta-1",
						"roots": ["a"]
					},
					"etag": "bar"
				},
				"bundle2": {
					"manifest": {
						"revision": "delta-2",
						"roots": ["b", "c"]
					},
					"etag": "baz"
				},
				"bundle3": {
					"manifest": {
						"revision": "",
						"roots": ["d"]
					},
					"etag": ""
				}
			}
		}
	}`)

	mockStore.AssertValid(t)
}

func TestDeltaBundleLazyModeWithDefaultRules(t *testing.T) {
	mockStore := mock.New()
	compiler := ast.NewCompiler()

	bundle1 := bundleFromRoundtrip(t, "bundle1", Bundle{
		Manifest: Manifest{Roots: &[]string{"a"}},
		Data: unpack(map[string]any{
			"a.b":   "foo",
			"a.e.f": "bar",
			"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
		}),
		Modules: []ModuleFile{moduleFile("policy.rego", "package a\ndefault p = true")},
		Etag:    "foo",
	})

	bundle2 := bundleFromRoundtrip(t, "bundle2", Bundle{
		Manifest:        Manifest{Roots: &[]string{"b", "c"}},
		Modules:         []ModuleFile{moduleFile("policy.rego", "package b\ndefault p = true")},
		Etag:            "foo",
		lazyLoadingMode: true,
		sizeLimitBytes:  DefaultSizeLimitBytes + 1,
	})

	bundles := map[string]*Bundle{"bundle1": &bundle1, "bundle2": &bundle2}

	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles})

	// Ensure the snapshot bundles were activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
	verifyBundleModulesCompiled(t, compiler, bundles)

	// create a delta bundle and activate it

	// add a new object member
	p1 := PatchOperation{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}}

	// append value to array
	p2 := PatchOperation{Op: "upsert", Path: "/a/c/d/-", Value: "baz"}

	// insert value in array
	p3 := PatchOperation{Op: "upsert", Path: "/a/x/1", Value: map[string]string{"name": "alice"}}

	// replace a value
	p4 := PatchOperation{Op: "replace", Path: "a/b", Value: "bar"}

	// remove a value
	p5 := PatchOperation{Op: "remove", Path: "a/e"}

	// add a new object with an escaped character in the path
	p6 := PatchOperation{Op: "upsert", Path: "a/y/~0z", Value: []int{1, 2, 3}}

	// add a new object root
	p7 := PatchOperation{Op: "upsert", Path: "/c/d", Value: []string{"foo", "bar"}}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{Revision: "delta-1", Roots: &[]string{"a"}},
			Patch:    Patch{Data: []PatchOperation{p1, p2, p3, p4, p5, p6}},
			Etag:     "bar",
		},
		"bundle2": {
			Manifest: Manifest{Revision: "delta-2", Roots: &[]string{"b", "c"}},
			Patch:    Patch{Data: []PatchOperation{p7}},
			Etag:     "baz",
		},
		"bundle3": {
			Manifest: Manifest{Roots: &[]string{"d"}},
			Data:     unpack(map[string]any{"d.e": "foo"}),
		},
	}

	expectedModuleCount := len(compiler.Modules)
	mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: deltaBundles})

	if expectedModuleCount != len(compiler.Modules) {
		t.Fatalf("Expected %d modules, got %d", expectedModuleCount, len(compiler.Modules))
	}

	// check the modules from the snapshot bundles are on the compiler
	verifyBundleModulesCompiled(t, compiler, bundles)

	// Ensure the patches were applied
	verifyResultRead(t, mockStore, `{
		"a": {
          "b": "bar",
	       "c": {
				"d": ["foo", "bar", "baz"]
          },
		   "x": [{"name": "john"}, {"name": "alice"}, {"name": "jane"}],
		   "y": {"~z": [1, 2, 3]}
		},
		"c": {"d": ["foo", "bar"]},
		"d": {"e": "foo"},
		"system": {
			"bundles": {
				"bundle1": {
					"manifest": {
						"revision": "delta-1",
						"roots": ["a"]
					},
					"etag": "bar"
				},
				"bundle2": {
					"manifest": {
						"revision": "delta-2",
						"roots": ["b", "c"]
					},
					"etag": "baz"
				},
				"bundle3": {
					"manifest": {
						"revision": "",
						"roots": ["d"]
					},
					"etag": ""
				}
			}
		}
	}`)
	mockStore.AssertValid(t)
}

func TestBundleLifecycle(t *testing.T) {
	tests := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New(inmem.OptReturnASTValuesOnRead(tc.readAst))
			compiler := ast.NewCompiler()
			extraMods := map[string]*ast.Module{
				"mod1": ast.MustParseModule("package x\np = true"),
			}

			bundles := map[string]*Bundle{
				"bundle1": {
					Manifest: Manifest{Roots: &[]string{"a"}},
					Data:     unpack(map[string]any{"a.b": "foo"}),
					Modules:  []ModuleFile{moduleFile("a/policy.rego", "package a\np = true")},
					Etag:     "foo",
				},
				"bundle2": {
					Manifest: Manifest{Roots: &[]string{"b", "c"}},
					Modules:  []ModuleFile{moduleFile("b/policy.rego", "package b\np = true")},
				},
			}

			mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles, ExtraModules: extraMods})

			// Ensure the bundle was activated
			verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
			verifyBundleModulesCompiled(t, compiler, bundles)

			actual := mustRead(t, mockStore, nil, storage.RootPath)
			expectedRaw := `{
				"a": {
					"b": "foo"
				},
				"system": {
					"bundles": {
						"bundle1": {
							"manifest": {
								"revision": "",
								"roots": ["a"]
							},
							"etag": "foo"
						},
						"bundle2": {
							"manifest": {
								"revision": "",
								"roots": ["b", "c"]
							},
							"etag": ""
						}
					},
					"modules": {
						"bundle1/a/policy.rego": {
							"rego_version": 1
						},
						"bundle2/b/policy.rego": {
							"rego_version": 1
						}
					}
				}
			}`
			assertEqual(t, tc.readAst, expectedRaw, actual)

			// Ensure that the extra module was included
			if _, ok := compiler.Modules["mod1"]; !ok {
				t.Fatalf("expected extra module to be compiled")
			}

			mustDeactivate(t, mockStore, &DeactivateOpts{
				BundleNames: map[string]struct{}{"bundle1": {}, "bundle2": {}},
			})

			// Expect the store to have been cleared out after deactivating the bundles
			txn := storage.NewTransactionOrDie(t.Context(), mockStore)
			verifyReadBundleNames(t, mockStore, txn)

			expectedRaw = `{"system": {"bundles": {}, "modules": {}}}`
			assertEqual(t, tc.readAst, expectedRaw, mustRead(t, mockStore, txn, storage.RootPath))

			mockStore.Abort(t.Context(), txn)
			mockStore.AssertValid(t)
		})
	}
}

func TestDeltaBundleLifecycle(t *testing.T) {
	tests := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New(inmem.OptReturnASTValuesOnRead(tc.readAst))
			compiler := ast.NewCompiler()

			bundles := map[string]*Bundle{
				"bundle1": {
					Manifest: Manifest{Roots: &[]string{"a"}},
					Data: unpack(map[string]any{
						"a.b":   "foo",
						"a.e.f": "bar",
						"a.x":   []map[string]string{{"name": "john"}, {"name": "jane"}},
					}),
					Modules: []ModuleFile{moduleFile("a/policy.rego", "package a\ndefault p = true")},
					Etag:    "foo",
				},
				"bundle2": {
					Manifest: Manifest{Roots: &[]string{"b", "c"}},
					Modules:  []ModuleFile{moduleFile("b/policy.rego", "package b\ndefault p = true")},
				},
			}

			mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: bundles})

			// Ensure the snapshot bundles were activated
			verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)
			verifyBundleModulesCompiled(t, compiler, bundles)

			// create a delta bundle and activate it

			// add a new object member
			p1 := PatchOperation{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}}

			// append value to array
			p2 := PatchOperation{Op: "upsert", Path: "/a/c/d/-", Value: "baz"}

			// insert value in array
			p3 := PatchOperation{Op: "upsert", Path: "/a/x/1", Value: map[string]string{"name": "alice"}}

			// replace a value
			p4 := PatchOperation{Op: "replace", Path: "a/b", Value: "bar"}

			// remove a value
			p5 := PatchOperation{Op: "remove", Path: "a/e"}

			// add a new object with an escaped character in the path
			p6 := PatchOperation{Op: "upsert", Path: "a/y/~0z", Value: []int{1, 2, 3}}

			// add a new object root
			p7 := PatchOperation{Op: "upsert", Path: "/c/d", Value: []string{"foo", "bar"}}

			deltaBundles := map[string]*Bundle{
				"bundle1": {
					Manifest: Manifest{Revision: "delta-1", Roots: &[]string{"a"}},
					Patch:    Patch{Data: []PatchOperation{p1, p2, p3, p4, p5, p6}},
					Etag:     "bar",
				},
				"bundle2": {
					Manifest: Manifest{Revision: "delta-2", Roots: &[]string{"b", "c"}},
					Patch:    Patch{Data: []PatchOperation{p7}},
					Etag:     "baz",
				},
				"bundle3": {
					Manifest: Manifest{Roots: &[]string{"d"}},
					Data:     unpack(map[string]any{"d.e": "foo"}),
				},
			}

			mustActivate(t, mockStore, &ActivateOpts{Compiler: compiler, Bundles: deltaBundles})

			// check the modules from the snapshot bundles are on the compiler
			verifyBundleModulesCompiled(t, compiler, bundles)

			// Ensure the patches were applied
			actual := mustRead(t, mockStore, nil, storage.RootPath)
			expectedRaw := `{
				"a": {
					"b": "bar",
					"c": {
						"d": ["foo", "bar", "baz"]
					},
					"x": [{"name": "john"}, {"name": "alice"}, {"name": "jane"}],
					"y": {"~z": [1, 2, 3]}
				},
				"c": {"d": ["foo", "bar"]},
				"d": {"e": "foo"},
				"system": {
					"bundles": {
						"bundle1": {
							"manifest": {
								"revision": "delta-1",
								"roots": ["a"]
							},
							"etag": "bar"
						},
						"bundle2": {
							"manifest": {
								"revision": "delta-2",
								"roots": ["b", "c"]
							},
							"etag": "baz"
						},
						"bundle3": {
							"manifest": {
								"revision": "",
								"roots": ["d"]
							},
							"etag": ""
						}
					},
					"modules":{
						"bundle1/a/policy.rego":{
							"rego_version":1
						},
						"bundle2/b/policy.rego":{
							"rego_version":1
						}
					}
				}
			}`
			assertEqual(t, tc.readAst, expectedRaw, actual)
			mockStore.AssertValid(t)
		})
	}
}

func TestDeltaBundleActivate(t *testing.T) {
	tests := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New(inmem.OptReturnASTValuesOnRead(tc.readAst))

			// create a delta bundle
			p1 := PatchOperation{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}}

			deltaBundles := map[string]*Bundle{"bundle1": {
				Manifest: Manifest{Revision: "delta", Roots: &[]string{"a"}},
				Patch:    Patch{Data: []PatchOperation{p1}},
				Etag:     "foo",
			}}

			mustActivate(t, mockStore, &ActivateOpts{Bundles: deltaBundles})

			txn := storage.NewTransactionOrDie(t.Context(), mockStore)

			// Ensure the delta bundle was activated
			verifyReadBundleNames(t, mockStore, txn, util.Keys(deltaBundles)...)

			// Ensure the patches were applied
			actual := mustRead(t, mockStore, txn, storage.RootPath)
			expectedRaw := `{
				"a": {
					"c": {
						"d": ["foo", "bar"]
					}
				},
				"system": {
					"bundles": {
						"bundle1": {
							"manifest": {
								"revision": "delta",
								"roots": ["a"]
							},
							"etag": "foo"
						}
					}
				}
			}`
			assertEqual(t, tc.readAst, expectedRaw, actual)

			// Stop the "read" transaction
			mockStore.Abort(t.Context(), txn)
			mockStore.AssertValid(t)
		})
	}
}

func TestDeltaBundleBadManifest(t *testing.T) {
	mockStore := mock.New()

	bundles := map[string]*Bundle{"bundle1": {
		Manifest: Manifest{Roots: &[]string{"a"}},
		Modules:  []ModuleFile{moduleFile("a/policy.rego", "package a\np = true")},
	}}

	mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles})

	// Ensure the snapshot bundle was activated
	verifyReadBundleNames(t, mockStore, nil, util.Keys(bundles)...)

	// create a delta bundle with a different manifest from the snapshot bundle
	deltaBundles := map[string]*Bundle{"bundle1": {
		Manifest: Manifest{Roots: &[]string{"b"}},
		Patch:    Patch{Data: []PatchOperation{{Op: "upsert", Path: "/a/c/d", Value: []string{"foo", "bar"}}}},
	}}

	txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:      t.Context(),
		Store:    mockStore,
		Txn:      txn,
		Compiler: ast.NewCompiler(),
		Metrics:  metrics.NoOp(),
		Bundles:  deltaBundles,
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expected := "delta bundle 'bundle1' has wasm resolvers or manifest roots that are different from those in the store"
	if err.Error() != expected {
		t.Fatalf("Expected error %v but got %v", expected, err.Error())
	}

	mockStore.AssertValid(t)
}

func TestEraseData(t *testing.T) {
	storeReadModes := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	cases := []struct {
		note        string
		initialData map[string]any
		roots       []string
		expectErr   bool
		expected    string
	}{
		{
			note: "erase all",
			initialData: map[string]any{
				"a.b": "foo",
				"b":   "bar",
			},
			roots:    []string{"a", "b"},
			expected: `{}`,
		},
		{
			note: "erase none",
			initialData: map[string]any{
				"a.b": "foo",
				"b":   "bar",
			},
			roots:    []string{},
			expected: `{"a": {"b": "foo"}, "b": "bar"}`,
		},
		{
			note: "erase partial",
			initialData: map[string]any{
				"a.b": "foo",
				"b":   "bar",
			},
			roots:    []string{"a"},
			expected: `{"b": "bar"}`,
		},
		{
			note: "erase partial path",
			initialData: map[string]any{
				"a.b":   "foo",
				"a.c.d": 123,
			},
			roots:    []string{"a/c/d"},
			expected: `{"a": {"b": "foo", "c":{}}}`,
		},
	}

	for _, rm := range storeReadModes {
		t.Run(rm.note, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.note, func(t *testing.T) {
					mockStore := mock.NewWithData(unpack(tc.initialData), inmem.OptReturnASTValuesOnRead(rm.readAst))
					txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

					roots := map[string]struct{}{}
					for _, root := range tc.roots {
						roots[root] = struct{}{}
					}

					if err := eraseData(t.Context(), mockStore, txn, roots); !tc.expectErr && err != nil {
						t.Fatalf("unepected error: %s", err)
					} else if tc.expectErr && err == nil {
						t.Fatalf("expected error, got: %s", err)
					}

					mustCommit(t, mockStore, txn)
					mockStore.AssertValid(t)

					txn = storage.NewTransactionOrDie(t.Context(), mockStore)
					assertEqual(t, rm.readAst, tc.expected, mustRead(t, mockStore, txn, storage.RootPath))
				})
			}
		})
	}
}

func TestErasePolicies(t *testing.T) {
	cases := []struct {
		note              string
		initialPolicies   map[string][]byte
		roots             []string
		expectErr         bool
		expectedRemaining []string
	}{
		{
			note: "erase all",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
			},
			roots:             []string{""},
			expectedRemaining: []string{},
		},
		{
			note: "erase none",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
				"mod2": []byte("package b\np = true"),
			},
			roots:             []string{"c"},
			expectedRemaining: []string{"mod1", "mod2"},
		},
		{
			note: "erase correct paths",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a.test\np = true"),
				"mod2": []byte("package a.test_v2\np = true"),
			},
			roots:             []string{"a/test"},
			expectedRemaining: []string{"mod2"},
		},
		{
			note: "erase some",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
				"mod2": []byte("package b\np = true"),
			},
			roots:             []string{"b"},
			expectedRemaining: []string{"mod1"},
		},
		{
			note: "error: parsing module",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
				"mod2": []byte("bad-policy-syntax"),
			},
			roots:             []string{"b"},
			expectErr:         true,
			expectedRemaining: []string{"mod1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

			for name, mod := range tc.initialPolicies {
				if err := mockStore.UpsertPolicy(t.Context(), txn, name, mod); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			roots := map[string]struct{}{}
			for _, root := range tc.roots {
				roots[root] = struct{}{}
			}
			remaining, _, err := erasePolicies(t.Context(), mockStore, txn, ast.ParserOptions{}, roots)
			if !tc.expectErr && err != nil {
				t.Fatalf("unepected error: %s", err)
			} else if tc.expectErr && err == nil {
				t.Fatalf("expected error, got: %s", err)
			}

			if !tc.expectErr {
				if len(remaining) != len(tc.expectedRemaining) {
					t.Fatalf("expected %d modules remaining, got %d", len(tc.expectedRemaining), len(remaining))
				}
				for _, name := range tc.expectedRemaining {
					if _, ok := remaining[name]; !ok {
						t.Fatalf("expected remaining module %s not found", name)
					}
				}

				mustCommit(t, mockStore, txn)
				mockStore.AssertValid(t)

				txn = storage.NewTransactionOrDie(t.Context(), mockStore)
				actualRemaining := must(mockStore.ListPolicies(t.Context(), txn))(t)

				if len(actualRemaining) != len(tc.expectedRemaining) {
					t.Fatalf("expected %d modules remaining in the store, got %d", len(tc.expectedRemaining), len(actualRemaining))
				}
				for _, expectedName := range tc.expectedRemaining {
					found := slices.Contains(actualRemaining, expectedName)
					if !found {
						t.Fatalf("expected remaining module %s not found", expectedName)
					}
				}
			}
		})
	}
}

func TestWriteData(t *testing.T) {
	storeReadModes := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	cases := []struct {
		note         string
		existingData map[string]any
		roots        []string
		data         map[string]any
		expected     string
		expectErr    bool
	}{
		{
			note:     "single root",
			roots:    []string{"a"},
			data:     map[string]any{"a.b.c": 123},
			expected: `{"a": {"b": {"c": 123}}}`,
		},
		{
			note:  "multiple roots",
			roots: []string{"a", "b/c/d"},
			data: map[string]any{
				"a":     "foo",
				"b.c.d": "bar",
			},
			expected: `{"a": "foo","b": {"c": {"d": "bar"}}}`,
		},
		{
			note:  "data not in roots",
			roots: []string{"a"},
			data: map[string]any{
				"a":     "foo",
				"b.c.d": "bar",
			},
			expected: `{"a": "foo"}`,
		},
		{
			note:         "no data",
			roots:        []string{"a"},
			existingData: map[string]any{},
			data:         map[string]any{},
			expected:     `{}`,
		},
		{
			note:         "no new data",
			roots:        []string{"a"},
			existingData: map[string]any{"a": "foo"},
			data:         map[string]any{},
			expected:     `{"a": "foo"}`,
		},
		{
			note:         "overwrite data",
			roots:        []string{"a"},
			existingData: map[string]any{"a.b": "foo"},
			data:         map[string]any{"a": "bar"},
			expected:     `{"a": "bar"}`,
		},
	}

	for _, rm := range storeReadModes {
		t.Run(rm.note, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.note, func(t *testing.T) {
					mockStore := mock.NewWithData(unpack(tc.existingData), inmem.OptReturnASTValuesOnRead(rm.readAst))
					txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

					err := writeData(t.Context(), mockStore, txn, tc.roots, unpack(tc.data))
					if !tc.expectErr && err != nil {
						t.Fatalf("unepected error: %s", err)
					} else if tc.expectErr && err == nil {
						t.Fatalf("expected error, got: %s", err)
					}

					mustCommit(t, mockStore, txn)
					mockStore.AssertValid(t)

					txn = storage.NewTransactionOrDie(t.Context(), mockStore)
					assertEqual(t, rm.readAst, tc.expected, mustRead(t, mockStore, txn, storage.RootPath))
				})
			}
		})
	}
}

type testWriteModuleCase struct {
	note         string
	bundles      map[string]*Bundle // Only need to give raw text and path for modules
	extraMods    map[string]*ast.Module
	compilerMods map[string]*ast.Module
	storeData    map[string]any
	expectErr    bool
}

func TestWriteModules(t *testing.T) {
	mod1 := ast.MustParseModule("package a\np = true")
	mod2 := ast.MustParseModule("package b\np = false")

	bundles := map[string]*Bundle{"bundle1": {Modules: []ModuleFile{moduleFile("mod1", "package a\np = true")}}}

	cases := []testWriteModuleCase{
		{
			note:    "module files only",
			bundles: bundles,
		},
		{
			note:      "extra modules only",
			extraMods: map[string]*ast.Module{"mod1": mod1},
		},
		{
			note:         "compiler modules only",
			compilerMods: map[string]*ast.Module{"mod1": mod1},
		},
		{
			note:      "module files and extra modules",
			bundles:   bundles,
			extraMods: map[string]*ast.Module{"mod2": mod2},
		},
		{
			note:         "module files and compiler modules",
			bundles:      bundles,
			compilerMods: map[string]*ast.Module{"mod2": mod2},
		},
		{
			note:         "extra modules and compiler modules",
			extraMods:    map[string]*ast.Module{"mod1": mod1},
			compilerMods: map[string]*ast.Module{"mod2": mod2},
		},
		{
			note:      "compile error: path conflict",
			bundles:   bundles,
			storeData: unpack(map[string]any{"a.p": "foo"}),
			expectErr: true,
		},
	}

	for _, tc := range cases {
		testWriteData(t, tc, false)
		testWriteData(t, tc, true)
	}
}

func testWriteData(t *testing.T, tc testWriteModuleCase, legacy bool) {
	t.Helper()

	testName := tc.note
	if legacy {
		testName += "_legacy"
	}

	t.Run(testName, func(t *testing.T) {
		mockStore := mock.NewWithData(tc.storeData)
		txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

		compiler := ast.NewCompiler().WithPathConflictsCheck(storage.NonEmpty(t.Context(), mockStore, txn))

		// if supplied, pre-parse the module files

		for _, b := range tc.bundles {
			parsedMods := make([]ModuleFile, 0, len(b.Modules))
			for _, mf := range b.Modules {
				parsedMods = append(parsedMods, ModuleFile{
					Path:   mf.Path,
					Raw:    mf.Raw,
					Parsed: ast.MustParseModule(string(mf.Raw)),
				})
			}
			b.Modules = parsedMods
		}

		// if supplied, setup the compiler with modules already compiled on it
		if len(tc.compilerMods) > 0 {
			if compiler.Compile(tc.compilerMods); len(compiler.Errors) > 0 {
				t.Fatalf("unexpected error: %s", compiler.Errors)
			}
		}

		err := writeModules(t.Context(), mockStore, txn, compiler, metrics.NoOp(), tc.bundles, tc.extraMods, legacy)
		if !tc.expectErr && err != nil {
			t.Fatalf("unepected error: %s", err)
		} else if tc.expectErr && err == nil {
			t.Fatalf("expected error, got: %s", err)
		}

		if !tc.expectErr {
			// ensure all policy files were saved to storage
			expectedNumMods := 0
			for _, b := range tc.bundles {
				expectedNumMods += len(b.Modules)
			}

			policies := must(mockStore.ListPolicies(t.Context(), txn))(t)
			if len(policies) != expectedNumMods {
				t.Fatalf("expected %d policies in storage, found %d", expectedNumMods, len(policies))
			}

			for bundleName, b := range tc.bundles {
				for _, mf := range b.Modules {
					found := false
					for _, p := range policies {
						var expectedPath string
						if legacy {
							expectedPath = mf.Path
						} else {
							expectedPath = filepath.Join(bundleName, mf.Path)
						}
						if p == expectedPath {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("policy %s not found in storage", mf.Path)
					}
				}
			}

			// ensure all the modules were compiled together and we aren't missing any
			expectedModCount := expectedNumMods + len(tc.extraMods) + len(tc.compilerMods)
			if len(compiler.Modules) != expectedModCount {
				t.Fatalf("expected %d modules on compiler, found %d", expectedModCount, len(compiler.Modules))
			}

			for moduleName := range compiler.Modules {
				found := false
				if _, ok := tc.extraMods[moduleName]; ok {
					continue
				}
				if _, ok := tc.compilerMods[moduleName]; ok {
					continue
				}
				for bundleName, b := range tc.bundles {
					if legacy {
						for _, mf := range b.Modules {
							if moduleName == mf.Path {
								found = true
								break
							}
						}
					} else {
						for bundleModuleName := range b.ParsedModules(bundleName) {
							if moduleName == bundleModuleName {
								found = true
								break
							}
						}
					}
				}
				if found {
					continue
				}
				t.Errorf("unexpected module %s on compiler", moduleName)
			}
		}

		mustCommit(t, mockStore, txn)
		mockStore.AssertValid(t)
	})
}

func TestDoDFS(t *testing.T) {
	cases := []struct {
		note    string
		input   map[string]json.RawMessage
		path    string
		roots   []string
		wantErr bool
		err     error
	}{
		{
			note:  "bundle owns all",
			path:  "/",
			roots: []string{""},
		},
		{
			note:  "data within roots root case",
			input: map[string]json.RawMessage{"a": json.RawMessage(`true`)},
			path:  "",
			roots: []string{"a"},
		},
		{
			note:  "data within roots nested 1",
			input: map[string]json.RawMessage{"d": json.RawMessage(`true`)},
			path:  filepath.Dir("a/b/c/data.json"),
			roots: []string{"a/b/c"},
		},
		{
			note:  "data within roots nested 2",
			input: map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:  filepath.Dir("a/b/c/data.json"),
			roots: []string{"a/b/c"},
		},
		{
			note:  "data within roots nested 3",
			input: map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:  filepath.Dir("a/data.json"),
			roots: []string{"a/d"},
		},
		{
			note:  "data within multiple roots 1",
			input: map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": json.RawMessage(`true`)},
			path:  ".",
			roots: []string{"a/b", "c"},
		},
		{
			note:  "data within multiple roots 2",
			input: map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": []byte(`{"d": {"e": {"f": true}}}`)},
			path:  ".",
			roots: []string{"a/b", "c/d/e"},
		},
		{
			note:    "data outside roots 1",
			input:   map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:    ".",
			roots:   []string{"a/d"},
			wantErr: true,
			err:     errors.New("manifest roots [a/d] do not permit data at path '/d' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside roots 2",
			input:   map[string]json.RawMessage{"a": []byte(`{"b": {"c": {"e": true}}}`)},
			path:    filepath.Dir("x/data.json"),
			roots:   []string{"x/a/b/c/d"},
			wantErr: true,
			err:     errors.New("manifest roots [x/a/b/c/d] do not permit data at path '/x/a/b/c/e' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside roots 3",
			input:   map[string]json.RawMessage{"a": []byte(`{"b": {"c": true}}`)},
			path:    ".",
			roots:   []string{"a/b/c/d"},
			wantErr: true,
			err:     errors.New("manifest roots [a/b/c/d] do not permit data at path '/a/b/c' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside multiple roots",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "e": []byte(`{"b": {"c": true}}`)},
			path:    ".",
			roots:   []string{"a/b", "c"},
			wantErr: true,
			err:     errors.New("manifest roots [a/b c] do not permit data at path '/e' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside multiple roots 2",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": []byte(`{"d": true}`)},
			path:    ".",
			roots:   []string{"a/b", "c/d/e"},
			wantErr: true,
			err:     errors.New("manifest roots [a/b c/d/e] do not permit data at path '/c/d' (hint: check bundle directory structure)"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			if err := doDFS(tc.input, tc.path, tc.roots); tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}
		})
	}
}

func TestHasRootsOverlap(t *testing.T) {
	cases := []struct {
		note           string
		storeRoots     map[string]*[]string
		newBundleRoots map[string]*[]string
		expectedError  string
	}{
		{
			note:           "no overlap between store and new bundles",
			storeRoots:     map[string]*[]string{"bundle1": {"a", "b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {"c"}},
		},
		{
			note:           "no overlap between store and multiple new bundles",
			storeRoots:     map[string]*[]string{"bundle1": {"a", "b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {"c"}, "bundle3": {"d"}},
		},
		{
			note:           "no overlap with empty store",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {"a", "b"}},
		},
		{
			note:           "no overlap between multiple new bundles with empty store",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"c"}},
		},
		{
			note:           "overlap between multiple new bundles with empty store",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"a", "c"}},
			expectedError:  "detected overlapping roots in manifests for these bundles: [bundle1, bundle2] (root a is in multiple bundles)",
		},
		{
			note:           "overlap between store and new bundle",
			storeRoots:     map[string]*[]string{"bundle1": {"a", "b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {"c", "a"}},
			expectedError:  "detected overlapping roots in manifests for these bundles: [bundle1, bundle2] (root a is in multiple bundles)",
		},
		{
			note:           "overlap between store and multiple new bundles",
			storeRoots:     map[string]*[]string{"bundle1": {"a", "b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {"c", "a"}, "bundle3": {"a"}},
			expectedError:  "detected overlapping roots in manifests for these bundles: [bundle1, bundle2, bundle3] (root a is in multiple bundles)",
		},
		{
			note:           "overlap between store bundle and new empty root bundle",
			storeRoots:     map[string]*[]string{"bundle1": {"a", "b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {""}},
			expectedError:  "bundles [bundle1, bundle2] have overlapping roots and cannot be activated simultaneously because bundle(s) [bundle2] specify empty root paths ('') which overlap with any other bundle root",
		},
		{
			note:           "overlap between multiple new empty root bundles",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {""}, "bundle2": {""}},
			expectedError:  "bundles [bundle1, bundle2] have overlapping roots and cannot be activated simultaneously because bundle(s) [bundle1, bundle2] specify empty root paths ('') which overlap with any other bundle root",
		},
		{
			note:           "overlap between new empty root and new regular root bundles",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {"a"}, "bundle2": {""}},
			expectedError:  "bundles [bundle1, bundle2] have overlapping roots and cannot be activated simultaneously because bundle(s) [bundle2] specify empty root paths ('') which overlap with any other bundle root",
		},
		{
			note:           "overlap between nested paths",
			storeRoots:     map[string]*[]string{},
			newBundleRoots: map[string]*[]string{"bundle1": {"a"}, "bundle2": {"a/b"}},
			expectedError:  "detected overlapping roots in manifests for these bundles: [bundle1, bundle2] (a overlaps a/b)",
		},
		{
			note:           "overlap between store nested path and new bundle path",
			storeRoots:     map[string]*[]string{"bundle1": {"a/b"}},
			newBundleRoots: map[string]*[]string{"bundle2": {"a"}},
			expectedError:  "detected overlapping roots in manifests for these bundles: [bundle1, bundle2] (a overlaps a/b)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(t.Context(), mockStore, storage.WriteParams)

			for name, roots := range tc.storeRoots {
				if err := WriteManifestToStore(t.Context(), mockStore, txn, name, Manifest{Roots: roots}); err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			bundles := map[string]*Bundle{}
			for name, roots := range tc.newBundleRoots {
				bundles[name] = &Bundle{Manifest: Manifest{Roots: roots}}
			}

			if err := hasRootsOverlap(t.Context(), mockStore, txn, bundles); tc.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.expectedError)
				}
				if err.Error() != tc.expectedError {
					t.Fatalf("expected error message %q, got %q", tc.expectedError, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			mustCommit(t, mockStore, txn)
			mockStore.AssertValid(t)
		})
	}
}

func TestBundleStoreHelpers(t *testing.T) {
	storeReadModes := []struct {
		note    string
		readAst bool
	}{
		{note: "read raw", readAst: false},
		{note: "read ast", readAst: true},
	}

	bundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{Roots: &[]string{}},
		},
		"bundle2": {
			Manifest: Manifest{
				Roots:    &[]string{"a"},
				Revision: "foo",
				Metadata: map[string]any{"a": "b"},
				WasmResolvers: []WasmResolver{{
					Entrypoint: "foo/bar",
					Module:     "m.wasm",
				}},
			},
			Etag: "bar",
			WasmModules: []WasmModuleFile{{
				Path: "/m.wasm",
				Raw:  []byte("d2FzbS1tb2R1bGU="),
			}},
		},
	}

	for _, srm := range storeReadModes {
		t.Run(srm.note, func(t *testing.T) {
			mockStore := mock.NewWithData(nil, inmem.OptReturnASTValuesOnRead(srm.readAst))

			mustActivate(t, mockStore, &ActivateOpts{Bundles: bundles})

			txn := storage.NewTransactionOrDie(t.Context(), mockStore)

			verifyReadBundleNames(t, mockStore, txn, util.Keys(bundles)...)

			// Etag

			if etag, err := ReadBundleEtagFromStore(t.Context(), mockStore, txn, "bundle1"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if etag != "" {
				t.Errorf("expected empty etag but got %s", etag)
			}

			if etag, err := ReadBundleEtagFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := "bar"; etag != exp {
				t.Errorf("expected etag %s but got %s", exp, etag)
			}

			// Revision

			if rev, err := ReadBundleRevisionFromStore(t.Context(), mockStore, txn, "bundle1"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if rev != "" {
				t.Errorf("expected empty revision but got %s", rev)
			}

			if rev, err := ReadBundleRevisionFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := "foo"; rev != exp {
				t.Errorf("expected revision %s but got %s", exp, rev)
			}

			// Roots

			if roots, err := ReadBundleRootsFromStore(t.Context(), mockStore, txn, "bundle1"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if len(roots) != 0 {
				t.Errorf("expected empty roots but got %v", roots)
			}

			if roots, err := ReadBundleRootsFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := *bundles["bundle2"].Manifest.Roots; !reflect.DeepEqual(exp, roots) {
				t.Errorf("expected roots %v but got %v", exp, roots)
			}

			// Bundle metadata

			if meta, err := ReadBundleMetadataFromStore(t.Context(), mockStore, txn, "bundle1"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if len(meta) != 0 {
				t.Errorf("expected empty metadata but got %v", meta)
			}

			if meta, err := ReadBundleMetadataFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := bundles["bundle2"].Manifest.Metadata; !reflect.DeepEqual(exp, meta) {
				t.Errorf("expected metadata %v but got %v", exp, meta)
			}

			// Wasm metadata

			if _, err := ReadWasmMetadataFromStore(t.Context(), mockStore, txn, "bundle1"); err == nil {
				t.Fatalf("expected error but got nil")
			} else if exp, act := "storage_not_found_error: /bundles/bundle1/manifest/wasm: document does not exist", err.Error(); !strings.Contains(act, exp) {
				t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", exp, act)
			}

			if resolvers, err := ReadWasmMetadataFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := bundles["bundle2"].Manifest.WasmResolvers; !reflect.DeepEqual(exp, resolvers) {
				t.Errorf("expected wasm metadata:\n\n%v\n\nbut got:\n\n%v", exp, resolvers)
			}

			// Wasm modules

			if _, err := ReadWasmModulesFromStore(t.Context(), mockStore, txn, "bundle1"); err == nil {
				t.Fatalf("expected error but got nil")
			} else if exp, act := "storage_not_found_error: /bundles/bundle1/wasm: document does not exist", err.Error(); !strings.Contains(act, exp) {
				t.Fatalf("expected error:\n\n%s\n\nbut got:\n\n%v", exp, act)
			}

			if modules, err := ReadWasmModulesFromStore(t.Context(), mockStore, txn, "bundle2"); err != nil {
				t.Fatalf("unexpected error: %s", err)
			} else if exp := bundles["bundle2"].WasmModules; len(exp) != len(modules) {
				t.Errorf("expected wasm modules:\n\n%v\n\nbut got:\n\n%v", exp, modules)
			} else {
				for _, exp := range bundles["bundle2"].WasmModules {
					act := modules[exp.Path]
					if act == nil {
						t.Errorf("expected wasm module %s but got nil", exp.Path)
					}
					if !bytes.Equal(exp.Raw, act) {
						t.Errorf("expected wasm module %s to have raw data:\n\n%v\n\nbut got:\n\n%v", exp.Path, exp.Raw, act)
					}
				}
			}

		})
	}
}

func TestActivate_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		module            string
		customRegoVersion ast.RegoVersion
		expErrs           []string
	}{
		// NOT default rego-version
		{
			note: "v0 module",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "rego.v1 import, v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// default rego-version
		{
			note: "v1 module, no v1 parse-time violations",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "v1 module, v1 parse-time violations",
			module: `package test

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// custom rego-version
		{
			note: "v0 module, v0 custom rego-version",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			store := mock.New()
			txn := storage.NewTransactionOrDie(t.Context(), store, storage.WriteParams)

			modulePath := "test/policy.rego"

			// We want to make assert that the default rego-version is used, which it is when a module is erased from storage and we don't know what version it has.
			// Therefore, we add a module to the store, which is the replaced by the Activate() call, causing an erase.
			err := store.UpsertPolicy(t.Context(), txn, modulePathWithPrefix("bundle1", modulePath), []byte(tc.module))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			bundles := map[string]*Bundle{"bundle1": {
				Manifest: Manifest{Roots: &[]string{"test"}},
				Modules:  []ModuleFile{moduleFile(modulePath, "package test")},
			}}

			opts := ActivateOpts{
				Ctx:      t.Context(),
				Txn:      txn,
				Store:    store,
				Compiler: ast.NewCompiler().WithDefaultRegoVersion(ast.RegoV0CompatV1),
				Metrics:  metrics.NoOp(),
				Bundles:  bundles,
			}

			if tc.customRegoVersion != ast.RegoUndefined {
				opts.ParserOptions.RegoVersion = tc.customRegoVersion
			}

			if err = Activate(&opts); len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error but got nil for test: %s", tc.note)
				}
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDeactivate_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		module            string
		customRegoVersion ast.RegoVersion
		expErrs           []string
	}{
		// NOT default rego-version
		{
			note: "v0 module",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "rego.v1 import, v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// default rego-version
		{
			note: "v1 module, no v1 parse-time violations",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "v1 module, v1 parse-time violations",
			module: `package test

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// custom rego-version
		{
			note: "v0 module, v0 custom rego-version",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			store := mock.New()
			txn := storage.NewTransactionOrDie(t.Context(), store, storage.WriteParams)
			modulePath := "test/policy.rego"

			// We want to make assert that the default rego-version is used, which it is when a module is erased from storage and we don't know what version it has.
			// Therefore, we add a module to the store, which is the replaced by the Activate() call, causing an erase.
			err := store.UpsertPolicy(t.Context(), txn, modulePathWithPrefix("bundle1", modulePath), []byte(tc.module))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			opts := DeactivateOpts{
				Ctx:         t.Context(),
				Txn:         txn,
				Store:       store,
				BundleNames: map[string]struct{}{modulePathWithPrefix("bundle1", modulePath): {}},
			}

			if tc.customRegoVersion != ast.RegoUndefined {
				opts.ParserOptions.RegoVersion = tc.customRegoVersion
			}

			if err := Deactivate(&opts); len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error but got nil for test: %s", tc.note)
				}
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func mustActivate(tb testing.TB, store storage.Store, opts *ActivateOpts) {
	tb.Helper()

	txn := storage.NewTransactionOrDie(tb.Context(), store, storage.WriteParams)
	base := &ActivateOpts{
		Ctx:           tb.Context(),
		Store:         store,
		Txn:           txn,
		Metrics:       metrics.NoOp(),
		Bundles:       opts.Bundles,
		Compiler:      opts.Compiler,
		ParserOptions: opts.ParserOptions,
		ExtraModules:  opts.ExtraModules,
	}

	if opts.Compiler == nil {
		base.Compiler = ast.NewCompiler()
	}

	if err := Activate(base); err != nil {
		tb.Fatalf("unexpected error activating bundles: %s", err)
	}

	if err := store.Commit(tb.Context(), txn); err != nil {
		tb.Fatalf("unexpected error committing transaction: %s", err)
	}
}

func mustDeactivate(tb testing.TB, store storage.Store, opts *DeactivateOpts) {
	tb.Helper()

	txn := storage.NewTransactionOrDie(tb.Context(), store, storage.WriteParams)
	base := &DeactivateOpts{
		Ctx:           tb.Context(),
		Store:         store,
		Txn:           txn,
		BundleNames:   opts.BundleNames,
		ParserOptions: opts.ParserOptions,
	}

	if err := Deactivate(base); err != nil {
		tb.Fatalf("unexpected error deactivating bundles: %s", err)
	}

	if err := store.Commit(tb.Context(), txn); err != nil {
		tb.Fatalf("unexpected error committing transaction: %s", err)
	}
}

func verifyDeleteManifest(tb testing.TB, store storage.Store, name string) {
	tb.Helper()

	if err := storage.Txn(tb.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
		return EraseManifestFromStore(tb.Context(), store, txn, name)
	}); err != nil {
		tb.Fatalf("Unexpected error deleting manifest: %s", err)
	}
}

func verifyWriteManifests(tb testing.TB, store storage.Store, bundles map[string]Manifest) {
	tb.Helper()

	for name, manifest := range bundles {
		err := storage.Txn(tb.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
			err := WriteManifestToStore(tb.Context(), store, txn, name, manifest)
			if err != nil {
				tb.Fatalf("Failed to write manifest to store: %s", err)
			}
			return err
		})
		if err != nil {
			tb.Fatalf("Unexpected error finishing transaction: %s", err)
		}
	}
}

func verifyReadBundleNames(tb testing.TB, store storage.Store, txn storage.Transaction, expected ...string) {
	tb.Helper()

	if txn == nil {
		txn = storage.NewTransactionOrDie(tb.Context(), store)
		defer store.Abort(tb.Context(), txn)
	}

	names, err := ReadBundleNamesFromStore(tb.Context(), store, txn)
	if err != nil {
		if storage.IsNotFound(err) && len(expected) == 0 {
			return
		}
		tb.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(expected) {
		tb.Fatalf("expected %d bundles in store, found %d", len(expected), len(names))
	}

	expMap := map[string]struct{}{}
	for _, name := range expected {
		expMap[name] = struct{}{}
	}

	for _, name := range names {
		if _, ok := expMap[name]; !ok {
			tb.Fatalf("unexpected bundle %s to be found in store, found %v", name, names)
		}
	}
}

func verifyReadLegacyRevision(tb testing.TB, store storage.Store, expected string) {
	tb.Helper()

	if err := storage.Txn(tb.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
		actual, err := LegacyReadRevisionFromStore(tb.Context(), store, txn)
		if err != nil && !storage.IsNotFound(err) {
			tb.Fatalf("Failed to read manifest revision from store: %s", err)
		}

		if actual != expected {
			tb.Fatalf("Expected revision %s, got %s", expected, actual)
		}

		return nil
	}); err != nil {
		tb.Fatalf("Unexpected error finishing transaction: %s", err)
	}
}

func verifyBundleModulesCompiled(tb testing.TB, compiler *ast.Compiler, bundles map[string]*Bundle) {
	tb.Helper()

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				tb.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}
}

func verifyResultRead(tb testing.TB, store storage.Store, expected string) {
	tb.Helper()

	txn := storage.NewTransactionOrDie(tb.Context(), store)
	defer store.Abort(tb.Context(), txn)

	act, err := store.Read(tb.Context(), txn, storage.RootPath)
	if err != nil {
		tb.Fatalf("unexpected error: %s", err)
	}

	if exp := loadExpectedResult(tb, expected); !reflect.DeepEqual(exp, act) {
		tb.Errorf("expected %v, got %v", exp, act)
	}
}

func assertEqual(tb testing.TB, expectAst bool, expected string, actual any) {
	tb.Helper()

	if expectAst {
		if exp := ast.MustParseTerm(expected); ast.Compare(exp, actual) != 0 {
			tb.Errorf("expected:\n\n%v\n\ngot:\n\n%v", expected, actual)
		}
	} else if exp := loadExpectedResult(tb, expected); !reflect.DeepEqual(exp, actual) {
		tb.Errorf("expected:\n\n%v\n\ngot:\n\n%v", expected, actual)
	}
}

func loadExpectedResult(tb testing.TB, input string) (data any) {
	if len(input) > 0 {
		if err := util.UnmarshalJSON(util.StringToByteSlice(input), &data); err != nil {
			tb.Fatalf("failed to unmarshal expected result: %s", err)
		}
	}
	return data
}

func bundleFromFiles(tb testing.TB, name string, files [][2]string) Bundle {
	tb.Helper()

	return bundleFromBuffer(tb, name, archive.MustWriteTarGz(files))
}

func bundleFromRoundtrip(tb testing.TB, name string, b Bundle) Bundle {
	tb.Helper()

	var buf bytes.Buffer
	if err := NewWriter(&buf).UseModulePath(true).Write(b); err != nil {
		tb.Fatal("Unexpected error:", err)
	}

	return bundleFromBuffer(tb, name, &buf)
}

func bundleFromBuffer(tb testing.TB, name string, buf *bytes.Buffer) Bundle {
	tb.Helper()

	return must(NewCustomReader(NewTarballLoaderWithBaseURL(buf, "")).
		WithLazyLoadingMode(true).
		WithBundleName(name).
		Read())(tb)
}

func must[T any](val T, err error) func(tb testing.TB) T {
	return func(tb testing.TB) T {
		tb.Helper()

		if err != nil {
			tb.Fatalf("unexpected error: %s", err)
		}
		return val
	}
}

func mustRead(tb testing.TB, s storage.Store, txn storage.Transaction, path storage.Path) any {
	tb.Helper()

	if txn == nil {
		txn = storage.NewTransactionOrDie(tb.Context(), s)
		defer s.Abort(tb.Context(), txn)
	}

	return must(s.Read(tb.Context(), txn, path))(tb)
}

func mustCommit(tb testing.TB, s storage.Store, txn storage.Transaction) {
	tb.Helper()

	if err := s.Commit(tb.Context(), txn); err != nil {
		tb.Fatalf("unexpected error trying to commit to store: %s", err)
	}
}

func moduleFile(path, raw string) ModuleFile {
	return ModuleFile{
		Path:   path,
		Raw:    util.StringToByteSlice(raw),
		Parsed: ast.MustParseModule(raw),
	}
}

// unpack takes flat map[string]any where nested keys are dot-separated,
// and returns a "unpacked" map[string]any with proper nesting.
func unpack(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))

	for k, v := range m {
		var ok bool
		currMap := result
		curr, rest, _ := strings.Cut(k, ".")

		for rest != "" {
			if _, ok = currMap[curr]; !ok {
				currMap[curr] = make(map[string]any)
			}
			if currMap, ok = currMap[curr].(map[string]any); !ok {
				panic("key conflict")
			}
			curr, rest, _ = strings.Cut(rest, ".")
		}

		currMap[curr] = v
	}

	return result
}
