package bundle

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/util"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"

	"github.com/open-policy-agent/opa/internal/storage/mock"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
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

func TestBundleLifecycle(t *testing.T) {
	ctx := context.Background()
	mockStore := mock.New()

	compiler := ast.NewCompiler()
	m := metrics.New()

	extraMods := map[string]*ast.Module{
		"mod1": ast.MustParseModule("package x\np = true"),
	}

	mod2 := "package a\np = true"
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
		},
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
				}
			},
			"bundle2": {
				"manifest": {
					"revision": "",
					"roots": ["b", "c"]
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
					t.Fatalf("expected %d modules remaining, got %d", len(remaining), len(tc.expectedRemaining))
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
	writeToStore bool
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
			expectErr:    false,
			writeToStore: true,
		},
		{
			note: "extra modules only",
			extraMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			expectErr:    false,
			writeToStore: true,
		},
		{
			note: "compiler modules only",
			compilerMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			expectErr:    false,
			writeToStore: true,
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
			expectErr:    false,
			writeToStore: true,
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
			expectErr:    false,
			writeToStore: true,
		},
		{
			note: "extra modules and compiler modules",
			extraMods: map[string]*ast.Module{
				"mod1": ast.MustParseModule("package a\np = true"),
			},
			compilerMods: map[string]*ast.Module{
				"mod2": ast.MustParseModule("package b\np = false"),
			},
			expectErr:    false,
			writeToStore: true,
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
			expectErr:    true,
			writeToStore: false,
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
