// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergedBundlePrefix(t *testing.T) {
	// Resolve symlinks so expected names match what the OS reports for "..".
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(root, "p", "q", "cwd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	tests := []struct {
		note string
		path string
		exp  string
	}{
		{note: "dot slash and trailing slash", path: "./a/", exp: "a"},
		{note: "file url", path: "file:///tmp/a", exp: "/tmp/a"},
		{note: "two parents", path: "../../a/b", exp: "p/a/b"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			got, err := MergedBundlePrefix(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Fatalf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

func TestMergedBundlePrefixFilesystemRoot(t *testing.T) {
	t.Chdir(string(filepath.Separator))
	if _, err := MergedBundlePrefix("../a"); err == nil {
		t.Fatal("expected error for path resolving to the filesystem root")
	}
}
