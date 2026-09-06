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
	"slices"

	"go.yaml.in/yaml/v3"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
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
	caseNodes := mapValue(root, "cases")
	if caseNodes == nil || len(caseNodes.Content) != len(set.Cases) {
		return fmt.Errorf("%s: expected a 'cases' sequence of %d entries", path, len(set.Cases))
	}

	for i := range set.Cases {
		tc := &set.Cases[i]
		*tc = tc.WithFilename(path)

		if !tc.Failure() {
			astJSON.SetOptions(parsercases.MarshalOptions(tc.Locations, false))

			module, err := parseModule(*tc, tc.Module)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
			}

			if tc.WantAST, err = MarshalAST(module); err != nil {
				return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
			}

			setMapValue(caseNodes.Content[i], "want_ast", literal(tc.WantAST), "want_equivalent")
		}

		if err := tc.Validate(); err != nil {
			return fmt.Errorf("%s: %s: %w", path, tc.Note, err)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if bytes.Equal(buf.Bytes(), bs) {
		return nil
	}

	return os.WriteFile(path, buf.Bytes(), mode)
}

func literal(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.LiteralStyle, Value: s}
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if i := keyIndex(n, key); i >= 0 {
		return n.Content[i+1]
	}
	return nil
}

// setMapValue sets key on the mapping n, inserting it ahead of the first of
// before that is present when it is not already there.
func setMapValue(n *yaml.Node, key string, value *yaml.Node, before ...string) {
	if i := keyIndex(n, key); i >= 0 {
		n.Content[i+1] = value
		return
	}

	at := len(n.Content)
	for _, b := range before {
		if i := keyIndex(n, b); i >= 0 && i < at {
			at = i
		}
	}

	n.Content = slices.Insert(n.Content, at, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func keyIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}
