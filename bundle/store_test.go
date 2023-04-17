package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/storage/mock"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/disk"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

func TestManifestStoreLifecycleSingleBundle(t *testing.T) {
	store := inmem.New()
	ctx := context.Background()
	tb := Manifest{
		Revision: "abc123",
		Roots:    &[]string{"/a/b", "/a/c"},
	}
	name := "test_bundle"
	verifyWriteManifests(ctx, t, store, map[string]Manifest{name: tb}) // write one
	verifyReadBundleNames(ctx, t, store, []string{name})               // read one
	verifyDeleteManifest(ctx, t, store, name)                          // delete it
	verifyReadBundleNames(ctx, t, store, []string{})                   // ensure it was removed
}

func TestManifestStoreLifecycleMultiBundle(t *testing.T) {
	store := inmem.New()
	ctx := context.Background()

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

	verifyWriteManifests(ctx, t, store, bundles)                         // write multiple
	verifyReadBundleNames(ctx, t, store, []string{"bundle1", "bundle2"}) // read them
	verifyDeleteManifest(ctx, t, store, "bundle1")                       // delete one
	verifyReadBundleNames(ctx, t, store, []string{"bundle2"})            // ensure it was removed
	verifyDeleteManifest(ctx, t, store, "bundle2")                       // delete the last one
	verifyReadBundleNames(ctx, t, store, []string{})                     // ensure it was removed
}

func TestLegacyManifestStoreLifecycle(t *testing.T) {
	store := inmem.New()
	ctx := context.Background()
	tb := Manifest{
		Revision: "abc123",
		Roots:    &[]string{"/a/b", "/a/c"},
	}

	// write a "legacy" manifest
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := LegacyWriteManifestToStore(ctx, store, txn, tb); err != nil {
			t.Fatalf("Failed to write manifest to store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}

	// make sure it can be retrieved
	verifyReadLegacyRevision(ctx, t, store, tb.Revision)

	// delete it
	err = storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := LegacyEraseManifestFromStore(ctx, store, txn); err != nil {
			t.Fatalf("Failed to erase manifest from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}

	verifyReadLegacyRevision(ctx, t, store, "")
}

func TestMixedManifestStoreLifecycle(t *testing.T) {
	store := inmem.New()
	ctx := context.Background()
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
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if err := LegacyWriteManifestToStore(ctx, store, txn, bundles["bundle1"]); err != nil {
			t.Fatalf("Failed to write manifest to store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}

	verifyReadBundleNames(ctx, t, store, []string{})

	// Write both new ones
	verifyWriteManifests(ctx, t, store, bundles)
	verifyReadBundleNames(ctx, t, store, []string{"bundle1", "bundle2"})

	// Ensure the original legacy one is still there
	verifyReadLegacyRevision(ctx, t, store, bundles["bundle1"].Revision)
}

func verifyDeleteManifest(ctx context.Context, t *testing.T, store storage.Store, name string) {
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		err := EraseManifestFromStore(ctx, store, txn, name)
		if err != nil {
			t.Fatalf("Failed to delete manifest from store: %s", err)
		}
		return err
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}
}

func verifyWriteManifests(ctx context.Context, t *testing.T, store storage.Store, bundles map[string]Manifest) {
	t.Helper()
	for name, manifest := range bundles {
		err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
			err := WriteManifestToStore(ctx, store, txn, name, manifest)
			if err != nil {
				t.Fatalf("Failed to write manifest to store: %s", err)
			}
			return err
		})
		if err != nil {
			t.Fatalf("Unexpected error finishing transaction: %s", err)
		}
	}
}

func verifyReadBundleNames(ctx context.Context, t *testing.T, store storage.Store, expected []string) {
	t.Helper()
	var actualNames []string
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		actualNames, err = ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest names from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}

	if len(actualNames) != len(expected) {
		t.Fatalf("Expected %d name, found %d \n\t\tActual: %v\n", len(expected), len(actualNames), actualNames)
	}

	for _, actualName := range actualNames {
		found := false
		for _, expectedName := range expected {
			if actualName == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Found unexpecxted bundle name %s, expected names: %+v", actualName, expected)
		}
	}
}

func verifyReadLegacyRevision(ctx context.Context, t *testing.T, store storage.Store, expected string) {
	t.Helper()
	var actual string
	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		var err error
		if actual, err = LegacyReadRevisionFromStore(ctx, store, txn); err != nil && !storage.IsNotFound(err) {
			t.Fatalf("Failed to read manifest revision from store: %s", err)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unexpected error finishing transaction: %s", err)
	}

	if actual != expected {
		t.Fatalf("Expected revision %s, got %s", expected, actual)
	}
}

func TestBundleLazyModeNoPolicyOrData(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	bundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Roots:    &[]string{"a"},
				Revision: "foo",
			},
			Etag:            "foo",
			lazyLoadingMode: true,
		},
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})

	if err != nil {
		t.Fatal(err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}

	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
{
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
}
`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}
}

func TestBundleLazyModeLifecycleRaw(t *testing.T) {
	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/authz/allow/policy.wasm", `wasm-module`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
		{"/.manifest", `{"revision": "foo", "roots": ["a", "example", "x", "authz"],"wasm":[{"entrypoint": "authz/allow", "module": "/authz/allow/policy.wasm"}]}`},
	}

	buf := archive.MustWriteTarGz(files)
	loader := NewTarballLoaderWithBaseURL(buf, "")
	br := NewCustomReader(loader).WithBundleEtag("bar").WithLazyLoadingMode(true)

	bundle, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	extraMods := map[string]*ast.Module{
		"mod1": ast.MustParseModule("package x\np = true"),
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:          ctx,
		Store:        mockStore,
		Txn:          txn,
		Compiler:     compiler,
		Metrics:      m,
		Bundles:      bundles,
		ExtraModules: extraMods,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw := `
{
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
					]
				},
				"etag": "bar",
				"wasm": {
					"/authz/allow/policy.wasm": "d2FzbS1tb2R1bGU="
				}
			}
		}
	}
}
`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Ensure that the extra module was included
	if _, ok := compiler.Modules["mod1"]; !ok {
		t.Fatalf("expected extra module to be compiled")
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Deactivate(&DeactivateOpts{
		Ctx:         ctx,
		Store:       mockStore,
		Txn:         txn,
		BundleNames: map[string]struct{}{"bundle1": {}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Expect the store to have been cleared out after deactivating the bundle
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err = ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 0 {
		t.Fatalf("expected 0 bundles in store, found %d", len(names))
	}

	actual, err = mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw = `{"system": {"bundles": {}}}`
	expected = loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	mockStore.AssertValid(t)
}

func TestBundleLazyModeLifecycleRawInvalidData(t *testing.T) {

	tests := map[string]struct {
		files [][2]string
		err   error
	}{
		"non-object root": {[][2]string{{"/data.json", `[1,2,3]`}}, fmt.Errorf("root value must be object")},
		"invalid yaml":    {[][2]string{{"/a/b/data.yaml", `"foo`}}, fmt.Errorf("yaml: found unexpected end of stream")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			buf := archive.MustWriteTarGz(tc.files)
			loader := NewTarballLoaderWithBaseURL(buf, "")
			br := NewCustomReader(loader).WithBundleEtag("bar").WithLazyLoadingMode(true)

			bundle, err := br.Read()
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			mockStore := mock.New()

			compiler := ast.NewCompiler()
			m := metrics.New()

			bundles := map[string]*Bundle{
				"bundle1": &bundle,
			}

			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			err = Activate(&ActivateOpts{
				Ctx:      ctx,
				Store:    mockStore,
				Txn:      txn,
				Compiler: compiler,
				Metrics:  m,
				Bundles:  bundles,
			})

			if tc.err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
			}
		})
	}
}

