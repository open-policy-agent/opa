package topdown

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

type countingExternalSource struct {
	refs      []ast.Ref
	rules     []*ast.Rule
	callCount int32
}

func (m *countingExternalSource) Init(context.Context, ast.Ref) (ast.ExternalRuleIndex, error) {
	return &countingExternalIndex{rules: m.rules, callCount: &m.callCount}, nil
}

func (m *countingExternalSource) Refs() []ast.Ref {
	return m.refs
}

type countingExternalIndex struct {
	rules     []*ast.Rule
	callCount *int32
}

func (*countingExternalIndex) Opts() *ast.ExternalSourceOptions {
	return nil
}

func (m *countingExternalIndex) Lookup(context.Context, ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	atomic.AddInt32(m.callCount, 1)
	return m.rules, nil, nil
}

func (m *countingExternalSource) getCallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

func setupCompiler(t *testing.T, packageRef ast.Ref, source ast.ExternalRuleSource, staticModule *ast.Module) *ast.Compiler {
	t.Helper()
	compiler := ast.NewCompiler()
	compiler.WithExternalSource(packageRef, source)
	modules := map[string]*ast.Module{}
	if staticModule != nil {
		modules["main.rego"] = staticModule
	}
	compiler.Compile(modules)
	if compiler.Failed() {
		t.Fatalf("Compiler failed: %v", compiler.Errors)
	}
	return compiler
}

func runQuery(t *testing.T, compiler *ast.Compiler, queryStr string, input *ast.Term) QueryResultSet {
	t.Helper()
	store := inmem.New()
	ctx := t.Context()
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Abort(ctx, txn)
	m := metrics.New()
	instr := NewInstrumentation(m)

	query := ast.MustParseBody(queryStr)
	q := NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithInstrumentation(instr)

	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	t.Logf("metrics: %v", m.All())
	return qrs
}

func TestExternalSourceE2EWithInputOverride(t *testing.T) {
	t.Parallel()

	externalModule := ast.MustParseModule(`package authz
allowed if input.user == "alice"`)

	packageRef := ast.MustParseRef("data.authz")
	source := &countingExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	staticModule := ast.MustParseModule(`package main
check if {
	data.authz.allowed
	data.authz.allowed with input as {"user": "bob"}
}`)

	compiler := setupCompiler(t, packageRef, source, staticModule)

	input := ast.MustParseTerm(`{"user": "alice"}`)
	qrs := runQuery(t, compiler, "data.main.check", input)

	if len(qrs) != 0 {
		t.Errorf("Expected 0 results (second check with bob should fail), got %d", len(qrs))
	}

	if callCount := source.getCallCount(); callCount != 2 {
		t.Errorf("Expected external source to be called twice (once per input), got %d calls", callCount)
	}
}

func TestExternalSourceE2EWithMultipleRulesFromSamePackage(t *testing.T) {
	t.Parallel()

	externalModule := ast.MustParseModule(`package authz
allow if input.user == "alice"
deny if input.action == "delete"
allowed if {
	allow
	not deny
}`)

	packageRef := ast.MustParseRef("data.authz")
	source := &countingExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	staticModule := ast.MustParseModule(`package main
check if data.authz.allowed`)

	compiler := setupCompiler(t, packageRef, source, staticModule)

	input := ast.MustParseTerm(`{"user": "alice", "action": "read"}`)
	qrs := runQuery(t, compiler, "data.main.check", input)

	if len(qrs) != 1 {
		t.Errorf("Expected 1 result, got %d", len(qrs))
	}

	if callCount := source.getCallCount(); callCount != 1 {
		t.Errorf("Expected external source to be called once (cached for same ref and input), got %d calls", callCount)
	}
}

type closableExternalSource struct {
	refs       []ast.Ref
	rules      []*ast.Rule
	closeCalls int32
}

func (m *closableExternalSource) Init(context.Context, ast.Ref) (ast.ExternalRuleIndex, error) {
	return &closableExternalIndex{rules: m.rules, closeCalls: &m.closeCalls}, nil
}

func (m *closableExternalSource) Refs() []ast.Ref {
	return m.refs
}

func (m *closableExternalSource) getCloseCalls() int {
	return int(atomic.LoadInt32(&m.closeCalls))
}

type closableExternalIndex struct {
	rules      []*ast.Rule
	closeCalls *int32
}

func (*closableExternalIndex) Opts() *ast.ExternalSourceOptions {
	return nil
}

func (m *closableExternalIndex) Lookup(context.Context, ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	return m.rules, nil, nil
}

func (m *closableExternalIndex) Close() error {
	atomic.AddInt32(m.closeCalls, 1)
	return nil
}

