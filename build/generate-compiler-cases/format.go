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
		sb.WriteString(formatBody(body, 1))
		sb.WriteString("}")
	}
	sb.WriteString(parts[len(parts)-1])

	return sb.String()
}

// formatBody writes one expression per line at depth tabs.
func formatBody(body ast.Body, depth int) string {
	var sb strings.Builder
	tabs := strings.Repeat("\t", depth)

	for _, expr := range body {
		sb.WriteString(tabs)
		sb.WriteString(formatExpr(expr, depth))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatExpr renders one expression, breaking the operand bodies of any `and` / `or`
// group it holds onto their own lines. depth is the indentation the expression itself
// sits at, so a nested body indents one further and its closing brace lines up with
// the line that opened it.
//
// The same placeholder trick formatRule uses: everything but the nested bodies still
// comes from OPA's printer, and formatModule reparses the result, so a shape this gets
// wrong falls back to the AST form rather than reaching a fixture.
func formatExpr(expr *ast.Expr, depth int) string {
	skeleton := expr.Copy()

	slots := groupOperands(skeleton)
	if len(slots) == 0 {
		return expr.String()
	}

	bodies := make([]ast.Body, 0, len(slots))
	for _, slot := range slots {
		bodies = append(bodies, *slot.body)
		*slot.body = placeholderBody

		// The operand already printed with braces, so saying so explicitly keeps the
		// placeholder braced too and leaves one split token to look for. Compare ignores
		// the flag, and this is a copy either way.
		if slot.explicit != nil {
			*slot.explicit = true
		}
	}

	parts := strings.Split(skeleton.String(), "{ "+bodyPlaceholder+" }")
	if len(parts) != len(bodies)+1 {
		return expr.String()
	}

	var sb strings.Builder
	for i, body := range bodies {
		sb.WriteString(parts[i])
		sb.WriteString("{\n")
		sb.WriteString(formatBody(body, depth+1))
		sb.WriteString(strings.Repeat("\t", depth))
		sb.WriteString("}")
	}
	sb.WriteString(parts[len(parts)-1])

	return sb.String()
}

// groupOperand is one nested body of an expression, with the flag that decides whether
// the printer braces it. A nil flag means the construct always braces its body.
type groupOperand struct {
	body     *ast.Body
	explicit *bool
}

// braced reports whether the printer writes this operand with braces, which is the only
// case there is anything to break onto its own line.
func (o groupOperand) braced() bool {
	return o.explicit == nil || *o.explicit || len(*o.body) != 1
}

// groupOperands returns the nested bodies of expr that print with braces, in the order
// the printer renders them: left before right, descending through a chain of groups.
//
// An operand that prints inline — one expression, no braces asked for — is left alone;
// there is nothing to break onto its own line. An operand that is itself a group is
// descended into rather than treated as a slot, which is how `a and b and c` chains.
func groupOperands(expr *ast.Expr) []groupOperand {
	var out []groupOperand

	var collect func(o groupOperand)
	collect = func(o groupOperand) {
		if !o.braced() {
			if inner := nestedBodies((*o.body)[0]); len(inner) > 0 {
				for _, next := range inner {
					collect(next)
				}
			}
			return
		}
		out = append(out, o)
	}

	for _, o := range nestedBodies(expr) {
		collect(o)
	}

	return out
}

// nestedBodies returns the bodies an expression writes inside its own text, in printing
// order, or nil where it holds none.
func nestedBodies(expr *ast.Expr) []groupOperand {
	switch t := expr.Terms.(type) {
	case *ast.LogicalAnd:
		return []groupOperand{{&t.Lhs, &t.ExplicitLhs}, {&t.Rhs, &t.ExplicitRhs}}
	case *ast.LogicalOr:
		return []groupOperand{{&t.Lhs, &t.ExplicitLhs}, {&t.Rhs, &t.ExplicitRhs}}
	case *ast.Not:
		return []groupOperand{{&t.Body, &t.ExplicitBody}}
	case *ast.Every:
		// An every body is always written in braces, so there is no flag to consult.
		return []groupOperand{{body: &t.Body}}
	}
	return nil
}
