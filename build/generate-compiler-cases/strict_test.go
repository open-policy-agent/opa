// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"fmt"
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

		same, detail, err := sameEitherWay(tc)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}

		if !same {
			t.Errorf("%s: no 'strict' field, but the outcome depends on strict mode; set strict: %s\n%s",
				tc.Note, compilecases.StrictDisabled, detail)
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

		same, _, err := sameEitherWay(tc)
		if err != nil {
			t.Errorf("%s: %v", tc.Note, err)
			continue
		}

		if same {
			t.Errorf("%s: strict is %q, but the outcome is the same either way; "+
				"drop the field so any consumer can run the case", tc.Note, tc.Strict)
		}
	}
}

// sameEitherWay reports whether a case reaches the same outcome with strict mode on
// and off. Both the diagnostics and the compiled modules count: a check strict mode
// adds under an unrelated code makes the setting matter, and so does a rewrite that
// reports nothing.
func sameEitherWay(tc compilecases.TestCase) (bool, string, error) {
	offDiags, offModules, err := outcome(tc, false)
	if err != nil {
		return false, "", err
	}

	onDiags, onModules, err := outcome(tc, true)
	if err != nil {
		return false, "", err
	}

	if offDiags != onDiags {
		return false, fmt.Sprintf("  off: %s\n  on:  %s", offDiags, onDiags), nil
	}

	for i := range offModules {
		if offModules[i] == nil || onModules[i] == nil || !offModules[i].Equal(onModules[i]) {
			return false, fmt.Sprintf("  %s compiles differently", compilecases.ModuleName(i)), nil
		}
	}

	return true, "", nil
}

// outcome compiles a case with the given strict setting and returns its
// diagnostics, rendered for comparison, and its compiled modules.
func outcome(tc compilecases.TestCase, strict bool) (string, []*ast.Module, error) {
	popts, err := parserOptions(tc)
	if err != nil {
		return "", nil, err
	}

	modules := make(map[string]*ast.Module, len(tc.Modules))
	for i, src := range tc.Modules {
		name := compilecases.ModuleName(i)
		m, perr := ast.ParseModuleWithOpts(name, src, popts)
		if perr != nil {
			return "", nil, perr
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

	out := make([]*ast.Module, 0, len(tc.Modules))
	for i := range tc.Modules {
		mod := c.Modules[compilecases.ModuleName(i)]
		if mod != nil {
			mod.Comments = nil
		}
		out = append(out, mod)
	}

	return strings.Join(lines, "; "), out, nil
}