func TestBundleLazyModeLifecycle(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	extraMods := map[string]*ast.Module{
		"mod1": ast.MustParseModule("package x\np = true"),
	}

	mod1 := "package a\np = true"
	mod2 := "package b\np = true"

	b1Files := [][2]string{
		{"/.manifest", `{"roots": ["a"]}`},
		{"a/policy.rego", mod1},
		{"/data.json", `{"a": {"b": "foo"}}`},
	}

	buf := archive.MustWriteTarGz(b1Files)
	loader := NewTarballLoaderWithBaseURL(buf, "")
	br := NewCustomReader(loader).WithBundleEtag("foo").WithLazyLoadingMode(true).WithBundleName("bundle1")

	bundle1, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	b2Files := [][2]string{
		{"/.manifest", `{"roots": ["b", "c"]}`},
		{"b/policy.rego", mod2},
		{"/data.json", `{}`},
	}

	buf = archive.MustWriteTarGz(b2Files)
	loader = NewTarballLoaderWithBaseURL(buf, "")
	br = NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2")

	bundle2, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
		"bundle2": &bundle2,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:          ctx,
		Store:        mockStore,
		Txn:          txn,
		Compiler:     compiler,
		Metrics:      m,
		Bundles:      bundles,
		ExtraModules: extraMods,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw := `
{
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
		}
	}
}
`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Ensure that the extra module was included
	if _, ok := compiler.Modules["mod1"]; !ok {
		t.Fatalf("expected extra module to be compiled")
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Deactivate(&DeactivateOpts{
		Ctx:         ctx,
		Store:       mockStore,
		Txn:         txn,
		BundleNames: map[string]struct{}{"bundle1": {}, "bundle2": {}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Expect the store to have been cleared out after deactivating the bundles
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err = ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 0 {
		t.Fatalf("expected 0 bundles in store, found %d", len(names))
	}

	actual, err = mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw = `{"system": {"bundles": {}}}`
	expected = loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	mockStore.AssertValid(t)
}

func TestBundleLazyModeLifecycleRawNoBundleRoots(t *testing.T) {
	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
		{"/.manifest", `{"revision": "rev-1"}`},
	}

	buf := archive.MustWriteTarGz(files)
	loader := NewTarballLoaderWithBaseURL(buf, "")
	br := NewCustomReader(loader).WithBundleEtag("foo").WithLazyLoadingMode(true)

	bundle, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	bundles := map[string]*Bundle{
		"bundle1": &bundle,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
{
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
}
`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	files = [][2]string{
		{"/c/data.json", `{"hello": "world"}`},
		{"/.manifest", `{"revision": "rev-2"}`},
	}

	buf = archive.MustWriteTarGz(files)
	loader = NewTarballLoaderWithBaseURL(buf, "")
	br = NewCustomReader(loader).WithBundleEtag("bar").WithLazyLoadingMode(true)

	bundle, err = br.Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles = map[string]*Bundle{
		"bundle1": &bundle,
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err = mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw = `
      {
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
      }`

	expected = loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

}

func TestBundleLazyModeLifecycleRawNoBundleRootsDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		files := [][2]string{
			{"/a/b/c/data.json", "[1,2,3]"},
			{"/a/b/d/data.json", "true"},
			{"/a/b/y/data.yaml", `foo: 1`},
			{"/example/example.rego", `package example`},
			{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}}`},
			{"/.manifest", `{"revision": "rev-1"}`},
		}

		buf := archive.MustWriteTarGz(files)
		loader := NewTarballLoaderWithBaseURL(buf, "")
		br := NewCustomReader(loader).WithBundleEtag("foo").WithLazyLoadingMode(true)

		bundle, err := br.Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)
		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
{
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
}
`
		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		files = [][2]string{
			{"/c/data.json", `{"hello": "world"}`},
			{"/.manifest", `{"revision": "rev-2"}`},
		}

		buf = archive.MustWriteTarGz(files)
		loader = NewTarballLoaderWithBaseURL(buf, "")
		br = NewCustomReader(loader).WithBundleEtag("bar").WithLazyLoadingMode(true)

		bundle, err = br.Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles = map[string]*Bundle{
			"bundle1": &bundle,
		}

		txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err = store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw = `
      {
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
      }`

		expected = loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)
	})
}

func TestBundleLazyModeLifecycleNoBundleRoots(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()
	compiler := ast.NewCompiler()
	m := metrics.New()

	mod1 := "package a\np = true"

	b := Bundle{
		Manifest: Manifest{Revision: "rev-1"},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": "foo",
				"e": map[string]interface{}{
					"f": "bar",
				},
				"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
			},
		},
		Modules: []ModuleFile{
			{
				Path:   "a/policy.rego",
				Raw:    []byte(mod1),
				Parsed: ast.MustParseModule(mod1),
			},
		},
		Etag: "foo",
	}

	var buf1 bytes.Buffer
	if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader := NewTarballLoaderWithBaseURL(&buf1, "")
	bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
      {
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
      }`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// add a new bundle with no roots. this means all the data from the currently activated should be removed
	b = Bundle{
		Manifest: Manifest{Revision: "rev-2"},
		Data: map[string]interface{}{
			"c": map[string]interface{}{
				"hello": "world",
			},
		},
		Etag: "bar",
	}

	var buf2 bytes.Buffer
	if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	loader = NewTarballLoaderWithBaseURL(&buf2, "")
	bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles = map[string]*Bundle{
		"bundle1": &bundle2,
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err = mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw = `
      {
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
      }`

	expected = loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)
}

func TestBundleLazyModeLifecycleNoBundleRootsDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1"},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)

		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
      {
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
      }`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// add a new bundle with no roots. this means all the data from the currently activated should be removed
		b = Bundle{
			Manifest: Manifest{Revision: "rev-2"},
			Data: map[string]interface{}{
				"c": map[string]interface{}{
					"hello": "world",
				},
			},
			Etag: "bar",
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}

		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles = map[string]*Bundle{
			"bundle1": &bundle2,
		}

		txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err = store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw = `
      {
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
      }`

		expected = loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

	})
}

func TestBundleLazyModeLifecycleMixBundleTypeActivationDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{
				Revision: "snap-1",
				Roots:    &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		// create a delta bundle and activate it

		// add a new object member
		p1 := PatchOperation{
			Op:    "upsert",
			Path:  "/x/y",
			Value: []string{"foo", "bar"},
		}

		b = Bundle{
			Manifest: Manifest{
				Revision: "delta-1",
				Roots:    &[]string{"x"},
			},
			Patch: Patch{Data: []PatchOperation{p1}},
			Etag:  "bar",
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}

		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
			"bundle2": &bundle2,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the patches were applied
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
      {
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
      }`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)
	})
}

