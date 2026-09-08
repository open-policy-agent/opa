// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/gobwas/glob"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
	"github.com/open-policy-agent/opa/v1/test/parsercases/testdata"
	"github.com/open-policy-agent/opa/v1/util"
)

var exceptionsFile = flag.String("parser-exceptions", "./exceptions.yaml", "set file to load a list of parser test names to exclude")

var (
	exceptions     map[string]string
	exceptionGlobs []*glob.Pattern
)

func setup() error {
	exceptions = map[string]string{}
	exceptionGlobs = nil

	bs, err := os.ReadFile(*exceptionsFile)
	if err != nil {
		return fmt.Errorf("unable to load exceptions file: %w", err)
	}
	if err := util.Unmarshal(bs, &exceptions); err != nil {
		return fmt.Errorf("unable to parse exceptions file: %w", err)
	}

	for pattern := range exceptions {
		if !strings.Contains(pattern, "*") {
			continue
		}

		g, err := glob.Compile(pattern, '/')
		if err != nil {
			return fmt.Errorf("invalid glob pattern %q in exceptions file: %w", pattern, err)
		}
		exceptionGlobs = append(exceptionGlobs, g)
	}

	return nil
}

func shouldSkip(tc parsercases.TestCase) bool {
	if _, ok := exceptions[tc.Note]; ok {
		return true
	}

	for _, g := range exceptionGlobs {
		if g.Match(tc.Note) {
			return true
		}
	}

	return false
}

// ParserTestCase is a corpus case together with the artifacts generated for it
// on request. Nothing below the embedded case is committed to the repository.
type ParserTestCase struct {
	parsercases.TestCase
	WantIR  *ir.Policy `json:"want_ir,omitempty"`  // the plan the module compiles to, absent where it does not compile or plan, or where a filter rejected the case
	IRError string     `json:"ir_error,omitempty"` // why a success case has no plan; diagnostic only, never an assertion about compilation
	Ignore  bool       `json:"ignore"`             // a filter rejected the case: it is reported, not runnable
}

// ParserSet is the set of cases loaded from one corpus file.
type ParserSet struct {
	Cases []*ParserTestCase `json:"cases"`
}

// Option configures what LoadParserTestCases generates on top of the committed
// corpus.
type Option func(*config)

type config struct {
	locations    bool
	locationText bool
	ir           bool
}

// WithASTLocations regenerates want_ast with row and col on every node type,
// overriding each case's Locations field. Because the positions of a case's
// want_equivalent module differ from those of its module, want_equivalent is
// dropped from the cases it is regenerated for.
func WithASTLocations() Option {
	return func(c *config) { c.locations = true }
}

// WithLocationText additionally includes source text spans. Implies
// WithASTLocations.
func WithLocationText() Option {
	return func(c *config) {
		c.locations = true
		c.locationText = true
	}
}

// WithIR compiles and plans each success case and populates WantIR where that
// succeeds. Not committed to the corpus.
func WithIR() Option {
	return func(c *config) { c.ir = true }
}

// Filters are functions that will return true if a test case should be filtered out
type Filters func(*ParserTestCase) bool

// CapabilitiesFilter will filter out any test case whose plan uses a builtin
// that is not in c. It pairs with WithIR, and passes any case that has no plan.
func CapabilitiesFilter(c *ast.Capabilities) Filters {
	builtins := make(map[string]struct{}, len(c.Builtins))
	for _, b := range c.Builtins {
		builtins[b.Name] = struct{}{}
	}

	return func(tc *ParserTestCase) bool {
		if len(builtins) == 0 || tc.WantIR == nil {
			return false
		}

		for _, b := range tc.WantIR.Static.BuiltinFuncs {
			// if the test case contains a builtin not in the capabilities file, reject it
			if _, ok := builtins[b.Name]; !ok {
				return true
			}
		}

		return false
	}
}

// RegoVersionFilter will filter out any test case written for a rego_version
// that is not in versions. Matching is exact: v0-compat-v1 is its own parsing
// mode, so supporting v0 or v1 does not imply it. Passing no version filters
// nothing.
//
// Unlike CapabilitiesFilter, which rejects only a plan a consumer cannot
// execute, this rejects the case outright — the module is written in a dialect
// the consumer does not parse, so neither its AST nor its diagnostics apply.
func RegoVersionFilter(versions ...ast.RegoVersion) Filters {
	return func(tc *ParserTestCase) bool {
		return corpusgen.RegoVersionRejected(tc.RegoVersion, versions)
	}
}

// LoadParserTestCases returns the parser conformance corpus. With no options it
// returns the committed cases unchanged, so a consumer that only wants the
// corpus can read the embedded YAML directly and skip this package entirely.
func LoadParserTestCases(opts ...Option) ([]ParserSet, error) {
	return LoadParserTestCasesFiltered(nil, opts...)
}

// LoadParserTestCasesFiltered returns the parser conformance corpus with Ignore
// set, and the plan dropped, on every case rejected by one of filters. The case
// itself is kept, so the corpus stays addressable by index.
//
// What Ignore permits depends on the filter that set it — CapabilitiesFilter
// rejects a plan a consumer cannot execute, RegoVersionFilter rejects the case
// outright — so skipping an ignored case entirely is always the safe reading.
func LoadParserTestCasesFiltered(filters []Filters, opts ...Option) ([]ParserSet, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	if err := setup(); err != nil {
		return nil, err
	}

	sets, err := readSets()
	if err != nil {
		return nil, err
	}

	if cfg.locations {
		if err := regenerateAST(sets, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.ir {
		if err := generateIR(sets); err != nil {
			return nil, err
		}
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			for _, filter := range filters {
				if filter(tc) {
					// The case is kept rather than removed, so the corpus stays
					// addressable by index. EntryPoints stays too, since a case
					// may have authored it.
					tc.Ignore = true
					tc.WantIR = nil
					break
				}
			}
		}
	}

	return sets, nil
}

func readSets() ([]ParserSet, error) {
	var results []ParserSet

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

		var x parsercases.Set
		if err := util.Unmarshal(bs, &x); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}

		set := ParserSet{}
		for i := range x.Cases {
			tc := x.Cases[i].WithFilename(p)
			if err := tc.Validate(); err != nil {
				return fmt.Errorf("%s: %s: %w", p, tc.Note, err)
			}
			if shouldSkip(tc) {
				continue
			}
			set.Cases = append(set.Cases, &ParserTestCase{TestCase: tc})
		}

		if len(set.Cases) > 0 {
			results = append(results, set)
		}

		return nil
	})

	return results, err
}

// regenerateAST rewrites want_ast for every success case with locations turned
// on. The marshalling options are global state, so they are set once for the
// whole pass rather than per case.
func regenerateAST(sets []ParserSet, cfg *config) error {
	restore := astJSON.GetOptions()
	astJSON.SetOptions(parsercases.MarshalOptions(true, cfg.locationText))
	defer astJSON.SetOptions(restore)

	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.Failure() {
				continue
			}

			module, err := parseModule(tc.TestCase, tc.Module)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
			}

			want, err := MarshalAST(module)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", tc.Filename, tc.Note, err)
			}

			tc.WantAST = want
			tc.WantEquivalent = ""
		}
	}

	return nil
}
