// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/test/conformance"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
)

// Generate fills in want_ast for every success case in the corpus rooted at
// dir, rewriting the YAML files in place. The fixture is what OPA's parser
// produces, so it is a golden file: it does not independently validate OPA, it
// catches unreviewed change. The gate is review of the regeneration diff.
func Generate(dir string) error {
	restore := astJSON.GetOptions()
	defer astJSON.SetOptions(restore)

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		version, err := conformance.RegoVersionForPath(dir, path)
		if err != nil {
			return err
		}

		return generateFile(path, version, info.Mode())
	})
}

func generateFile(path, regoVersion string, mode fs.FileMode) error {
	bs, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var set parsercases.Set
	if err := conformance.Unmarshal(bs, &set); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(bs, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if len(doc.Content) == 0 {
		return nil
	}

	root := doc.Content[0]
	caseNodes := corpusgen.MapValue(root, "cases")
	if caseNodes == nil || len(caseNodes.Content) != len(set.Cases) {
		return fmt.Errorf("%s: expected a 'cases' sequence of %d entries", path, len(set.Cases))
	}

	for i := range set.Cases {
		tc := &set.Cases[i]
		*tc = tc.WithSource(path, regoVersion)

		astJSON.SetOptions(conformance.MarshalOptions(tc.Locations, false))
		node, perr := parseCase(*tc)

		switch {
		case perr != nil && tc.WantAST != "":
			return fmt.Errorf("%s: %s: the case asserts 'want_ast', but the policy no longer parses: %w", path, tc.Note, perr)

		case perr == nil && tc.Failure():
			return fmt.Errorf("%s: %s: the case asserts 'want_errors', but the policy parses", path, tc.Note)

		case perr != nil && !tc.Failure():
			// Fill in the diagnostic only where the case has none. A message
			// that changes has to fail the runner, not be quietly rewritten
			// underneath it, so an existing want_errors is never touched.
			//
			// An exhaustive case gets every diagnostic: it asserts that the recorded set
			// is the whole one, so recording the first alone would write a fixture the
			// runner rejects for the rest.
			if tc.Exhaustive {
				filled, derr := allDiagnostics(perr)
				if derr != nil {
					return fmt.Errorf("%s: %s: %w", path, tc.Note, derr)
				}
				tc.WantErrors = filled
			} else {
				tc.WantErrors = []parsercases.Error{firstDiagnostic(perr)}
			}
			corpusgen.SetMapValue(caseNodes.Content[i], "want_errors", corpusgen.ErrorsNode(tc.WantErrors), "exhaustive")

		case perr != nil && positionless(tc.WantErrors):
			// A case that names the diagnostic it asserts, because OPA reports it after
			// another one. The message is the author's; the position and the code are
			// filled in here, the way they are for a case that names nothing.
			completed, cerr := completeDiagnostics(tc.WantErrors, perr)
			if cerr != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, cerr)
			}
			if tc.Exhaustive {
				if cerr := coversAll(completed, perr); cerr != nil {
					return fmt.Errorf("%s: %s: %w", path, tc.Note, cerr)
				}
			}
			tc.WantErrors = completed
			corpusgen.SetMapValue(caseNodes.Content[i], "want_errors", corpusgen.ErrorsNode(tc.WantErrors), "exhaustive")

		case perr == nil:
			var err error
			if tc.WantAST, err = MarshalAST(node); err != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
			}
			corpusgen.SetMapValue(caseNodes.Content[i], "want_ast", corpusgen.Literal(tc.WantAST), "want_equivalent")
		}

		if err := tc.Validate(); err != nil {
			return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
		}
	}

	out, err := corpusgen.Encode(root)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if bytes.Equal(out, bs) {
		return nil
	}

	return os.WriteFile(path, out, mode)
}

