package ast

import (
	"bytes"
	"strconv"
	"strings"
)

type grammarOptimizer struct {
	rule            string
	protectedRules  map[string]struct{}
	rules           map[string]*Rule
	ruleUsesRules   map[string]map[string]struct{}
	ruleUsedByRules map[string]map[string]struct{}
	visitor         func(expr Expression) Visitor
	optimized       bool
}

func newGrammarOptimizer(protectedRules []string) *grammarOptimizer {
	pr := make(map[string]struct{}, len(protectedRules))
	for _, nm := range protectedRules {
		pr[nm] = struct{}{}
	}

	r := grammarOptimizer{
		protectedRules:  pr,
		rules:           make(map[string]*Rule),
		ruleUsesRules:   make(map[string]map[string]struct{}),
		ruleUsedByRules: make(map[string]map[string]struct{}),
	}
	r.visitor = r.init
	return &r
}

// Visit is a generic Visitor to be used with Walk
// The actual function, which should be used during Walk
// is held in ruleRefOptimizer.visitor
func (r *grammarOptimizer) Visit(expr Expression) Visitor {
	return r.visitor(expr)
}

// init is a Visitor, which is used with the Walk function
// The purpose of this function is to initialize the reference
// maps rules, ruleUsesRules and ruleUsedByRules.
func (r *grammarOptimizer) init(expr Expression) Visitor {
	switch expr := expr.(type) {
	case *Rule:
		// Keep track of current rule, which is processed
		r.rule = expr.Name.Val
		r.rules[expr.Name.Val] = expr
	case *RuleRefExpr:
		// Fill ruleUsesRules and ruleUsedByRules for every RuleRefExpr
		set(r.ruleUsesRules, r.rule, expr.Name.Val)
		set(r.ruleUsedByRules, expr.Name.Val, r.rule)
	}
	return r
}

// Add element to map of maps, initialize the inner map
// if necessary.
func set(m map[string]map[string]struct{}, src, dst string) {
	if _, ok := m[src]; !ok {
		m[src] = make(map[string]struct{})
	}
	m[src][dst] = struct{}{}
}

