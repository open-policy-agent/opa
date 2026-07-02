package ast_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestFromBuiltinNames(t *testing.T) {
	for _, name := range []string{"io.jwt.decode", "http.send", "net.cidr_contains"} {
		if s, ok := ast.BuiltinNameFromRef(ast.MustParseRef(name)); !ok {
			t.Fatalf("Expected to find match for %v", name)
		} else if s != name {
			t.Fatalf("Expected %v but got: %v", name, s)
		}
	}
}

func BenchmarkFromBuiltinNames(b *testing.B) {
	tests := []struct {
		name string
		ref  ast.Ref
		ok   bool
	}{
		{
			name: "single part",
			ref:  ast.Ref{ast.VarTerm("count")},
			ok:   true,
		},
		{
			name: "two parts",
			ref:  ast.MustParseRef("http.send"),
			ok:   true,
		},
		{
			name: "three parts",
			ref:  ast.MustParseRef("io.jwt.decode"),
			ok:   true,
		},
		{
			name: "no match",
			ref:  ast.MustParseRef("foo.bar.baz"),
			ok:   false,
		},
		{
			name: "no match long",
			ref:  ast.MustParseRef("a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z"),
			ok:   false,
		},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				if _, ok := ast.BuiltinNameFromRef(tc.ref); ok != tc.ok {
					b.Fatalf("Expected ok=%v but got: %v", tc.ok, ok)
				}
			}
		})
	}
}
