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

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Generate fills in want_errors for every case in the corpus rooted at dir,
// rewriting the YAML files in place. The fixture is what OPA's compiler
// produces, so it is a golden file: it does not independently validate OPA, it
// catches unreviewed change. The gate is review of the regeneration diff.
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

		return generateFile(path, info.Mode())
	})
}

func generateFile(path string, mode fs.FileMode) error {
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
		*tc = tc.WithFilename(path)

		reported, err := caseDiagnostics(*tc)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
		}

		switch {
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
			// that changes has to fail the runner, not be quietly rewritten
			// underneath it, so an existing want_errors is never touched.
			if !tc.Failure() {
				tc.WantErrors = reported
				corpusgen.SetMapValue(caseNodes.Content[i], "want_errors", corpusgen.ErrorsNode(reported), "exhaustive")
			}
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

// fillStages writes what the modules look like at each stage the case names, and
// drops a stage whose form is the full-pipeline one again.
//
// The drop is the point of the field. An intermediate assertion equal to the
// endpoint asserts nothing the endpoint does not, and carrying it would pin OPA's
// stage decomposition — which the StageID identifiers are explicitly not stable
// enough to bear — for no gain. What survives is the set of stages that do
// something the endpoint hides.
//
// A case asserting want_errors keeps every stage it names: with no full-pipeline
// form to compare against, the intermediate one is the only form it has.
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
