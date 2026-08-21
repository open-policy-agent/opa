// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// dataResolver answers the compiler's data lookups from a snapshot of the store.
// This is the part a caller enabling WithIndexData has to provide; a
// store-backed one lives with the plugin that recompiles on data changes.
type dataResolver struct {
	data ast.Value
}

func newDataResolver(t *testing.T, ctx context.Context, store storage.Store) dataResolver {
	t.Helper()

	v, err := storage.ReadOne(ctx, store, storage.Path{})
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	data, err := ast.InterfaceToValue(v)
	if err != nil {
		t.Fatalf("convert data: %v", err)
	}
	return dataResolver{data: data}
}

func (r dataResolver) Resolve(ref ast.Ref) (ast.Value, error) {
	if !ref.HasPrefix(ast.DefaultRootRef) {
		return nil, ast.UnknownValueErr{}
	}
	v, err := r.data.Find(ref[1:])
	if err != nil {
		return nil, nil // undefined, not an error
	}
	return v, nil
}

const groupsModule = `package test

allow if input.subject in data.groups.admins.members

allow if input.subject in data.groups.devs.members
`

func TestIndexMaterializedDataEndToEnd(t *testing.T) {
	ctx := t.Context()

	store := inmem.NewFromObject(map[string]any{
		"groups": map[string]any{
			"admins": map[string]any{"members": []any{"alice"}},
			"devs":   map[string]any{"members": []any{"bob"}},
		},
	})

	prepare := func(t *testing.T, maxCollection int) PreparedEvalQuery {
		t.Helper()

		resolver := newDataResolver(t, ctx, store)
		pq, err := New(
			Query("data.test.allow"),
			Module("test.rego", groupsModule),
			Store(store),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(resolver, maxCollection)
			}),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}
		return pq
	}

	eval := func(t *testing.T, pq PreparedEvalQuery, subject string, opts ...EvalOption) bool {
		t.Helper()

		rs, err := pq.Eval(ctx, append(opts, EvalInput(map[string]any{"subject": subject}))...)
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if len(rs) == 0 {
			return false
		}
		allowed, ok := rs[0].Expressions[0].Value.(bool)
		if !ok {
			t.Fatalf("expected boolean result, got %[1]v (%[1]T)", rs[0].Expressions[0].Value)
		}
		return allowed
	}

	rulesEntered := func(tracer *topdown.BufferTracer) int {
		count := 0
		for _, event := range *tracer {
			if event.Op == topdown.EnterOp {
				if _, ok := event.Node.(*ast.Rule); ok {
					count++
				}
			}
		}
		return count
	}

	t.Run("materialized values prune without resolving any collection", func(t *testing.T) {
		pq := prepare(t, 1000)
		tracer := topdown.NewBufferTracer()

		if !eval(t, pq, "alice", EvalQueryTracer(tracer)) {
			t.Error("expected alice to be allowed")
		}
		if exp, act := 1, rulesEntered(tracer); exp != act {
			t.Errorf("expected %d rule(s) to be evaluated, got %d", exp, act)
		}
		if eval(t, pq, "eve") {
			t.Error("expected eve not to be allowed")
		}
	})

	t.Run("compiler reports the collection refs", func(t *testing.T) {
		resolver := newDataResolver(t, ctx, store)
		c := ast.NewCompiler()
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModuleWithOpts(groupsModule, ast.ParserOptions{}),
		})
		if c.Failed() {
			t.Fatal(c.Errors)
		}
		c.MaterializeIndexData(resolver, 1000)

		refs := c.IndexDataRefs()
		if len(refs) != 2 {
			t.Fatalf("expected 2 materialized refs, got %v", refs)
		}
		for _, exp := range []string{"data.groups.admins.members", "data.groups.devs.members"} {
			if !refs[0].Equal(ast.MustParseRef(exp)) && !refs[1].Equal(ast.MustParseRef(exp)) {
				t.Errorf("expected %v among the materialized refs, got %v", exp, refs)
			}
		}
	})

	// A `with` statement replaces the collection for one evaluation only, so the
	// materialized values say nothing about it: the index stops pruning instead of
	// answering from stale data.
	t.Run("honors with-replacement of a materialized collection", func(t *testing.T) {
		resolver := newDataResolver(t, ctx, store)
		pq, err := New(
			Query(`data.test.allow with data.groups.admins.members as ["eve"]`),
			Module("test.rego", groupsModule),
			Store(store),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(resolver, 1000)
			}),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}

		tracer := topdown.NewBufferTracer()
		if !eval(t, pq, "eve", EvalQueryTracer(tracer)) {
			t.Error("expected eve to be allowed through the replaced collection")
		}
		if eval(t, pq, "alice") {
			t.Error("expected alice not to be allowed through the replaced collection")
		}

		// Only the rule over the replaced collection loses its pruning: the one
		// over data.groups.devs.members is still indexed on its own values.
		if exp, act := 1, rulesEntered(tracer); exp != act {
			t.Errorf("expected %d rule(s) to be evaluated, got %d", exp, act)
		}
	})

	// ... and so does a replacement *inside* a materialized collection's path.
	t.Run("honors with-replacement above a materialized collection", func(t *testing.T) {
		resolver := newDataResolver(t, ctx, store)
		pq, err := New(
			Query(`data.test.allow with data.groups as {"admins": {"members": ["eve"]}}`),
			Module("test.rego", groupsModule),
			Store(store),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(resolver, 1000)
			}),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}

		if !eval(t, pq, "eve") {
			t.Error("expected eve to be allowed through the replaced collection")
		}
	})

	// A replacement *inside* a materialized collection leaves the ref itself in
	// place while changing what it holds, so it has to be caught by comparing
	// refs in both directions (refStack.Overlaps, not Prefixed).
	t.Run("honors with-replacement inside a materialized collection", func(t *testing.T) {
		membersStore := inmem.NewFromObject(map[string]any{
			"members": map[string]any{"a": "alice", "b": "bob"},
		})
		module := `package test

		allow if input.subject in data.members`

		pq, err := New(
			Query(`data.test.allow with data.members.a as "eve"`),
			Module("test.rego", module),
			Store(membersStore),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(newDataResolver(t, ctx, membersStore), 1000)
			}),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}

		if !eval(t, pq, "eve") {
			t.Error("expected eve to be allowed through the replaced member")
		}
		if eval(t, pq, "alice") {
			t.Error("expected alice not to be allowed once her entry was replaced")
		}
	})

	t.Run("partial evaluation keeps all candidates", func(t *testing.T) {
		resolver := newDataResolver(t, ctx, store)
		pq, err := New(
			Query("data.test.allow"),
			Module("test.rego", groupsModule),
			Store(store),
			Unknowns([]string{"input"}),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(resolver, 1000)
			}),
		).PrepareForPartial(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}

		pqs, err := pq.Partial(ctx, EvalInput(map[string]any{"subject": "eve"}))
		if err != nil {
			t.Fatalf("partial: %v", err)
		}
		if exp, act := 2, len(pqs.Queries); exp != act {
			t.Errorf("expected %d partially evaluated queries, got %d: %v", exp, act, pqs.Queries)
		}
	})

	// The contract of WithIndexData: a data change alone doesn't reach the
	// index, so whoever enables it has to recompile.
	t.Run("a data change needs a recompile", func(t *testing.T) {
		pq := prepare(t, 1000)

		if err := storage.WriteOne(ctx, store, storage.AddOp,
			storage.MustParsePath("/groups/admins/members/-"), "eve"); err != nil {
			t.Fatalf("write: %v", err)
		}

		if eval(t, pq, "eve") {
			t.Error("expected the index built before the write to still prune eve")
		}
		if !eval(t, prepare(t, 1000), "eve") {
			t.Error("expected eve to be allowed after recompiling")
		}
	})

	// A collection past the cap isn't unrolled -- but it is still watched, so
	// that shrinking it below the cap (or any other change) is noticed.
	t.Run("collections over the cap are watched but not materialized", func(t *testing.T) {
		capStore := inmem.NewFromObject(map[string]any{
			"groups": map[string]any{
				"admins": map[string]any{"members": []any{"alice", "eve"}},
				"devs":   map[string]any{"members": []any{"bob"}},
			},
		})

		prepareCapped := func(t *testing.T, maxCollection int) PreparedEvalQuery {
			t.Helper()

			resolver := newDataResolver(t, ctx, capStore)
			pq, err := New(
				Query("data.test.allow"),
				Module("test.rego", groupsModule),
				Store(capStore),
				CompilerHook(func(c *ast.Compiler) {
					c.MaterializeIndexData(resolver, maxCollection)
				}),
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			return pq
		}

		// "zed" is in neither collection: with both materialized, no rule survives
		// the lookup; with admins over the cap, its rule has to be evaluated.
		for _, tc := range []struct {
			maxCollection int
			entered       int
		}{
			{maxCollection: 1000, entered: 0},
			{maxCollection: 1, entered: 1},
		} {
			tracer := topdown.NewBufferTracer()
			if eval(t, prepareCapped(t, tc.maxCollection), "zed", EvalQueryTracer(tracer)) {
				t.Error("expected zed not to be allowed")
			}
			if act := rulesEntered(tracer); tc.entered != act {
				t.Errorf("maxCollection %d: expected %d rule(s) evaluated, got %d",
					tc.maxCollection, tc.entered, act)
			}
		}

		resolver := newDataResolver(t, ctx, capStore)
		c := ast.NewCompiler()
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModuleWithOpts(groupsModule, ast.ParserOptions{}),
		})
		if c.Failed() {
			t.Fatal(c.Errors)
		}
		c.MaterializeIndexData(resolver, 1)

		if refs := c.IndexDataRefs(); len(refs) != 2 {
			t.Errorf("expected both collections to be watched, got %v", refs)
		}
	})
}

