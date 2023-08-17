// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

var defaultModuleLoader ModuleLoader

// DefaultModuleLoader lets you inject an `ast.ModuleLoader` that will
// always be used. If another one is provided with the ast package,
// they will both be consulted to enrich the set of modules dynamically.
func DefaultModuleLoader(ml ModuleLoader) {
	defaultModuleLoader = ml
}
