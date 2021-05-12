// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/format"

	"github.com/ghodss/yaml"

	iCache "github.com/open-policy-agent/opa/topdown/cache"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func TestTopDownQueryIDsUnique(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	inputTerm := &ast.Term{}
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package x
  p { 1 }
  p { 2 }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.x.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr)).
		WithInput(inputTerm)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	queryIDs := map[uint64]bool{} // set of seen queryIDs (in EnterOps)
	for _, evt := range tr {
		if evt.Op != EnterOp {
			continue
		}
		if queryIDs[evt.QueryID] {
			t.Errorf("duplicate queryID: %v", evt)
		}
		queryIDs[evt.QueryID] = true
	}
}

func TestTopDownIndexExpr(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package test

		p = true {
		     1 > 0
		     q
		}

		q = true { true }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr))

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	exp := []*ast.Expr{
		ast.MustParseExpr("data.test.p"),
		ast.MustParseExpr("data.test.q"),
	}

	i := 0
	for _, evt := range tr {
		if evt.Op != IndexOp {
			continue
		}

		expr, ok := evt.Node.(*ast.Expr)
		if !ok {
			t.Fatal("Expected expr node but got:", evt.Node)
		}

		exp[i].Index = i
		if ast.Compare(expr, exp[i]) != 0 {
			t.Fatalf("Expected %v but got: %v", exp[i], expr)
		}
		i++
	}
}

func TestTopDownWithKeyword(t *testing.T) {

	tests := []struct {
		note    string
		rules   []string
		modules []string
		input   string
		exp     interface{}
	}{
		{
			// NOTE(tsandall): This case assumes that partial sets are not memoized.
			// If we change that, it'll be harder to test that the comprehension
			// cache is invalidated.
			note: "invalidate comprehension cache",
			exp:  `[[{"b": ["a", "c"]}], [{"b": ["a"]}]]`,
			modules: []string{`package ex
				s[x] {
					x = {v: ks |
						v = input[i]
						ks = {k | v = input[k]}
					}
				}
			`},
			rules: []string{`p = [x, y] {
				x = data.ex.s with input as {"a": "b", "c": "b"}
				y = data.ex.s with input as {"a": "b"}
			}`},
		},
	}

	for _, tc := range tests {
		runTopDownTestCaseWithModules(t, loadSmallTestData(), tc.note, tc.rules, tc.modules, tc.input, tc.exp)
	}
}

func TestTopDownUnsupportedBuiltin(t *testing.T) {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "unsupported_builtin",
	})

	body := ast.MustParseBody(`unsupported_builtin()`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn)
	_, err := q.Run(ctx)

	expected := unsupportedBuiltinErr(body[0].Location)

	if !reflect.DeepEqual(err, expected) {
		t.Fatalf("Expected %v but got: %v", expected, err)
	}

}

func TestTopDownQueryCancellation(t *testing.T) {

	ctx := context.Background()

	compiler := compileModules([]string{
		`
		package test

		p { data.arr[_] = _; test.sleep("1ms") }
		`,
	})

	data := map[string]interface{}{
		"arr": make([]interface{}, 1000),
	}

	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	cancel := NewCancel()

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithCancel(cancel)

	go func() {
		time.Sleep(time.Millisecond * 50)
		cancel.Cancel()
	}()

	qrs, err := query.Run(ctx)
	if err == nil || err.(*Error).Code != CancelErr {
		t.Fatalf("Expected cancel error but got: %v (err: %v)", qrs, err)
	}

}

type contextPropagationMock struct{}

// contextPropagationStore will accumulate values from the contexts provided to
// read calls so that the test can verify that contexts are being propagated as
// expected.
type contextPropagationStore struct {
	storage.WritesNotSupported
	storage.TriggersNotSupported
	storage.PolicyNotSupported
	calls []interface{}
}

func (m *contextPropagationStore) NewTransaction(context.Context, ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (m *contextPropagationStore) Commit(context.Context, storage.Transaction) error {
	return nil
}

func (m *contextPropagationStore) Abort(context.Context, storage.Transaction) {
}

func (m *contextPropagationStore) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	val := ctx.Value(contextPropagationMock{})
	m.calls = append(m.calls, val)
	return nil, nil
}

