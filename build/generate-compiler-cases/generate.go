// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cases fills in the fixtures of the compiler conformance corpus in
// v1/test/compilecases/testdata.
package cases

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Generate fills in what each case asserts — want, want_stages, a query's want, and
// want_errors where a failure case has none — in the corpus rooted at dir, rewriting the
// YAML files in place.
//
// What that does and does not assert is v1/test/compilecases/README.md's "Adding a case".
func Generate(dir string) error {
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

	var set compilecases.Set
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

		reported, err := caseDiagnostics(*tc)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
		}

		switch {
		case tc.QueryCase() && len(reported) == 0:
			// A query that compiles: what the case asserts is the compiled query, not
			// what its environment modules turned into.
			rego, marshalled, qerr := compiledQueryWant(*tc)
			if qerr != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, qerr)
			}

			// Inside the query group, so the query and what it compiles to read as one
			// thing rather than two keys a reader has to associate. Whichever form is
			// written, the other is removed: a query that becomes printable — or stops
			// being printable — must not end up carrying both.
			node := corpusgen.MapValue(caseNodes.Content[i], "query")
			if node == nil {
				return fmt.Errorf("%s: %s: the case has a query but no 'query' mapping to fill", path, tc.Note)
			}

			tc.Query.Want, tc.Query.WantAST = rego, marshalled

			if rego != "" {
				corpusgen.DeleteMapValue(node, "want_ast")
				corpusgen.SetMapValue(node, "want", corpusgen.Literal(strings.TrimRight(rego, "\n")+"\n"))
			} else {
				corpusgen.DeleteMapValue(node, "want")
				corpusgen.SetMapValue(node, "want_ast", corpusgen.Literal(marshalled))
			}

		case tc.Transform() && len(reported) > 0:
			return fmt.Errorf("%s: %s: the case asserts what its modules compile to, but they report %d diagnostic(s), starting with %s",
				path, tc.Note, len(reported), reported[0])

		case len(reported) == 0 && tc.Failure():
			return fmt.Errorf("%s: %s: the case asserts 'want_errors', but the modules compile", path, tc.Note)

		case len(reported) == 0:
			// A clean compile is a transformation case. Unlike want_errors, want is
			// regenerated every time; review of the diff is the gate.
			if err := fillTransform(tc, caseNodes.Content[i]); err != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
			}

		default:
			// Fill in the diagnostics only where the case has none. A message
			// that changes has to fail the runner.
			if !tc.Failure() {
				tc.WantErrors = reported
				corpusgen.SetMapValue(caseNodes.Content[i], "want_errors",
					corpusgen.ErrorsNode(reported), "exhaustive", "want_stages")
				break
			}

			// detail is generated rather than authored.
			withDetails(tc, reported)
			corpusgen.SetMapValue(caseNodes.Content[i], "want_errors",
				corpusgen.ErrorsNode(tc.WantErrors), "exhaustive", "want_stages")
		}

		if err := fillStages(tc, caseNodes.Content[i]); err != nil {
			return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
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

// withDetails copies the detail onto each committed diagnostic that names a reported
// one. A committed entry with no counterpart keeps none, so a message that has drifted
// still fails the runner rather than being papered over here.
func withDetails(tc *compilecases.TestCase, reported []conformance.Error) {
	used := make([]bool, len(reported))

	for i := range tc.WantErrors {
		want := &tc.WantErrors[i]
		want.Detail = ""

		for j, got := range reported {
			if used[j] || !sameDiagnostic(*want, got) {
				continue
			}
			want.Detail, used[j] = got.Detail, true
			break
		}
	}
}

// sameDiagnostic reports whether a committed entry names a reported one, ignoring the
// detail it is about to be given.
func sameDiagnostic(want, got conformance.Error) bool {
	return want.ModuleOrDefault() == got.ModuleOrDefault() &&
		want.Code == got.Code &&
		want.Row == got.Row &&
		(want.Col == 0 || want.Col == got.Col) &&
		want.Message == got.Message
}

// fillTransform writes what the case's modules compile to, as Rego where OPA's
// printer can express it and as marshalled AST where it cannot. Whichever it
// writes, the other is removed, so a case that becomes printable — or stops being
// printable — does not end up carrying both.
func fillTransform(tc *compilecases.TestCase, node *yaml.Node) error {
	want, reasons, err := compiledWant(*tc)
	if err != nil {
		return err
	}

	tc.Want = want

	// Ahead of want_stages: the endpoint is what every consumer reads, and the
	// intermediate forms refine it.
	corpusgen.SetMapValue(node, "want", wantNode(want, reasons), "want_stages")

	return nil
}

// wantNode renders the entries, with the reason on any that fell back to the AST
// form.
func wantNode(want []compilecases.Want, reasons []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}

	for i, w := range want {
		entry := &yaml.Node{Kind: yaml.MappingNode}

		if w.AST != "" {
			corpusgen.SetMapValue(entry, "ast", corpusgen.Literal(w.AST))
			if reasons[i] != "" {
				corpusgen.SetComment(entry, "ast rather than module: "+reasons[i]+".")
			}
			seq.Content = append(seq.Content, entry)
			continue
		}

		if len(w.Imports) > 0 {
			corpusgen.SetMapValue(entry, "imports", corpusgen.StringsNode(w.Imports))
			corpusgen.SetComment(entry,
				"The compiler drops directive imports; a consumer must put them in effect.")
		}
		corpusgen.SetMapValue(entry, "module", corpusgen.Literal(w.Module))

		seq.Content = append(seq.Content, entry)
	}

	return seq
}

