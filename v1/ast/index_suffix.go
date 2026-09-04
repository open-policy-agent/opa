// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// The suffix statements index exactly as the prefix ones do -- see
// index_prefix.go, which holds the trie both of them use. The only difference
// is which end of the value the base strings are anchored to, and a suffix of a
// string is a prefix of that string reversed, so recording them reversed is the
// whole of it (see trieNode.InsertSuffix).

// updateEndsWith indexes `endswith(x, "base")`: x has to be a string ending
// with base for the rule to hold.
func (i *refindices) updateEndsWith(rule *Rule, expr *Expr, constants map[Var]Value) {
	ref := i.resolveAndValidateRef(rule, rule.Head.Args, expr.Operand(0))
	if ref == nil {
		return
	}

	suffix, ok := constantString(expr.Operand(1), constants)
	if !ok {
		return
	}

	i.insert(rule, &refindex{Ref: ref, Value: suffix, Affix: affixSuffix})
}

// updateAnySuffixMatch indexes `strings.any_suffix_match(x, base)`, the
// disjunction of endswith calls, the way updateAnyPrefixMatch does for
// strings.any_prefix_match.
func (i *refindices) updateAnySuffixMatch(rule *Rule, expr *Expr, constants map[Var]Value) {
	ref := i.resolveAndValidateRef(rule, rule.Head.Args, expr.Operand(0))
	if ref == nil {
		return
	}

	base := expr.Operand(1).Value
	if v, ok := base.(Var); ok {
		resolved, ok := constants[v]
		if !ok {
			return
		}
		base = resolved
	}

	if s, ok := base.(String); ok {
		i.insert(rule, &refindex{Ref: ref, Value: s, Affix: affixSuffix})
		return
	}

	// As for prefixes: a base that is not ground strings throughout leaves the
	// rule unindexed rather than partly indexed, since dropping one would
	// exclude rules it would have matched.
	suffixes, ok := groundStrings(base)
	if !ok || len(suffixes) == 0 {
		return
	}

	i.insertAffixes(rule, ref, suffixes, affixSuffix)
}
