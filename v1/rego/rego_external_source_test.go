package rego

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
)

type mockExternalSource struct {
	refRules    map[string][]*ast.Rule
	visibleRefs []ast.Ref
}

func newMockExternalSource(refs []ast.Ref, rules []*ast.Rule) *mockExternalSource {
	refRules := make(map[string][]*ast.Rule)
	for _, ref := range refs {
		refRules[ref.String()] = rules
	}
	return &mockExternalSource{
		refRules: refRules,
	}
}

func (m *mockExternalSource) Refs() []ast.Ref {
	refs := make([]ast.Ref, 0, len(m.refRules))
	for refStr := range m.refRules {
		refs = append(refs, ast.MustParseRef(refStr))
	}
	return refs
}

func (m *mockExternalSource) Init(_ context.Context, ref ast.Ref) (ast.ExternalRuleIndex, error) {
	rules, ok := m.refRules[ref.String()]
	if !ok {
		return nil, nil
	}
	return &mockExternalIndex{rules: rules, visibleRefs: m.visibleRefs}, nil
}

type mockExternalIndex struct {
	rules       []*ast.Rule
	visibleRefs []ast.Ref
}

func (m *mockExternalIndex) Opts() *ast.ExternalSourceOptions {
	if m.visibleRefs == nil {
		return nil
	}
	return &ast.ExternalSourceOptions{VisibleRefs: m.visibleRefs}
}

func (m *mockExternalIndex) Lookup(ctx context.Context, _ ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	return m.rules, nil, nil
}

func evalWithExternalSource(t *testing.T, ctx context.Context, query, module string, source *mockExternalSource, input map[string]any) ResultSet {
	t.Helper()
	opts := []func(*Rego){
		Query(query),
		Module("test.rego", module),
		ExternalSource(source),
	}
	if input != nil {
		opts = append(opts, Input(input))
	}
	r := New(opts...)
	rs, err := r.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	return rs
}

func partialEvalWithExternalSource(t *testing.T, ctx context.Context, query, module string, source *mockExternalSource) *PartialQueries {
	t.Helper()
	r := New(
		Query(query),
		Module("test.rego", module),
		ExternalSource(source),
	)
	pq, err := r.Partial(ctx)
	if err != nil {
		t.Fatalf("Partial failed: %v", err)
	}
	return pq
}

func assertBoolResult(t *testing.T, rs ResultSet, expected bool, msg string) {
	t.Helper()
	if len(rs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rs))
	}
	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		t.Fatalf("Expected boolean result, got %T", rs[0].Expressions[0].Value)
	}
	if result != expected {
		t.Errorf("%s: expected %v, got %v", msg, expected, result)
	}
}

func assertNoResults(t *testing.T, rs ResultSet, msg string) {
	t.Helper()
	if len(rs) != 0 {
		t.Errorf("%s: expected 0 results, got %d", msg, len(rs))
	}
}

func assertPartialQuery(t *testing.T, pq *PartialQueries, expectedQueries string) {
	t.Helper()
	expected := ast.MustParseBody(expectedQueries)
	if len(pq.Queries) != 1 {
		t.Fatalf("Expected 1 query, got %d", len(pq.Queries))
	}
	if !pq.Queries[0].Equal(expected) {
		t.Errorf("Expected PE result:\n%v\n\nGot:\n%v", expected, pq.Queries[0])
	}
}

func TestExternalSourceDecisionMaking(t *testing.T) {
	ctx := t.Context()

	externalModule := ast.MustParseModule(`package external.authz
allow if input.role == "admin"`)

	packageRef := ast.MustParseRef("data.external.authz")
	source := newMockExternalSource([]ast.Ref{packageRef}, externalModule.Rules)

	staticModule := `package authz
default allow := false
allow if data.external.authz.allow`

	t.Run("admin role allowed", func(t *testing.T) {
		rs := evalWithExternalSource(t, ctx, "data.authz.allow", staticModule, source, map[string]any{"role": "admin"})
		assertBoolResult(t, rs, true, "Expected allow=true for admin role")
	})

	t.Run("non-admin role not allowed", func(t *testing.T) {
		rs := evalWithExternalSource(t, ctx, "data.authz.allow", staticModule, source, map[string]any{"role": "user"})
		assertBoolResult(t, rs, false, "Expected allow=false for user role")
	})
}

