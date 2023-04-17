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
	} else if errs[0].Code != ParseErr || errs[0].Location.Row != 7 || errs[0].Message != "unexpected ident token: expected \\n or ; or }" {
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
