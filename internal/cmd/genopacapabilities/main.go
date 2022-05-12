// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/compiler/wasm"
	"github.com/open-policy-agent/opa/types"
)

func main() {

	f := ast.CapabilitiesForThisVersion()

	fd, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")

	if err := enc.Encode(f); err != nil {
		panic(err)
	}

	if err := fd.Close(); err != nil {
		panic(err)
	}

	sorted := append(sortedCaps(), versionedCaps{version: "edge", caps: f})

	mdata := make(map[string]interface{})
	for _, bi := range f.Builtins {
		latest := getLatest(bi.Name, sorted)
		argTypes := make([]map[string]interface{}, len(latest.Decl.FuncArgs().Args))

		for i, typ := range latest.Decl.NamedFuncArgs().Args {
			if n, ok := typ.(*types.NamedType); ok {
				argTypes[i] = map[string]interface{}{
					"name":        n.Name,
					"description": n.Descr,
					"type":        n.Type.String(),
				}
			} else {
				argTypes[i] = map[string]interface{}{
					"type": typ.String(),
				}
			}
		}
		res := map[string]interface{}{}
		resType := latest.Decl.NamedResult()
		if n, ok := resType.(*types.NamedType); ok {
			res["name"] = n.Name
			if n.Descr != "" {
				res["description"] = n.Descr
			}
			res["type"] = n.Type.String()
		} else if resType != nil {
			res["type"] = resType.String()
		}
		md := map[string]interface{}{
			"introduced": getFirstVersion(bi.Name, sorted),
			"wasm":       getWasm(bi.Name),
			"args":       argTypes,
			"result":     res,
		}
		if latest.Infix != "" {
			md["infix"] = latest.Infix
		}
		if latest.Description != "" {
			md["description"] = latest.Description
		}
		mdata[bi.Name] = md
	}

	md, err := os.Create(os.Args[2]) // metadata
	if err != nil {
		panic(err)
	}

	enc = json.NewEncoder(md)
	enc.SetIndent("", "  ")

	if err := enc.Encode(mdata); err != nil {
		panic(err)
	}

	if err := md.Close(); err != nil {
		panic(err)
	}
}

func getFirstVersion(bi string, sorted []versionedCaps) string {
	for i := range sorted {
		for j := range sorted[i].caps.Builtins {
			if sorted[i].caps.Builtins[j].Name == bi {
				return sorted[i].version
			}
		}
	}
	panic("unreachable")
}

func getLatest(bi string, sorted []versionedCaps) *ast.Builtin {
	for i := len(sorted) - 1; i >= 0; i++ {
		for j := range sorted[i].caps.Builtins {
			if sorted[i].caps.Builtins[j].Name == bi {
				return sorted[i].caps.Builtins[j]
			}
		}
	}
	panic("unreachable")
}

func getWasm(bi string) bool {
	return wasm.IsWasmEnabled(bi)
}

type versionedCaps struct {
	version string
	caps    *ast.Capabilities
}

func sortedCaps() []versionedCaps {
	vers, err := ast.LoadCapabilitiesVersions()
	if err != nil {
		panic(err)
	}
	sorted := make([]versionedCaps, len(vers))
	for i, v := range vers {
		caps, err := ast.LoadCapabilitiesVersion(v)
		if err != nil {
			panic(err)
		}
		sorted[i] = versionedCaps{
			version: v,
			caps:    caps,
		}
	}
	return sorted
}
