// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	fileurl "github.com/open-policy-agent/opa/internal/file/url"
	"github.com/open-policy-agent/opa/v1/bundle"
)

var sourceExts = []string{bundle.RegoExt, ".json", ".yaml", ".yml", ".wasm", ".pb"}

// sourcesAboveManifest returns the directory holding the manifest under root,
// and the source files under root that are outside of it. Those files are not
// part of the bundle. Both are empty if the manifest is missing or at root, or
// if manifests are in more than one directory (loading fails then anyway).
// The filter is applied as when loading the bundle.
func sourcesAboveManifest(root string, filter func(string, os.FileInfo, int) bool) (string, []string, error) {
	var manifestDir string
	var sources []string
	var multiple bool
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filter != nil && filter(filepath.ToSlash(path), info, depth(path, info.IsDir())) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		switch name := info.Name(); {
		case name == bundle.ManifestExt || name == bundle.ManifestProtoExt:
			if manifestDir != "" && manifestDir != filepath.Dir(path) {
				multiple = true
			}
			manifestDir = filepath.Dir(path)
		case slices.Contains(sourceExts, filepath.Ext(name)):
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil || multiple || manifestDir == "" || manifestDir == filepath.Clean(root) {
		return "", nil, err
	}

	var outside []string
	for _, s := range sources {
		if !strings.HasPrefix(s, manifestDir+string(filepath.Separator)) {
			outside = append(outside, s)
		}
	}
	return manifestDir, outside, nil
}

// WarnSourcesAboveManifest writes a warning for each bundle directory in paths
// that has source files outside the directory holding its manifest.
func WarnSourcesAboveManifest(w io.Writer, paths []string, filter func(string, os.FileInfo, int) bool) {
	for _, p := range paths {
		root, err := fileurl.Clean(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			continue
		}
		manifestDir, outside, err := sourcesAboveManifest(root, filter)
		if err != nil || len(outside) == 0 {
			continue
		}
		fmt.Fprintf(w, "Warning: bundle %q is rooted at %q, which contains its manifest. These files are not included in the bundle:\n", p, manifestDir)
		for _, f := range outside {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}
}

// depth matches how the bundle directory loader computes filter depth.
func depth(path string, isDir bool) int {
	if !isDir {
		path = filepath.Dir(path)
	}
	return strings.Count(strings.Trim(filepath.ToSlash(path), "/"), "/") + 1
}
