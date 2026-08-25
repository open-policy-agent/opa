package topdown

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
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

func TestExternalSourceDirectQuery(t *testing.T) {
	t.Parallel()

	externalModule := ast.MustParseModule(`package authz
allow if input.user == "alice"
deny if input.user == "bob"`)

	packageRef := ast.MustParseRef("data.authz")
	source := &countingExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	compiler := setupCompiler(t, packageRef, source, nil)

	t.Run("direct query allowed", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "alice"}`)
		qrs := runQuery(t, compiler, "data.authz.allow", input)
		if len(qrs) != 1 {
			t.Errorf("Expected 1 result for direct external query, got %d", len(qrs))
		}
	})

	t.Run("direct query denied", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "bob"}`)
		qrs := runQuery(t, compiler, "data.authz.deny", input)
		if len(qrs) != 1 {
			t.Errorf("Expected 1 result for direct external query, got %d", len(qrs))
		}
	})

	t.Run("direct query no match", func(t *testing.T) {
		input := ast.MustParseTerm(`{"user": "charlie"}`)
		qrs := runQuery(t, compiler, "data.authz.allow", input)
		if len(qrs) != 0 {
			t.Errorf("Expected 0 results, got %d", len(qrs))
		}
	})
}

func TestExternalSourceCompilationFailure(t *testing.T) {
	t.Parallel()

	// Rules with an unsafe variable should fail compilation
	externalModule := ast.MustParseModule(`package authz
allow if { x }`)

	packageRef := ast.MustParseRef("data.authz")
	source := &countingExternalSource{refs: []ast.Ref{packageRef}, rules: externalModule.Rules}

	staticModule := ast.MustParseModule(`package main
check if data.authz.allow`)

	compiler := setupCompiler(t, packageRef, source, staticModule)

	store := inmem.New()
	ctx := t.Context()
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Abort(ctx, txn)

	query := ast.MustParseBody("data.main.check")
	q := NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn)

	_, err = q.Run(ctx)
	if err == nil {
		t.Fatal("Expected compilation error from external source, got nil")
	}
	var errs ast.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("Expected ast.Errors, got: %T: %v", err, err)
	}
	if !slices.ContainsFunc(errs, func(e *ast.Error) bool { return e.Code == ast.UnsafeVarErr }) {
		t.Errorf("Expected unsafe var error, got: %v", err)
	}
}

// paramExternalSource is a parametrized (prefix) external source: it is
// registered under a prefix ref (e.g. data.directory.user) with ParamArity 1, and
// ParametrizedExternalRuleIndex with arity 1, and
// synthesizes a distinct module per key on each Lookup from the parameter term.
// It stands in for a real source that serves a different set of rules per key.
type paramExternalSource struct {
	refs      []ast.Ref
	arity     int
	callCount int32
	keys      []string
}

func (s *paramExternalSource) Refs() []ast.Ref { return s.refs }

func (s *paramExternalSource) getCallCount() int { return int(atomic.LoadInt32(&s.callCount)) }

func (s *paramExternalSource) Init(_ context.Context, ref ast.Ref) (ast.ExternalRuleIndex, error) {
	return &paramExternalIndex{prefix: ref, arity: s.arity, src: s}, nil
}

type paramExternalIndex struct {
	prefix ast.Ref
	arity  int
	src    *paramExternalSource
}

func (*paramExternalIndex) Opts() *ast.ExternalSourceOptions {
	return &ast.ExternalSourceOptions{}
}

// ParamArity declares this source parametrized: the registered prefix is
// followed by idx.arity key segment(s) consumed as lookup parameters.
func (idx *paramExternalIndex) ParamArity(ast.Ref) int {
	return idx.arity
}

func (idx *paramExternalIndex) Lookup(_ context.Context, opts ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	atomic.AddInt32(&idx.src.callCount, 1)

	o := ast.LookupOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	params := o.Params()
	if len(params) != idx.arity {
		return nil, nil, fmt.Errorf("expected %d param(s), got %d", idx.arity, len(params))
	}
	key, ok := params[0].(ast.String)
	if !ok {
		return nil, nil, fmt.Errorf("expected string key, got %v", params[0])
	}
	idx.src.keys = append(idx.src.keys, string(key))

	// The prefix (e.g. data.directory.user) is the Rego package of the synthesized
	// module; its rules echo the key and gate on input.
	pkgPath := idx.prefix.String()[len("data."):]
	mod := ast.MustParseModule(fmt.Sprintf(
		"package %s\nid := %q\nallow if input.account == %q",
		pkgPath, string(key), string(key)))
	return mod.Rules, nil, nil
}