// One rule per group, membership through a data ref, resource conditions
// unknown: the Compile API shape. The unknowns say nothing about the groups, so
// the index has to keep pruning on the subject -- which is the whole point of
// unrolling them.
func TestIndexMaterializedDataPartialEval(t *testing.T) {
	ctx := t.Context()

	store := inmem.NewFromObject(map[string]any{
		"groups": map[string]any{
			"admins": map[string]any{"members": []any{"alice"}},
			"devs":   map[string]any{"members": []any{"bob"}},
		},
	})

	const module = `package test

filter if {
	input.subject in data.groups.admins.members
	input.resource.root == "admins-root"
}

filter if {
	input.subject in data.groups.devs.members
	input.resource.root == "devs-root"
}
`

	partial := func(t *testing.T, unknowns []string) (*PartialQueries, int) {
		t.Helper()

		resolver := newDataResolver(t, ctx, store)
		pq, err := New(
			Query("data.test.filter = true"),
			Module("test.rego", module),
			Store(store),
			Unknowns(unknowns),
			CompilerHook(func(c *ast.Compiler) {
				c.MaterializeIndexData(resolver, 1000)
			}),
		).PrepareForPartial(ctx)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}

		tracer := topdown.NewBufferTracer()
		pqs, err := pq.Partial(ctx,
			EvalInput(map[string]any{"subject": "alice"}),
			EvalQueryTracer(tracer))
		if err != nil {
			t.Fatalf("partial: %v", err)
		}

		entered := 0
		for _, event := range *tracer {
			if event.Op == topdown.EnterOp {
				if _, ok := event.Node.(*ast.Rule); ok {
					entered++
				}
			}
		}

		return pqs, entered
	}

	t.Run("prunes on data the unknowns don't cover", func(t *testing.T) {
		pqs, entered := partial(t, []string{"input.resource"})

		if exp, act := 1, len(pqs.Queries); exp != act {
			t.Errorf("expected %d query, got %d: %v", exp, act, pqs.Queries)
		}
		if exp, act := 1, entered; exp != act {
			t.Errorf("expected %d rule(s) to be evaluated, got %d", exp, act)
		}
	})

	t.Run("keeps candidates when the collections are unknown", func(t *testing.T) {
		_, entered := partial(t, []string{"input.resource", "data.groups"})

		if exp, act := 2, entered; exp != act {
			t.Errorf("expected %d rule(s) to be evaluated, got %d", exp, act)
		}
	})
}