func TestTopDownContextPropagation(t *testing.T) {

	ctx := context.WithValue(context.Background(), contextPropagationMock{}, "bar")

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"mod1": ast.MustParseModule(`package ex

p[x] { data.a[i] = x }`,
		),
	})

	mockStore := &contextPropagationStore{}
	txn := storage.NewTransactionOrDie(ctx, mockStore)
	query := NewQuery(ast.MustParseBody("data.ex.p")).
		WithCompiler(compiler).
		WithStore(mockStore).
		WithTransaction(txn)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected query error: %v", err)
	}

	expectedCalls := []interface{}{"bar"}

	if !reflect.DeepEqual(expectedCalls, mockStore.calls) {
		t.Fatalf("Expected %v but got: %v", expectedCalls, mockStore.calls)
	}
}

func compileModules(input []string) *ast.Compiler {

	mods := map[string]*ast.Module{}

	for idx, i := range input {
		id := fmt.Sprintf("testMod%d", idx)
		mods[id] = ast.MustParseModule(i)
	}

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		panic(c.Errors)
	}

	return c
}

func compileRules(imports []string, input []string, modules []string) (*ast.Compiler, error) {

	is := []*ast.Import{}
	for _, i := range imports {
		is = append(is, &ast.Import{
			Path: ast.MustParseTerm(i),
		})
	}

	m := &ast.Module{
		Package: ast.MustParsePackage("package generated"),
		Imports: is,
	}

	rules := []*ast.Rule{}
	for i := range input {
		rules = append(rules, ast.MustParseRule(input[i]))
		rules[i].Module = m
	}

	m.Rules = rules

	for i := range rules {
		rules[i].Module = m
	}

	mods := map[string]*ast.Module{"testMod": m}

	for i, s := range modules {
		mods[fmt.Sprintf("testMod%d", i)] = ast.MustParseModule(s)
	}

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		return nil, c.Errors
	}

	return c, nil
}

// loadSmallTestData returns base documents that are referenced
// throughout the topdown test suite.
//
// Avoid the following top-level keys: i, j, k, p, q, r, v, x, y, z.
// These are used for rule names, local variables, etc.
//
func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
        "a": [1,2,3,4],
        "b": {
            "v1": "hello",
            "v2": "goodbye"
        },
        "c": [{
            "x": [true, false, "foo"],
            "y": [null, 3.14159],
            "z": {"p": true, "q": false}
        }],
        "d": {
            "e": ["bar", "baz"]
        },
        "f": [
            {"xs": [1.0], "ys": [2.0]},
            {"xs": [2.0], "ys": [3.0]}
        ],
        "g": {
            "a": [1, 0, 0, 0],
            "b": [0, 2, 0, 0],
            "c": [0, 0, 0, 4]
        },
        "h": [
            [1,2,3],
            [2,3,4]
        ],
        "l": [
            {
                "a": "bob",
                "b": -1,
                "c": [1,2,3,4]
            },
            {
                "a": "alice",
                "b": 1,
                "c": [2,3,4,5],
                "d": null
            }
        ],
		"strings": {
			"foo": 1,
			"bar": 2,
			"baz": 3
		},
		"three": 3,
        "m": [],
		"numbers": [
			"1",
			"2",
			"3",
			"4"
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func setTime(t time.Time) func(*Query) *Query {
	return func(q *Query) *Query {
		return q.WithTime(t)
	}
}

func runTopDownTestCase(t *testing.T, data map[string]interface{}, note string, rules []string, expected interface{}, options ...func(*Query) *Query) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, nil, "", expected, options...)
}

func runTopDownTestCaseWithModules(t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{}) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, modules, input, expected)
}

func runTopDownTestCaseWithContext(ctx context.Context, t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{},
	options ...func(*Query) *Query) {
	t.Helper()

	imports := []string{}
	for k := range data {
		imports = append(imports, "data."+k)
	}

	compiler, err := compileRules(imports, rules, modules)
	if err != nil {
		if _, ok := expected.(error); ok {
			assertError(t, expected, err)
		} else {
			t.Errorf("%v: Compiler error: %v", note, err)
		}
		return
	}

	store := inmem.NewFromObject(data)

	assertTopDownWithPathAndContext(ctx, t, compiler, store, note, []string{"generated", "p"}, input, expected, options...)
}