func TestExternalSourceParametrizedDistinctKeys(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.directory.user")
	source := &paramExternalSource{refs: []ast.Ref{prefix}, arity: 1}

	// Two distinct keys in a single evaluation must not collide.
	staticModule := ast.MustParseModule(`package main
check if {
	data.directory.user["123"].id == "123"
	data.directory.user["456"].id == "456"
}`)

	compiler := setupCompiler(t, prefix, source, staticModule)

	qrs := runQuery(t, compiler, "data.main.check", nil)
	if len(qrs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(qrs))
	}
	if got := source.getCallCount(); got != 2 {
		t.Errorf("Expected external source to be called twice (once per distinct key), got %d (keys=%v)", got, source.keys)
	}
}

func TestExternalSourceParametrizedWrongKeyUndefined(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.directory.user")
	source := &paramExternalSource{refs: []ast.Ref{prefix}, arity: 1}

	// The key's module echoes its own id; asking for a mismatched id is undefined.
	staticModule := ast.MustParseModule(`package main
check if data.directory.user["123"].id == "999"`)

	compiler := setupCompiler(t, prefix, source, staticModule)

	qrs := runQuery(t, compiler, "data.main.check", nil)
	if len(qrs) != 0 {
		t.Errorf("Expected 0 results for mismatched id, got %d", len(qrs))
	}
}

func TestExternalSourceParametrizedInputGate(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.directory.user")
	source := &paramExternalSource{refs: []ast.Ref{prefix}, arity: 1}

	staticModule := ast.MustParseModule(`package main
check if data.directory.user["777"].allow`)

	compiler := setupCompiler(t, prefix, source, staticModule)

	t.Run("input matches key", func(t *testing.T) {
		input := ast.MustParseTerm(`{"account": "777"}`)
		qrs := runQuery(t, compiler, "data.main.check", input)
		if len(qrs) != 1 {
			t.Errorf("Expected 1 result, got %d", len(qrs))
		}
	})

	t.Run("input does not match key", func(t *testing.T) {
		input := ast.MustParseTerm(`{"account": "000"}`)
		qrs := runQuery(t, compiler, "data.main.check", input)
		if len(qrs) != 0 {
			t.Errorf("Expected 0 results, got %d", len(qrs))
		}
	})
}

func TestExternalSourceParametrizedDynamicKeyFromInput(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.directory.user")
	source := &paramExternalSource{refs: []ast.Ref{prefix}, arity: 1}

	// The key itself comes from input and is ground at eval time.
	staticModule := ast.MustParseModule(`package main
check if data.directory.user[input.account].id == input.account`)

	compiler := setupCompiler(t, prefix, source, staticModule)

	input := ast.MustParseTerm(`{"account": "555"}`)
	qrs := runQuery(t, compiler, "data.main.check", input)
	if len(qrs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(qrs))
	}
	if len(source.keys) == 0 || source.keys[len(source.keys)-1] != "555" {
		t.Errorf("Expected source to be queried with key 555, got keys=%v", source.keys)
	}
}

// unevenExternalSource is a parametrized (prefix) external source whose arity
// varies with the reference tail, so a single registered prefix (data.reg)
// serves sub-references of different nesting depth: data.reg.user[<k>] nests one
// level while data.reg.pair[<a>][<b>] nests two. This is only expressible
// because ParamArity is consulted at eval time with the reference tail — a
// compile-time constant, uniform across the prefix subtree, could not describe
// an uneven-depth tree under one prefix.
type unevenExternalSource struct {
	refs  []ast.Ref
	calls int32
}

func (s *unevenExternalSource) Refs() []ast.Ref { return s.refs }

func (s *unevenExternalSource) getCallCount() int { return int(atomic.LoadInt32(&s.calls)) }

func (s *unevenExternalSource) Init(_ context.Context, ref ast.Ref) (ast.ExternalRuleIndex, error) {
	return &unevenExternalIndex{prefix: ref, src: s}, nil
}

type unevenExternalIndex struct {
	prefix ast.Ref
	src    *unevenExternalSource
}

