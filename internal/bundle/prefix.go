// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	fileurl "github.com/open-policy-agent/opa/internal/file/url"
)

// MergedBundlePrefix returns the slash-separated directory that holds the
// files of the bundle loaded from p, when several bundles are merged into
// one. Leading ".." segments are replaced by the name of the directory they
// resolve to, so that entries stay inside the merged bundle.
func MergedBundlePrefix(p string) (string, error) {
	cleaned, err := fileurl.Clean(p)
	if err != nil {
		return "", err
	}
	cleaned = path.Clean(filepath.ToSlash(cleaned))

	n, rest := 0, cleaned
	for rest == ".." || strings.HasPrefix(rest, "../") {
		n++
		rest = strings.TrimPrefix(strings.TrimPrefix(rest, ".."), "/")
	}
	if n == 0 {
		return cleaned, nil
	}

	dir, err := filepath.Abs(strings.Repeat(".."+string(filepath.Separator), n))
	if err != nil {
		return "", err
	}
	if filepath.Dir(dir) == dir {
		return "", fmt.Errorf("bundle %s: path resolves to the filesystem root, which has no name to use in the merged bundle", p)
	}
	return path.Join(filepath.Base(dir), rest), nil
}