func assertTopDownWithPathAndContext(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, note string, path []string, input string, expected interface{},
	options ...func(*Query) *Query) {
	t.Helper()

	var inputTerm *ast.Term

	if len(input) > 0 {
		inputTerm = ast.MustParseTerm(input)
	}

	txn := storage.NewTransactionOrDie(ctx, store)

	defer store.Abort(ctx, txn)

	var lhs *ast.Term
	if len(path) == 0 {
		lhs = ast.NewTerm(ast.DefaultRootRef)
	} else {
		lhs = ast.MustParseTerm("data." + strings.Join(path, "."))
	}

	rhs := ast.VarTerm(ast.WildcardPrefix + "result")
	body := ast.NewBody(ast.Equality.Expr(lhs, rhs))

	var requiresSort bool

	if rules := compiler.GetRulesExact(lhs.Value.(ast.Ref)); len(rules) > 0 && rules[0].Head.DocKind() == ast.PartialSetDoc {
		requiresSort = true
	}

	if os.Getenv("OPA_DUMP_TEST") != "" {

		data, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatal(err)
		}

		dump(note, compiler.Modules, data, path, inputTerm, expected, requiresSort)
	}

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	var strictBuiltinErrors bool

	switch expected.(type) {
	case *Error, error:
		strictBuiltinErrors = true
	}

	query := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm).
		WithInterQueryBuiltinCache(interQueryCache).
		WithStrictBuiltinErrors(strictBuiltinErrors)

	var tracer BufferTracer

	if os.Getenv("OPA_TRACE_TEST") != "" {
		query = query.WithTracer(&tracer)
	}

	for _, opt := range options {
		query = opt(query)
	}

	t.Run(note, func(t *testing.T) {
		t.Helper()

		switch e := expected.(type) {
		case *Error, error:
			_, err := query.Run(ctx)
			assertError(t, expected, err)
		case string:
			qrs, err := query.Run(ctx)

			if tracer != nil {
				PrettyTrace(os.Stdout, tracer)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(e) == 0 {
				if len(qrs) != 0 {
					t.Fatalf("Expected undefined result but got: %v", qrs)
				}
				return
			}

			if len(qrs) == 0 {
				t.Fatalf("Expected %v but got undefined", e)
			}

			result, err := ast.JSON(qrs[0][rhs.Value.(ast.Var)].Value)
			if err != nil {
				t.Fatal(err)
			}

			expected := util.MustUnmarshalJSON([]byte(e))

			if requiresSort {
				sort.Sort(resultSet(result.([]interface{})))
				if sl, ok := expected.([]interface{}); ok {
					sort.Sort(resultSet(sl))
				}
			}

			if util.Compare(expected, result) != 0 {
				t.Fatalf("Unexpected result:\nGot: %+v\nExp:\n%+v", result, expected)
			}

			// If the test case involved the input document, re-run it with partial
			// evaluation enabled and input marked as unknown. Then replay the query and
			// verify the partial evaluation result is the same. Note, we cannot evaluate
			// the result of a query against `data` because the queries need to be
			// converted into rules (which would result in recursion.)
			if len(path) > 0 {
				runTopDownPartialTestCase(ctx, t, compiler, store, txn, inputTerm, rhs, body, requiresSort, expected)
			}
		default:
			t.Fatalf("Unexpected expected value type: %+v", e)
		}
	})
}

