//go:build !opa_no_oci

// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/plugins"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func TestOCIStorePathWiring(t *testing.T) {
	tests := []struct {
		note    string
		persist bool
		// stop exercises discovery shutdown, so the downloader cleanup is
		// covered too.
		stop bool
	}{
		{note: "persistence directory configured", persist: true},
		{note: "no persistence directory", persist: false},
		{note: "stop removes temporary store", stop: true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := t.Context()

			// Redirect the process temp directory so a private store can be
			// told apart from the shared one, and so it doesn't outlive the test.
			tmpDir := t.TempDir()
			t.Setenv("TMPDIR", tmpDir)
			t.Setenv("TMP", tmpDir)
			t.Setenv("TEMP", tmpDir)

			persistDir := filepath.Join(t.TempDir(), "persist")

			config := `{
				"services": {"acmecorp": {"url": "http://127.0.0.1:1", "type": "oci"}},
				"discovery": {"resource": "ghcr.io/org/repo:latest", "trigger": "manual"}`
			if tc.persist {
				config += ", " + strconv.Quote("persistence_directory") + ": " + strconv.Quote(persistDir)
			}
			config += "}"

			manager, err := plugins.New([]byte(config), "test-id", inmem.New())
			if err != nil {
				t.Fatal(err)
			}

			disco, err := New(manager)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { disco.Stop(ctx) })

			if disco.downloader == nil {
				t.Fatal("expected a discovery downloader to be configured")
			}

			// The shared path OPA used before OCI stores became per-downloader.
			sharedPath := filepath.Join(t.TempDir(), "opa", "oci")
			if _, err := os.Stat(sharedPath); !os.IsNotExist(err) {
				t.Fatalf("expected no store at the shared path %q but got: %v", sharedPath, err)
			}

			privateStores, err := filepath.Glob(filepath.Join(tmpDir, "opa-oci-*"))
			if err != nil {
				t.Fatal(err)
			}

			ociStorePath := filepath.Join(persistDir, "oci")

			if tc.persist {
				if _, err := os.Stat(filepath.Join(ociStorePath, "oci-layout")); err != nil {
					t.Fatalf("expected a store below the persistence directory: %v", err)
				}
				if len(privateStores) != 0 {
					t.Fatalf("expected no temporary store but got %v", privateStores)
				}
				return
			}

			if _, err := os.Stat(ociStorePath); !os.IsNotExist(err) {
				t.Fatalf("expected no store at %q but got: %v", ociStorePath, err)
			}
			if len(privateStores) != 1 {
				t.Fatalf("expected exactly one temporary store but got %v", privateStores)
			}
			if _, err := os.Stat(filepath.Join(privateStores[0], "oci-layout")); err != nil {
				t.Fatalf("expected an initialized store at %q: %v", privateStores[0], err)
			}

			if !tc.stop {
				return
			}

			disco.Stop(ctx)

			if _, err := os.Stat(privateStores[0]); !os.IsNotExist(err) {
				t.Fatalf("expected %q to have been removed but got: %v", privateStores[0], err)
			}
		})
	}
}
