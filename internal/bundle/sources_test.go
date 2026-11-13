// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestSourcesAboveManifest(t *testing.T) {
	nested := map[string]string{
		"dir/top.rego":      "package top",
		"dir/data.json":     "{}",
		"dir/sub/.manifest": `{"roots":["x"]}`,
		"dir/sub/x.rego":    "package x",
	}

	tests := []struct {
		note       string
		files      map[string]string
		filter     func(string, os.FileInfo, int) bool
		expDir     string
		expOutside []string
	}{
		{
			note:  "filter excludes a source",
			files: nested,
			filter: func(path string, _ os.FileInfo, _ int) bool {
				return strings.HasSuffix(path, "dir/top.rego")
			},
			expDir:     "dir/sub",
			expOutside: []string{"dir/data.json"},
		},
		{
			note: "manifest at root",
			files: map[string]string{
				".manifest":  `{}`,
				"top.rego":   "package top",
				"sub/x.rego": "package x",
			},
		},
		{
			note: "manifests in multiple directories",
			files: map[string]string{
				"top.rego":    "package top",
				"a/.manifest": `{}`,
				"b/.manifest": `{}`,
				"b/x.rego":    "package x",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				dir, outside, err := sourcesAboveManifest(root, tc.filter)
				if err != nil {
					t.Fatal(err)
				}

				expDir := ""
				if tc.expDir != "" {
					expDir = filepath.Join(root, filepath.FromSlash(tc.expDir))
				}
				if dir != expDir {
					t.Fatalf("expected manifest dir %q, got %q", expDir, dir)
				}

				var exp []string
				for _, p := range tc.expOutside {
					exp = append(exp, filepath.Join(root, filepath.FromSlash(p)))
				}
				slices.Sort(outside)
				if !slices.Equal(exp, outside) {
					t.Fatalf("expected %v, got %v", exp, outside)
				}
			})
		})
	}
}

func TestDepthUsesOSSeparators(t *testing.T) {
	if d := depth(filepath.Join("a", "b", "c.rego"), false); d != 2 {
		t.Fatalf("expected depth 2 for file, got %d", d)
	}
	if d := depth(filepath.Join("a", "b"), true); d != 2 {
		t.Fatalf("expected depth 2 for dir, got %d", d)
	}
}
