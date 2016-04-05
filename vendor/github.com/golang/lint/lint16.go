// Copyright (c) 2016 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

// +build go1.6

package lint

import "go/types"

// This is in its own file so it can be ignored under Go 1.5.

func (i importer) ImportFrom(path, srcDir string, mode types.ImportMode) (*types.Package, error) {
	return i.impFn(i.packages, path, srcDir)
}
