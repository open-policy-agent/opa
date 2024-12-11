// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/loader"
)

type loaderFilter struct {
	Ignore   []string
	OnlyRego bool
}

func (f loaderFilter) Apply(abspath string, info os.FileInfo, depth int) bool {
	// if set to only load rego files, skip all non-rego files
	if f.OnlyRego && !info.IsDir() && filepath.Ext(info.Name()) != bundle.RegoExt {
		return true
	}
	for _, s := range f.Ignore {
		if loader.GlobExcludeName(s, 1)(abspath, info, depth) {
			return true
		}
	}
	return false
}
