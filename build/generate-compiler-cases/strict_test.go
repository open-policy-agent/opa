// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/compilecases/testdata"
)

// TestStrictIsImmaterialWhereUnset checks the claim an absent strict field makes:
// that the case asserts the same thing with the compiler's strict mode on or off.
// A consumer whose own strict mode is not switchable relies on it to decide which
// cases it can run, so it is enforced rather than trusted.
//
// A case whose diagnostics do change has to say which setting it means, with
// strict: enabled or strict: disabled.
func TestStrictIsImmaterialWhereUnset(t *testing.T) {
	set, err := compilecases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Cases) == 0 {
		t.Fatal("expected the committed corpus to hold cases")
	}

	var checked int

	for _, tc := range set.Sorted().Cases {
		if tc.Strict != "" {
			continue
		}
		checked++

		off, err := diagnostics(tc, false)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}
		on, err := diagnostics(tc, true)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}

		if off != on {
			t.Errorf("%s: no 'strict' field, but the diagnostics depend on strict mode; "+
				"set strict: %s\n  off: %s\n  on:  %s",
				tc.Note, compilecases.StrictDisabled, off, on)
		}
	}

	if checked == 0 {
		t.Fatal("expected some cases to leave strict unset")
	}
}

// TestStrictMattersWhereSet is the converse: a case that names a strict setting
// should need it, or the field is excluding consumers for nothing.
func TestStrictMattersWhereSet(t *testing.T) {
	set, err := compilecases.LoadFS(testdata.FS, ".")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range set.Sorted().Cases {
		if tc.Strict == "" {
			continue
		}

		off, err := diagnostics(tc, false)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}
		on, err := diagnostics(tc, true)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}

		if off == on {
			t.Errorf("%s: strict is %q, but the diagnostics are the same either way; "+
				"drop the field so any consumer can run the case", tc.Note, tc.Strict)
		}
	}
}

// diagnostics renders everything the compiler reports for a case, as a single
// comparable string. Every diagnostic counts, not only those the case asserts: a
// check strict mode adds under some other code still makes the setting matter.
func diagnostics(tc compilecases.TestCase, strict bool) (string, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return "", err
	}

	modules := make(map[string]*ast.Module, len(tc.Modules))
	for i, src := range tc.Modules {
		name := compilecases.ModuleName(i)
		m, perr := ast.ParseModuleWithOpts(name, src, popts)
		if perr != nil {
			return "", perr
		}
		modules[name] = m
	}

	c := ast.NewCompiler().
		SetErrorLimit(0).
		WithStrict(strict).
		WithEnablePrintStatements(tc.PrintStatements)
	c.Compile(modules)

	lines := make([]string, 0, len(c.Errors))
	for _, e := range c.Errors {
		lines = append(lines, caseError(e).String())
	}
	slices.Sort(lines)

	return strings.Join(lines, "; "), nil
}