func TestExternalSourceCloseCalled(t *testing.T) {
	t.Parallel()

	externalModule := ast.MustParseModule(`package authz
allowed if input.user == "alice"`)

	packageRef := ast.MustParseRef("data.authz")
	source := &closableExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	staticModule := ast.MustParseModule(`package main
check if data.authz.allowed`)

	compiler := setupCompiler(t, packageRef, source, staticModule)

	input := ast.MustParseTerm(`{"user": "alice"}`)
	qrs := runQuery(t, compiler, "data.main.check", input)

	if len(qrs) != 1 {
		t.Errorf("Expected 1 result, got %d", len(qrs))
	}

	if closeCalls := source.getCloseCalls(); closeCalls != 1 {
		t.Errorf("Expected Close() to be called once, got %d calls", closeCalls)
	}
}

type preCompiledRulesSource struct {
	refs          []ast.Ref
	compiledRules []*ast.Rule
}

func (s *preCompiledRulesSource) Init(context.Context, ast.Ref) (ast.ExternalRuleIndex, error) {
	return &preCompiledRulesIndex{compiledRules: s.compiledRules}, nil
}

func (s *preCompiledRulesSource) Refs() []ast.Ref {
	return s.refs
}

type preCompiledRulesIndex struct {
	compiledRules []*ast.Rule
}

func (*preCompiledRulesIndex) Opts() *ast.ExternalSourceOptions {
	// For pre-compiled rules, skip all stages except those essential for
	// integrating the rules into the compiler
	var skippedStages []ast.StageID
	essentialStages := []ast.StageID{
		ast.StageSetModuleTree,
		ast.StageSetRuleTree, ast.StageBuildRuleIndices,
	}

	for _, stage := range ast.AllStages() {
		if !slices.Contains(essentialStages, stage) {
			skippedStages = append(skippedStages, stage)
		}
	}

	return &ast.ExternalSourceOptions{
		SkippedStages: skippedStages,
	}
}

func (idx *preCompiledRulesIndex) Lookup(_ context.Context, _ ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	return idx.compiledRules, nil, nil
}

func TestExternalSourceWithPreCompiledRules(t *testing.T) {
	t.Parallel()

	// Create and pre-compile an external module
	externalModule := ast.MustParseModule(`package authz

allow if input.user == "admin"
deny if input.action == "delete"

permitted if {
	allow
	not deny
}`)

	// Pre-compile the rules using a separate compiler
	preCompiler := ast.NewCompiler()
	preCompiler.Compile(map[string]*ast.Module{"authz.rego": externalModule})

	if preCompiler.Failed() {
		t.Fatalf("Pre-compilation failed: %v", preCompiler.Errors)
	}

	// Extract the pre-compiled rules
	compiledRules := make([]*ast.Rule, 0, len(preCompiler.Modules))
	for _, mod := range preCompiler.Modules {
		compiledRules = append(compiledRules, mod.Rules...)
	}

	if len(compiledRules) == 0 {
		t.Fatal("No compiled rules found")
	}

	// Create an external source that returns pre-compiled rules
	packageRef := ast.MustParseRef("data.authz")
	source := &preCompiledRulesSource{
		refs:          []ast.Ref{packageRef},
		compiledRules: compiledRules,
	}

	// Create a static module that uses the externally-provided rules
	staticModule := ast.MustParseModule(`package main

check if data.authz.permitted`)

	// Set up compiler with the external source
	compiler := setupCompiler(t, packageRef, source, staticModule)

	t.Run("admin with read action should be allowed", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "admin", "action": "read"}`)
		qrs := runQuery(t, compiler, "data.main.check", input)

		if len(qrs) != 1 {
			t.Errorf("Expected 1 result (allowed), got %d", len(qrs))
		}
	})

	t.Run("admin with delete action should be denied", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "admin", "action": "delete"}`)
		qrs := runQuery(t, compiler, "data.main.check", input)

		if len(qrs) != 0 {
			t.Errorf("Expected 0 results (denied), got %d", len(qrs))
		}
	})

	t.Run("non-admin user should be denied", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "bob", "action": "read"}`)
		qrs := runQuery(t, compiler, "data.main.check", input)

		if len(qrs) != 0 {
			t.Errorf("Expected 0 results (denied), got %d", len(qrs))
		}
	})
}

func TestExternalSourceE2EWithInputOverrideNilInput(t *testing.T) {
	t.Parallel()

	externalModule := ast.MustParseModule(`package authz
allowed if input.user == "alice"`)

	packageRef := ast.MustParseRef("data.authz")
	source := &countingExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	staticModule := ast.MustParseModule(`package main
check if data.authz.allowed with input as {"user": "alice"}`)

	compiler := setupCompiler(t, packageRef, source, staticModule)

	// Query with nil input — the with clause provides it.
	// This previously panicked because evalWithPop skipped PopFrame
	// when oldInput was nil, leaking a frame on the externalTreeStack.
	qrs := runQuery(t, compiler, "data.main.check", nil)

	if len(qrs) != 1 {
		t.Errorf("Expected 1 result, got %d", len(qrs))
	}
}
