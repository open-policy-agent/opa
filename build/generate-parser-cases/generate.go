// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/build/internal/corpusgen"
	"github.com/open-policy-agent/opa/v1/ast"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/test/conformance"
	"github.com/open-policy-agent/opa/v1/test/parsercases"
	"github.com/open-policy-agent/opa/v1/util"
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

		return generateFile(path, info.Mode())
	})
}

func generateFile(path string, mode fs.FileMode) error {
	bs, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var set parsercases.Set
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

		astJSON.SetOptions(conformance.MarshalOptions(tc.Locations, false))
		module, perr := parseModule(*tc, tc.Module)

		switch {
		case perr != nil && tc.WantAST != "":
			return fmt.Errorf("%s: %s: the case asserts 'want_ast', but the module no longer parses: %w", path, tc.Note, perr)

		case perr == nil && tc.Failure():
			return fmt.Errorf("%s: %s: the case asserts 'want_errors', but the module parses", path, tc.Note)

		case perr != nil && !tc.Failure():
			// Fill in the diagnostic only where the case has none. A message
			// that changes has to fail the runner, not be quietly rewritten
			// underneath it, so an existing want_errors is never touched.
			tc.WantErrors = []parsercases.Error{firstDiagnostic(perr)}
			corpusgen.SetMapValue(caseNodes.Content[i], "want_errors", corpusgen.ErrorsNode(tc.WantErrors), "exhaustive")

		case perr == nil:
			var err error
			if tc.WantAST, err = MarshalAST(module); err != nil {
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

// firstDiagnostic returns the diagnostic a fixture records. Only the first is
// taken: the ones that follow are usually a cascade of the same mistake, and
// holding another implementation to OPA's cascade is not a language rule.
func firstDiagnostic(err error) parsercases.Error {
	errs, ok := err.(ast.Errors)
	if !ok || len(errs) == 0 {
		return parsercases.Error{Message: err.Error()}
	}

	e := errs[0]
	out := parsercases.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	return out
}
