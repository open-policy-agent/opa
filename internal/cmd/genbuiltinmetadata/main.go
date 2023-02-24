// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/compiler/wasm"
	"github.com/open-policy-agent/opa/types"
)

func main() {
	f := ast.CapabilitiesForThisVersion()
	sorted := sortedCaps()
	sorted = append(sorted, versionedCaps{version: "edge", caps: f})

	mdata := make(map[string]interface{})
	categories := make(map[string][]string)

	for _, bi := range f.Builtins {
		latest := getLatest(bi.Name, sorted)
		for _, cat := range builtinCategories(latest) {
			categories[cat] = append(categories[cat], bi.Name)
		}

		argTypes := make([]map[string]interface{}, len(latest.Decl.FuncArgs().Args))

		for i, typ := range latest.Decl.NamedFuncArgs().Args {
			if n, ok := typ.(*types.NamedType); ok {
				argTypes[i] = map[string]interface{}{
					"name": n.Name,
					"type": n.Type.String(),
				}
				if n.Descr != "" {
					argTypes[i]["description"] = n.Descr
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
		versions := getVersions(bi.Name, sorted)
		md := map[string]interface{}{
			"introduced": versions[0],
			"available":  versions,
			"wasm":       getWasm(bi.Name),
			"args":       argTypes,
			"result":     res,
		}
		if latest.Relation {
			md["relation"] = true
		}
		if latest.Infix != "" {
			md["infix"] = latest.Infix
		}
		if latest.Description != "" {
			md["description"] = latest.Description
		}
		if latest.IsDeprecated() {
			md["deprecated"] = true
		}
		mdata[bi.Name] = md
	}

	mdata["_categories"] = categories

	md, err := os.Create(os.Args[1]) // metadata
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(md)
	enc.SetIndent("", "  ")

	if err := enc.Encode(mdata); err != nil {
		panic(err)
	}

	if err := md.Close(); err != nil {
		panic(err)
	}
}

func getVersions(bi string, sorted []versionedCaps) []string {
	vers := []string{}
	for i := range sorted {
		for j := range sorted[i].caps.Builtins {
			if sorted[i].caps.Builtins[j].Name == bi {
				vers = append(vers, sorted[i].version)
			}
		}
	}
	return vers
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

func builtinCategories(b *ast.Builtin) []string {
	if b.IsDeprecated() {
		return nil
	}
	if len(b.Categories) > 0 {
		return b.Categories
	}
	if s := strings.Split(b.Name, "."); len(s) > 1 {
		return []string{s[0]}
	}

	switch b.Name {
	case "assign", "eq", "print":
		// Do nothing.
	default:
		log.Printf("WARN: not categorized: %s", b.Name)
	}

	return nil
}
