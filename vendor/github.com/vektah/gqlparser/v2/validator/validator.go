package validator

import (
	//nolint:staticcheck // bad, yeah
	. "github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type AddErrFunc func(options ...ErrorOption)

type RuleFunc func(observers *Events, addError AddErrFunc)

type Rule struct {
	Name     string
	RuleFunc RuleFunc
}

var specifiedRules []Rule

// AddRule adds a rule to the rule set.
// ruleFunc is called once each time `Validate` is executed.
func AddRule(name string, ruleFunc RuleFunc) {
	specifiedRules = append(specifiedRules, Rule{Name: name, RuleFunc: ruleFunc})
}

// RemoveRule removes an existing rule from the rule set
// if one of the same name exists.
// The rule set is global, so it is not safe for concurrent changes
func RemoveRule(name string) {
	var result []Rule // nolint:prealloc // using initialized with len(rules) produces a race condition
	for _, r := range specifiedRules {
		if r.Name == name {
			continue
		}
		result = append(result, r)
	}
	specifiedRules = result
}

// ReplaceRule replaces an existing rule from the rule set
// if one of the same name exists.
// If no match is found, it will add a new rule to the rule set.
// The rule set is global, so it is not safe for concurrent changes
func ReplaceRule(name string, ruleFunc RuleFunc) {
	var found bool
	var result []Rule // nolint:prealloc // using initialized with len(rules) produces a race condition
	for _, r := range specifiedRules {
		if r.Name == name {
			found = true
			result = append(result, Rule{Name: name, RuleFunc: ruleFunc})
			continue
		}
		result = append(result, r)
	}
	if !found {
		specifiedRules = append(specifiedRules, Rule{Name: name, RuleFunc: ruleFunc})
		return
	}
	specifiedRules = result
}

func Validate(schema *Schema, doc *QueryDocument, rules ...Rule) gqlerror.List {
	if rules == nil {
		rules = specifiedRules
	}

	var errs gqlerror.List
	if schema == nil {
		errs = append(errs, gqlerror.Errorf("cannot validate as Schema is nil"))
	}
	if doc == nil {
		errs = append(errs, gqlerror.Errorf("cannot validate as QueryDocument is nil"))
	}
	if len(errs) > 0 {
		return errs
	}
	observers := &Events{}
	for i := range rules {
		rule := rules[i]
		rule.RuleFunc(observers, func(options ...ErrorOption) {
			err := &gqlerror.Error{
				Rule: rule.Name,
			}
			for _, o := range options {
				o(err)
			}
			errs = append(errs, err)
		})
	}

	Walk(schema, doc, observers)
	return errs
}