func TestBundleLazyModeLifecycleOldBundleEraseDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1", Roots: &[]string{"a"}},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)

		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
      {
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
      }`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// add a new bundle and verify data from the currently activated is removed
		b = Bundle{
			Manifest: Manifest{Revision: "rev-2", Roots: &[]string{"c"}},
			Data: map[string]interface{}{
				"c": map[string]interface{}{
					"hello": "world",
				},
			},
			Etag: "bar",
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}

		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles = map[string]*Bundle{
			"bundle1": &bundle2,
		}

		txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err = store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw = `
      {
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
      }`

		expected = loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

	})
}

func TestBundleLazyModeLifecycleRestoreBackupDB(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		mod1 := "package a\np = true"

		b := Bundle{
			Manifest: Manifest{Revision: "rev-1", Roots: &[]string{"a"}},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundle was activated
		txn = storage.NewTransactionOrDie(ctx, store)

		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
      {
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
      }`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// add a new bundle but abort the transaction and verify only old the bundle data is kept in store
		b = Bundle{
			Manifest: Manifest{Revision: "rev-2", Roots: &[]string{"c"}},
			Data: map[string]interface{}{
				"c": map[string]interface{}{
					"hello": "world",
				},
			},
			Etag: "bar",
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}

		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles = map[string]*Bundle{
			"bundle1": &bundle2,
		}

		txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		store.Abort(ctx, txn)

		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err = store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw = `
      {
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
      }`

		expected = loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// check symlink is created
		symlink := filepath.Join(dir, "active")
		_, err = os.Lstat(symlink)
		if err != nil {
			t.Fatal(err)
		}

		// check symlink target
		_, err = filepath.EvalSymlinks(symlink)
		if err != nil {
			t.Fatalf("eval symlinks: %v", err)
		}
	})
}

func TestDeltaBundleLazyModeLifecycleDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		mod1 := "package a\np = true"
		mod2 := "package b\np = true"

		b := Bundle{
			Manifest: Manifest{
				Roots: &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		b = Bundle{
			Manifest: Manifest{
				Roots: &[]string{"b", "c"},
			},
			Data: nil,
			Modules: []ModuleFile{
				{
					Path:   "b/policy.rego",
					Raw:    []byte(mod2),
					Parsed: ast.MustParseModule(mod2),
				},
			},
			Etag: "foo",
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}

		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
			"bundle2": &bundle2,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundles were activated
		txn = storage.NewTransactionOrDie(ctx, store)
		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// create a delta bundle and activate it

		// add a new object member
		p1 := PatchOperation{
			Op:    "upsert",
			Path:  "/a/c/d",
			Value: []string{"foo", "bar"},
		}

		// append value to array
		p2 := PatchOperation{
			Op:    "upsert",
			Path:  "/a/c/d/-",
			Value: "baz",
		}

		// replace a value
		p3 := PatchOperation{
			Op:    "replace",
			Path:  "a/b",
			Value: "bar",
		}

		// add a new object root
		p4 := PatchOperation{
			Op:    "upsert",
			Path:  "/c/d",
			Value: []string{"foo", "bar"},
		}

		deltaBundles := map[string]*Bundle{
			"bundle1": {
				Manifest: Manifest{
					Revision: "delta-1",
					Roots:    &[]string{"a"},
				},
				Patch: Patch{Data: []PatchOperation{p1, p2, p3}},
				Etag:  "bar",
			},
			"bundle2": {
				Manifest: Manifest{
					Revision: "delta-2",
					Roots:    &[]string{"b", "c"},
				},
				Patch: Patch{Data: []PatchOperation{p4}},
				Etag:  "baz",
			},
			"bundle3": {
				Manifest: Manifest{
					Roots: &[]string{"d"},
				},
				Data: map[string]interface{}{
					"d": map[string]interface{}{
						"e": "foo",
					},
				},
			},
		}

		txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  deltaBundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// check the modules from the snapshot bundles are on the compiler
		for bundleName, bundle := range bundles {
			for modName := range bundle.ParsedModules(bundleName) {
				if _, ok := compiler.Modules[modName]; !ok {
					t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
				}
			}
		}

		// Ensure the patches were applied
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
		{
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
		}`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)
	})
}

func TestBundleLazyModeLifecycleOverlappingBundleRoots(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	b := Bundle{
		Manifest: Manifest{
			Revision: "foo",
			Roots:    &[]string{"a/b", "a/c", "a/d"},
		},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": "foo",
				"c": map[string]interface{}{
					"d": "bar",
				},
				"d": []map[string]string{{"name": "john"}, {"name": "jane"}},
			},
		},
	}

	var buf1 bytes.Buffer
	if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader := NewTarballLoaderWithBaseURL(&buf1, "")
	bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	b = Bundle{
		Manifest: Manifest{
			Revision: "bar",
			Roots:    &[]string{"a/e"},
		},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"e": map[string]interface{}{
					"f": "bar",
				},
			},
		},
	}

	var buf2 bytes.Buffer
	if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader = NewTarballLoaderWithBaseURL(&buf2, "")
	bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
		"bundle2": &bundle2,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundles were activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
		{
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
		}`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)
}

