package ast

import (
	"path"
	"testing"

	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestParserCatchesIllegalCapabilities(t *testing.T) {
	tests := []struct {
		note         string
		regoVersion  RegoVersion
		capabilities Capabilities
		expErr       string
	}{
		{
			note:        "v0, bad future keyword",
			regoVersion: RegoV0,
			capabilities: Capabilities{
				FutureKeywords: []string{"deadbeef"},
			},
			expErr: "illegal capabilities: unknown keyword: deadbeef",
		},
		{
			note:        "v1, bad future keyword",
			regoVersion: RegoV1,
			capabilities: Capabilities{
				Features:       []string{FeatureRegoV1},
				FutureKeywords: []string{"deadbeef"},
			},
			expErr: "illegal capabilities: unknown keyword: deadbeef",
		},
		{
			note:         "v1, no rego_v1 feature",
			regoVersion:  RegoV1,
			capabilities: Capabilities{},
			expErr:       "illegal capabilities: rego_v1 feature required for parsing v1 Rego",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var opts ParserOptions
			opts.Capabilities = &tc.capabilities

			opts.RegoVersion = tc.regoVersion

			_, _, err := ParseStatementsWithOpts("test.rego", "true", opts)
			if err == nil {
				t.Fatal("expected error")
			} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
				t.Fatal("expected exactly one error but got:", err)
			} else if errs[0].Code != ParseErr || errs[0].Message != tc.expErr {
				t.Fatal("unexpected error:", err)
			}
		})
	}
}

func TestParserCatchesIllegalFutureKeywordsBasedOnCapabilities(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion RegoVersion
	}{
		{
			note:        "v0",
			regoVersion: RegoV0,
		},
		{
			note:        "v1",
			regoVersion: RegoV1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var opts ParserOptions
			opts.Capabilities = CapabilitiesForThisVersion()
			opts.FutureKeywords = []string{"deadbeef"}

			opts.RegoVersion = tc.regoVersion

			_, _, err := ParseStatementsWithOpts("test.rego", "true", opts)
			if err == nil {
				t.Fatal("expected error")
			} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
				t.Fatal("expected exactly one error but got:", err)
			} else if errs[0].Code != ParseErr || errs[0].Message != "unknown future keyword: deadbeef" {
				t.Fatal("unexpected error:", err)
			}
		})
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

	opts := ParserOptions{
		Capabilities: &Capabilities{},
		RegoVersion:  RegoV0,
	}

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
	opts := ParserOptions{
		Capabilities: &Capabilities{},
		RegoVersion:  RegoV0,
	}

	_, err := ParseModuleWithOpts("test.rego", src, opts)
	if err == nil {
		t.Fatal("expected error")
	} else if errs, ok := err.(Errors); !ok || len(errs) != 1 {
		t.Fatal("expected exactly one error but got:", err)
	} else if errs[0].Code != ParseErr || errs[0].Location.Row != 7 || errs[0].Message != "unexpected identifier token: expected \\n or ; or }" {
		t.Fatal("unexpected error:", err)
	}
}

