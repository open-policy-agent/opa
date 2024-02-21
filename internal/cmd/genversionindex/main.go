package main

import (
	"encoding/json"
	"os"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/semver"
)

func minVersionIndex() ast.VersionIndex {

	index := ast.VersionIndex{
		Builtins: map[string]semver.Version{},
		Features: map[string]semver.Version{},
		Keywords: map[string]semver.Version{},
	}

	versions, err := ast.LoadCapabilitiesVersions()
	if err != nil {
		panic(err)
	}

	for _, v := range versions {
		var sv semver.Version
		if err := sv.Set(v[1:]); err != nil {
			panic(err)
		}

		c, err := ast.LoadCapabilitiesVersion(v)
		if err != nil {
			panic(err)
		}

		for _, bi := range c.Builtins {
			exist, ok := index.Builtins[bi.Name]
			if !ok || exist.Compare(sv) > 0 {
				index.Builtins[bi.Name] = sv
			}
		}

		for _, kw := range c.FutureKeywords {
			exist, ok := index.Keywords[kw]
			if !ok || exist.Compare(sv) > 0 {
				index.Keywords[kw] = sv
			}
		}

		for _, feat := range c.Features {
			exist, ok := index.Features[feat]
			if !ok || exist.Compare(sv) > 0 {
				index.Features[feat] = sv
			}
		}
	}

	return index
}

func main() {
	fd, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(fd)
	enc.SetIndent("", "  ")

	vi := minVersionIndex()

	if err := enc.Encode(vi); err != nil {
		panic(err)
	}

	if err := fd.Close(); err != nil {
		panic(err)
	}
}