func TestBundleLazyModeLifecycleOverlappingBundleRootsDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		b := Bundle{
			Manifest: Manifest{
				Revision: "foo",
				Roots:    &[]string{"a/b/c", "a/b/d", "a/b/e"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "bar",
						"d": []map[string]string{{"name": "john"}, {"name": "jane"}},
						"e": []string{"foo", "bar"},
					},
				},
			},
		}

		var buf1 bytes.Buffer
		if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader := NewTarballLoaderWithBaseURL(&buf1, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		b = Bundle{
			Manifest: Manifest{
				Revision: "bar",
				Roots:    &[]string{"a/b/f"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"f": map[string]interface{}{
							"hello": "world",
						},
					},
				},
			},
		}

		var buf2 bytes.Buffer
		if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
			t.Fatal("Unexpected error:", err)
		}
		loader = NewTarballLoaderWithBaseURL(&buf2, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
			"bundle2": &bundle2,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundles were activated
		txn = storage.NewTransactionOrDie(ctx, store)
		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// Ensure the patches were applied
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
		{
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
		}`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)
	})
}

func TestBundleLazyModeLifecycleRawOverlappingBundleRoots(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	files := [][2]string{
		{"/a/b/x/data.json", "[1,2,3]"},
		{"/a/c/y/data.json", "true"},
		{"/a/d/z/data.yaml", `foo: 1`},
		{"/data.json", `{"a": {"b": {"z": true}}}`},
		{"/.manifest", `{"revision": "foo", "roots": ["a/b", "a/c", "a/d"]}`},
	}

	buf := archive.MustWriteTarGz(files)
	loader := NewTarballLoaderWithBaseURL(buf, "")
	bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	files = [][2]string{
		{"/a/e/x/data.json", "[4,5,6]"},
		{"/data.json", `{"a": {"e": {"f": true}}}`},
		{"/.manifest", `{"revision": "bar", "roots": ["a/e"]}`},
	}

	buf = archive.MustWriteTarGz(files)
	loader = NewTarballLoaderWithBaseURL(buf, "")
	bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
		"bundle2": &bundle2,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundles were activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
		{
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
		}`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)
}

