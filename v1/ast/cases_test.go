// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/test/conformance"
)

func caseRegoVersion(s string) (RegoVersion, error) {
	switch s {
	case "", "v1":
		return RegoV1, nil
	case "v0":
		return RegoV0, nil
	case "v0-compat-v1":
		return RegoV0CompatV1, nil
	}
	return RegoUndefined, fmt.Errorf("unknown rego_version %q", s)
}

// TestCaseRegoVersionCoversTheSchema fails when a version is added to the corpus
// schema and not mapped here. The generator has the same mapping and the same test,
// since neither package can import the other; this is what keeps the two from
// drifting into generating fixtures under a version they are not checked under.
func TestCaseRegoVersionCoversTheSchema(t *testing.T) {
	seen := map[RegoVersion]string{}

	for _, s := range conformance.RegoVersions {
		v, err := caseRegoVersion(s)
		if err != nil {
			t.Errorf("%q is an accepted rego_version but has no parser version: %v", s, err)
			continue
		}
		if other, ok := seen[v]; ok {
			t.Errorf("%q and %q both map to %v", other, s, v)
		}
		seen[v] = s
	}

	// Not in RegoVersions: absent is structural rather than an accepted value.
	if v, err := caseRegoVersion(""); err != nil || v != RegoV1 {
		t.Errorf("expected an absent rego_version to be v1, got %v, %v", v, err)
	}
	if _, err := caseRegoVersion("v2"); err == nil {
		t.Error("expected an unknown rego_version to be rejected")
	}
}

// caseParserOptions translates a case's parse options onto OPA's parser, for both
// corpora. Every decision is the schema's; nothing is derived from the case here. Each
// generator does the same translation, and cannot share this one — it would have to
// import package ast, which imports it — so TestCaseParserOptionsCarryEveryField on each
// side is what keeps fixtures from being generated under options they are not checked
// under.
func caseParserOptions(opts conformance.ParseOptions) (ParserOptions, error) {
	version, err := caseRegoVersion(opts.RegoVersion)
	if err != nil {
		return ParserOptions{}, err
	}

	return ParserOptions{
		RegoVersion:       version,
		ProcessAnnotation: opts.ProcessAnnotations,
		FutureKeywords:    opts.FutureKeywords,
		AllFutureKeywords: opts.AllFutureKeywords,
		SkipRules:         opts.SkipRules,
	}, nil
}

// TestCaseParserOptionsCarryEveryField fails when a field is added to the schema's
// ParseOptions and caseParserOptions does not carry it.
func TestCaseParserOptionsCarryEveryField(t *testing.T) {
	carried := map[string]func(*testing.T){
		"RegoVersion": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{RegoVersion: "v0"},
				func(p ParserOptions) bool { return p.RegoVersion == RegoV0 })
		},
		"FutureKeywords": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{FutureKeywords: []string{"in"}},
				func(p ParserOptions) bool { return slices.Equal(p.FutureKeywords, []string{"in"}) })
		},
		"AllFutureKeywords": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{AllFutureKeywords: true},
				func(p ParserOptions) bool { return p.AllFutureKeywords })
		},
		"ProcessAnnotations": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{ProcessAnnotations: true},
				func(p ParserOptions) bool { return p.ProcessAnnotation })
		},
		"SkipRules": func(t *testing.T) {
			assertCarried(t, conformance.ParseOptions{SkipRules: true},
				func(p ParserOptions) bool { return p.SkipRules })
		},
	}

	for _, field := range conformance.ParseOptionFields() {
		check, ok := carried[field]
		if !ok {
			t.Errorf("conformance.ParseOptions.%s is not carried by caseParserOptions; carry it, and say here how", field)
			continue
		}
		t.Run(field, check)
	}
}

func assertCarried(t *testing.T, opts conformance.ParseOptions, carried func(ParserOptions) bool) {
	t.Helper()

	popts, err := caseParserOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !carried(popts) {
		t.Errorf("the field was not carried onto %+v", popts)
	}
}

// caseErrors converts every reported diagnostic into the corpus's form.
func caseErrors(errs Errors) []conformance.Error {
	out := make([]conformance.Error, 0, len(errs))
	for _, e := range errs {
		out = append(out, caseError(e))
	}
	return out
}

func caseError(e *Error) conformance.Error {
	out := conformance.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Module = e.Location.File
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	if e.Details != nil {
		out.Detail = strings.Join(e.Details.Lines(), "\n")
	}
	return out
}

// assertCaseErrors checks that every expected diagnostic was reported, and, for
// an exhaustive case, that nothing else was.
func assertCaseErrors(t *testing.T, filename string, want, got []conformance.Error, exhaustive bool) {
	t.Helper()

	missing, unexpected := conformance.MatchErrors(want, got, exhaustive)
	if len(missing) == 0 && len(unexpected) == 0 {
		return
	}

	var sb strings.Builder
	for _, e := range missing {
		fmt.Fprintf(&sb, "\n  missing:    %s", e)
	}
	for _, e := range unexpected {
		fmt.Fprintf(&sb, "\n  unexpected: %s", e)
	}
	fmt.Fprintf(&sb, "\n\nreported:")
	for _, e := range got {
		fmt.Fprintf(&sb, "\n  %s", e)
	}

	t.Fatalf("%s: diagnostics do not match:%s", filename, sb.String())
}

// indented renders diagnostics one per line, for a failure message that lists what was
// reported.
func indented(errs []conformance.Error) string {
	var sb strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&sb, "\n  %s", e)
	}
	return sb.String()
}
