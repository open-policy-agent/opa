// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"embed"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/open-policy-agent/opa/internal/compiler/wasm"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/types"
)

// implementationsFS holds the builtins implemented by Rego interpreters other than this one, so
// the reference documentation can report where each builtin is available. Refresh it with
// build/update-implementations.sh; it is not maintained by hand.
//
//go:embed implementations.json
var implementationsFS embed.FS

// implementation is one non-OPA Rego interpreter, as recorded in implementations.json. Builtins
// maps each builtin the interpreter implements to the version it arrived in, which is empty for
// interpreters that publish only their latest capabilities and so have no history to attribute.
type implementation struct {
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Repo     string            `json:"repo"`
	Version  string            `json:"version"`
	Builtins map[string]string `json:"builtins"`
}

func main() {
	f := ast.CapabilitiesForThisVersion()
	sorted := sortedCaps()
	sorted = append(sorted, versionedCaps{version: "edge", caps: f})

	impls := loadImplementations(f)

	mdata := make(map[string]any)
	categories := make(map[string][]string)

	for _, bi := range f.Builtins {
		latest := getLatest(bi.Name, sorted)
		for _, cat := range builtinCategories(latest) {
			categories[cat] = append(categories[cat], bi.Name)
		}

		argTypes := make([]map[string]any, len(latest.Decl.FuncArgs().Args))

		for i, typ := range latest.Decl.NamedFuncArgs().Args {
			if n, ok := typ.(*types.NamedType); ok {
				argTypes[i] = map[string]any{
					"name": n.Name,
					"type": n.Type.String(),
				}
				if n.Descr != "" {
					argTypes[i]["description"] = n.Descr
				}
			} else {
				argTypes[i] = map[string]any{
					"type": typ.String(),
				}
			}
		}
		res := map[string]any{}
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
		md := map[string]any{
			"introduced":      versions[0],
			"available":       versions,
			"wasm":            getWasm(bi.Name),
			"implementations": getImplementations(bi.Name, impls),
			"args":            argTypes,
			"result":          res,
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
	mdata["_implementations"] = implementationIndex(impls)

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

// loadImplementations reads the recorded non-OPA interpreters, warning about any builtin they
// claim that this version of OPA does not know about: that means either implementations.json is
// stale or the name is misspelled, and the documentation would silently drop it.
func loadImplementations(caps *ast.Capabilities) []implementation {
	bs, err := implementationsFS.ReadFile("implementations.json")
	if err != nil {
		panic(err)
	}

	var doc struct {
		Implementations []implementation `json:"implementations"`
	}
	if err := json.Unmarshal(bs, &doc); err != nil {
		panic(err)
	}

	known := make(map[string]struct{}, len(caps.Builtins))
	for _, bi := range caps.Builtins {
		known[bi.Name] = struct{}{}
	}

	for _, impl := range doc.Implementations {
		for name := range impl.Builtins {
			if _, ok := known[name]; !ok {
				log.Printf("WARN: %s reports unknown builtin: %s", impl.ID, name)
			}
		}
	}

	return doc.Implementations
}

// getImplementations maps each non-OPA interpreter implementing bi to the version it arrived in,
// or to null where that interpreter publishes no version history. The map is non-nil so it
// encodes as {} instead of null.
func getImplementations(bi string, impls []implementation) map[string]any {
	found := map[string]any{}
	for _, impl := range impls {
		version, ok := impl.Builtins[bi]
		if !ok {
			continue
		}
		if version == "" {
			found[impl.ID] = nil
		} else {
			found[impl.ID] = version
		}
	}
	return found
}

// implementationIndex describes each interpreter for the documentation to render. The builtin
// lists are omitted: they are already folded into the per-builtin metadata.
func implementationIndex(impls []implementation) []map[string]any {
	index := make([]map[string]any, 0, len(impls))
	for _, impl := range impls {
		index = append(index, map[string]any{
			"id":      impl.ID,
			"label":   impl.Label,
			"repo":    impl.Repo,
			"version": impl.Version,
		})
	}
	return index
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