func TestBundleLazyModeLifecycleRawOverlappingBundleRootsDiskStorage(t *testing.T) {
	ctx := context.Background()

	test.WithTempFS(nil, func(dir string) {
		store, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{
			Dir: dir,
		})
		if err != nil {
			t.Fatal(err)
		}

		compiler := ast.NewCompiler()
		m := metrics.New()

		files := [][2]string{
			{"/a/b/u/data.json", "[1,2,3]"},
			{"/a/b/v/data.json", "true"},
			{"/a/b/w/data.yaml", `foo: 1`},
			{"/data.json", `{"a": {"b": {"x": true}}}`},
			{"/.manifest", `{"revision": "foo", "roots": ["a/b"]}`},
		}

		buf := archive.MustWriteTarGz(files)
		loader := NewTarballLoaderWithBaseURL(buf, "")
		bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
		if err != nil {
			t.Fatal(err)
		}

		files = [][2]string{
			{"/a/c/x/data.json", "[4,5,6]"},
			{"/data.json", `{"a": {"c": {"y": true}}}`},
			{"/.manifest", `{"revision": "bar", "roots": ["a/c"]}`},
		}

		buf = archive.MustWriteTarGz(files)
		loader = NewTarballLoaderWithBaseURL(buf, "")
		bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
		if err != nil {
			t.Fatal(err)
		}

		bundles := map[string]*Bundle{
			"bundle1": &bundle1,
			"bundle2": &bundle2,
		}

		txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

		err = Activate(&ActivateOpts{
			Ctx:      ctx,
			Store:    store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  bundles,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		err = store.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Ensure the snapshot bundles were activated
		txn = storage.NewTransactionOrDie(ctx, store)
		names, err := ReadBundleNamesFromStore(ctx, store, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(names) != len(bundles) {
			t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
		}
		for _, name := range names {
			if _, ok := bundles[name]; !ok {
				t.Fatalf("unexpected bundle name found in store: %s", name)
			}
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)

		// Ensure the patches were applied
		txn = storage.NewTransactionOrDie(ctx, store)

		actual, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		expectedRaw := `
		{
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
		}`

		expected := loadExpectedSortedResult(expectedRaw)
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
		}

		// Stop the "read" transaction
		store.Abort(ctx, txn)
	})
}

func TestDeltaBundleLazyModeLifecycle(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	mod1 := "package a\np = true"
	mod2 := "package b\np = true"

	b := Bundle{
		Manifest: Manifest{
			Roots: &[]string{"a"},
		},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": "foo",
				"e": map[string]interface{}{
					"f": "bar",
				},
				"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
			},
		},
		Modules: []ModuleFile{
			{
				Path:   "policy.rego",
				Raw:    []byte(mod1),
				Parsed: ast.MustParseModule(mod1),
			},
		},
		Etag: "foo",
	}

	var buf1 bytes.Buffer
	if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader := NewTarballLoaderWithBaseURL(&buf1, "")
	bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	b = Bundle{
		Manifest: Manifest{
			Roots: &[]string{"b", "c"},
		},
		Data: nil,
		Modules: []ModuleFile{
			{
				Path:   "policy.rego",
				Raw:    []byte(mod2),
				Parsed: ast.MustParseModule(mod2),
			},
		},
		Etag:            "foo",
		lazyLoadingMode: true,
		sizeLimitBytes:  DefaultSizeLimitBytes + 1,
	}

	var buf2 bytes.Buffer
	if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader = NewTarballLoaderWithBaseURL(&buf2, "")
	bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
		"bundle2": &bundle2,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundles were activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// create a delta bundle and activate it

	// add a new object member
	p1 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	// append value to array
	p2 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d/-",
		Value: "baz",
	}

	// insert value in array
	p3 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/x/1",
		Value: map[string]string{"name": "alice"},
	}

	// replace a value
	p4 := PatchOperation{
		Op:    "replace",
		Path:  "a/b",
		Value: "bar",
	}

	// remove a value
	p5 := PatchOperation{
		Op:   "remove",
		Path: "a/e",
	}

	// add a new object with an escaped character in the path
	p6 := PatchOperation{
		Op:    "upsert",
		Path:  "a/y/~0z",
		Value: []int{1, 2, 3},
	}

	// add a new object root
	p7 := PatchOperation{
		Op:    "upsert",
		Path:  "/c/d",
		Value: []string{"foo", "bar"},
	}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Revision: "delta-1",
				Roots:    &[]string{"a"},
			},
			Patch: Patch{Data: []PatchOperation{p1, p2, p3, p4, p5, p6}},
			Etag:  "bar",
		},
		"bundle2": {
			Manifest: Manifest{
				Revision: "delta-2",
				Roots:    &[]string{"b", "c"},
			},
			Patch: Patch{Data: []PatchOperation{p7}},
			Etag:  "baz",
		},
		"bundle3": {
			Manifest: Manifest{
				Roots: &[]string{"d"},
			},
			Data: map[string]interface{}{
				"d": map[string]interface{}{
					"e": "foo",
				},
			},
		},
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  deltaBundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// check the modules from the snapshot bundles are on the compiler
	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
	{
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
	}`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	mockStore.AssertValid(t)
}

func TestDeltaBundleLazyModeWithDefaultRules(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	mod1 := "package a\ndefault p = true"
	mod2 := "package b\ndefault p = true"

	b := Bundle{
		Manifest: Manifest{
			Roots: &[]string{"a"},
		},
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": "foo",
				"e": map[string]interface{}{
					"f": "bar",
				},
				"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
			},
		},
		Modules: []ModuleFile{
			{
				Path:   "policy.rego",
				Raw:    []byte(mod1),
				Parsed: ast.MustParseModule(mod1),
			},
		},
		Etag: "foo",
	}

	var buf1 bytes.Buffer
	if err := NewWriter(&buf1).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader := NewTarballLoaderWithBaseURL(&buf1, "")
	bundle1, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle1").Read()
	if err != nil {
		t.Fatal(err)
	}

	b = Bundle{
		Manifest: Manifest{
			Roots: &[]string{"b", "c"},
		},
		Data: nil,
		Modules: []ModuleFile{
			{
				Path:   "policy.rego",
				Raw:    []byte(mod2),
				Parsed: ast.MustParseModule(mod2),
			},
		},
		Etag:            "foo",
		lazyLoadingMode: true,
		sizeLimitBytes:  DefaultSizeLimitBytes + 1,
	}

	var buf2 bytes.Buffer
	if err := NewWriter(&buf2).UseModulePath(true).Write(b); err != nil {
		t.Fatal("Unexpected error:", err)
	}
	loader = NewTarballLoaderWithBaseURL(&buf2, "")
	bundle2, err := NewCustomReader(loader).WithLazyLoadingMode(true).WithBundleName("bundle2").Read()
	if err != nil {
		t.Fatal(err)
	}

	bundles := map[string]*Bundle{
		"bundle1": &bundle1,
		"bundle2": &bundle2,
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundles were activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// create a delta bundle and activate it

	// add a new object member
	p1 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	// append value to array
	p2 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d/-",
		Value: "baz",
	}

	// insert value in array
	p3 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/x/1",
		Value: map[string]string{"name": "alice"},
	}

	// replace a value
	p4 := PatchOperation{
		Op:    "replace",
		Path:  "a/b",
		Value: "bar",
	}

	// remove a value
	p5 := PatchOperation{
		Op:   "remove",
		Path: "a/e",
	}

	// add a new object with an escaped character in the path
	p6 := PatchOperation{
		Op:    "upsert",
		Path:  "a/y/~0z",
		Value: []int{1, 2, 3},
	}

	// add a new object root
	p7 := PatchOperation{
		Op:    "upsert",
		Path:  "/c/d",
		Value: []string{"foo", "bar"},
	}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Revision: "delta-1",
				Roots:    &[]string{"a"},
			},
			Patch: Patch{Data: []PatchOperation{p1, p2, p3, p4, p5, p6}},
			Etag:  "bar",
		},
		"bundle2": {
			Manifest: Manifest{
				Revision: "delta-2",
				Roots:    &[]string{"b", "c"},
			},
			Patch: Patch{Data: []PatchOperation{p7}},
			Etag:  "baz",
		},
		"bundle3": {
			Manifest: Manifest{
				Roots: &[]string{"d"},
			},
			Data: map[string]interface{}{
				"d": map[string]interface{}{
					"e": "foo",
				},
			},
		},
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	expectedModuleCount := len(compiler.Modules)
	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  deltaBundles,
	})
	if expectedModuleCount != len(compiler.Modules) {
		t.Fatalf("Expected %d modules, got %d", expectedModuleCount, len(compiler.Modules))
	}
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// check the modules from the snapshot bundles are on the compiler
	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
	{
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
	}`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	mockStore.AssertValid(t)
}

