// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/compilecases/testdata"
	"github.com/open-policy-agent/opa/v1/util"
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

// LoadCompilerTestCases returns the compiler conformance corpus, which is the
// committed cases unchanged — a consumer that wants only those can read the
// embedded YAML directly and skip this package entirely.
func LoadCompilerTestCases() ([]CompilerSet, error) {
	return LoadCompilerTestCasesFiltered(nil)
}

// LoadCompilerTestCasesFiltered returns the compiler conformance corpus with
// Ignore set on every case rejected by one of filters. The case itself is kept,
// so the corpus stays addressable by index.
func LoadCompilerTestCasesFiltered(filters []Filters) ([]CompilerSet, error) {
	sets, err := readSets()
	if err != nil {
		return nil, err
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
		if err := util.Unmarshal(bs, &x); err != nil {
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
