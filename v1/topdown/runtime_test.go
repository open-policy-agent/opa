// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestOPARuntime(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	q := NewQuery(ast.MustParseBody("opa.runtime(x)")) // no runtime info
	rs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected result set to contain exactly one result")
	}

	term := rs[0][ast.Var("x")]
	exp := ast.ObjectTerm()

	if ast.Compare(term, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, term)
	}

	q = NewQuery(ast.MustParseBody("opa.runtime(x)")).WithRuntime(ast.MustParseTerm(`{"config": {"a": 1}}`))
	rs, err = q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected result set to contain exactly one result")
	}

	term = rs[0][ast.Var("x")]
	exp = ast.MustParseTerm(`{"config": {"a": 1}}`)

	if ast.Compare(term, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, term)
	}

}

func TestOPARuntimeConfigMasking(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	q := NewQuery(ast.MustParseBody("opa.runtime(x)")).WithRuntime(ast.MustParseTerm(`{"config": {
		"labels": {"foo": "bar"},
		"services": {
			"foo": {
				"url": "https://remote.example.com",
				"credentials": {
					"oauth2": {
						"client_id": "opa_client",
						"client_secret": "sup3rs3cr3t"
					}
				}
			}
		}
	}}`))
	rs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 {
		t.Fatal("Expected result set to contain exactly one result")
	}

	term := rs[0][ast.Var("x")]
	exp := ast.MustParseTerm(`{"config": {
		"labels": {"foo": "bar"},
		"services": {
			"foo": {
				"url": "https://remote.example.com"
			}
		}
	}}`)

	if ast.Compare(term, exp) != 0 {
		t.Fatalf("Expected %v but got %v", exp, term)
	}
}

func TestOPARuntimeRedactionPerRuntime(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	a := ast.MustParseTerm(`{"config": {"labels": {"id": "a"}, "services": {"s": {"url": "https://a", "credentials": {"bearer": {"token": "a"}}}}}}`)
	b := ast.MustParseTerm(`{"config": {"labels": {"id": "b"}, "services": {"s": {"url": "https://b", "credentials": {"bearer": {"token": "b"}}}}}}`)
	expA := ast.MustParseTerm(`{"config": {"labels": {"id": "a"}, "services": {"s": {"url": "https://a"}}}}`)
	expB := ast.MustParseTerm(`{"config": {"labels": {"id": "b"}, "services": {"s": {"url": "https://b"}}}}`)

	// Alternate between runtimes so that a memoized redaction of one must
	// never be returned for the other.
	for _, tc := range []struct{ rt, exp *ast.Term }{{a, expA}, {a, expA}, {b, expB}, {a, expA}, {b, expB}, {b, expB}} {
		rs, err := NewQuery(ast.MustParseBody("opa.runtime(x)")).WithRuntime(tc.rt).Run(ctx)
		if err != nil {
			t.Fatal(err)
		} else if len(rs) != 1 {
			t.Fatal("Expected result set to contain exactly one result")
		}
		if term := rs[0][ast.Var("x")]; ast.Compare(term, tc.exp) != 0 {
			t.Fatalf("Expected %v but got %v", tc.exp, term)
		}
	}
}

// 123581 ns/op	  303611 B/op	    4101 allocs/op  // Before memoizing redaction
// 734.9 ns/op	    1809 B/op	      29 allocs/op  // After memoizing redaction
func BenchmarkOPARuntime(b *testing.B) {
	env := ast.NewObject()
	for i := range 1000 {
		env.Insert(ast.StringTerm(fmt.Sprintf("SVC_%d_SERVICE_HOST", i)), ast.StringTerm("10.0.0.1"))
	}
	rt := ast.MustParseTerm(`{"config": {"services": {"s": {"url": "https://a", "credentials": {"bearer": {"token": "a"}}}}}}`)
	rt.Value.(ast.Object).Insert(ast.InternedTerm("env"), ast.NewTerm(env))

	ctx := b.Context()
	q := NewQuery(ast.MustParseBody("opa.runtime(x)")).WithRuntime(rt)

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
