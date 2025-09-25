// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/open-policy-agent/opa/internal/levenshtein"
	"github.com/open-policy-agent/opa/internal/ucast"
	"github.com/open-policy-agent/opa/v1/ast"
)

const (
	invalidUnknownCode = "invalid_unknown"
)

type UCASTNode struct {
	internal *ucast.UCASTNode
}

func (u *UCASTNode) Map() map[string]any {
	if u.internal == nil { // unconditional YES
		return map[string]any{}
	}

	// TODO(sr): find a better way
	ret := map[string]any{}
	bs, err := json.Marshal(u.internal)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(bs, &ret); err != nil {
		panic(err)
	}
	return ret
}

func QueriesToUCAST(queries []ast.Body, mappings map[string]any) *UCASTNode {
	return &UCASTNode{internal: BodiesToUCAST(queries, &Opts{Translations: mappings})}
}

func QueriesToSQL(queries []ast.Body, mappings map[string]any, dialect string) (string, error) {
	sql := ""
	ucast := BodiesToUCAST(queries, &Opts{Translations: mappings})
	if ucast != nil { // ucast == nil means unconditional YES, for which we'll keep `sql = ""`
		sql0, err := ucast.AsSQL(dialect)
		if err != nil {
			return "", err
		}
		sql = sql0
	}

	return sql, nil
}

func ExtractUnknownsFromAnnotations(comp *ast.Compiler, ref ast.Ref) ([]ast.Ref, []*ast.Error) {
	// find ast.Rule for ref
	rules := comp.GetRulesExact(ref)
	if len(rules) == 0 {
		return nil, nil
	}
	rule := rules[0] // rule scope doesn't make sense here, so it doesn't matter which rule we use
	return unknownsFromAnnotationsSet(comp.GetAnnotationSet(), rule)
}

func unknownsFromAnnotationsSet(as *ast.AnnotationSet, rule *ast.Rule) ([]ast.Ref, []*ast.Error) {
	if as == nil {
		return nil, nil
	}
	var unknowns []ast.Ref
	var errs []*ast.Error

	for _, ar := range as.Chain(rule) {
		ann := ar.Annotations
		if ann == nil || ann.Compile == nil {
			continue
		}
		unkArray := ann.Compile.Unknowns
		for _, ref := range unkArray {
			if ref.HasPrefix(ast.DefaultRootRef) || ref.HasPrefix(ast.InputRootRef) {
				unknowns = append(unknowns, ref)
			} else {
				errs = append(errs, ast.NewError(invalidUnknownCode, ann.Loc(), "unknowns must be prefixed with `input` or `data`: %v", ref))
			}
		}
	}

	return unknowns, errs
}

func ExtractMaskRuleRefFromAnnotations(comp *ast.Compiler, ref ast.Ref) (ast.Ref, *ast.Error) {
	// find ast.Rule for ref
	rules := comp.GetRulesExact(ref)
	if len(rules) == 0 {
		return nil, nil
	}
	rule := rules[0] // rule scope doesn't make sense here, so it doesn't matter which rule we use
	return maskRuleFromAnnotationsSet(comp.GetAnnotationSet(), rule)
}

func maskRuleFromAnnotationsSet(as *ast.AnnotationSet, rule *ast.Rule) (ast.Ref, *ast.Error) {
	if as == nil {
		return nil, nil
	}

	for _, ar := range as.Chain(rule) {
		ann := ar.Annotations
		if ann == nil || ann.Compile == nil {
			continue
		}
		if maskRule := ann.Compile.MaskRule; maskRule != nil {
			if !maskRule.HasPrefix(ast.DefaultRootRef) {
				// If the mask_rule is not a data ref, add package prefix.
				maskRule = rule.Module.Package.Path.Extend(maskRule)
			}
			return maskRule, nil
		}
	}

	return nil, nil // No mask rule found.
}

func ShortsFromMappings(mappings map[string]any) Set[string] {
	shorts := NewSet[string]()
	for _, mapping := range mappings {
		m, ok := mapping.(map[string]any)
		if !ok {
			continue
		}
		for n, nmap := range m {
			m, ok := nmap.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := m["$table"]; ok {
				shorts = shorts.Add(n)
			}
		}
	}
	return shorts
}

// Returns a list of similar rule names that might match the input string.
// Warning(philip): This is expensive, as the cost grows linearly with the
// number of rules present on the compiler. It should be used only for
// error messages.
func FuzzyRuleNameMatchHint(comp *ast.Compiler, input string) string {
	rules := comp.GetRules(ast.Ref{ast.DefaultRootDocument})
	ruleNames := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.Default {
			continue
		}
		ruleNames = append(ruleNames, rule.Module.Package.Path.String()+"."+rule.Head.Name.String())
	}
	closest := levenshtein.ClosestStrings(65536, input, slices.Values(ruleNames))
	proposals := slices.Compact(closest)

	var msg string
	switch len(proposals) {
	case 0:
		return ""
	case 1:
		msg = fmt.Sprintf("%s undefined, did you mean %s?", input, proposals[0])
	default:
		msg = fmt.Sprintf("%s undefined, did you mean one of %v?", input, proposals)
	}
	return msg
}