// optimize is a Visitor, which is used with the Walk function
// The purpose of this function is to perform the actual optimizations.
// See Optimize for a detailed list of the performed optimizations.
func (r *grammarOptimizer) optimize(expr0 Expression) Visitor {
	switch expr := expr0.(type) {
	case *ActionExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *AndExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *ChoiceExpr:
		expr.Alternatives = r.optimizeRules(expr.Alternatives)

		// Optimize choice nested in choice
		for i := 0; i < len(expr.Alternatives); i++ {
			if choice, ok := expr.Alternatives[i].(*ChoiceExpr); ok {
				r.optimized = true
				if i+1 < len(expr.Alternatives) {
					expr.Alternatives = append(expr.Alternatives[:i], append(choice.Alternatives, expr.Alternatives[i+1:]...)...)
				} else {
					expr.Alternatives = append(expr.Alternatives[:i], choice.Alternatives...)
				}
			}

			// Combine sequence of single char LitMatcher to CharClassMatcher
			if i > 0 {
				l0, lok0 := expr.Alternatives[i-1].(*LitMatcher)
				l1, lok1 := expr.Alternatives[i].(*LitMatcher)
				c0, cok0 := expr.Alternatives[i-1].(*CharClassMatcher)
				c1, cok1 := expr.Alternatives[i].(*CharClassMatcher)

				combined := false

				switch {
				// Combine two LitMatcher to CharClassMatcher
				// "a" / "b" => [ab]
				case lok0 && lok1 && len([]rune(l0.Val)) == 1 && len([]rune(l1.Val)) == 1 && l0.IgnoreCase == l1.IgnoreCase:
					combined = true
					cm := CharClassMatcher{
						Chars:      append([]rune(l0.Val), []rune(l1.Val)...),
						IgnoreCase: l0.IgnoreCase,
						posValue:   l0.posValue,
					}
					expr.Alternatives[i-1] = &cm

				// Combine LitMatcher with CharClassMatcher
				// "a" / [bc] => [abc]
				case lok0 && cok1 && len([]rune(l0.Val)) == 1 && l0.IgnoreCase == c1.IgnoreCase && !c1.Inverted:
					combined = true
					c1.Chars = append(c1.Chars, []rune(l0.Val)...)
					expr.Alternatives[i-1] = c1

				// Combine CharClassMatcher with LitMatcher
				// [ab] / "c" => [abc]
				case cok0 && lok1 && len([]rune(l1.Val)) == 1 && c0.IgnoreCase == l1.IgnoreCase && !c0.Inverted:
					combined = true
					c0.Chars = append(c0.Chars, []rune(l1.Val)...)

				// Combine CharClassMatcher with CharClassMatcher
				// [ab] / [cd] => [abcd]
				case cok0 && cok1 && c0.IgnoreCase == c1.IgnoreCase && c0.Inverted == c1.Inverted:
					combined = true
					c0.Chars = append(c0.Chars, c1.Chars...)
					c0.Ranges = append(c0.Ranges, c1.Ranges...)
					c0.UnicodeClasses = append(c0.UnicodeClasses, c1.UnicodeClasses...)
				}

				// If one of the optimizations was applied, remove the second element from Alternatives
				if combined {
					r.optimized = true
					if i+1 < len(expr.Alternatives) {
						expr.Alternatives = append(expr.Alternatives[:i], expr.Alternatives[i+1:]...)
					} else {
						expr.Alternatives = expr.Alternatives[:i]
					}
				}
			}
		}

	case *Grammar:
		// Reset optimized at the start of each Walk.
		r.optimized = false
		for i := 0; i < len(expr.Rules); i++ {
			rule := expr.Rules[i]
			// Remove Rule, if it is no longer used by any other Rule and it is not the first Rule.
			_, used := r.ruleUsedByRules[rule.Name.Val]
			_, protected := r.protectedRules[rule.Name.Val]
			if !used && !protected {
				expr.Rules = append(expr.Rules[:i], expr.Rules[i+1:]...)
				r.optimized = true
				continue
			}
		}
	case *LabeledExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *NotExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *OneOrMoreExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *Rule:
		r.rule = expr.Name.Val
	case *SeqExpr:
		expr.Exprs = r.optimizeRules(expr.Exprs)

		for i := 0; i < len(expr.Exprs); i++ {
			// Optimize nested sequences
			if seq, ok := expr.Exprs[i].(*SeqExpr); ok {
				r.optimized = true
				if i+1 < len(expr.Exprs) {
					expr.Exprs = append(expr.Exprs[:i], append(seq.Exprs, expr.Exprs[i+1:]...)...)
				} else {
					expr.Exprs = append(expr.Exprs[:i], seq.Exprs...)
				}
			}

			// Combine sequence of LitMatcher
			if i > 0 {
				l0, ok0 := expr.Exprs[i-1].(*LitMatcher)
				l1, ok1 := expr.Exprs[i].(*LitMatcher)
				if ok0 && ok1 && l0.IgnoreCase == l1.IgnoreCase {
					r.optimized = true
					l0.Val += l1.Val
					expr.Exprs[i-1] = l0
					if i+1 < len(expr.Exprs) {
						expr.Exprs = append(expr.Exprs[:i], expr.Exprs[i+1:]...)
					} else {
						expr.Exprs = expr.Exprs[:i]
					}
				}
			}
		}

	case *ZeroOrMoreExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	case *ZeroOrOneExpr:
		expr.Expr = r.optimizeRule(expr.Expr)
	}
	return r
}

func (r *grammarOptimizer) optimizeRules(exprs []Expression) []Expression {
	for i := 0; i < len(exprs); i++ {
		exprs[i] = r.optimizeRule(exprs[i])
	}
	return exprs
}

