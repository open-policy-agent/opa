// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "strings"

func (c *Compiler) inlineRefAliases() {
	if len(c.Modules) == 0 {
		return
	}

	scopes := c.withScopes()
	cache := map[*Rule]map[String]inlinedAlias{}

	for _, name := range c.sorted {
		mod := c.Modules[name]
		WalkRules(mod, func(rule *Rule) bool {
			if strings.HasPrefix(string(rule.Head.Name), "test_") {
				return false
			}

			blocked := blockedForRule(scopes, rule)
			if cache[rule] == nil {
				cache[rule] = map[String]inlinedAlias{}
			}
			c.inlineAliasesInBody(rule.Body, blocked, cache[rule])
			return false
		})
	}
}

type withScope struct {
	target    Ref
	reachable map[*Rule]struct{}
}

func (c *Compiler) withScopes() []withScope {
	var scopes []withScope

	for _, name := range c.sorted {
		WalkExprs(c.Modules[name], func(expr *Expr) bool {
			if len(expr.With) == 0 {
				return false
			}
			reachable := c.rulesReachableFrom(expr)
			for _, w := range expr.With {
				ref, ok := w.Target.Value.(Ref)
				if !ok {
					continue
				}
				ground := ref.ConstantPrefix()
				if len(ground) == 0 {
					continue
				}
				scopes = append(scopes, withScope{target: ground, reachable: reachable})
			}
			return false
		})
	}

	return scopes
}

func (c *Compiler) rulesReachableFrom(expr *Expr) map[*Rule]struct{} {
	reachable := map[*Rule]struct{}{}

	var queue []*Rule
	WalkRefs(expr, func(ref Ref) bool {
		for _, rule := range c.GetRulesDynamic(ref.ConstantPrefix()) {
			if _, seen := reachable[rule]; !seen {
				reachable[rule] = struct{}{}
				queue = append(queue, rule)
			}
		}
		return false
	})

	for len(queue) > 0 {
		rule := queue[0]
		queue = queue[1:]
		for dep := range c.Graph.Dependencies(rule) {
			depRule, ok := dep.(*Rule)
			if !ok {
				continue
			}
			if _, seen := reachable[depRule]; !seen {
				reachable[depRule] = struct{}{}
				queue = append(queue, depRule)
			}
		}
	}

	return reachable
}

func blockedForRule(scopes []withScope, rule *Rule) []Ref {
	var blocked []Ref
	for _, scope := range scopes {
		if _, ok := scope.reachable[rule]; ok {
			blocked = append(blocked, scope.target)
		}
	}

	return blocked
}

type inlinedAlias struct {
	resolved Ref
	ok       bool
}

func (c *Compiler) inlineAliasesInBody(body Body, blocked []Ref, cache map[String]inlinedAlias) {
	for _, expr := range body {
		if len(expr.With) > 0 {
			continue
		}

		WalkTerms(expr, func(term *Term) bool {
			switch term.Value.(type) {
			case *ArrayComprehension, *SetComprehension, *ObjectComprehension:
				return true
			}

			ref, ok := term.Value.(Ref)
			if !ok {
				return false
			}
			if resolved, ok := c.inlinableAlias(ref, blocked, cache); ok {
				term.Value = resolved
				return true
			}
			return false
		})

		WalkClosures(expr, func(x any) bool {
			switch x := x.(type) {
			case *ArrayComprehension:
				c.inlineAliasesInBody(x.Body, blocked, cache)
			case *SetComprehension:
				c.inlineAliasesInBody(x.Body, blocked, cache)
			case *ObjectComprehension:
				c.inlineAliasesInBody(x.Body, blocked, cache)
			case *Every:
				c.inlineAliasesInBody(x.Body, blocked, cache)
			}
			return true
		})
	}
}

func (c *Compiler) inlinableAlias(ref Ref, blocked []Ref, cache map[String]inlinedAlias) (Ref, bool) {
	if !RootDocumentNames.Contains(ref[0]) || !ref.IsGround() || ref.IsNested() {
		return nil, false
	}

	if !c.isVirtual(ref) {
		return nil, false
	}

	key := String(ref.String())
	if hit, ok := cache[key]; ok {
		return hit.resolved, hit.ok
	}

	resolved, sources := c.resolveRefAlias(ref)
	ok := resolved != nil
	if ok {
		for _, src := range append(sources, resolved) {
			if refOverlapsAny(src, blocked) {
				ok = false
				break
			}
		}
	}
	if !ok {
		resolved = nil
	}

	cache[key] = inlinedAlias{resolved: resolved, ok: ok}

	return resolved, ok
}

func refOverlapsAny(ref Ref, others []Ref) bool {
	for _, other := range others {
		if ref.HasPrefix(other) || other.HasPrefix(ref) {
			return true
		}
	}

	return false
}

func (c *Compiler) resolveRefAlias(ref Ref) (Ref, []Ref) {
	var sources []Ref
	resolved := ref
	// Arbitrary depth limit, practically more than will be used
	for range 8 {
		if c.injectedVirtualRef(resolved) {
			return nil, nil
		}
		prefixLen, rules := c.ruleNodeFor(resolved)
		if rules == nil {
			return nil, nil
		}
		target := pureAliasTarget(rules)
		if target == nil {
			return nil, nil
		}
		sources = append(sources, resolved[:prefixLen])
		next := make(Ref, 0, len(target)+len(resolved)-prefixLen)
		next = append(next, target...)
		next = append(next, resolved[prefixLen:]...)
		resolved = next
		if !RootDocumentNames.Contains(resolved[0]) || !resolved.IsGround() || resolved.IsNested() {
			return nil, nil
		}
		if !c.isVirtual(resolved) {
			return resolved, sources
		}
	}

	return nil, nil
}

func (c *Compiler) injectedVirtualRef(ref Ref) bool {
	return c.injectedVirtual != nil && c.injectedVirtual(ref)
}

func (c *Compiler) ruleNodeFor(ref Ref) (int, []*Rule) {
	node := c.RuleTree
	for i := range ref {
		child := node.Child(ref[i].Value)
		if child == nil || child.External != nil {
			return 0, nil
		}
		if c.injectedVirtualRef(ref[:i+1]) {
			return 0, nil
		}
		if len(child.Values) > 0 {
			return i + 1, child.Values
		}
		node = child
	}
	return 0, nil
}

func pureAliasTarget(rules []*Rule) Ref {
	if len(rules) != 1 {
		return nil
	}

	rule := rules[0]
	if rule.Default || rule.Else != nil || len(rule.Head.Args) > 0 ||
		rule.Head.RuleKind() != SingleValue || rule.Head.Value == nil ||
		!rule.Head.Reference.IsGround() {
		return nil
	}

	v, ok := rule.Head.Value.Value.(Var)
	if !ok || len(rule.Body) != 1 {
		return nil
	}

	expr := rule.Body[0]
	if expr.Negated || len(expr.With) > 0 || !expr.IsEquality() {
		return nil
	}

	a, b := expr.Operand(0), expr.Operand(1)
	if av, ok := a.Value.(Var); ok && av.Equal(v) {
		if ref, ok := b.Value.(Ref); ok {
			return groundUnnestedRootRef(ref)
		}
	}
	if bv, ok := b.Value.(Var); ok && bv.Equal(v) {
		if ref, ok := a.Value.(Ref); ok {
			return groundUnnestedRootRef(ref)
		}
	}

	return nil
}

func groundUnnestedRootRef(ref Ref) Ref {
	if RootDocumentNames.Contains(ref[0]) && ref.IsGround() && !ref.IsNested() {
		return ref
	}
	return nil
}
