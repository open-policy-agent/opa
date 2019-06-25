package bundle

import (
	"context"
	"testing"

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
