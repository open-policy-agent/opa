//go:build !opa_no_oci

// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/plugins"
)

func TestOCIStorePathWiring(t *testing.T) {
	tests := []struct {
		note    string
		persist bool
		// start runs the full plugin lifecycle rather than just building a
		// downloader, so plugin shutdown is covered too.
		start bool
	}{
		{note: "persistence directory configured", persist: true},
		{note: "no persistence directory", persist: false},
		{note: "plugin lifecycle removes temporary store", start: true},
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

			config := `{"services": {"acmecorp": {"url": "http://127.0.0.1:1", "type": "oci"}}`
			if tc.persist {
				config += ", " + strconv.Quote("persistence_directory") + ": " + strconv.Quote(persistDir)
			}
			config += "}"

			manager := getTestManagerWithOpts([]byte(config))

			trigger := plugins.TriggerManual
			downloadConf := download.Config{Trigger: &trigger}
			if err := downloadConf.ValidateAndInjectDefaults(); err != nil {
				t.Fatal(err)
			}

			source := &Source{
				Config:   downloadConf,
				Service:  "acmecorp",
				Resource: "ghcr.io/org/repo:latest",
			}

			plugin := New(&Config{Bundles: map[string]*Source{"test": source}}, manager)

			if tc.start {
				if err := plugin.Start(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				dl := plugin.newDownloader("test", source, map[string]*Source{"test": source})
				t.Cleanup(func() { dl.Stop(ctx) })
			}

			// The shared path OPA used before OCI stores became per-downloader.
			sharedPath := filepath.Join(tmpDir, "opa", "oci")
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

			if !tc.start {
				return
			}

			plugin.Stop(ctx)

			if _, err := os.Stat(privateStores[0]); !os.IsNotExist(err) {
				t.Fatalf("expected %q to have been removed but got: %v", privateStores[0], err)
			}
		})
	}
}
