package core

import (
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type AddErrFunc func(options ...ErrorOption)

type RuleFunc func(observers *Events, addError AddErrFunc)

type Rule struct {
	Name     string
	RuleFunc RuleFunc
}

// NameSorter sorts Rules by name.
// usage: sort.Sort(core.NameSorter(specifiedRules))
type NameSorter []Rule

func (a NameSorter) Len() int           { return len(a) }
func (a NameSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a NameSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

type ErrorOption func(err *gqlerror.Error)
