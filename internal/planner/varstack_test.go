package planner

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ir"
)

func TestVarStackPushPop(t *testing.T) {
	p := New()
	vs := newVarstack()
	vs.Push(map[ast.Var]ir.Local{})
	loc0, loc1 := p.newLocal(), p.newLocal()
	vs.Put(ast.Var("x"), loc0)
	vs.Push(map[ast.Var]ir.Local{})
	loc, ok := vs.Get(ast.Var("x"))
	if exp, act := true, ok; exp != act {
		t.Errorf("Get(x): expected %v, got %v", exp, act)
	}
	if exp, act := loc0, loc; exp != act {
		t.Errorf("Get(x) expected %v, got %v", exp, act)
	}
	vs.Put(ast.Var("y"), loc1)
	pop := vs.Pop()
	if exp, act := 1, len(pop); exp != act {
		t.Errorf("Pop(): expected len %v, got %v", exp, act)
	}

	// x still there
	loc, ok = vs.Get(ast.Var("x"))
	if exp, act := true, ok; exp != act {
		t.Errorf("Get(x): expected %v, got %v", exp, act)
	}
	if exp, act := loc0, loc; exp != act {
		t.Errorf("Get(x) expected %v, got %v", exp, act)
	}

	// y not there
	_, ok = vs.Get(ast.Var("y"))
	if exp, act := false, ok; exp != act {
		t.Errorf("Get(y): expected %v, got %v", exp, act)
	}
}