// fillStages writes what the modules look like at each stage the case names, and drops a
// stage whose form is the full-pipeline one again.
func fillStages(tc *compilecases.TestCase, node *yaml.Node) error {
	if len(tc.WantStages) == 0 {
		return nil
	}

	filled := make(map[string][]compilecases.Want, len(tc.WantStages))
	reasons := make(map[string][]string, len(tc.WantStages))

	for _, stage := range tc.SortedStages() {
		if compilecases.StageIndex(stage) < 0 {
			return fmt.Errorf("'want_stages' names %q, which is not a compiler stage the corpus knows", stage)
		}

		want, why, err := compiledWantAtStage(*tc, stage)
		if err != nil {
			return fmt.Errorf("want_stages.%s: %w", stage, err)
		}

		// Dropping these is the point of the field: an intermediate assertion equal to the
		// endpoint asserts nothing the endpoint does not, and carrying it would pin OPA's
		// stage decomposition — which the StageID identifiers are explicitly not stable
		// enough to bear — for no gain. What survives is the stages that do something the
		// endpoint hides. A case asserting want_errors has no endpoint form to compare
		// against, so it keeps every stage it names.
		if tc.Transform() && slices.EqualFunc(want, tc.Want, sameWant) {
			continue
		}

		filled[stage], reasons[stage] = want, why
	}

	tc.WantStages = filled

	if len(filled) == 0 {
		corpusgen.DeleteMapValue(node, "want_stages")
		return nil
	}

	corpusgen.SetMapValue(node, "want_stages", wantStagesNode(*tc, reasons))

	return nil
}

// sameWant reports whether two expectations say the same thing.
func sameWant(a, b compilecases.Want) bool {
	return a.Module == b.Module && a.AST == b.AST && slices.Equal(a.Imports, b.Imports)
}

// wantStagesNode renders the pinned stages in pipeline order. A mapping's key order
// does not survive loading, so the file's order is the generator's to choose, and
// pipeline order is the one a reader can follow.
func wantStagesNode(tc compilecases.TestCase, reasons map[string][]string) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}

	for _, stage := range tc.SortedStages() {
		corpusgen.SetMapValue(m, stage, wantNode(tc.WantStages[stage], reasons[stage]))
	}

	return m
}