func TestBundleLifecycle(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	extraMods := map[string]*ast.Module{
		"mod1": ast.MustParseModule("package x\np = true"),
	}

	const mod2 = "package a\np = true"
	mod3 := "package b\np = true"

	bundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Roots: &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod2),
					Parsed: ast.MustParseModule(mod2),
				},
			},
			Etag: "foo"},
		"bundle2": {
			Manifest: Manifest{
				Roots: &[]string{"b", "c"},
			},
			Data: nil,
			Modules: []ModuleFile{
				{
					Path:   "b/policy.rego",
					Raw:    []byte(mod3),
					Parsed: ast.MustParseModule(mod3),
				},
			},
		},
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:          ctx,
		Store:        mockStore,
		Txn:          txn,
		Compiler:     compiler,
		Metrics:      m,
		Bundles:      bundles,
		ExtraModules: extraMods,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw := `
{
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
		}
	}
}
`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Ensure that the extra module was included
	if _, ok := compiler.Modules["mod1"]; !ok {
		t.Fatalf("expected extra module to be compiled")
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Deactivate(&DeactivateOpts{
		Ctx:         ctx,
		Store:       mockStore,
		Txn:         txn,
		BundleNames: map[string]struct{}{"bundle1": {}, "bundle2": {}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Expect the store to have been cleared out after deactivating the bundles
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err = ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 0 {
		t.Fatalf("expected 0 bundles in store, found %d", len(names))
	}

	actual, err = mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedRaw = `{"system": {"bundles": {}}}`
	expected = loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	mockStore.AssertValid(t)
}

func TestDeltaBundleLifecycle(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	mod1 := "package a\np = true"
	mod2 := "package b\np = true"

	bundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Roots: &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"e": map[string]interface{}{
						"f": "bar",
					},
					"x": []map[string]string{{"name": "john"}, {"name": "jane"}},
				},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
			Etag: "foo",
		},
		"bundle2": {
			Manifest: Manifest{
				Roots: &[]string{"b", "c"},
			},
			Data: nil,
			Modules: []ModuleFile{
				{
					Path:   "b/policy.rego",
					Raw:    []byte(mod2),
					Parsed: ast.MustParseModule(mod2),
				},
			},
		},
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundles were activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// create a delta bundle and activate it

	// add a new object member
	p1 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	// append value to array
	p2 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d/-",
		Value: "baz",
	}

	// insert value in array
	p3 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/x/1",
		Value: map[string]string{"name": "alice"},
	}

	// replace a value
	p4 := PatchOperation{
		Op:    "replace",
		Path:  "a/b",
		Value: "bar",
	}

	// remove a value
	p5 := PatchOperation{
		Op:   "remove",
		Path: "a/e",
	}

	// add a new object with an escaped character in the path
	p6 := PatchOperation{
		Op:    "upsert",
		Path:  "a/y/~0z",
		Value: []int{1, 2, 3},
	}

	// add a new object root
	p7 := PatchOperation{
		Op:    "upsert",
		Path:  "/c/d",
		Value: []string{"foo", "bar"},
	}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Revision: "delta-1",
				Roots:    &[]string{"a"},
			},
			Patch: Patch{Data: []PatchOperation{p1, p2, p3, p4, p5, p6}},
			Etag:  "bar",
		},
		"bundle2": {
			Manifest: Manifest{
				Revision: "delta-2",
				Roots:    &[]string{"b", "c"},
			},
			Patch: Patch{Data: []PatchOperation{p7}},
			Etag:  "baz",
		},
		"bundle3": {
			Manifest: Manifest{
				Roots: &[]string{"d"},
			},
			Data: map[string]interface{}{
				"d": map[string]interface{}{
					"e": "foo",
				},
			},
		},
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  deltaBundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// check the modules from the snapshot bundles are on the compiler
	for bundleName, bundle := range bundles {
		for modName := range bundle.ParsedModules(bundleName) {
			if _, ok := compiler.Modules[modName]; !ok {
				t.Fatalf("expected module %s from bundle %s to have been compiled", modName, bundleName)
			}
		}
	}

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
	{
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
	}`

	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	mockStore.AssertValid(t)
}

func TestDeltaBundleActivate(t *testing.T) {

	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	// create a delta bundle
	p1 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Revision: "delta",
				Roots:    &[]string{"a"},
			},
			Patch: Patch{Data: []PatchOperation{p1}},
			Etag:  "foo",
		},
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  deltaBundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the delta bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(deltaBundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(deltaBundles), len(names))
	}

	for _, name := range names {
		if _, ok := deltaBundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// Ensure the patches were applied
	txn = storage.NewTransactionOrDie(ctx, mockStore)

	actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedRaw := `
	{
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
	}
	`
	expected := loadExpectedSortedResult(expectedRaw)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, got %v", expectedRaw, string(util.MustMarshalJSON(actual)))
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	mockStore.AssertValid(t)
}

func TestDeltaBundleBadManifest(t *testing.T) {

	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	mod1 := "package a\np = true"

	bundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Roots: &[]string{"a"},
			},
			Modules: []ModuleFile{
				{
					Path:   "a/policy.rego",
					Raw:    []byte(mod1),
					Parsed: ast.MustParseModule(mod1),
				},
			},
		},
	}

	txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err := Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
		Bundles:  bundles,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = mockStore.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Ensure the snapshot bundle was activated
	txn = storage.NewTransactionOrDie(ctx, mockStore)
	names, err := ReadBundleNamesFromStore(ctx, mockStore, txn)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(names) != len(bundles) {
		t.Fatalf("expected %d bundles in store, found %d", len(bundles), len(names))
	}
	for _, name := range names {
		if _, ok := bundles[name]; !ok {
			t.Fatalf("unexpected bundle name found in store: %s", name)
		}
	}

	// Stop the "read" transaction
	mockStore.Abort(ctx, txn)

	// create a delta bundle with a different manifest from the snapshot bundle

	p1 := PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	deltaBundles := map[string]*Bundle{
		"bundle1": {
			Manifest: Manifest{
				Roots: &[]string{"b"},
			},
			Patch: Patch{Data: []PatchOperation{p1}},
		},
	}

	txn = storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

	err = Activate(&ActivateOpts{
		Ctx:      ctx,
		Store:    mockStore,
		Txn:      txn,
		Compiler: compiler,
		Metrics:  m,
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
	ctx := context.Background()
	cases := []struct {
		note        string
		initialData map[string]interface{}
		roots       []string
		expectErr   bool
		expected    string
	}{
		{
			note: "erase all",
			initialData: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"b": "bar",
			},
			roots:     []string{"a", "b"},
			expectErr: false,
			expected:  `{}`,
		},
		{
			note: "erase none",
			initialData: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"b": "bar",
			},
			roots:     []string{},
			expectErr: false,
			expected:  `{"a": {"b": "foo"}, "b": "bar"}`,
		},
		{
			note: "erase partial",
			initialData: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"b": "bar",
			},
			roots:     []string{"a"},
			expectErr: false,
			expected:  `{"b": "bar"}`,
		},
		{
			note: "erase partial path",
			initialData: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
					"c": map[string]interface{}{
						"d": 123,
					},
				},
			},
			roots:     []string{"a/c/d"},
			expectErr: false,
			expected:  `{"a": {"b": "foo", "c":{}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.NewWithData(tc.initialData)
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			roots := map[string]struct{}{}
			for _, root := range tc.roots {
				roots[root] = struct{}{}
			}

			err := eraseData(ctx, mockStore, txn, roots)
			if !tc.expectErr && err != nil {
				t.Fatalf("unepected error: %s", err)
			} else if tc.expectErr && err == nil {
				t.Fatalf("expected error, got: %s", err)
			}

			err = mockStore.Commit(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			mockStore.AssertValid(t)

			txn = storage.NewTransactionOrDie(ctx, mockStore)
			actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			expected := loadExpectedSortedResult(tc.expected)
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestErasePolicies(t *testing.T) {
	ctx := context.Background()
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
			expectErr:         false,
			expectedRemaining: []string{},
		},
		{
			note: "erase none",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
				"mod2": []byte("package b\np = true"),
			},
			roots:             []string{"c"},
			expectErr:         false,
			expectedRemaining: []string{"mod1", "mod2"},
		},
		{
			note: "erase correct paths",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a.test\np = true"),
				"mod2": []byte("package a.test_v2\np = true"),
			},
			roots:             []string{"a/test"},
			expectErr:         false,
			expectedRemaining: []string{"mod2"},
		},
		{
			note: "erase some",
			initialPolicies: map[string][]byte{
				"mod1": []byte("package a\np = true"),
				"mod2": []byte("package b\np = true"),
			},
			roots:             []string{"b"},
			expectErr:         false,
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
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			for name, mod := range tc.initialPolicies {
				err := mockStore.UpsertPolicy(ctx, txn, name, mod)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			roots := map[string]struct{}{}
			for _, root := range tc.roots {
				roots[root] = struct{}{}
			}
			remaining, err := erasePolicies(ctx, mockStore, txn, roots)
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

				err = mockStore.Commit(ctx, txn)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				mockStore.AssertValid(t)

				txn = storage.NewTransactionOrDie(ctx, mockStore)
				actualRemaining, err := mockStore.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}

				if len(actualRemaining) != len(tc.expectedRemaining) {
					t.Fatalf("expected %d modules remaining in the store, got %d", len(tc.expectedRemaining), len(actualRemaining))
				}
				for _, expectedName := range tc.expectedRemaining {
					found := false
					for _, actualName := range actualRemaining {
						if expectedName == actualName {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("expected remaining module %s not found", expectedName)
					}
				}
			}
		})
	}
}

