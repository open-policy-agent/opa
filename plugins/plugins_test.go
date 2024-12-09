// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package plugins

import (
	"testing"

	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestNew_DefaultRegoVersion(t *testing.T) {
	popts := ast.ParserOptions{
		Capabilities: &ast.Capabilities{
			Features: []string{
				"my_custom_feature",
			},
		},
		ProcessAnnotation: true,
		AllFutureKeywords: true,
		FutureKeywords:    []string{"foo", "bar"},
	}
	m, err := New([]byte(`{"plugins": {"someplugin": {}}}`), "test", inmem.New(),
		WithParserOptions(popts))
	if err != nil {
		t.Fatal(err)
	}

	if exp, act := ast.RegoV0, m.ParserOptions().RegoVersion; exp != act {
		t.Fatalf("Expected default Rego version to be %v but got %v", exp, act)
	}

	// Check a couple of other options to make sure they haven't changed
	if exp, act := popts.ProcessAnnotation, m.ParserOptions().ProcessAnnotation; exp != act {
		t.Fatalf("Expected ProcessAnnotation to be %v but got %v", exp, act)
	}

	if exp, act := popts.AllFutureKeywords, m.ParserOptions().AllFutureKeywords; exp != act {
		t.Fatalf("Expected AllFutureKeywords to be %v but got %v", exp, act)
	}

	if exp, act := popts.Capabilities, m.ParserOptions().Capabilities; exp != act {
		t.Fatalf("Expected Capabilities to be %v but got %v", exp, act)
	}
}
