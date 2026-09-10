// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/compilecases/testdata"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// CompilerTestCase is a corpus case together with whatever a filter had to say
// about it. Nothing below the embedded case is committed to the repository.
type CompilerTestCase struct {
	compilecases.TestCase
	Ignore bool `json:"ignore"` // a filter rejected the case: it is reported, not runnable
}

// CompilerSet is the set of cases loaded from one corpus file.
type CompilerSet struct {
	Cases []*CompilerTestCase `json:"cases"`
}

// Option configures what LoadCompilerTestCases returns on top of the committed
// corpus.
type Option func(*config)

type config struct {
	directiveImports bool
}

// WithDirectiveImports rewrites each want entry's Module to carry its own imports,
// and clears them, for a consumer that cannot put the directives in effect out of
// band.
//
// The result then has imports OPA's compiled module does not, so it no longer parses
// to the AST OPA produces: use it when your pipeline keeps its imports, not to
// compare against OPA. Nothing lands in the corpus.
func WithDirectiveImports() Option {
	return func(c *config) { c.directiveImports = true }
}

// Filters are functions that will return true if a test case should be filtered out
type Filters func(*CompilerTestCase) bool

// RegoVersionFilter will filter out any test case written for a rego_version
// that is not in versions. Matching is exact: v0-compat-v1 is its own parsing
// mode, so supporting v0 or v1 does not imply it. Passing no version filters
// nothing.
func RegoVersionFilter(versions ...ast.RegoVersion) Filters {
	return func(tc *CompilerTestCase) bool {
		return corpusgen.RegoVersionRejected(tc.RegoVersion, versions)
	}
}

// StrictModeFilter will filter out any test case that pins a strict mode setting
// the consumer cannot produce. Pass the settings your compiler can be run under:
//
//	StrictModeFilter(compilecases.StrictDisabled)  // no strict mode, or always off
//	StrictModeFilter(compilecases.StrictEnabled)   // strict checks are unconditional
//
// A compiler whose strict mode is switchable supports both, which is what passing
// neither means — an empty set filters nothing.
//
// Cases that leave strict unset always pass, whatever is named here: an absent
// value means the case asserts the same thing either way, which
// TestStrictIsImmaterialWhereUnset enforces rather than assumes.
func StrictModeFilter(supported ...string) Filters {
	return func(tc *CompilerTestCase) bool {
		if tc.Strict == "" || len(supported) == 0 {
			return false
		}
		return !slices.Contains(supported, tc.Strict)
	}
}

// LoadCompilerTestCases returns the compiler conformance corpus, which is the
// committed cases unchanged — a consumer that wants only those can read the
// embedded YAML directly and skip this package entirely.
func LoadCompilerTestCases(opts ...Option) ([]CompilerSet, error) {
	return LoadCompilerTestCasesFiltered(nil, opts...)
}

// LoadCompilerTestCasesFiltered returns the compiler conformance corpus with
// Ignore set on every case rejected by one of filters. The case itself is kept,
// so the corpus stays addressable by index.
func LoadCompilerTestCasesFiltered(filters []Filters, opts ...Option) ([]CompilerSet, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	sets, err := readSets()
	if err != nil {
		return nil, err
	}

	if cfg.directiveImports {
		if err := addDirectiveImports(sets); err != nil {
			return nil, err
		}
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			for _, filter := range filters {
				if filter(tc) {
					tc.Ignore = true
					break
				}
			}
		}
	}

	return sets, nil
}

func readSets() ([]CompilerSet, error) {
	var results []CompilerSet

	err := fs.WalkDir(testdata.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || (path.Ext(p) != ".yaml" && path.Ext(p) != ".yml") {
			return nil
		}

		bs, err := testdata.FS.ReadFile(p)
		if err != nil {
			return err
		}

		var x compilecases.Set
		if err := conformance.Unmarshal(bs, &x); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}

		set := CompilerSet{}
		for i := range x.Cases {
			tc := x.Cases[i].WithFilename(p)
			if err := tc.Validate(); err != nil {
				return fmt.Errorf("%s: %s: %w", p, tc.Note, err)
			}
			set.Cases = append(set.Cases, &CompilerTestCase{TestCase: tc})
		}

		if len(set.Cases) > 0 {
			results = append(results, set)
		}

		return nil
	})

	return results, err
}

// addDirectiveImports puts back the imports the compiler dropped, so each expected
// module parses with nothing but its rego version.
func addDirectiveImports(sets []CompilerSet) error {
	for _, set := range sets {
		for _, tc := range set.Cases {
			for i := range tc.Want {
				want := &tc.Want[i]
				if len(want.Imports) == 0 {
					continue
				}

				imports := make([]string, 0, len(want.Imports))
				for _, path := range want.Imports {
					imports = append(imports, "import "+path)
				}

				with, err := withImports(want.Module, imports)
				if err != nil {
					return fmt.Errorf("%s: %s: want[%d]: %w", tc.Filename, tc.Note, i, err)
				}

				want.Module, want.Imports = with, nil
			}
		}
	}

	return nil
}

// withImports inserts the imports after the package clause, the only place they are
// legal.
func withImports(module string, imports []string) (string, error) {
	head, rest, found := strings.Cut(module, "\n")
	if !found || !strings.HasPrefix(head, "package ") {
		return "", errors.New("the expected module does not open with a package clause")
	}

	return head + "\n\n" + strings.Join(imports, "\n") + "\n" + rest, nil
}
