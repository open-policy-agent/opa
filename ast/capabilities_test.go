package ast

import (
	"path"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestParserCatchesIllegalCapabilities(t *testing.T) {
	var opts ParserOptions
	opts.Capabilities = &Capabilities{
		FutureKeywords: []string{"deadbeef"},
	}

	_, _, err := ParseStatementsWithOpts("test.rego", "true", opts)
	if err == nil {
		t.Fatal("expected error")
	} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", err)
	} else if errs[0].Code != ParseErr || errs[0].Message != "illegal capabilities: unknown keyword: deadbeef" {
		t.Fatal("unexpected error:", err)
	}
}

func TestParserCatchesIllegalFutureKeywordsBasedOnCapabilities(t *testing.T) {
	var opts ParserOptions
	opts.Capabilities = CapabilitiesForThisVersion()
	opts.FutureKeywords = []string{"deadbeef"}

	_, _, err := ParseStatementsWithOpts("test.rego", "true", opts)
	if err == nil {
		t.Fatal("expected error")
	} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", err)
	} else if errs[0].Code != ParseErr || errs[0].Message != "unknown future keyword: deadbeef" {
		t.Fatal("unexpected error:", err)
	}
}

func TestParserCapabilitiesWithSpecificOptInAndOlderOPA(t *testing.T) {

	src := `
		package test

		import future.keywords.in

		p {
			1 in [3,2,1]
		}
	`

	var opts ParserOptions
	opts.Capabilities = &Capabilities{}

	_, err := ParseModuleWithOpts("test.rego", src, opts)
	if err == nil {
		t.Fatal("expected error")
	} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", err)
	} else if errs[0].Code != ParseErr || errs[0].Location.Row != 4 || errs[0].Message != "unexpected keyword, must be one of []" {
		t.Fatal("unexpected error:", err)
	}
}

func TestParserCapabilitiesWithWildcardOptInAndOlderOPA(t *testing.T) {

	src := `
		package test

		import future.keywords

		p {
			1 in [3,2,1]
		}
	`
	var opts ParserOptions
	opts.Capabilities = &Capabilities{}

	_, err := ParseModuleWithOpts("test.rego", src, opts)
	if err == nil {
		t.Fatal("expected error")
	} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", err)
	} else if errs[0].Code != ParseErr || errs[0].Location.Row != 7 || errs[0].Message != "unexpected identifier token: expected \\n or ; or }" {
		t.Fatal("unexpected error:", err)
	}
}

func TestLoadCapabilitiesVersion(t *testing.T) {

	capabilitiesVersions, err := LoadCapabilitiesVersions()
	if err != nil {
		t.Fatal("expected success", err)
	}

	if len(capabilitiesVersions) == 0 {
		t.Fatal("expected a non-empty array of capabilities versions")
	}
	for _, cv := range capabilitiesVersions {
		if _, err := LoadCapabilitiesVersion(cv); err != nil {
			t.Fatal("expected success", err)
		}
	}
}

func TestLoadCapabilitiesFile(t *testing.T) {

	files := map[string]string{
		"test-capabilities.json": `
		{
			"builtins": []
		}
		`,
	}

	test.WithTempFS(files, func(root string) {
		_, err := LoadCapabilitiesFile(path.Join(root, "test-capabilities.json"))
		if err != nil {
			t.Fatal("expected success", err)
		}
	})

}

func TestCapabilitiesAddBuiltinSorted(t *testing.T) {

	c := CapabilitiesForThisVersion()

	indexOfEq := findBuiltinIndex(c, "eq")
	if indexOfEq < 0 {
		panic("expected to find eq")
	}

	c.addBuiltinSorted(&Builtin{Name: "eq"})

	if c.Builtins[indexOfEq].Decl != nil {
		t.Fatal("expected builtin to get overwritten")
	}

	c.addBuiltinSorted(&Builtin{Name: "~foo"}) // non-existent but always sorts to the end

	if findBuiltinIndex(c, "~foo") != len(c.Builtins)-1 {
		t.Fatal("expected builtin to be last in slice")
	}

	c.addBuiltinSorted(&Builtin{Name: " foo"}) // non-existent but always sorts to start

	if findBuiltinIndex(c, " foo") != 0 {
		t.Fatal("expected builtin to be first in slice")
	}

	c.addBuiltinSorted(&Builtin{Name: "plus1"}) // non-existent but always after plus in middle

	if findBuiltinIndex(c, "plus1") != findBuiltinIndex(c, "plus")+1 {
		t.Fatal("expected builtin to be immediately after plus")
	}
}

func TestCapabilitiesMinimumCompatibleVersion(t *testing.T) {

	tests := []struct {
		note    string
		module  string
		version string
	}{
		{
			note: "builtins",
			module: `
				package x
				p { array.reverse([1,2,3]) }
			`,
			version: "0.36.0",
		},
		{
			note: "keywords",
			module: `
				package x
				import future.keywords.every
			`,
			version: "0.38.0",
		},
		{
			note: "features (string prefix ref)",
			module: `
				package x
				import future.keywords.if
				p.a.b.c.d if { true }
			`,
			version: "0.46.0",
		},
		{
			note: "features (general ref)",
			module: `
				package x
				import future.keywords.if
				p.a.b[c].d if { c := "foo" }
			`,
			version: "0.59.0",
		},
		{
			note: "features (general ref + string prefix ref)",
			module: `
				package x
				import future.keywords.if
				p.a.b.c.d if { true }
				p.a.b[c].d if { c := "foo" }
			`,
			version: "0.59.0",
		},
		{
			note: "rego.v1 import",
			module: `
				package x
				import rego.v1`,
			version: "0.59.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := MustCompileModules(map[string]string{"test.rego": tc.module})
			minVersion, found := c.Required.MinimumCompatibleVersion()
			if !found || minVersion != tc.version {
				t.Fatal("expected", tc.version, "but got", minVersion)
			}
		})
	}
}

func findBuiltinIndex(c *Capabilities, name string) int {
	for i, bi := range c.Builtins {
		if bi.Name == name {
			return i
		}
	}
	return -1
}
