// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// formatModule renders a compiled module as a want_modules fixture: one
// expression per line, which is how the Rego was written before the compiler got
// to it and how a reader can follow what changed.
//
// The result is parsed back under popts and compared to the module it came from,
// and an error is returned if they differ. A fixture that does not parse to the
// AST the compiler produced would assert something no implementation should have to
// reproduce, so the layout is only allowed to change the layout.
//
// It is deliberately not v1/format. That formatter normalises as it prints — `p =
// x` becomes `p := x`, wildcards are rewritten — so its output parses to a
// different AST than the one it was given, which is the one thing a fixture must
// not do. Only two-thirds of modules survive it.
//
// What this does instead is take OPA's own one-line rendering and break the rule
// bodies. Every token still comes from `Rule.String()`, so nothing here has to
// know about Rego syntax, or about the differences between v0 and v1 — the head of
// a v0 partial set is `p[x]` and of a v1 one is `p contains x`, and neither spelling
// appears in this file.
func formatModule(mod *ast.Module, popts ast.ParserOptions) (string, error) {
	text := layout(mod)

	// The filename plays no part: Module.Compare looks at the package, the imports,
	// the annotations and the rules, none of which carry it.
	reparsed, err := ast.ParseModuleWithOpts("formatted.rego", text, popts)
	if err != nil {
		return "", fmt.Errorf("the formatted module does not parse:\n%s\n%w", text, err)
	}

	if !mod.Equal(reparsed) {
		return "", fmt.Errorf("the formatted module parses to a different AST:\n--- formatted\n%s\n--- reparsed\n%v",
			text, reparsed)
	}

	return text, nil
}

// layout does the rendering. It is separate from the check so that the check
// cannot accidentally be the thing under test.
func layout(mod *ast.Module) string {
	var sb strings.Builder

	sb.WriteString(mod.Package.String())
	sb.WriteString("\n")

	for _, imp := range mod.Imports {
		sb.WriteString("\n")
		sb.WriteString(imp.String())
	}
	if len(mod.Imports) > 0 {
		sb.WriteString("\n")
	}

	for _, rule := range mod.Rules {
		sb.WriteString("\n")
		sb.WriteString(formatRule(rule))
		sb.WriteString("\n")
	}

	return sb.String()
}

// bodyPlaceholder stands in for a rule body while the rule is rendered, so that
// the surrounding text — annotations, `default`, the head, `if`, `else = 2 if`,
// the braces — comes from OPA's printer rather than from here. It is a var so that
// it cannot appear in a module by accident.
const bodyPlaceholder = "__corpus_body__"

var placeholderBody = ast.NewBody(ast.NewExpr(ast.VarTerm(bodyPlaceholder)))

// formatRule breaks the bodies of a rule and its else chain onto their own lines.
// Where the shape is not one it recognises it returns the one-line form, which is
// always correct if less readable.
func formatRule(rule *ast.Rule) string {
	var bodies []ast.Body

	skeleton := rule.Copy()
	for orig, cpy := rule, skeleton; orig != nil; orig, cpy = orig.Else, cpy.Else {
		bodies = append(bodies, orig.Body)
		cpy.Body = placeholderBody
	}

	// Splitting on the rendered placeholder leaves the text around each body, so
	// the pieces interleave with the bodies they belong to.
	parts := strings.Split(skeleton.String(), "{ "+bodyPlaceholder+" }")
	if len(parts) != len(bodies)+1 {
		// A default rule renders no body at all, and anything else unexpected is
		// better left alone than guessed at.
		return rule.String()
	}

	var sb strings.Builder
	for i, body := range bodies {
		sb.WriteString(parts[i])
		sb.WriteString("{\n")
		for _, expr := range body {
			sb.WriteString("\t")
			sb.WriteString(expr.String())
			sb.WriteString("\n")
		}
		sb.WriteString("}")
	}
	sb.WriteString(parts[len(parts)-1])

	return sb.String()
}
