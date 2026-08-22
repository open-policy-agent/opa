package ast

import (
	"context"
	"slices"
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

// fakeEvalResolver mimics the topdown evaluator's save-set-aware resolver:
// refs covered by an unknown prefix are UnknownValueErr, input refs present in
// the concrete input resolve to their value, and input refs that are simply
// absent resolve to (nil, nil).
type fakeEvalResolver struct {
	unknowns []Ref
	input    Value
}

func (r fakeEvalResolver) Resolve(ref Ref) (Value, error) {
	if slices.ContainsFunc(r.unknowns, ref.HasPrefix) {
		return nil, UnknownValueErr{}
	}
	if ref.HasPrefix(InputRootRef) {
		if r.input == nil {
			return nil, nil
		}
		v, err := r.input.Find(ref[1:])
		if err != nil {
			return nil, nil
		}
		return v, nil
	}
	return nil, UnknownValueErr{}
}

type resolveResult struct {
	val     Value
	unknown bool
	err     error
}

// resolverCapturingIndex records what the resolver handed to Lookup returns for
// a fixed set of refs, so tests can assert how absent/unknown/known are
// surfaced under each ExternalSourceOptions setting.
type resolverCapturingIndex struct {
	distinguish bool
	got         map[string]resolveResult
}

func (idx *resolverCapturingIndex) Opts() *ExternalSourceOptions {
	return &ExternalSourceOptions{DistinguishAbsentFromUnknown: idx.distinguish}
}

func (idx *resolverCapturingIndex) Lookup(_ context.Context, opts ...LookupOption) ([]*Rule, ExternalRuleIndex, error) {
	o := LookupOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	res := o.Resolver()
	for _, s := range []string{"input.foo", "input.bar", "input.baz"} {
		v, err := res.Resolve(MustParseRef(s))
		idx.got[s] = resolveResult{val: v, unknown: IsUnknownValueErr(err), err: err}
	}
	return nil, nil, nil
}

func TestExternalSourceResolverDistinguishesAbsentFromUnknown(t *testing.T) {
	rt := NewRuleTree(NewModuleTree(nil))
	prefix := MustParseRef("data.authz")

	// input.foo is unknown (partial eval), input.bar is concretely known, and
	// input.baz is neither declared unknown nor present -> genuinely absent.
	resolver := fakeEvalResolver{
		unknowns: []Ref{MustParseRef("input.foo")},
		input:    MustParseTerm(`{"bar": "known"}`).Value,
	}

	t.Run("opted-in source tells absent from unknown", func(t *testing.T) {
		idx := &resolverCapturingIndex{distinguish: true, got: map[string]resolveResult{}}
		ei := &ExternalIndex{Index: idx, Ref: prefix}
		if _, _, err := ei.Tree(t.Context(), rt, prefix, nil, resolver, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if got := idx.got["input.foo"]; !got.unknown {
			t.Errorf("input.foo: want UnknownValueErr, got val=%v err=%v", got.val, got.err)
		}
		if got := idx.got["input.bar"]; got.unknown || got.val == nil || got.val.String() != `"known"` {
			t.Errorf("input.bar: want value \"known\", got val=%v unknown=%v", got.val, got.unknown)
		}
		if got := idx.got["input.baz"]; got.unknown || got.err != nil || got.val != nil {
			t.Errorf("input.baz: want absent (nil,nil), got val=%v unknown=%v err=%v", got.val, got.unknown, got.err)
		}
	})

	t.Run("default collapses absent into unknown", func(t *testing.T) {
		idx := &resolverCapturingIndex{distinguish: false, got: map[string]resolveResult{}}
		ei := &ExternalIndex{Index: idx, Ref: prefix}
		if _, _, err := ei.Tree(t.Context(), rt, prefix, nil, resolver, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if got := idx.got["input.foo"]; !got.unknown {
			t.Errorf("input.foo: want UnknownValueErr, got %+v", got)
		}
		if got := idx.got["input.bar"]; got.unknown || got.val == nil {
			t.Errorf("input.bar: want value, got %+v", got)
		}
		if got := idx.got["input.baz"]; !got.unknown {
			t.Errorf("input.baz: want UnknownValueErr (legacy collapse), got %+v", got)
		}
	})

	t.Run("nil resolver is treated as all-unknown", func(t *testing.T) {
		idx := &resolverCapturingIndex{distinguish: true, got: map[string]resolveResult{}}
		ei := &ExternalIndex{Index: idx, Ref: prefix}
		if _, _, err := ei.Tree(t.Context(), rt, prefix, nil, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{"input.foo", "input.bar", "input.baz"} {
			if got := idx.got[ref]; !got.unknown {
				t.Errorf("%s: want UnknownValueErr with nil resolver, got %+v", ref, got)
			}
		}
	})
}
