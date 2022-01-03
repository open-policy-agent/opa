// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestEventEqual(t *testing.T) {

	a := ast.NewValueMap()
	a.Put(ast.String("foo"), ast.Number("1"))
	b := ast.NewValueMap()
	b.Put(ast.String("foo"), ast.Number("2"))

	tests := []struct {
		a     *Event
		b     *Event
		equal bool
	}{
		{&Event{}, &Event{}, true},
		{&Event{Op: EvalOp}, &Event{Op: EnterOp}, false},
		{&Event{QueryID: 1}, &Event{QueryID: 2}, false},
		{&Event{ParentID: 1}, &Event{ParentID: 2}, false},
		{&Event{Node: ast.MustParseBody("true")}, &Event{Node: ast.MustParseBody("false")}, false},
		{&Event{Node: ast.MustParseBody("true")[0]}, &Event{Node: ast.MustParseBody("false")[0]}, false},
		{&Event{Node: ast.MustParseRule(`p = true { true }`)}, &Event{Node: ast.MustParseRule(`p = true { false }`)}, false},
	}

	for _, tc := range tests {
		if tc.a.Equal(tc.b) != tc.equal {
			var s string
			if tc.equal {
				s = "=="
			} else {
				s = "!="
			}
			t.Errorf("Expected %v %v %v", tc.a, s, tc.b)
		}
	}

}