func (r *grammarOptimizer) optimizeRule(expr Expression) Expression {
	// Optimize RuleRefExpr
	if ruleRef, ok := expr.(*RuleRefExpr); ok {
		if _, ok := r.ruleUsesRules[ruleRef.Name.Val]; !ok {
			r.optimized = true
			delete(r.ruleUsedByRules[ruleRef.Name.Val], r.rule)
			if len(r.ruleUsedByRules[ruleRef.Name.Val]) == 0 {
				delete(r.ruleUsedByRules, ruleRef.Name.Val)
			}
			delete(r.ruleUsesRules[r.rule], ruleRef.Name.Val)
			if len(r.ruleUsesRules[r.rule]) == 0 {
				delete(r.ruleUsesRules, r.rule)
			}
			// TODO: Check if reference exists, otherwise raise an error, which reference is missing!
			return cloneExpr(r.rules[ruleRef.Name.Val].Expr)
		}
	}

	// Remove Choices with only one Alternative left
	if choice, ok := expr.(*ChoiceExpr); ok {
		if len(choice.Alternatives) == 1 {
			r.optimized = true
			return choice.Alternatives[0]
		}
	}

	// Remove Sequence with only one Expression
	if seq, ok := expr.(*SeqExpr); ok {
		if len(seq.Exprs) == 1 {
			r.optimized = true
			return seq.Exprs[0]
		}
	}

	return expr
}

// cloneExpr takes an Expression and deep clones it (including all children)
// This is necessary because referenced Rules are denormalized and therefore
// have to become independent from their original Expression
func cloneExpr(expr Expression) Expression {
	switch expr := expr.(type) {
	case *ActionExpr:
		return &ActionExpr{
			Code:   expr.Code,
			Expr:   cloneExpr(expr.Expr),
			FuncIx: expr.FuncIx,
			p:      expr.p,
		}
	case *AndExpr:
		return &AndExpr{
			Expr: cloneExpr(expr.Expr),
			p:    expr.p,
		}
	case *AndCodeExpr:
		return &AndCodeExpr{
			Code:   expr.Code,
			FuncIx: expr.FuncIx,
			p:      expr.p,
		}
	case *CharClassMatcher:
		return &CharClassMatcher{
			Chars:          append([]rune{}, expr.Chars...),
			IgnoreCase:     expr.IgnoreCase,
			Inverted:       expr.Inverted,
			posValue:       expr.posValue,
			Ranges:         append([]rune{}, expr.Ranges...),
			UnicodeClasses: append([]string{}, expr.UnicodeClasses...),
		}
	case *ChoiceExpr:
		alts := make([]Expression, 0, len(expr.Alternatives))
		for i := 0; i < len(expr.Alternatives); i++ {
			alts = append(alts, cloneExpr(expr.Alternatives[i]))
		}
		return &ChoiceExpr{
			Alternatives: alts,
			p:            expr.p,
		}
	case *LabeledExpr:
		return &LabeledExpr{
			Expr:  cloneExpr(expr.Expr),
			Label: expr.Label,
			p:     expr.p,
		}
	case *NotExpr:
		return &NotExpr{
			Expr: cloneExpr(expr.Expr),
			p:    expr.p,
		}
	case *NotCodeExpr:
		return &NotCodeExpr{
			Code:   expr.Code,
			FuncIx: expr.FuncIx,
			p:      expr.p,
		}
	case *OneOrMoreExpr:
		return &OneOrMoreExpr{
			Expr: cloneExpr(expr.Expr),
			p:    expr.p,
		}
	case *SeqExpr:
		exprs := make([]Expression, 0, len(expr.Exprs))
		for i := 0; i < len(expr.Exprs); i++ {
			exprs = append(exprs, cloneExpr(expr.Exprs[i]))
		}
		return &SeqExpr{
			Exprs: exprs,
			p:     expr.p,
		}
	case *StateCodeExpr:
		return &StateCodeExpr{
			p:      expr.p,
			Code:   expr.Code,
			FuncIx: expr.FuncIx,
		}
	case *ZeroOrMoreExpr:
		return &ZeroOrMoreExpr{
			Expr: cloneExpr(expr.Expr),
			p:    expr.p,
		}
	case *ZeroOrOneExpr:
		return &ZeroOrOneExpr{
			Expr: expr.Expr,
			p:    expr.p,
		}
	}
	return expr
}