func (*unevenExternalIndex) Opts() *ast.ExternalSourceOptions { return &ast.ExternalSourceOptions{} }

// ParamArity keys off the leading tail segment to decide how many elements are
// consumed as parameters: "user" nests one level deep, "pair" nests two.
func (*unevenExternalIndex) ParamArity(tail ast.Ref) int {
	if len(tail) == 0 {
		return 0
	}
	switch tail[0].Value {
	case ast.String("user"):
		return 2 // "user", <k>
	case ast.String("pair"):
		return 3 // "pair", <a>, <b>
	default:
		return 0
	}
}

func (idx *unevenExternalIndex) Lookup(_ context.Context, opts ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	atomic.AddInt32(&idx.src.calls, 1)

	o := ast.LookupOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	params := o.Params()

	// Derive a value from the consumed parameters, distinct per depth.
	var value string
	switch len(params) {
	case 2: // data.reg.user[<k>] -> params are "user", <k>
		value = string(params[1].(ast.String))
	case 3: // data.reg.pair[<a>][<b>] -> params are "pair", <a>, <b>
		value = fmt.Sprintf("%s-%s", string(params[1].(ast.String)), string(params[2].(ast.String)))
	default:
		return nil, nil, fmt.Errorf("unexpected param count %d", len(params))
	}

	// The synthesized module is rooted at the registered prefix (data.reg); the
	// evaluator layers the consumed parameter levels back on top, so id resolves
	// at data.reg.user[<k>].id or data.reg.pair[<a>][<b>].id respectively.
	pkgPath := idx.prefix.String()[len("data."):]
	mod := ast.MustParseModule(fmt.Sprintf("package %s\nid := %q", pkgPath, value))
	return mod.Rules, nil, nil
}

var _ ast.ParametrizedExternalRuleIndex = (*unevenExternalIndex)(nil)

func TestExternalSourceParametrizedUnevenDepth(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.reg")

	for _, tc := range []struct {
		note   string
		module string
		calls  int
	}{
		{
			note: "one level deep (data.reg.user[k])",
			module: `package main
check if data.reg.user["u1"].id == "u1"`,
			calls: 1,
		},
		{
			note: "two levels deep (data.reg.pair[a][b])",
			module: `package main
check if data.reg.pair["a"]["b"].id == "a-b"`,
			calls: 1,
		},
		{
			note: "both depths under one prefix in a single evaluation",
			module: `package main
check if {
	data.reg.user["u1"].id == "u1"
	data.reg.pair["a"]["b"].id == "a-b"
}`,
			calls: 2,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			source := &unevenExternalSource{refs: []ast.Ref{prefix}}
			compiler := setupCompiler(t, prefix, source, ast.MustParseModule(tc.module))

			qrs := runQuery(t, compiler, "data.main.check", nil)
			if len(qrs) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(qrs))
			}
			if got := source.getCallCount(); got != tc.calls {
				t.Errorf("Expected %d lookup(s), got %d", tc.calls, got)
			}
		})
	}
}