// allDiagnostics returns every diagnostic the parse reported, sorted so the file is
// stable whatever order the parser produced them in. The runner matches as a set.
func allDiagnostics(err error) ([]parsercases.Error, error) {
	errs, ok := err.(ast.Errors)
	if !ok {
		return nil, fmt.Errorf("the policy failed with %v, which is not an ast.Errors", err)
	}

	out := make([]parsercases.Error, 0, len(errs))
	for _, e := range errs {
		out = append(out, diagnostic(e))
	}

	slices.SortFunc(out, func(a, b parsercases.Error) int {
		return cmp.Or(
			cmp.Compare(a.Row, b.Row),
			cmp.Compare(a.Col, b.Col),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Message, b.Message),
		)
	})

	return out, nil
}

// coversAll checks that an exhaustive case names every diagnostic the parse reported.
// The case claims its set is the whole one, so a diagnostic it leaves out is a fixture
// the runner would reject.
func coversAll(want []parsercases.Error, err error) error {
	errs, ok := err.(ast.Errors)
	if !ok {
		return fmt.Errorf("the policy failed with %v, which is not an ast.Errors", err)
	}

	for _, e := range errs {
		named := false
		for _, w := range want {
			if w.Message == e.Message {
				named = true
				break
			}
		}
		if !named {
			return fmt.Errorf("'exhaustive' says 'want_errors' is the whole set, but the parser also reports %q; name it too, or drop 'exhaustive'",
				e.Message)
		}
	}

	return nil
}

// positionless reports whether every authored diagnostic still needs its position,
// which is how a case names the message it asserts and leaves the rest to the
// generator. A case whose diagnostics carry positions is complete and never touched.
func positionless(want []parsercases.Error) bool {
	if len(want) == 0 {
		return false
	}
	for _, e := range want {
		if e.Row != 0 || e.Col != 0 || e.Code != "" {
			return false
		}
	}
	return true
}

// completeDiagnostics fills in the code and position of each authored message by finding
// it among the ones OPA reported. A message that is not reported at all is an error: the
// case would otherwise assert something no parse produces.
//
// This is what lets a case assert a diagnostic OPA reports *after* another one. Matching
// is a subset, so recording one of several is a complete assertion; which one is the
// case's to say, and for a handful of cases the first is a low-level token error while
// the second names the rule the case is about.
func completeDiagnostics(want []parsercases.Error, err error) ([]parsercases.Error, error) {
	errs, ok := err.(ast.Errors)
	if !ok {
		return nil, fmt.Errorf("the policy failed with %v, which is not an ast.Errors", err)
	}

	out := make([]parsercases.Error, 0, len(want))
	used := make([]bool, len(errs))

	for _, w := range want {
		// Each authored message takes a reported diagnostic of its own: the same message
		// can be reported at two positions — both operands of an `and` rejected, say — and
		// naming it twice asks for both.
		match := -1
		for i, e := range errs {
			if !used[i] && e.Message == w.Message {
				match = i
				break
			}
		}
		if match < 0 {
			reported := make([]string, 0, len(errs))
			seen := 0
			for _, e := range errs {
				reported = append(reported, e.Message)
				if e.Message == w.Message {
					seen++
				}
			}
			if seen == 0 {
				return nil, fmt.Errorf("'want_errors' names %q, which the parser does not report; it reports %q",
					w.Message, reported)
			}
			return nil, fmt.Errorf("'want_errors' names %q more often than the parser reports it, which is %d time(s)",
				w.Message, seen)
		}

		used[match] = true
		out = append(out, diagnostic(errs[match]))
	}

	return out, nil
}

// firstDiagnostic returns the diagnostic a fixture records. Only the first is
// taken: the ones that follow are usually a cascade of the same mistake, and
// holding another implementation to OPA's cascade is not a language rule. A case
// that asserts one of the later ones names it, and completeDiagnostics fills it in.
func firstDiagnostic(err error) parsercases.Error {
	errs, ok := err.(ast.Errors)
	if !ok || len(errs) == 0 {
		return parsercases.Error{Message: err.Error()}
	}

	return diagnostic(errs[0])
}

func diagnostic(e *ast.Error) parsercases.Error {
	out := parsercases.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	return out
}