// cleanupCharClassMatcher is a Visitor, which is used with the Walk function
// The purpose of this function is to cleanup the redundancies created by the
// optimize Visitor. This includes to remove redundant entries in Chars, Ranges
// and UnicodeClasses of the given CharClassMatcher as well as regenerating the
// correct content for the Val field (string representation of the CharClassMatcher)
func (r *grammarOptimizer) cleanupCharClassMatcher(expr0 Expression) Visitor {
	// We are only interested in nodes of type *CharClassMatcher
	if chr, ok := expr0.(*CharClassMatcher); ok {
		// Remove redundancies in Chars
		chars := make([]rune, 0, len(chr.Chars))
		charsMap := make(map[rune]struct{})
		for _, c := range chr.Chars {
			if _, ok := charsMap[c]; !ok {
				charsMap[c] = struct{}{}
				chars = append(chars, c)
			}
		}
		if len(chars) > 0 {
			chr.Chars = chars
		} else {
			chr.Chars = nil
		}

		// Remove redundancies in Ranges
		ranges := make([]rune, 0, len(chr.Ranges))
		rangesMap := make(map[string]struct{})
		for i := 0; i < len(chr.Ranges); i += 2 {
			rangeKey := string(chr.Ranges[i]) + "-" + string(chr.Ranges[i+1])
			if _, ok := rangesMap[rangeKey]; !ok {
				rangesMap[rangeKey] = struct{}{}
				ranges = append(ranges, chr.Ranges[i], chr.Ranges[i+1])
			}
		}
		if len(ranges) > 0 {
			chr.Ranges = ranges
		} else {
			chr.Ranges = nil
		}

		// Remove redundancies in UnicodeClasses
		unicodeClasses := make([]string, 0, len(chr.UnicodeClasses))
		unicodeClassesMap := make(map[string]struct{})
		for _, u := range chr.UnicodeClasses {
			if _, ok := unicodeClassesMap[u]; !ok {
				unicodeClassesMap[u] = struct{}{}
				unicodeClasses = append(unicodeClasses, u)
			}
		}
		if len(unicodeClasses) > 0 {
			chr.UnicodeClasses = unicodeClasses
		} else {
			chr.UnicodeClasses = nil
		}

		// Regenerate the content for Val
		var val bytes.Buffer
		val.WriteString("[")
		if chr.Inverted {
			val.WriteString("^")
		}
		for _, c := range chr.Chars {
			val.WriteString(escapeRune(c))
		}
		for i := 0; i < len(chr.Ranges); i += 2 {
			val.WriteString(escapeRune(chr.Ranges[i]))
			val.WriteString("-")
			val.WriteString(escapeRune(chr.Ranges[i+1]))
		}
		for _, u := range chr.UnicodeClasses {
			val.WriteString("\\p" + u)
		}
		val.WriteString("]")
		if chr.IgnoreCase {
			val.WriteString("i")
		}
		chr.posValue.Val = val.String()
	}
	return r
}

func escapeRune(r rune) string {
	return strings.Trim(strconv.QuoteRune(r), `'`)
}

// Optimize walks a given grammar and optimizes the grammar in regards
// of parsing performance. This is done with several optimizations:
// * removal of unreferenced rules
// * replace rule references with a copy of the referenced Rule, if the
// 	 referenced rule it self has no references.
// * resolve nested choice expressions
// * resolve choice expressions with only one alternative
// * resolve nested sequences expression
// * resolve sequence expressions with only one element
// * combine character class matcher and literal matcher, where possible
func Optimize(g *Grammar, alternateEntrypoints ...string) {
	entrypoints := alternateEntrypoints
	if len(g.Rules) > 0 {
		entrypoints = append(entrypoints, g.Rules[0].Name.Val)
	}

	r := newGrammarOptimizer(entrypoints)
	Walk(r, g)

	r.visitor = r.optimize
	r.optimized = true
	for r.optimized {
		Walk(r, g)
	}

	r.visitor = r.cleanupCharClassMatcher
	Walk(r, g)
}
