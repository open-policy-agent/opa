package rules

import (
	"slices"

	"github.com/vektah/gqlparser/v2/validator/core"
)

// Rules manages GraphQL validation rules.
type Rules struct {
	rules        map[string]core.RuleFunc
	ruleNameKeys []string // for deterministic order
}

// NewRules creates a Rules instance with the specified rules.
func NewRules(rs ...core.Rule) *Rules {
	r := &Rules{
		rules: make(map[string]core.RuleFunc),
	}

	for _, rule := range rs {
		r.AddRule(rule.Name, rule.RuleFunc)
	}

	return r
}

// NewDefaultRules creates a Rules instance containing the default GraphQL validation rule set.
func NewDefaultRules() *Rules {
	rules := []core.Rule{
		FieldsOnCorrectTypeRule,
		FragmentsOnCompositeTypesRule,
		KnownArgumentNamesRule,
		KnownDirectivesRule,
		KnownFragmentNamesRule,
		KnownRootTypeRule,
		KnownTypeNamesRule,
		LoneAnonymousOperationRule,
		MaxIntrospectionDepth,
		NoFragmentCyclesRule,
		NoUndefinedVariablesRule,
		NoUnusedFragmentsRule,
		NoUnusedVariablesRule,
		OverlappingFieldsCanBeMergedRule,
		PossibleFragmentSpreadsRule,
		ProvidedRequiredArgumentsRule,
		ScalarLeafsRule,
		SingleFieldSubscriptionsRule,
		UniqueArgumentNamesRule,
		UniqueDirectivesPerLocationRule,
		UniqueFragmentNamesRule,
		UniqueInputFieldNamesRule,
		UniqueOperationNamesRule,
		UniqueVariableNamesRule,
		ValuesOfCorrectTypeRule,
		VariablesAreInputTypesRule,
		VariablesInAllowedPositionRule,
	}

	r := NewRules(rules...)

	return r
}

// AddRule adds a rule with the specified name and rule function to the rule set.
// If a rule with the same name already exists, it will not be added.
func (r *Rules) AddRule(name string, ruleFunc core.RuleFunc) {
	if r.rules == nil {
		r.rules = make(map[string]core.RuleFunc)
	}

	if _, exists := r.rules[name]; !exists {
		r.rules[name] = ruleFunc
		r.ruleNameKeys = append(r.ruleNameKeys, name)
	}
}

// GetInner returns the internal rule map.
// If the map is not initialized, it returns an empty map.
func (r *Rules) GetInner() map[string]core.RuleFunc {
	if r == nil {
		return nil // impossible nonsense, hopefully
	}
	if r.rules == nil {
		return make(map[string]core.RuleFunc)
	}
	return r.rules
}

// RemoveRule removes a rule with the specified name from the rule set.
// If no rule with the specified name exists, it does nothing.
func (r *Rules) RemoveRule(name string) {
	if r == nil {
		return // impossible nonsense, hopefully
	}
	if r.rules != nil {
		delete(r.rules, name)
	}

	if len(r.ruleNameKeys) > 0 {
		r.ruleNameKeys = slices.DeleteFunc(r.ruleNameKeys, func(s string) bool {
			return s == name // delete the name rule key
		})
	}
}

// ReplaceRule replaces a rule with the specified name with a new rule function.
// If no rule with the specified name exists, it does nothing.
func (r *Rules) ReplaceRule(name string, ruleFunc core.RuleFunc) {
	if r == nil {
		return // impossible nonsense, hopefully
	}
	if r.rules == nil {
		r.rules = make(map[string]core.RuleFunc)
	}
	if _, exists := r.rules[name]; exists {
		r.rules[name] = ruleFunc
	}
}