func TestPrettyTrace(t *testing.T) {
	module := `package test

	p { q[x]; plus(x, 1, n) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `Enter data.test.p = _
| Eval data.test.p = _
| Index data.test.p (matched 1 rule, early exit)
| Enter data.test.p
| | Eval data.test.q[x]
| | Index data.test.q (matched 1 rule)
| | Enter data.test.q
| | | Eval x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | Eval plus(x, 1, n)
| | Exit data.test.p early
| Exit data.test.p = _
Redo data.test.p = _
| Redo data.test.p = _
| Redo data.test.p
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTrace(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestPrettyTraceWithLocation(t *testing.T) {
	module := `package test

	p { q[x]; plus(x, 1, n) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1     Enter data.test.p = _
query:1     | Eval data.test.p = _
query:1     | Index data.test.p (matched 1 rule, early exit)
query:3     | Enter data.test.p
query:3     | | Eval data.test.q[x]
query:3     | | Index data.test.q (matched 1 rule)
query:4     | | Enter data.test.q
query:4     | | | Eval x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:3     | | Eval plus(x, 1, n)
query:3     | | Exit data.test.p early
query:1     | Exit data.test.p = _
query:1     Redo data.test.p = _
query:1     | Redo data.test.p = _
query:3     | Redo data.test.p
query:3     | | Redo plus(x, 1, n)
query:3     | | Redo data.test.q[x]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestPrettyTraceWithLocationTruncatedPaths(t *testing.T) {
	ctx := context.Background()

	compiler := ast.MustCompileModules(map[string]string{
		"authz_bundle/com/foo/bar/baz/qux/acme/corp/internal/authz/policies/abac/v1/beta/policy.rego": `package test

		import data.utils.q

		p = true { q[x]; plus(x, 1, n) }
		`,
		"authz_bundle/com/foo/bar/baz/qux/acme/corp/internal/authz/policies/utils/utils.rego": `package utils

		q[x] { x = data.a[_] }
		`,
	})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1                                                              Enter data.test.p = _
query:1                                                              | Eval data.test.p = _
query:1                                                              | Index data.test.p (matched 1 rule, early exit)
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | Enter data.test.p
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Eval data.utils.q[x]
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Index data.utils.q (matched 1 rule)
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | Enter data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Eval x = data.a[_]
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Exit data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | Redo data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Redo x = data.a[_]
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Exit data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | Redo data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Redo x = data.a[_]
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Exit data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | Redo data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Redo x = data.a[_]
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Exit data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | Redo data.utils.q
authz_bundle/...ternal/authz/policies/utils/utils.rego:3             | | | Redo x = data.a[_]
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Eval plus(x, 1, n)
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Exit data.test.p early
query:1                                                              | Exit data.test.p = _
query:1                                                              Redo data.test.p = _
query:1                                                              | Redo data.test.p = _
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | Redo data.test.p
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Redo plus(x, 1, n)
authz_bundle/...ternal/authz/policies/abac/v1/beta/policy.rego:5     | | Redo data.utils.q[x]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestPrettyTracePartialWithLocationTruncatedPaths(t *testing.T) {
	ctx := context.Background()

	compiler := ast.MustCompileModules(map[string]string{
		"authz_bundle/com/foo/bar/baz/qux/acme/corp/internal/authz/policies/rbac/v1/beta/policy.rego": `
		package example_rbac

		default allow = false

		allow {
		    data.utils.user_has_role[role_name]

		    data.utils.role_has_permission[role_name]
		}

		`,
		"authz_bundle/com/foo/bar/baz/qux/acme/corp/internal/authz/policies/utils/user.rego": `
		package utils

		user_has_role[role_name] {
		    role_binding = data.bindings[_]
		    role_binding.role = role_name
		    role_binding.user = input.subject.user
		}

		role_has_permission[role_name] {
		    role = data.roles[_]
		    role.name = role_name
		    role.operation = input.action.operation
		    role.resource = input.action.resource
		}
		`,
	})

	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
    "roles": [
        {
            "operation": "read",
            "resource": "widgets",
            "name": "widget-reader"
        },
        {
            "operation": "write",
            "resource": "widgets",
            "name": "widget-writer"
        }
    ],
    "bindings": [
        {
            "user": "inspector-alice",
            "role": "widget-reader"
        },
        {
            "user": "maker-bob",
            "role": "widget-writer"
        }
    ]
	}`), &data)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.example_rbac.allow")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
		WithTracer(tracer)

	_, _, err = query.PartialRun(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1                                                              Enter data.example_rbac.allow
query:1                                                              | Eval data.example_rbac.allow
query:1                                                              | Index data.example_rbac.allow (matched 1 rule, early exit)
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:6     | Enter data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:7     | | Eval data.utils.user_has_role[role_name]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:7     | | Index data.utils.user_has_role (matched 1 rule)
authz_bundle/...ternal/authz/policies/utils/user.rego:4              | | Enter data.utils.user_has_role
authz_bundle/...ternal/authz/policies/utils/user.rego:5              | | | Eval role_binding = data.bindings[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:6              | | | Eval role_binding.role = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Eval role_binding.user = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Save "inspector-alice" = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:4              | | | Exit data.utils.user_has_role
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Eval data.utils.role_has_permission[role_name]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Index data.utils.role_has_permission (matched 1 rule)
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | Enter data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Eval role = data.roles[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Eval role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Eval role.operation = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Save "read" = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Eval role.resource = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Save "widgets" = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | | Exit data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:6     | | Exit data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:6     | Redo data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Redo data.utils.role_has_permission[role_name]
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | Redo data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Redo role.resource = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Redo role.operation = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Redo role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Redo role = data.roles[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Eval role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Fail role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Redo role = data.roles[_]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:7     | | Redo data.utils.user_has_role[role_name]
authz_bundle/...ternal/authz/policies/utils/user.rego:4              | | Redo data.utils.user_has_role
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Redo role_binding.user = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:6              | | | Redo role_binding.role = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:5              | | | Redo role_binding = data.bindings[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:6              | | | Eval role_binding.role = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Eval role_binding.user = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Save "maker-bob" = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:4              | | | Exit data.utils.user_has_role
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Eval data.utils.role_has_permission[role_name]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Index data.utils.role_has_permission (matched 1 rule)
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | Enter data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Eval role = data.roles[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Eval role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Fail role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Redo role = data.roles[_]
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Eval role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Eval role.operation = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Save "write" = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Eval role.resource = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Save "widgets" = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | | Exit data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:6     | | Exit data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:6     | Redo data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:9     | | Redo data.utils.role_has_permission[role_name]
authz_bundle/...ternal/authz/policies/utils/user.rego:10             | | Redo data.utils.role_has_permission
authz_bundle/...ternal/authz/policies/utils/user.rego:14             | | | Redo role.resource = input.action.resource
authz_bundle/...ternal/authz/policies/utils/user.rego:13             | | | Redo role.operation = input.action.operation
authz_bundle/...ternal/authz/policies/utils/user.rego:12             | | | Redo role.name = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:11             | | | Redo role = data.roles[_]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:7     | | Redo data.utils.user_has_role[role_name]
authz_bundle/...ternal/authz/policies/utils/user.rego:4              | | Redo data.utils.user_has_role
authz_bundle/...ternal/authz/policies/utils/user.rego:7              | | | Redo role_binding.user = input.subject.user
authz_bundle/...ternal/authz/policies/utils/user.rego:6              | | | Redo role_binding.role = role_name
authz_bundle/...ternal/authz/policies/utils/user.rego:5              | | | Redo role_binding = data.bindings[_]
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:4     | Enter data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:4     | | Eval true
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:4     | | Exit data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:4     | Redo data.example_rbac.allow
authz_bundle/...ternal/authz/policies/rbac/v1/beta/policy.rego:4     | | Redo true
query:1                                                              | Save data.partial.example_rbac.allow = _
query:1                                                              | Save _
query:1                                                              | Exit data.example_rbac.allow
query:1                                                              Redo data.example_rbac.allow
query:1                                                              | Fail data.example_rbac.allow
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Errorf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Errorf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}

	if t.Failed() {
		fmt.Println("Trace output:")
		fmt.Println(buf.String())
	}
}

func TestTraceDuplicate(t *testing.T) {
	// NOTE(sr): We're explicitly bypassing a caching optimization here:
	// When the first query for a partial is `p[x]`, and `x` is not ground,
	// we'll have the evaluation eval the full extent of the partial and
	// cache that. Thus the second `p[1]` here will not trigger a duplicate
	// event, because the query eval uses a different code path.
	// Having `p[1]` queried first will side-step the caching optimization.
	module := `package test

	p[1]
	p[2]
	p[1]
	`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p[1]; data.test.p[x] = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	n := 0
	for _, event := range *tracer {
		if event.Op == DuplicateOp {
			n++
		}
	}

	if n != 1 {
		t.Fatalf("Expected one duplicate event but got %v", n)
	}
}

func TestTraceNote(t *testing.T) {
	module := `package test

	p { q[x]; plus(x, 1, n); trace(sprintf("n=%v", [n])) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `Enter data.test.p = _
| Eval data.test.p = _
| Index data.test.p (matched 1 rule, early exit)
| Enter data.test.p
| | Eval data.test.q[x]
| | Index data.test.q (matched 1 rule)
| | Enter data.test.q
| | | Eval x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Redo data.test.q
| | | Redo x = data.a[_]
| | Eval plus(x, 1, n)
| | Eval sprintf("n=%v", [n], __local0__)
| | Eval trace(__local0__)
| | Note "n=2"
| | Exit data.test.p early
| Exit data.test.p = _
Redo data.test.p = _
| Redo data.test.p = _
| Redo data.test.p
| | Redo trace(__local0__)
| | Redo sprintf("n=%v", [n], __local0__)
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTrace(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestTraceNoteWithLocation(t *testing.T) {
	module := `package test

	p { q[x]; plus(x, 1, n); trace(sprintf("n=%v", [n])) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1     Enter data.test.p = _
query:1     | Eval data.test.p = _
query:1     | Index data.test.p (matched 1 rule, early exit)
query:3     | Enter data.test.p
query:3     | | Eval data.test.q[x]
query:3     | | Index data.test.q (matched 1 rule)
query:4     | | Enter data.test.q
query:4     | | | Eval x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:4     | | | Exit data.test.q
query:4     | | Redo data.test.q
query:4     | | | Redo x = data.a[_]
query:3     | | Eval plus(x, 1, n)
query:3     | | Eval sprintf("n=%v", [n], __local0__)
query:3     | | Eval trace(__local0__)
query:3     | | Note "n=2"
query:3     | | Exit data.test.p early
query:1     | Exit data.test.p = _
query:1     Redo data.test.p = _
query:1     | Redo data.test.p = _
query:3     | Redo data.test.p
query:3     | | Redo trace(__local0__)
query:3     | | Redo sprintf("n=%v", [n], __local0__)
query:3     | | Redo plus(x, 1, n)
query:3     | | Redo data.test.q[x]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")
	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestMultipleTracers(t *testing.T) {

	ctx := context.Background()

	buf1 := NewBufferTracer()
	buf2 := NewBufferTracer()
	q := NewQuery(ast.MustParseBody("a = 1")).
		WithTracer(buf1).
		WithTracer(buf2)

	_, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(*buf1) != len(*buf2) {
		t.Fatalf("Expected buffer lengths to be equal but got: %d and %d", len(*buf1), len(*buf2))
	}

	for i := range *buf1 {
		if !(*buf1)[i].Equal((*buf2)[i]) {
			t.Fatalf("Expected all events to be equal but at index %d got %v and %v", i, (*buf1)[i], (*buf2)[i])
		}
	}

}

func TestTraceRewrittenQueryVars(t *testing.T) {
	module := `package test

	y = [1, 2, 3]`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	queryCompiler := compiler.QueryCompiler()
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiledQuery, err := queryCompiler.Compile(ast.MustParseBody("z := {a | a := data.y[_]}"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	tracer := NewBufferTracer()
	query := NewQuery(compiledQuery).
		WithQueryCompiler(queryCompiler).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err = query.Run(ctx)
	if err != nil {
		panic(err)
	}

	foundQueryVar := false

	for _, event := range *tracer {
		if event.LocalMetadata != nil {
			name, ok := event.LocalMetadata["__localq1__"]
			if ok && name.Name == "z" {
				foundQueryVar = true
				break
			}
		}
	}

	if !foundQueryVar {
		t.Error("Expected to find trace with rewritten var 'z' -> '__localq__")
	}

	// Rewrite the vars in the first event (which is a query) and verify that
	// that vars have been mapped to user-provided names.
	cpy := rewrite((*tracer)[0])
	node := cpy.Node.(ast.Body)
	exp := ast.MustParseBody("z = {a | a = data.y[_]}")

	if !node.Equal(exp) {
		t.Errorf("Expected %v but got %v", exp, node)
	}
}

func TestTraceRewrittenVarsIssue2022(t *testing.T) {

	input := &Event{
		Node: &ast.Expr{
			Terms: ast.VarTerm("foo"),
		},
		LocalMetadata: map[ast.Var]VarMetadata{
			ast.Var("foo"): {Name: ast.Var("bar")},
		},
	}

	output := rewrite(input)

	if input.Node == output.Node {
		t.Fatal("expected node to have been copied")
	} else if !output.Node.(*ast.Expr).Equal(ast.NewExpr(ast.VarTerm("bar"))) {
		t.Fatal("expected copy to contain rewritten var")
	}
}

func TestShortTraceFileNames(t *testing.T) {
	longFilePath1 := "/really/long/file/path/longer/than/most/would/really/ever/be/policy.rego"
	longFilePath1Similar := "/really/long/file/path/longer/than/most/policy.rego"
	longFilePath2 := "GfjEjnMA6coNiPoMoRMVk7KeorGeRmjRkIYUsWtr564SQ7yDo4Yss2SoN8PMoe0TOfVaNFd1HQbC9NhK.rego"
	longFilePath3 := "RqS50uWAOxqqHmzdKVM3OCVsZDb12FJikUYHhz9pNqMWx3wjeQBKY3UYXsJXzYGOzuYZbidag5SfKVdk.rego"

	cases := []struct {
		note            string
		trace           []*Event
		expectedNames   map[string]string
		expectedLongest int
	}{
		{
			note:            "empty trace",
			trace:           nil,
			expectedNames:   map[string]string{},
			expectedLongest: 0,
		},
		{
			note: "no locations",
			trace: []*Event{
				{Op: EnterOp, Node: ast.MustParseBody("true")},
				{Op: EvalOp, Node: ast.MustParseBody("true")},
			},
			expectedNames:   map[string]string{},
			expectedLongest: 0,
		},
		{
			note: "no file names",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), "", 1, 1)},
				{Location: ast.NewLocation([]byte("foo2"), "", 2, 1)},
				{Location: ast.NewLocation([]byte("foo100"), "", 100, 1)},
				{Location: ast.NewLocation([]byte("foo3"), "", 3, 1)},
				{Location: ast.NewLocation([]byte("foo4"), "", 4, 1)},
			},
			expectedNames:   map[string]string{},
			expectedLongest: minLocationWidth + len(":100"),
		},
		{
			note: "single file name not shortened",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), "policy.rego", 1, 1)},
			},
			expectedNames: map[string]string{
				"policy.rego": "policy.rego",
			},
			expectedLongest: len("policy.rego:1"),
		},
		{
			note: "single file name not shortened different rows",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), "policy.rego", 1, 1)},
				{Location: ast.NewLocation([]byte("foo1234"), "policy.rego", 1234, 1)},
				{Location: ast.NewLocation([]byte("foo12"), "policy.rego", 12, 1)},
				{Location: ast.NewLocation([]byte("foo123"), "policy.rego", 123, 1)},
			},
			expectedNames: map[string]string{
				"policy.rego": "policy.rego",
			},
			expectedLongest: len("policy.rego:1234"),
		},
		{
			note: "multiple files name not shortened",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("a1"), "a.rego", 1, 1)},
				{Location: ast.NewLocation([]byte("a1234"), "a.rego", 1234, 1)},
				{Location: ast.NewLocation([]byte("x1"), "x.rego", 12, 1)},
				{Location: ast.NewLocation([]byte("foo123"), "policy.rego", 123, 1)},
			},
			expectedNames: map[string]string{
				"a.rego":      "a.rego",
				"x.rego":      "x.rego",
				"policy.rego": "policy.rego",
			},
			expectedLongest: len("policy.rego:123"),
		},
		{
			note: "single file name shortened",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), longFilePath1, 1, 1)},
			},
			expectedNames: map[string]string{
				longFilePath1: "/really/...h/longer/than/most/would/really/ever/be/policy.rego",
			},
			expectedLongest: maxIdealLocationWidth,
		},
		{
			note: "single file name shortened different rows",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), longFilePath1, 1, 1)},
				{Location: ast.NewLocation([]byte("foo1234"), longFilePath1, 1234, 1)},
				{Location: ast.NewLocation([]byte("foo123"), longFilePath1, 123, 1)},
				{Location: ast.NewLocation([]byte("foo12"), longFilePath1, 12, 1)},
			},
			expectedNames: map[string]string{
				longFilePath1: "/really/...onger/than/most/would/really/ever/be/policy.rego",
			},
			expectedLongest: maxIdealLocationWidth,
		},
		{
			note: "multiple files name shortened different rows",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("similar1"), longFilePath1Similar, 1, 1)},
				{Location: ast.NewLocation([]byte("foo1234"), longFilePath1, 1234, 1)},
				{Location: ast.NewLocation([]byte("similar12"), longFilePath1Similar, 12, 1)},
				{Location: ast.NewLocation([]byte("foo123"), longFilePath1, 123, 1)},
			},
			expectedNames: map[string]string{
				longFilePath1:        "/really/...onger/than/most/would/really/ever/be/policy.rego",
				longFilePath1Similar: "/really/...onger/than/most/policy.rego",
			},
			expectedLongest: maxIdealLocationWidth,
		},
		{
			note: "multiple files name cannot be shortened",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), longFilePath2, 1, 1)},
				{Location: ast.NewLocation([]byte("foo1234"), longFilePath3, 1234, 1)},
			},
			expectedNames: map[string]string{
				longFilePath2: longFilePath2,
				longFilePath3: longFilePath3,
			},
			expectedLongest: len(longFilePath3 + ":1234"),
		},
		{
			note: "single file name shortened no leading slash",
			trace: []*Event{
				{Location: ast.NewLocation([]byte("foo1"), longFilePath1[1:], 1, 1)},
			},
			expectedNames: map[string]string{
				longFilePath1[1:]: "really/...th/longer/than/most/would/really/ever/be/policy.rego",
			},
			expectedLongest: maxIdealLocationWidth,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			actualNames, actualLongest := getShortenedFileNames(tc.trace)
			if actualLongest != tc.expectedLongest {
				t.Errorf("Expected longest location to be %d, got %d", tc.expectedLongest, actualLongest)
			}

			if !reflect.DeepEqual(actualNames, tc.expectedNames) {
				t.Errorf("Expected %+v got %+v", tc.expectedNames, actualNames)
			}
		})
	}
}

func TestBufferTracerTraceConfig(t *testing.T) {
	ct := QueryTracer(NewBufferTracer())
	conf := ct.Config()

	expected := TraceConfig{
		PlugLocalVars: true,
	}

	if !reflect.DeepEqual(expected, conf) {
		t.Fatalf("Expected config: %+v, got %+v", expected, conf)
	}
}
