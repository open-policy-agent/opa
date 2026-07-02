package ast

import (
	"context"
	"sync/atomic"
	"testing"
)

type mockExternalSource struct {
	refs      []Ref
	rules     []*Rule
	callCount int32
}

func newMockExternalSource(refs []Ref, rules []*Rule) *mockExternalSource {
	return &mockExternalSource{
		refs:  refs,
		rules: rules,
	}
}

func (m *mockExternalSource) Refs() []Ref {
	return m.refs
}

func (m *mockExternalSource) Init(context.Context, Ref) (ExternalRuleIndex, error) {
	return &mockExternalIndex{rules: m.rules, callCount: &m.callCount}, nil
}

type mockExternalIndex struct {
	rules     []*Rule
	callCount *int32
}

func (*mockExternalIndex) Opts() *ExternalSourceOptions {
	return nil
}

func (m *mockExternalIndex) Lookup(context.Context, ...LookupOption) ([]*Rule, ExternalRuleIndex, error) {
	atomic.AddInt32(m.callCount, 1)
	return m.rules, nil, nil
}

func (m *mockExternalSource) getCallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

func TestCompilerRuleIndexReturnsNilForExternalSources(t *testing.T) {
	rule := &Rule{
		Head: &Head{
			Reference: MustParseRef("data.external.test.foo"),
			Value:     BooleanTerm(true),
		},
		Body: NewBody(
			Equality.Expr(VarTerm("x"), IntNumberTerm(1)),
		),
	}

	packageRef := MustParseRef("data.external.test")
	source := newMockExternalSource([]Ref{packageRef}, []*Rule{rule})
	compiler := NewCompiler()
	compiler.WithExternalSource(packageRef, source)

	index := compiler.RuleIndex(packageRef)
	if index != nil {
		t.Error("Expected RuleIndex to return nil for external source path (delegation to evaluation-time)")
	}

	if source.getCallCount() != 0 {
		t.Errorf("Expected GetRules NOT to be called at compile-time, got %d calls", source.getCallCount())
	}
}
