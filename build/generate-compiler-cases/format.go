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
		reason, keyword := parseFailureReason(text, err, popts)
		return "", notPrintableError{reason: reason, text: text, keyword: keyword}
	}

	if !mod.Equal(reparsed) {
		return "", notPrintableError{reason: divergenceReason(mod, reparsed), text: text}
	}

	return text, nil
}

// notPrintableError says that a compiled module has no Rego spelling that parses
// back to it. Reason is a sentence short enough to carry into a corpus comment, so
// that a reader of a want_ast fixture can see why it is not want_modules without
// reproducing the failure.
type notPrintableError struct {
	reason string
	text   string

	// keyword is the future keyword whose activation would make the text parse,
	// where one would. The caller declares it on the case rather than falling back
	// to want_ast, so this is the difference between a readable fixture and an
	// unreadable one.
	keyword string
}

func (e notPrintableError) Error() string {
	return fmt.Sprintf("%s\n--- printed as\n%s", e.reason, e.text)
}

// Keyword returns the future keyword whose activation would make the text parse,
// or "" where activating one is not what it needs.
func (e notPrintableError) Keyword() string { return e.keyword }

// Reason returns the short form.
func (e notPrintableError) Reason() string { return e.reason }

// parseFailureReason names the printed line the parser choked on, which is more
// use than the position: the positions are into text nobody has in front of them.
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

	// A future keyword whose import the compiler resolved away is the usual cause,
	// and the parse error for it points somewhere else entirely: without `or`
	// active, `{ x = 1 } or { y = 2 }` reads as an unterminated set. Say what is
	// actually missing rather than repeating a message that misleads.
	//
	// Reaching this means the keywords derived from the case's imports were not
	// enough, so it points at the derivation rather than at the printer.
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

// futureKeywordCandidates are the keywords worth trying to activate.
//
// ast.Keywords is KeywordsForRegoVersion(DefaultRegoVersion), which covers `if`,
// `contains`, `in` and `every` but not `and` or `or`: those are future keywords
// that never became standard in any version, so they appear in neither
// KeywordsV0 nor KeywordsV1 and have to be named here.
var futureKeywordCandidates = append(slices.Clone(ast.Keywords), "and", "or")

// missingFutureKeyword returns the one future keyword whose activation makes text
// parse, or "" if activating one is not what it needs.
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

// divergenceReason names the rule that survived printing but not reparsing. This
// is the harder failure to see, because the text usually looks right — `else :=`
// prints as `else =`, so only the AST differs.
func divergenceReason(mod, reparsed *ast.Module) string {
	for i, rule := range mod.Rules {
		if i >= len(reparsed.Rules) || rule.Compare(reparsed.Rules[i]) != 0 {
			return fmt.Sprintf("the compiled rule `%s` prints as Rego that parses to a different AST", rule)
		}
	}
	return "the compiled module prints as Rego that parses to a different AST"
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
