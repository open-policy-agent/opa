// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestFuncstack(t *testing.T) {
	fs := newFuncstack()

	fs.Add("data.foo.bar", "g0.data.foo.bar")

	fs.Push(map[string]string{}) // g0 -> g1
	fs.Add("data.foo.bar", "g1.data.foo.bar")
	f, ok := fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g1.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	if exp, act := 1, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	g1 := fs.Pop() // g1 -> g0
	if exp, act := 1, len(g1); exp != act {
		t.Errorf("expected g1 func map to have length %d, got %d", exp, act)
	}
	if exp, act := 0, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatalf("expected func to be found")
	}
	if exp, act := "g0.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}

	fs.Push(map[string]string{}) // g0 -> g2
	fs.Add("data.foo.bar", "g2.data.foo.bar")
	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g2.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	if exp, act := 2, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	fs.Push(map[string]string{}) // g2 -> g3
	fs.Add("data.foo.bar", "g3.data.foo.bar")
	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g3.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	_ = fs.Pop() // g3 -> g2
	_ = fs.Pop() // g2 -> g0
	if exp, act := 0, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	fs.Push(map[string]string{}) // g0 -> g4
	if exp, act := 4, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}
}

func TestDataRefsShadowRuletrie(t *testing.T) {
	p := New()
	rt := p.rules
	rt.Insert(ast.MustParseRef(("data.foo.bar")))
	rt.Insert(ast.MustParseRef(("data.foo.baz")))
	rt.Insert(ast.MustParseRef(("data.foo.bar.quz")))

	tests := []struct {
		note string
		refs []ast.Ref
		exp  bool
	}{
		{
			note: "no refs",
			refs: nil,
			exp:  false,
		},
		{
			note: "data root node",
			refs: []ast.Ref{ast.MustParseRef("data")},
			exp:  true,
		},
		{
			note: "one ref only, mismatch in first level",
			refs: []ast.Ref{ast.MustParseRef("data.quz")},
			exp:  false,
		},
		{
			note: "two refs, matching 2nd",
			refs: []ast.Ref{
				ast.MustParseRef("data.quz"),
				ast.MustParseRef("data.foo"),
			},
			exp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			act := p.dataRefsShadowRuletrie(tc.refs)
			if tc.exp != act {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}