func runTopDownPartialTestCase(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, txn storage.Transaction, input *ast.Term, output *ast.Term, body ast.Body, requiresSort bool, expected interface{}) {
	t.Helper()

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	partialQuery := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
		WithTransaction(txn).
		WithInterQueryBuiltinCache(interQueryCache)

	partials, support, err := partialQuery.PartialRun(ctx)

	if err != nil {
		t.Fatal("Unexpected error on partial evaluation comparison:", err)
	}

	module := ast.MustParseModule("package topdown_test_partial")
	module.Rules = make([]*ast.Rule, len(partials))
	for i, body := range partials {
		module.Rules[i] = &ast.Rule{
			Head:   ast.NewHead(ast.Var("__result__"), nil, output),
			Body:   body,
			Module: module,
		}
	}

	compiler.Modules["topdown_test_partial"] = module
	for i, module := range support {
		compiler.Modules[fmt.Sprintf("topdown_test_support_%d", i)] = module
	}

	compiler.Compile(compiler.Modules)
	if compiler.Failed() {
		t.Fatal("Unexpected error on partial evaluation result compile:", compiler.Errors)
	}

	query := NewQuery(ast.MustParseBody("data.topdown_test_partial.__result__ = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithInterQueryBuiltinCache(interQueryCache)

	qrs, err := query.Run(ctx)
	if err != nil {
		t.Fatal("Unexpected error on query after partial evaluation:", err)
	}

	if len(qrs) == 0 {
		t.Fatalf("Expected %v but got undefined from query after partial evaluation", expected)
	}

	result, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		t.Fatal(err)
	}

	if requiresSort {
		sort.Sort(resultSet(result.([]interface{})))
		if sl, ok := expected.([]interface{}); ok {
			sort.Sort(resultSet(sl))
		}
	}

	if util.Compare(expected, result) != 0 {
		t.Fatalf("Unexpected result after partial evaluation:\nGot:\n%v\nExp:\n%v", result, expected)
	}
}

type resultSet []interface{}

func (rs resultSet) Less(i, j int) bool {
	return util.Compare(rs[i], rs[j]) < 0
}

func (rs resultSet) Swap(i, j int) {
	tmp := rs[i]
	rs[i] = rs[j]
	rs[j] = tmp
}

func (rs resultSet) Len() int {
	return len(rs)
}

func init() {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	RegisterFunctionalBuiltin1("test.sleep", func(a ast.Value) (ast.Value, error) {
		d, _ := time.ParseDuration(string(a.(ast.String)))
		time.Sleep(d)
		return ast.Null{}, nil
	})

}

var testID = 0
var testIDMutex sync.Mutex

func getTestNamespace() string {
	programCounters := make([]uintptr, 20)
	n := runtime.Callers(0, programCounters)
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more := true; more; {
			var f runtime.Frame
			f, more = frames.Next()
			if strings.HasPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test") {
				return strings.TrimPrefix(strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test"), "TopDown")), "builtin")
			}
		}
	}
	return ""
}

func dump(note string, modules map[string]*ast.Module, data interface{}, docpath []string, input *ast.Term, exp interface{}, requiresSort bool) {

	moduleSet := []string{}
	for _, module := range modules {
		moduleSet = append(moduleSet, string(bytes.ReplaceAll(format.MustAst(module), []byte("\t"), []byte("  "))))
	}

	namespace := getTestNamespace()

	test := map[string]interface{}{
		"note":    namespace + "/" + note,
		"data":    data,
		"modules": moduleSet,
		"query":   strings.Join(append([]string{"data"}, docpath...), ".") + " = x",
	}

	if input != nil {
		test["input_term"] = input.String()
	}

	switch e := exp.(type) {
	case string:
		rs := []map[string]interface{}{}
		if len(e) > 0 {
			exp := util.MustUnmarshalJSON([]byte(e))
			if requiresSort {
				sl := exp.([]interface{})
				sort.Sort(resultSet(sl))
			}
			rs = append(rs, map[string]interface{}{"x": exp})
		}
		test["want_result"] = rs
		if requiresSort {
			test["sort_bindings"] = true
		}
	case error:
		test["want_error_code"] = e.(*Error).Code
		test["want_error"] = e.(*Error).Message
	default:
		panic("Unexpected test expectation. Cowardly refusing to generate test cases.")
	}

	bs, err := yaml.Marshal(map[string]interface{}{"cases": []interface{}{test}})
	if err != nil {
		panic(err)
	}

	dir := path.Join(os.Getenv("OPA_DUMP_TEST"), namespace)

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}

	testIDMutex.Lock()
	testID++
	c := testID
	testIDMutex.Unlock()

	filename := fmt.Sprintf("test-%v-%04d.yaml", namespace, c)

	if err := ioutil.WriteFile(filepath.Join(dir, filename), bs, 0644); err != nil {
		panic(err)
	}

}

func assertError(t *testing.T, expected interface{}, actual error) {
	t.Helper()
	if actual == nil {
		t.Errorf("Expected error but got: %v", actual)
		return
	}

	errString := actual.Error()

	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		t.Errorf("Expected error of type '%T', got '%T'", expected, actual)
	}

	switch e := expected.(type) {
	case Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *ast.Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case ast.Errors:
		for _, astErr := range e {
			assertErrorContains(t, errString, astErr.Code)
			assertErrorContains(t, errString, astErr.Message)
		}
	case error:
		assertErrorContains(t, errString, e.Error())
	}
}

func assertErrorContains(t *testing.T, actualErrMsg string, expected string) {
	t.Helper()
	if !strings.Contains(actualErrMsg, expected) {
		t.Errorf("Expected error '%v' but got: '%v'", expected, actualErrMsg)
	}
}