func TestExternalSourcePartialEval(t *testing.T) {
	ctx := t.Context()

	externalModule := ast.MustParseModule(`package external.authz
allow if input.role == "admin"`)

	packageRef := ast.MustParseRef("data.external.authz")
	source := newMockExternalSource([]ast.Ref{packageRef}, externalModule.Rules)

	staticModule := `package authz
default allow := false
allow if data.external.authz.allow`

	t.Run("partial eval into external rule", func(t *testing.T) {
		pq := partialEvalWithExternalSource(t, ctx, "data.authz.allow", staticModule, source)
		assertPartialQuery(t, pq, `input.role = "admin"`)
	})
}

func TestExternalSourceCallBackIntoStaticRego(t *testing.T) {
	ctx := t.Context()

	externalModule := ast.MustParseModule(`package external.authz
allow if data.static.authz.foo == "bar"`)

	packageRef := ast.MustParseRef("data.external.authz")
	source := &mockExternalSource{
		refRules: map[string][]*ast.Rule{
			packageRef.String(): externalModule.Rules,
		},
		visibleRefs: []ast.Ref{ast.MustParseRef("data")},
	}

	staticModule := `package static.authz
default allow := false
allow if data.external.authz.allow

foo := "bar"`

	rs := evalWithExternalSource(t, ctx, "data.static.authz.allow", staticModule, source, nil)
	assertBoolResult(t, rs, true, "Expected allow=true")
}

func TestExternalSourceCallBackIntoStaticRegoWithRecursion(t *testing.T) {
	externalModule := ast.MustParseModule(`package external.authz
allow if data.static.authz.allow`)

	packageRef := ast.MustParseRef("data.external.authz")

	staticModule := `package static.authz
default allow := false
allow if data.external.authz.allow`

	t.Run("isolated by default prevents recursion", func(t *testing.T) {
		ctx := t.Context()
		source := newMockExternalSource([]ast.Ref{packageRef}, externalModule.Rules)
		rs := evalWithExternalSource(t, ctx, "data.static.authz.allow", staticModule, source, nil)
		assertBoolResult(t, rs, false, "Expected allow=false (isolated external source cannot access static policy)")
	})

	t.Run("visible refs allows recursion and hits deadline", func(t *testing.T) {
		ctx := t.Context()
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				packageRef.String(): externalModule.Rules,
			},
			visibleRefs: []ast.Ref{ast.MustParseRef("data")},
		}

		r := New(
			Query("data.static.authz.allow"),
			Module("authz.rego", staticModule),
			ExternalSource(source),
		)

		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		t.Cleanup(cancel)
		_, err := r.Eval(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline-exceeded, got err: %v", err)
		}
	})
}

func TestExternalSourceCrossRefCalls(t *testing.T) {
	ctx := t.Context()

	fooModule := ast.MustParseModule(`package external.foo
result if data.external.bar.value == 42`)

	barModule := ast.MustParseModule(`package external.bar
value := 42`)

	staticModule := `package main
result := data.external.foo.result`

	t.Run("isolated prevents cross-ref calls", func(t *testing.T) {
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				"data.external.foo": fooModule.Rules,
				"data.external.bar": barModule.Rules,
			},
			visibleRefs: []ast.Ref{}, // explicitly isolated
		}

		rs := evalWithExternalSource(t, ctx, "data.main.result", staticModule, source, nil)
		assertNoResults(t, rs, "Expected 0 results when isolated (external.foo cannot access external.bar)")
	})

	t.Run("visible refs allows cross-ref calls", func(t *testing.T) {
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				"data.external.foo": fooModule.Rules,
				"data.external.bar": barModule.Rules,
			},
			visibleRefs: []ast.Ref{ast.MustParseRef("data")},
		}

		rs := evalWithExternalSource(t, ctx, "data.main.result", staticModule, source, nil)
		assertBoolResult(t, rs, true, "Expected result=true when all refs visible")
	})
}

