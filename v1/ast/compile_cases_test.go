// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/test/compilecases"
)

// TestCompileCases runs the compiler diagnostic corpus in v1/test/compilecases.
func TestCompileCases(t *testing.T) {
	for _, dir := range []string{"v0", "v1"} {
		for _, tc := range compilecases.MustLoad("../test/compilecases/testdata/" + dir).Sorted().Cases {
			t.Run(dir+"/"+tc.Note, func(t *testing.T) {
				runCompileCase(t, tc)
			})
		}
	}
}

func runCompileCase(t *testing.T, tc compilecases.TestCase) {
	t.Helper()

	if len(tc.WantErrors) == 0 {
		t.Fatalf("%s: expected at least one entry in 'want_errors'", tc.Filename)
	}

	regoVersion, err := compileCaseRegoVersion(tc.RegoVersion)
	if err != nil {
		t.Fatalf("%s: %v", tc.Filename, err)
	}

	popts := ParserOptions{RegoVersion: regoVersion}
	if tc.ExperimentalKeywords {
		popts.Capabilities = CapabilitiesForThisVersion(CapabilitiesExperimentalKeywords(true))
	}

	modules := make(map[string]*Module, len(tc.Modules))
	for i, module := range tc.Modules {
		name := compilecases.ModuleName(i)
		parsed, err := ParseModuleWithOpts(name, module, popts)
		if err != nil {
			// Parse errors are reported by their own stage, and are not (yet)
			// covered by this corpus.
			t.Fatalf("unexpected parse error: %v", err)
		}
		modules[name] = parsed
	}

	c := NewCompiler().WithStrict(tc.Strict).WithEnablePrintStatements(tc.PrintStatements)
	c.Compile(modules)

	if !c.Failed() {
		t.Fatal("expected compilation to fail, but it succeeded")
	}

	got := make([]compilecases.Error, 0, len(c.Errors))
	for _, e := range c.Errors {
		got = append(got, compileCaseError(e))
	}

	assertCompileCaseErrors(t, tc, got)
}

// assertCompileCaseErrors checks that every expected diagnostic was reported,
// and, for an exhaustive case, that nothing else was. Diagnostics are compared
// as a set: the order the compiler reports them in is not part of the contract.
func assertCompileCaseErrors(t *testing.T, tc compilecases.TestCase, got []compilecases.Error) {
	t.Helper()

	matched := make([]bool, len(got))
	var missing []compilecases.Error

	for _, want := range tc.WantErrors {
		found := false
		for i, g := range got {
			if matched[i] || !compileCaseErrorMatches(want, g) {
				continue
			}
			matched[i] = true
			found = true
			break
		}
		if !found {
			missing = append(missing, want)
		}
	}

	var unexpected []compilecases.Error
	if tc.Exhaustive {
		for i, g := range got {
			if !matched[i] {
				unexpected = append(unexpected, g)
			}
		}
	}

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

	t.Fatalf("%s: diagnostics do not match:%s", tc.Filename, sb.String())
}

func compileCaseErrorMatches(want, got compilecases.Error) bool {
	return want.ModuleOrDefault() == got.ModuleOrDefault() &&
		want.Code == got.Code &&
		want.Row == got.Row &&
		(want.Col == 0 || want.Col == got.Col) &&
		want.Message == got.Message
}

func compileCaseError(e *Error) compilecases.Error {
	out := compilecases.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Module = e.Location.File
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	return out
}

func compileCaseRegoVersion(s string) (RegoVersion, error) {
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