func TestWriteData(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		note         string
		existingData map[string]interface{}
		roots        []string
		data         map[string]interface{}
		expected     string
		expectErr    bool
	}{
		{
			note:  "single root",
			roots: []string{"a"},
			data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 123,
					},
				},
			},
			expected:  `{"a": {"b": {"c": 123}}}`,
			expectErr: false,
		},
		{
			note:  "multiple roots",
			roots: []string{"a", "b/c/d"},
			data: map[string]interface{}{
				"a": "foo",
				"b": map[string]interface{}{
					"c": map[string]interface{}{
						"d": "bar",
					},
				},
			},
			expected:  `{"a": "foo","b": {"c": {"d": "bar"}}}`,
			expectErr: false,
		},
		{
			note:  "data not in roots",
			roots: []string{"a"},
			data: map[string]interface{}{
				"a": "foo",
				"b": map[string]interface{}{
					"c": map[string]interface{}{
						"d": "bar",
					},
				},
			},
			expected:  `{"a": "foo"}`,
			expectErr: false,
		},
		{
			note:         "no data",
			roots:        []string{"a"},
			existingData: map[string]interface{}{},
			data:         map[string]interface{}{},
			expected:     `{}`,
			expectErr:    false,
		},
		{
			note:  "no new data",
			roots: []string{"a"},
			existingData: map[string]interface{}{
				"a": "foo",
			},
			data:      map[string]interface{}{},
			expected:  `{"a": "foo"}`,
			expectErr: false,
		},
		{
			note:  "overwrite data",
			roots: []string{"a"},
			existingData: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			data: map[string]interface{}{
				"a": "bar",
			},
			expected:  `{"a": "bar"}`,
			expectErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.NewWithData(tc.existingData)
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			err := writeData(ctx, mockStore, txn, tc.roots, tc.data)
			if !tc.expectErr && err != nil {
				t.Fatalf("unepected error: %s", err)
			} else if tc.expectErr && err == nil {
				t.Fatalf("expected error, got: %s", err)
			}

			err = mockStore.Commit(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			mockStore.AssertValid(t)

			txn = storage.NewTransactionOrDie(ctx, mockStore)
			actual, err := mockStore.Read(ctx, txn, storage.MustParsePath("/"))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			expected := loadExpectedSortedResult(tc.expected)
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func loadExpectedResult(input string) interface{} {
	if len(input) == 0 {
		return nil
	}
	var data interface{}
	if err := util.UnmarshalJSON([]byte(input), &data); err != nil {
		panic(err)
	}
	return data
}

func loadExpectedSortedResult(input string) interface{} {
	data := loadExpectedResult(input)
	switch data := data.(type) {
	case []interface{}:
		return data
	default:
		return data
	}
}

type testWriteModuleCase struct {
	note         string
	bundles      map[string]*Bundle // Only need to give raw text and path for modules
	extraMods    map[string]*ast.Module
	compilerMods map[string]*ast.Module
	storeData    map[string]interface{}
	expectErr    bool
}

func TestWriteModules(t *testing.T) {

	cases := []testWriteModuleCase{
		{
			note: "module files only",
			bundles: map[string]*Bundle{
				"bundle1": {
					Modules: []ModuleFile{
						{
							Path: "mod1",
							Raw:  []byte("package a\np = true"),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			note: "extra modules only",
			extraMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			expectErr: false,
		},
		{
			note: "compiler modules only",
			compilerMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			expectErr: false,
		},
		{
			note: "module files and extra modules",
			bundles: map[string]*Bundle{
				"bundle1": {
					Modules: []ModuleFile{
						{
							Path: "mod1",
							Raw:  []byte("package a\np = true"),
						},
					},
				},
			},
			extraMods: map[string]*ast.Module{
				"mod2": ast.MustParseModule("package b\np = false"),
			},
			expectErr: false,
		},
		{
			note: "module files and compiler modules",
			bundles: map[string]*Bundle{
				"bundle1": {
					Modules: []ModuleFile{
						{
							Path: "mod1",
							Raw:  []byte("package a\np = true"),
						},
					},
				},
			},
			compilerMods: map[string]*ast.Module{
				"mod2": ast.MustParseModule("package b\np = false"),
			},
			expectErr: false,
		},
		{
			note: "extra modules and compiler modules",
			extraMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			compilerMods: map[string]*ast.Module{
				"mod2": ast.MustParseModule("package b\np = false"),
			},
			expectErr: false,
		},
		{
			note: "compile error: path conflict",
			bundles: map[string]*Bundle{
				"bundle1": {
					Modules: []ModuleFile{
						{
							Path: "mod1",
							Raw:  []byte("package a\np = true"),
						},
					},
				},
			},
			storeData: map[string]interface{}{
				"a": map[string]interface{}{
					"p": "foo",
				},
			},
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

		ctx := context.Background()
		mockStore := mock.NewWithData(tc.storeData)
		txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

		compiler := ast.NewCompiler().WithPathConflictsCheck(storage.NonEmpty(ctx, mockStore, txn))
		m := metrics.New()

		// if supplied, pre-parse the module files

		for _, b := range tc.bundles {
			var parsedMods []ModuleFile
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
			compiler.Compile(tc.compilerMods)
			if len(compiler.Errors) > 0 {
				t.Fatalf("unexpected error: %s", compiler.Errors)
			}
		}

		err := writeModules(ctx, mockStore, txn, compiler, m, tc.bundles, tc.extraMods, legacy)
		if !tc.expectErr && err != nil {
			t.Fatalf("unepected error: %s", err)
		} else if tc.expectErr && err == nil {
			t.Fatalf("expected error, got: %s", err)
		}

		if !tc.expectErr {
			// ensure all policy files were saved to storage
			policies, err := mockStore.ListPolicies(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expectedNumMods := 0
			for _, b := range tc.bundles {
				expectedNumMods += len(b.Modules)
			}

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

		err = mockStore.Commit(ctx, txn)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

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
			note:    "bundle owns all",
			input:   nil,
			path:    "/",
			roots:   []string{""},
			wantErr: false,
		},
		{
			note:    "data within roots root case",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`true`)},
			path:    "",
			roots:   []string{"a"},
			wantErr: false,
		},
		{
			note:    "data within roots nested 1",
			input:   map[string]json.RawMessage{"d": json.RawMessage(`true`)},
			path:    filepath.Dir(strings.Trim("a/b/c/data.json", "/")),
			roots:   []string{"a/b/c"},
			wantErr: false,
		},
		{
			note:    "data within roots nested 2",
			input:   map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:    filepath.Dir(strings.Trim("a/b/c/data.json", "/")),
			roots:   []string{"a/b/c"},
			wantErr: false,
		},
		{
			note:    "data within roots nested 3",
			input:   map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:    filepath.Dir(strings.Trim("a/data.json", "/")),
			roots:   []string{"a/d"},
			wantErr: false,
		},
		{
			note:    "data within multiple roots 1",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": json.RawMessage(`true`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/b", "c"},
			wantErr: false,
		},
		{
			note:    "data within multiple roots 2",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": []byte(`{"d": {"e": {"f": true}}}`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/b", "c/d/e"},
			wantErr: false,
		},
		{
			note:    "data outside roots 1",
			input:   map[string]json.RawMessage{"d": json.RawMessage(`{"hello": "world"}`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/d"},
			wantErr: true,
			err:     fmt.Errorf("manifest roots [a/d] do not permit data at path '/d' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside roots 2",
			input:   map[string]json.RawMessage{"a": []byte(`{"b": {"c": {"e": true}}}`)},
			path:    filepath.Dir(strings.Trim("/x/data.json", "/")),
			roots:   []string{"x/a/b/c/d"},
			wantErr: true,
			err:     fmt.Errorf("manifest roots [x/a/b/c/d] do not permit data at path '/x/a/b/c/e' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside roots 3",
			input:   map[string]json.RawMessage{"a": []byte(`{"b": {"c": true}}`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/b/c/d"},
			wantErr: true,
			err:     fmt.Errorf("manifest roots [a/b/c/d] do not permit data at path '/a/b/c' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside multiple roots",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "e": []byte(`{"b": {"c": true}}`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/b", "c"},
			wantErr: true,
			err:     fmt.Errorf("manifest roots [a/b c] do not permit data at path '/e' (hint: check bundle directory structure)"),
		},
		{
			note:    "data outside multiple roots 2",
			input:   map[string]json.RawMessage{"a": json.RawMessage(`{"b": "c"}`), "c": []byte(`{"d": true}`)},
			path:    filepath.Dir(strings.Trim("/data.json", "/")),
			roots:   []string{"a/b", "c/d/e"},
			wantErr: true,
			err:     fmt.Errorf("manifest roots [a/b c/d/e] do not permit data at path '/c/d' (hint: check bundle directory structure)"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			err := doDFS(tc.input, tc.path, tc.roots)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}

func TestHasRootsOverlap(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		note        string
		storeRoots  map[string]*[]string
		bundleRoots map[string]*[]string
		overlaps    bool
	}{
		{
			note:        "no overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "no overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}, "bundle3": {"d"}},
			overlaps:    false,
		},
		{
			note:        "no overlap no existing roots",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}},
			overlaps:    false,
		},
		{
			note:        "no overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"a", "c"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}, "bundle3": {"a"}},
			overlaps:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			for name, roots := range tc.storeRoots {
				err := WriteManifestToStore(ctx, mockStore, txn, name, Manifest{Roots: roots})
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			bundles := map[string]*Bundle{}
			for name, roots := range tc.bundleRoots {
				bundles[name] = &Bundle{
					Manifest: Manifest{
						Roots: roots,
					},
				}
			}

			err := hasRootsOverlap(ctx, mockStore, txn, bundles)
			if !tc.overlaps && err != nil {
				t.Fatalf("unepected error: %s", err)
			} else if tc.overlaps && (err == nil || !strings.Contains(err.Error(), "detected overlapping roots in bundle manifest")) {
				t.Fatalf("expected overlapping roots error, got: %s", err)
			}

			err = mockStore.Commit(ctx, txn)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			mockStore.AssertValid(t)
		})
	}
}