func TestExternalSourceNestedPackage(t *testing.T) {
	ctx := t.Context()

	externalModule := ast.MustParseModule(`package external.project.authz

allowed if other_rule
other_rule if input.foo == "bar"`)

	parentRef := ast.MustParseRef("data.external")
	source := newMockExternalSource([]ast.Ref{parentRef}, externalModule.Rules)

	staticModule := `package main
result := data.external.project.authz.allowed`

	t.Run("eval allowed when foo is bar", func(t *testing.T) {
		rs := evalWithExternalSource(t, ctx, "data.main.result", staticModule, source, map[string]any{"foo": "bar"})
		assertBoolResult(t, rs, true, "Expected allowed=true when foo is bar")
	})

	t.Run("eval denied when foo is not bar", func(t *testing.T) {
		rs := evalWithExternalSource(t, ctx, "data.main.result", staticModule, source, map[string]any{"foo": "baz"})
		assertNoResults(t, rs, "Expected no results when foo is not bar")
	})

	t.Run("partial eval", func(t *testing.T) {
		pq := partialEvalWithExternalSource(t, ctx, "data.main.result", staticModule, source)
		assertPartialQuery(t, pq, `input.foo = "bar"`)
	})

	t.Run("call external function with argument fails", func(t *testing.T) {
		externalModuleWithFunc := ast.MustParseModule(`package external.authz

foo(x) if x == "bar"`)

		funcSource := newMockExternalSource([]ast.Ref{ast.MustParseRef("data.external.authz")}, externalModuleWithFunc.Rules)

		staticModuleWithFuncCall := `package main
allow if data.external.authz.foo("bar")`

		r := New(
			Query("data.main.allow"),
			Module("test.rego", staticModuleWithFuncCall),
			ExternalSource(funcSource),
		)
		_, err := r.Eval(ctx)
		if err == nil {
			t.Fatal("Expected error when calling external function with argument, but got none")
		}
		t.Logf("Expected error occurred: %v", err)
	})

	t.Run("call into external rule that uses function internally", func(t *testing.T) {
		externalModuleWithInternalFunc := ast.MustParseModule(`package external.authz

allowed if foo("bar")
foo(x) if x == "bar"`)

		funcSource := newMockExternalSource([]ast.Ref{ast.MustParseRef("data.external.authz")}, externalModuleWithInternalFunc.Rules)

		staticModuleWithRuleCall := `package main
allow if data.external.authz.allowed`

		rs := evalWithExternalSource(t, ctx, "data.main.allow", staticModuleWithRuleCall, funcSource, nil)
		assertBoolResult(t, rs, true, "Expected allow=true when external rule internally uses function")
	})
}

func TestExternalSourcePartialVisibility(t *testing.T) {
	ctx := t.Context()

	externalModule := ast.MustParseModule(`package external.authz
allow if {
	data.pkg_a.check
	data.pkg_b.check
}`)

	packageRef := ast.MustParseRef("data.external.authz")

	modA := `package pkg_a
check if input.role == "admin"`

	modB := `package pkg_b
check := true`

	t.Run("only pkg_a visible", func(t *testing.T) {
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				packageRef.String(): externalModule.Rules,
			},
			visibleRefs: []ast.Ref{ast.MustParseRef("data.pkg_a")},
		}

		r := New(
			Query("data.external.authz.allow"),
			Module("a.rego", modA),
			Module("b.rego", modB),
			ExternalSource(source),
			Input(map[string]any{"role": "admin"}),
		)
		rs, err := r.Eval(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assertNoResults(t, rs, "Expected 0 results (pkg_b not visible)")
	})

	t.Run("both visible", func(t *testing.T) {
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				packageRef.String(): externalModule.Rules,
			},
			visibleRefs: []ast.Ref{
				ast.MustParseRef("data.pkg_a"),
				ast.MustParseRef("data.pkg_b"),
			},
		}

		r := New(
			Query("data.external.authz.allow"),
			Module("a.rego", modA),
			Module("b.rego", modB),
			ExternalSource(source),
			Input(map[string]any{"role": "admin"}),
		)
		rs, err := r.Eval(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assertBoolResult(t, rs, true, "Expected allow=true when both packages visible")
	})

	t.Run("all of data visible", func(t *testing.T) {
		source := &mockExternalSource{
			refRules: map[string][]*ast.Rule{
				packageRef.String(): externalModule.Rules,
			},
			visibleRefs: []ast.Ref{ast.MustParseRef("data")},
		}

		r := New(
			Query("data.external.authz.allow"),
			Module("a.rego", modA),
			Module("b.rego", modB),
			ExternalSource(source),
			Input(map[string]any{"role": "admin"}),
		)
		rs, err := r.Eval(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assertBoolResult(t, rs, true, "Expected allow=true when all of data is visible")
	})
}

func TestExternalSourceUnsupportedTarget(t *testing.T) {
	externalModule := ast.MustParseModule(`package external.authz
allow if input.role == "admin"`)

	packageRef := ast.MustParseRef("data.external.authz")
	source := newMockExternalSource([]ast.Ref{packageRef}, externalModule.Rules)

	for _, target := range []string{"wasm", "plan", "custom-plugin"} {
		t.Run(target, func(t *testing.T) {
			r := New(
				Query("data.external.authz.allow"),
				Module("test.rego", `package test`),
				ExternalSource(source),
				Target(target),
			)
			_, err := r.Eval(t.Context())
			if err == nil {
				t.Fatal("Expected error for non-rego target with external sources")
			}
			if !strings.Contains(err.Error(), "external rule sources are not supported") {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}