func TestParserCapabilitiesFutureKeywordOptIn(t *testing.T) {
	tests := []struct {
		note       string
		advertised []string
		module     string
		expErr     bool
	}{
		{
			note:       "in advertised, specific import",
			advertised: []string{"in"},
			module: `package test
				import future.keywords.in
				p { 1 in [1, 2] }`,
		},
		{
			note:       "in advertised, wildcard import",
			advertised: []string{"in"},
			module: `package test
				import future.keywords
				p { 1 in [1, 2] }`,
		},
		{
			note:       "in advertised, wildcard import, not used",
			advertised: []string{"in"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "every advertised, specific import",
			advertised: []string{"every"},
			module: `package test
				import future.keywords.every
				p { every x in [1, 2] { x } }`,
		},
		{
			note:       "every advertised, wildcard import",
			advertised: []string{"every"},
			module: `package test
				import future.keywords
				p { every x in [1, 2] { x } }`,
		},
		{
			note:       "every advertised, wildcard import, not used",
			advertised: []string{"every"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "contains advertised, specific import",
			advertised: []string{"contains"},
			module: `package test
				import future.keywords.contains
				p contains "a" { true }`,
		},
		{
			note:       "contains advertised, wildcard import",
			advertised: []string{"contains"},
			module: `package test
				import future.keywords
				p contains "a" { true }`,
		},
		{
			note:       "contains advertised, wildcard import, not used",
			advertised: []string{"contains"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "if advertised, specific import",
			advertised: []string{"if"},
			module: `package test
				import future.keywords.if
				p if { true }`,
		},
		{
			note:       "if advertised, wildcard import",
			advertised: []string{"if"},
			module: `package test
				import future.keywords
				p if { true }`,
		},
		{
			note:       "if advertised, wildcard import, not used",
			advertised: []string{"if"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "not advertised, specific import",
			advertised: []string{"not"},
			module: `package test
				import future.keywords.not
				p { not {input.a; input.b} }`,
		},
		{
			note:       "not advertised, wildcard import",
			advertised: []string{"not"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
		},
		{
			note:       "not advertised, wildcard import, and used",
			advertised: []string{"not"},
			module: `package test
				import future.keywords
				p { input.a and input.b }`,
			expErr: true,
		},
		{
			note:       "and advertised, specific import",
			advertised: []string{"and"},
			module: `package test
				import future.keywords.and
				p { input.a and input.b }`,
		},
		{
			note:       "and advertised, wildcard import",
			advertised: []string{"and"},
			module: `package test
				import future.keywords
				p { input.a and input.b }`,
		},
		{
			note:       "and advertised, wildcard import, not used",
			advertised: []string{"and"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "or advertised, specific import",
			advertised: []string{"or"},
			module: `package test
				import future.keywords.or
				p { input.a or input.b }`,
		},
		{
			note:       "or advertised, wildcard import",
			advertised: []string{"or"},
			module: `package test
				import future.keywords
				p { input.a or input.b }`,
		},
		{
			note:       "or advertised, wildcard import, not used",
			advertised: []string{"or"},
			module: `package test
				import future.keywords
				p { not {input.a; input.b} }`,
			expErr: true,
		},
		{
			note:       "and and or advertised, wildcard import",
			advertised: []string{"and", "or"},
			module: `package test
				import future.keywords
				p { input.a and input.b or input.c }`,
		},
		{
			note:       "and and or advertised, wildcard import, not used",
			advertised: []string{"and", "or"},
			module: `package test
				import future.keywords
				p { not (input.a or input.b) }`,
			expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := ParserOptions{
				Capabilities: &Capabilities{FutureKeywords: tc.advertised},
				RegoVersion:  RegoV0, // lock it down to v0 to get every future keyword
			}

			mod, err := ParseModuleWithOpts("test.rego", tc.module, opts)
			if tc.expErr {
				if err == nil {
					t.Fatalf("expected error, got: %v", mod)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
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
		note        string
		module      string
		regoVersion RegoVersion
		version     string
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
		{
			note: "keywords (not)",
			module: `
				package x
				import future.keywords.not
			`,
			version: "1.17.0",
		},
		{
			note: "keywords (and)",
			module: `
				package x
				import future.keywords.and
			`,
			version: "1.20.0",
		},
		{
			note: "keywords (or)",
			module: `
				package x
				import future.keywords.or
			`,
			version: "1.20.0",
		},
		{
			// The wildcard import requires every keyword the module's
			// rego-version allows, so the newest of them decides the version.
			note:        "keywords (wildcard import, v0 module)",
			regoVersion: RegoV0,
			module: `
				package x
				import future.keywords
			`,
			version: "1.20.0",
		},
		{
			note:        "keywords (wildcard import, v1 module)",
			regoVersion: RegoV1,
			module: `
				package x
				import future.keywords
			`,
			version: "1.20.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			regoVersion := RegoV0
			if tc.regoVersion != RegoUndefined {
				regoVersion = tc.regoVersion
			}

			c := MustCompileModulesWithOpts(map[string]string{"test.rego": tc.module}, CompileOpts{
				ParserOptions: ParserOptions{
					RegoVersion: regoVersion,
				},
			})
			minVersion, found := c.Required.MinimumCompatibleVersion()
			if !found || minVersion != tc.version {
				t.Fatal("expected", tc.version, "but got", minVersion)
			}
		})
	}
}

func BenchmarkCapabilitiesCurrentVersion(b *testing.B) {
	var caps *Capabilities
	for b.Loop() {
		caps = CapabilitiesForThisVersion()
	}
	if caps == nil {
		b.Fatal("expected capabilities to be non-nil")
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