func TestExternalSourceParametrizedUnevenDepthInsufficientDepthUndefined(t *testing.T) {
	t.Parallel()

	prefix := ast.MustParseRef("data.reg")
	source := &unevenExternalSource{refs: []ast.Ref{prefix}}

	// "pair" declares arity 3 (the prefix followed by two keys), but this
	// reference supplies only one key. There aren't enough elements to
	// parametrize the source, so the reference is undefined and the source is
	// never consulted.
	staticModule := ast.MustParseModule(`package main
check if data.reg.pair["a"]`)

	compiler := setupCompiler(t, prefix, source, staticModule)

	qrs := runQuery(t, compiler, "data.main.check", nil)
	if len(qrs) != 0 {
		t.Errorf("Expected 0 results for insufficient depth, got %d", len(qrs))
	}
	if got := source.getCallCount(); got != 0 {
		t.Errorf("Expected source not to be consulted, got %d lookup(s)", got)
	}
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

// spExternalSource stands in for a structured-policy plugin with incremental
// loading: the rules arrive per query and compare against data delivered in a
// bundle -- one rule per group.
type spExternalSource struct {
	rules []*ast.Rule
	opts  *ast.ExternalSourceOptions
}

func (s *spExternalSource) Init(context.Context, ast.Ref) (ast.ExternalRuleIndex, error) {
	return &spExternalIndex{rules: s.rules, opts: s.opts}, nil
}

func (*spExternalSource) Refs() []ast.Ref { return []ast.Ref{ast.MustParseRef("data.sp")} }

type spExternalIndex struct {
	rules []*ast.Rule
	opts  *ast.ExternalSourceOptions
}

func (i *spExternalIndex) Opts() *ast.ExternalSourceOptions { return i.opts }

func (i *spExternalIndex) Lookup(context.Context, ...ast.LookupOption) ([]*ast.Rule, ast.ExternalRuleIndex, error) {
	return i.rules, nil, nil
}

// storeDataResolver resolves data refs for the compiler, as the plugin manager
// does for the compilers it installs.
type storeDataResolver struct{ data *ast.Term }

func (r storeDataResolver) Resolve(ref ast.Ref) (ast.Value, error) {
	if !ref.HasPrefix(ast.DefaultRootRef) {
		return nil, ast.UnknownValueErr{}
	}
	v, err := r.data.Value.Find(ref[1:])
	if err != nil {
		return nil, nil
	}
	return v, nil
}

func TestExternalSourceMaterializesIndexData(t *testing.T) {
	const groups = 5

	data := ast.MustParseTerm(`{"groups": {
		"g0": {"members": ["person-0"]},
		"g1": {"members": ["person-1"]},
		"g2": {"members": ["person-2"]},
		"g3": {"members": ["person-3"]},
		"g4": {"members": ["person-4"]}
	}}`)

	rules := make([]*ast.Rule, 0, groups)
	for g := range groups {
		id := "g" + strconv.Itoa(g)
		rules = append(rules, ast.MustParseModule(`package sp

		allow contains "` + id + `" if input.subject in data.groups.` + id + `.members`).Rules[0])
	}

	// what the index was able to exclude, which is the point of reading the data
	matched := func(t *testing.T, maxIndexData int, opts *ast.ExternalSourceOptions) []string {
		t.Helper()

		compiler := ast.NewCompiler()
		compiler.WithExternalSource(ast.MustParseRef("data.sp"), &spExternalSource{rules: rules, opts: opts})
		compiler.Compile(map[string]*ast.Module{})
		if compiler.Failed() {
			t.Fatal(compiler.Errors)
		}
		compiler.MaterializeIndexData(storeDataResolver{data: data}, maxIndexData)

		store := inmem.NewFromObject(map[string]any{
			"groups": map[string]any{
				"g0": map[string]any{"members": []any{"person-0"}},
				"g1": map[string]any{"members": []any{"person-1"}},
				"g2": map[string]any{"members": []any{"person-2"}},
				"g3": map[string]any{"members": []any{"person-3"}},
				"g4": map[string]any{"members": []any{"person-4"}},
			},
		})
		ctx := t.Context()
		txn, err := store.NewTransaction(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Abort(ctx, txn)

		tracer := NewBufferTracer()
		if _, err := NewQuery(ast.MustParseBody("data.sp.allow")).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn).
			WithInput(ast.MustParseTerm(`{"subject": "person-3"}`)).
			WithQueryTracer(tracer).
			Run(ctx); err != nil {
			t.Fatal(err)
		}

		var messages []string
		for _, event := range *tracer {
			if event.Op == IndexOp {
				messages = append(messages, event.Message)
			}
		}
		return messages
	}

	t.Run("only the group the subject is in survives the lookup", func(t *testing.T) {
		if act := matched(t, 1000, nil); len(act) != 1 || act[0] != "(matched 1 rule)" {
			t.Errorf("expected one rule to match, got %v", act)
		}
	})

	t.Run("nothing is read in when the budget is zero", func(t *testing.T) {
		if act := matched(t, 0, nil); len(act) != 1 || act[0] != "(matched 5 rules)" {
			t.Errorf("expected every rule to match, got %v", act)
		}
	})

	// A source handing over pre-compiled rules and skipping index building gets
	// no indices, and so nothing to read data into either.
	t.Run("a source that skips index building is left alone", func(t *testing.T) {
		opts := &ast.ExternalSourceOptions{SkippedStages: []ast.StageID{ast.StageBuildRuleIndices}}
		if act := matched(t, 1000, opts); len(act) != 0 {
			t.Errorf("expected no index lookup at all, got %v", act)
		}
	})
}
