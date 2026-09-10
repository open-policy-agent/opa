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

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
	"github.com/open-policy-agent/opa/v1/util"
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
	if err := util.Unmarshal(bs, &set); err != nil {
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
			return fmt.Errorf("%s: %s: the case asserts 'want_modules', but the modules report %d diagnostic(s), starting with %s",
				path, tc.Note, len(reported), reported[0])

		case tc.Compiles && len(reported) > 0:
			return fmt.Errorf("%s: %s: the case asserts 'compiles', but the modules report %d diagnostic(s), starting with %s",
				path, tc.Note, len(reported), reported[0])

		case tc.Compiles:
			// Nothing to fill in: the assertion is that there is nothing to fill in.

		case len(reported) == 0 && !tc.Failure():
			// A clean compile with no 'compiles' assertion is a transformation
			// case. Unlike want_errors, want_modules is regenerated every time:
			// it is the compiled form, and review of the diff is the gate.
			want, err := compiledModules(*tc)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
			}
			tc.WantModules = want
			corpusgen.SetMapValue(caseNodes.Content[i], "want_modules", corpusgen.ModulesNode(want))

		case len(reported) == 0 && tc.Failure():
			return fmt.Errorf("%s: %s: the case asserts 'want_errors', but the modules compile", path, tc.Note)

		case !tc.Failure():
			// Fill in the diagnostics only where the case has none. A message
			// that changes has to fail the runner, not be quietly rewritten
			// underneath it, so an existing want_errors is never touched.
			tc.WantErrors = reported
			corpusgen.SetMapValue(caseNodes.Content[i], "want_errors", corpusgen.ErrorsNode(reported), "exhaustive")
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