// A data change doesn't recompile anything: the refs it moved are marked, and the
// values read from them stop excluding rules until the next materialization.
func TestIndexMaterializedDataMarkedStale(t *testing.T) {
	ctx := t.Context()

	store := inmem.NewFromObject(map[string]any{
		"groups": map[string]any{
			"admins": map[string]any{"members": []any{"alice"}},
			"devs":   map[string]any{"members": []any{"bob"}},
		},
	})

	var compiler *ast.Compiler
	pq, err := New(
		Query("data.test.allow"),
		Module("test.rego", groupsModule),
		Store(store),
		CompilerHook(func(c *ast.Compiler) {
			compiler = c
			c.MaterializeIndexData(newDataResolver(t, ctx, store), 1000)
		}),
	).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	eval := func(t *testing.T, subject string) (bool, int) {
		t.Helper()

		tracer := topdown.NewBufferTracer()
		rs, err := pq.Eval(ctx,
			EvalInput(map[string]any{"subject": subject}),
			EvalQueryTracer(tracer))
		if err != nil {
			t.Fatalf("eval: %v", err)
		}

		entered := 0
		for _, event := range *tracer {
			if event.Op == topdown.EnterOp {
				if _, ok := event.Node.(*ast.Rule); ok {
					entered++
				}
			}
		}

		if len(rs) == 0 {
			return false, entered
		}
		allowed, ok := rs[0].Expressions[0].Value.(bool)
		if !ok {
			t.Fatalf("expected boolean result, got %[1]v (%[1]T)", rs[0].Expressions[0].Value)
		}
		return allowed, entered
	}

	if allowed, entered := eval(t, "eve"); allowed || entered != 0 {
		t.Fatalf("expected eve to be denied without evaluating a rule, got allowed=%v entered=%d", allowed, entered)
	}

	// eve joins the admins, and the data ref is marked instead of recompiled
	if err := storage.WriteOne(ctx, store, storage.AddOp,
		storage.MustParsePath("/groups/admins/members/-"), "eve"); err != nil {
		t.Fatalf("write: %v", err)
	}
	compiler.MarkIndexDataStale([]ast.Ref{ast.MustParseRef("data.groups.admins.members")})

	// the admins rule has to be evaluated again, and eve is allowed by it -- while
	// the devs rule stays indexed on the values it read
	if allowed, entered := eval(t, "eve"); !allowed || entered != 1 {
		t.Errorf("expected eve to be allowed after one rule, got allowed=%v entered=%d", allowed, entered)
	}

	// materializing again picks the new members up, and stops marking
	compiler.MaterializeIndexData(newDataResolver(t, ctx, store), 1000)
	if compiler.IndexDataStale(ast.MustParseRef("data.groups.admins.members")) {
		t.Error("expected materializing to clear the mark")
	}
	if allowed, entered := eval(t, "eve"); !allowed || entered != 1 {
		t.Errorf("expected eve to be allowed after one rule, got allowed=%v entered=%d", allowed, entered)
	}
}
