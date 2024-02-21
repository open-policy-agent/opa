// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"

	"github.com/open-policy-agent/opa/ast"
)

func main() {
	f := ast.CapabilitiesForThisVersion()

	fd, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")

	for i, bi := range f.Builtins {
		// NOTE(sr): This ensures that there are no type names and descriptions in capabilities.json
		f.Builtins[i] = bi.Minimal()
	}

	if err := enc.Encode(f); err != nil {
		panic(err)
	}

	if err := fd.Close(); err != nil {
		panic(err)
	}
}
