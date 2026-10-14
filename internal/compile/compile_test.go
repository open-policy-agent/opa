// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestExtractSchemaFromAnnotations(t *testing.T) {
	const module = `# METADATA
# scope: package
# schemas:
#   - input: {"type": "object", "properties": {"doc": {"type": "object", "properties": {"a": {"type": "string"}}}}}
package p

include if input.doc.a == "x"

# METADATA
# schemas:
#   - input.doc: {"type": "object", "properties": {"b": {"type": "string"}}}
closer if input.doc.b == "x"

other if input.other.a == "x"

# METADATA
# schemas:
#   - input.doc.tags: {"type": "array", "items": {"type": "string"}}
#   - input.doc.address.city: {"type": "string"}
fields if input.doc.a == "x"
`
	c := ast.NewCompiler()
	m := ast.MustParseModuleWithOpts(module, ast.ParserOptions{ProcessAnnotation: true})
	if c.Compile(map[string]*ast.Module{"p.rego": m}); c.Failed() {
		t.Fatal(c.Errors)
	}
	for _, tc := range []struct {
		rule, doc string
		exp       any
	}{
		{rule: "data.p.include", doc: "input.doc", exp: map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"type": "string"}}}},
		{rule: "data.p.closer", doc: "input.doc", exp: map[string]any{"type": "object", "properties": map[string]any{"b": map[string]any{"type": "string"}}}},
		{rule: "data.p.fields", doc: "input.doc", exp: map[string]any{"type": "object", "properties": map[string]any{
			"a":       map[string]any{"type": "string"},
			"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"address": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		}}},
		{rule: "data.p.other", doc: "input.other"},
		{rule: "data.p.missing", doc: "input.doc"},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			act := ExtractSchemaFromAnnotations(c, ast.MustParseRef(tc.rule), ast.MustParseRef(tc.doc))
			if diff := cmp.Diff(tc.exp, act); diff != "" {
				t.Errorf("unexpected schema (-want +got):\n%s", diff)
			}
		})
	}
}
