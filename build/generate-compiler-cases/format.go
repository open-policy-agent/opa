// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cases

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// formatModule renders a compiled module one expression per line, and returns an
// error if the result does not parse back to the module it came from.
//
// Not v1/format: that normalises as it prints, so its output parses to a different
// AST than it was given, which is the one thing a fixture must not do. This breaks
// the bodies of OPA's own one-line rendering instead, so every token — and every
// v0/v1 spelling — still comes from Rule.String().
func formatModule(mod *ast.Module, popts ast.ParserOptions) (string, error) {
	text := layout(mod)

	// The filename plays no part: Module.Compare looks at the package, the imports,
	// the annotations and the rules, none of which carry it.
	reparsed, err := ast.ParseModuleWithOpts("formatted.rego", text, popts)
	if err != nil {
		reason, keyword := parseFailureReason(text, err, popts)
		return "", notPrintableError{reason: reason, text: text, keyword: keyword}
	}

	if !mod.Equal(reparsed) {
		return "", notPrintableError{reason: divergenceReason(mod, reparsed), text: text}
	}

	return text, nil
}

// notPrintableError says a compiled module has no Rego spelling that parses back to
// it. Reason is short enough to carry into a corpus comment.
type notPrintableError struct {
	reason  string
	text    string
	keyword string // a future keyword whose activation would fix it, if any
}

func (e notPrintableError) Error() string {
	return fmt.Sprintf("%s\n--- printed as\n%s", e.reason, e.text)
}

// Keyword returns the future keyword whose activation would make the text parse,
// or "" where activating one is not what it needs.
func (e notPrintableError) Keyword() string { return e.keyword }

// Reason returns the short form.
func (e notPrintableError) Reason() string { return e.reason }

// parseFailureReason names the printed line the parser choked on, which is more use
// than a position into text nobody has in front of them.
func parseFailureReason(text string, err error, popts ast.ParserOptions) (string, string) {
	message, line := "does not parse", ""

	if errs, ok := errors.AsType[ast.Errors](err); ok && len(errs) > 0 {
		message = errs[0].Message
		if loc := errs[0].Location; loc != nil {
			if lines := strings.Split(text, "\n"); loc.Row >= 1 && loc.Row <= len(lines) {
				line = strings.TrimSpace(lines[loc.Row-1])
			}
		}
	}

	// The usual cause, and the parse error for it misleads: without `or` active,
	// `{ x = 1 } or { y = 2 }` reads as an unterminated set. Reaching this means the
	// case's own imports were not enough, so it points at the derivation.
	if kw := missingFutureKeyword(text, popts); kw != "" {
		where := "the compiled module"
		if line != "" {
			where = fmt.Sprintf("`%s`", line)
		}
		return fmt.Sprintf("%s needs `import future.keywords.%s`, which the compiler resolves away "+
			"and the printer does not put back", where, kw), kw
	}

	if line == "" {
		return "the compiled module prints as Rego that does not parse: " + message, ""
	}
	return fmt.Sprintf("the compiled module prints as `%s`, which does not parse: %s", line, message), ""
}

// futureKeywordCandidates are the keywords worth trying to activate. ast.Keywords
// omits `and` and `or`, which never became standard in any version.
var futureKeywordCandidates = append(slices.Clone(ast.Keywords), "and", "or")

// missingFutureKeyword returns the future keyword whose activation makes text parse,
// or "" if that is not what it needs.
func missingFutureKeyword(text string, popts ast.ParserOptions) string {
	for _, kw := range futureKeywordCandidates {
		with := popts
		with.FutureKeywords = append(slices.Clone(popts.FutureKeywords), kw)

		if _, err := ast.ParseModuleWithOpts("formatted.rego", text, with); err == nil {
			return kw
		}
	}
	return ""
}

// divergenceReason names the rule that survived printing but not reparsing — the
// harder failure to see, since the text usually looks right.
func divergenceReason(mod, reparsed *ast.Module) string {
	for i, rule := range mod.Rules {
		if i >= len(reparsed.Rules) || rule.Compare(reparsed.Rules[i]) != 0 {
			return fmt.Sprintf("the compiled rule `%s` prints as Rego that parses to a different AST", rule)
		}
	}
	return "the compiled module prints as Rego that parses to a different AST"
}

// layout does the rendering, kept separate so the check is not the thing under test.
func layout(mod *ast.Module) string {
	var sb strings.Builder

	// Package-scoped annotations sit above the package clause, where
	// Module.AppendText puts them. Rule-scoped ones come out with their rule.
	for _, a := range mod.Annotations {
		if a.Scope != "package" && a.Scope != "subpackages" {
			continue
		}
		sb.WriteString("# METADATA\n# ")
		sb.WriteString(a.String())
		sb.WriteString("\n")
	}

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
// everything around it — annotations, `default`, the head, `if`, `else = 2 if`, the
// braces — comes from OPA's printer.
const bodyPlaceholder = "__corpus_body__"

var placeholderBody = ast.NewBody(ast.NewExpr(ast.VarTerm(bodyPlaceholder)))

// formatRule breaks the bodies of a rule and its else chain onto their own lines,
// falling back to the one-line form for a shape it does not recognise.
func formatRule(rule *ast.Rule) string {
	var bodies []ast.Body

	skeleton := rule.Copy()
	for orig, cpy := rule, skeleton; orig != nil; orig, cpy = orig.Else, cpy.Else {
		bodies = append(bodies, orig.Body)
		cpy.Body = placeholderBody
	}

	// The pieces between the rendered placeholders interleave with the real bodies.
	parts := strings.Split(skeleton.String(), "{ "+bodyPlaceholder+" }")
	if len(parts) != len(bodies)+1 {
		// A default rule renders no body at all.
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
