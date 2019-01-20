// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package planner contains a query planner for Rego queries.
package planner

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/ir"
)

type planiter func() error
type binaryiter func(ir.Local, ir.Local) error

// Planner implements a query planner for Rego queries.
type Planner struct {
	strings []ir.StringConst
	blocks  []ir.Block
	curr    *ir.Block
	vars    map[ast.Var]ir.Local
	queries []ast.Body
	ltarget ir.Local
	lcurr   ir.Local
}

// New returns a new Planner object.
func New() *Planner {
	return &Planner{
		lcurr: ir.Input + 1,
		vars: map[ast.Var]ir.Local{
			ast.InputRootDocument.Value.(ast.Var): ir.Input,
		},
	}
}

// WithQueries sets the query set to generate a plan for.
func (p *Planner) WithQueries(queries []ast.Body) *Planner {
	p.queries = queries
	return p
}

// Plan returns a IR plan for the policy query.
func (p *Planner) Plan() (*ir.Policy, error) {

	for _, q := range p.queries {
		p.curr = &ir.Block{}
		defined := false

		if err := p.planQuery(q, 0, func() error {
			p.appendStmt(ir.ReturnStmt{
				Code: ir.Defined,
			})
			defined = true
			return nil
		}); err != nil {
			return nil, err
		}

		if defined {
			p.blocks = append(p.blocks, *p.curr)
		}
	}

	p.blocks = append(p.blocks, ir.Block{
		Stmts: []ir.Stmt{
			ir.ReturnStmt{
				Code: ir.Undefined,
			},
		},
	})

	policy := ir.Policy{
		Static: ir.Static{
			Strings: p.strings,
		},
		Plan: ir.Plan{
			Blocks: p.blocks,
		},
	}

	return &policy, nil
}

func (p *Planner) planQuery(q ast.Body, index int, iter planiter) error {

	if index >= len(q) {
		return iter()
	}

	return p.planExpr(q[index], func() error {
		return p.planQuery(q, index+1, iter)
	})
}

// TODO(tsandall): improve errors to include location information.
func (p *Planner) planExpr(e *ast.Expr, iter planiter) error {

	if e.Negated {
		return p.planNot(e, iter)
	}

	if len(e.With) > 0 {
		return fmt.Errorf("with keyword not implemented")
	}

	if e.IsCall() {
		return p.planExprCall(e, iter)
	}

	return p.planExprTerm(e, iter)
}

func (p *Planner) planNot(e *ast.Expr, iter planiter) error {

	cond := p.newLocal()

	p.appendStmt(ir.MakeBooleanStmt{
		Value:  true,
		Target: cond,
	})

	not := ir.NotStmt{
		Cond: cond,
	}

	prev := p.curr
	p.curr = &not.Block

	if err := p.planExpr(e.Complement(), func() error {
		p.appendStmt(ir.AssignBooleanStmt{
			Value:  false,
			Target: cond,
		})
		return nil
	}); err != nil {
		return err
	}

	p.curr = prev
	p.appendStmt(not)

	truth := p.newLocal()

	p.appendStmt(ir.MakeBooleanStmt{
		Value:  true,
		Target: truth,
	})

	p.appendStmt(ir.EqualStmt{
		A: cond,
		B: truth,
	})

	return iter()
}

func (p *Planner) planExprTerm(e *ast.Expr, iter planiter) error {
	return p.planTerm(e.Terms.(*ast.Term), func() error {
		falsy := p.newLocal()
		p.appendStmt(ir.MakeBooleanStmt{
			Value:  false,
			Target: falsy,
		})
		p.appendStmt(ir.NotEqualStmt{
			A: p.ltarget,
			B: falsy,
		})
		return iter()
	})
}

func (p *Planner) planExprCall(e *ast.Expr, iter planiter) error {

	switch e.Operator().String() {
	case ast.Equality.Name:
		return p.planUnify(e.Operand(0), e.Operand(1), iter)
	case ast.Equal.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.EqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.LessThan.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.LessThanStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.LessThanEq.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.LessThanEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.GreaterThan.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.GreaterThanStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.GreaterThanEq.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.GreaterThanEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.NotEqual.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(ir.NotEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	default:
		return fmt.Errorf("%v operator not implemented", e.Operator())
	}
}

func (p *Planner) planUnify(a, b *ast.Term, iter planiter) error {

	switch va := a.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Ref:
		return p.planTerm(a, func() error {
			return p.planUnifyLocal(p.ltarget, b, iter)
		})
	case ast.Var:
		return p.planUnifyVar(va, b, iter)
	case ast.Array:
		switch vb := b.Value.(type) {
		case ast.Var:
			return p.planUnifyVar(vb, a, iter)
		case ast.Ref:
			return p.planTerm(b, func() error {
				return p.planUnifyLocalArray(p.ltarget, va, iter)
			})
		case ast.Array:
			if len(va) == len(vb) {
				return p.planUnifyArraysRec(va, vb, 0, iter)
			}
			return nil
		}
	case ast.Object:
		switch vb := b.Value.(type) {
		case ast.Var:
			return p.planUnifyVar(vb, a, iter)
		case ast.Ref:
			return p.planTerm(b, func() error {
				return p.planUnifyLocalObject(p.ltarget, va, iter)
			})
		case ast.Object:
			if va.Len() == vb.Len() {
				return p.planUnifyObjectsRec(va, vb, va.Keys(), 0, iter)
			}
			return nil
		}
	}

	return fmt.Errorf("not implemented: unify(%v, %v)", a, b)
}

func (p *Planner) planUnifyVar(a ast.Var, b *ast.Term, iter planiter) error {

	if la, ok := p.vars[a]; ok {
		return p.planUnifyLocal(la, b, iter)
	}

	return p.planTerm(b, func() error {
		target := p.newLocal()
		p.vars[a] = target
		p.appendStmt(ir.AssignVarStmt{
			Source: p.ltarget,
			Target: target,
		})
		return iter()
	})
}

func (p *Planner) planUnifyLocal(a ir.Local, b *ast.Term, iter planiter) error {
	switch vb := b.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Ref:
		return p.planTerm(b, func() error {
			p.appendStmt(ir.EqualStmt{
				A: a,
				B: p.ltarget,
			})
			return iter()
		})
	case ast.Var:
		if lv, ok := p.vars[vb]; ok {
			p.appendStmt(ir.EqualStmt{
				A: a,
				B: lv,
			})
			return iter()
		}
		lv := p.newLocal()
		p.vars[vb] = lv
		p.appendStmt(ir.AssignVarStmt{
			Source: a,
			Target: lv,
		})
		return iter()
	case ast.Array:
		return p.planUnifyLocalArray(a, vb, iter)
	case ast.Object:
		return p.planUnifyLocalObject(a, vb, iter)
	}

	return fmt.Errorf("not implemented: unifyLocal(%v, %v)", a, b)
}

func (p *Planner) planUnifyLocalArray(a ir.Local, b ast.Array, iter planiter) error {
	p.appendStmt(ir.IsArrayStmt{
		Source: a,
	})

	blen := p.newLocal()
	alen := p.newLocal()

	p.appendStmt(ir.LenStmt{
		Source: a,
		Target: alen,
	})

	p.appendStmt(ir.MakeNumberIntStmt{
		Value:  int64(len(b)),
		Target: blen,
	})

	p.appendStmt(ir.EqualStmt{
		A: alen,
		B: blen,
	})

	lkey := p.newLocal()

	p.appendStmt(ir.MakeNumberIntStmt{
		Target: lkey,
	})

	lval := p.newLocal()

	return p.planUnifyLocalArrayRec(a, 0, b, lkey, lval, iter)
}

func (p *Planner) planUnifyLocalArrayRec(a ir.Local, index int, b ast.Array, lkey, lval ir.Local, iter planiter) error {
	if len(b) == index {
		return iter()
	}

	p.appendStmt(ir.AssignIntStmt{
		Value:  int64(index),
		Target: lkey,
	})

	p.appendStmt(ir.DotStmt{
		Source: a,
		Key:    lkey,
		Target: lval,
	})

	return p.planUnifyLocal(lval, b[index], func() error {
		return p.planUnifyLocalArrayRec(a, index+1, b, lkey, lval, iter)
	})
}

func (p *Planner) planUnifyLocalObject(a ir.Local, b ast.Object, iter planiter) error {
	p.appendStmt(ir.IsObjectStmt{
		Source: a,
	})

	blen := p.newLocal()
	alen := p.newLocal()

	p.appendStmt(ir.LenStmt{
		Source: a,
		Target: alen,
	})

	p.appendStmt(ir.MakeNumberIntStmt{
		Value:  int64(b.Len()),
		Target: blen,
	})

	p.appendStmt(ir.EqualStmt{
		A: alen,
		B: blen,
	})

	lkey := p.newLocal()
	lval := p.newLocal()
	bkeys := b.Keys()

	return p.planUnifyLocalObjectRec(a, 0, bkeys, b, lkey, lval, iter)
}

func (p *Planner) planUnifyLocalObjectRec(a ir.Local, index int, keys []*ast.Term, b ast.Object, lkey, lval ir.Local, iter planiter) error {

	if index == len(keys) {
		return iter()
	}

	return p.planTerm(keys[index], func() error {
		p.appendStmt(ir.AssignVarStmt{
			Source: p.ltarget,
			Target: lkey,
		})
		p.appendStmt(ir.DotStmt{
			Source: a,
			Key:    lkey,
			Target: lval,
		})
		return p.planUnifyLocal(lval, b.Get(keys[index]), func() error {
			return p.planUnifyLocalObjectRec(a, index+1, keys, b, lkey, lval, iter)
		})
	})
}

func (p *Planner) planUnifyArraysRec(a, b ast.Array, index int, iter planiter) error {
	if index == len(a) {
		return iter()
	}
	return p.planUnify(a[index], b[index], func() error {
		return p.planUnifyArraysRec(a, b, index+1, iter)
	})
}

func (p *Planner) planUnifyObjectsRec(a, b ast.Object, keys []*ast.Term, index int, iter planiter) error {
	if index == len(keys) {
		return iter()
	}

	aval := a.Get(keys[index])
	bval := b.Get(keys[index])
	if aval == nil || bval == nil {
		return nil
	}

	return p.planUnify(aval, bval, func() error {
		return p.planUnifyObjectsRec(a, b, keys, index+1, iter)
	})
}

func (p *Planner) planBinaryExpr(e *ast.Expr, iter binaryiter) error {
	return p.planTerm(e.Operand(0), func() error {
		a := p.ltarget
		return p.planTerm(e.Operand(1), func() error {
			b := p.ltarget
			return iter(a, b)
		})
	})
}

func (p *Planner) planTerm(t *ast.Term, iter planiter) error {

	switch v := t.Value.(type) {
	case ast.Null:
		return p.planNull(v, iter)
	case ast.Boolean:
		return p.planBoolean(v, iter)
	case ast.Number:
		return p.planNumber(v, iter)
	case ast.String:
		return p.planString(v, iter)
	case ast.Var:
		return p.planVar(v, iter)
	case ast.Ref:
		return p.planRef(v, iter)
	case ast.Array:
		return p.planArray(v, iter)
	case ast.Object:
		return p.planObject(v, iter)
	default:
		return fmt.Errorf("%v term not implemented", ast.TypeName(v))
	}
}

func (p *Planner) planNull(null ast.Null, iter planiter) error {

	target := p.newLocal()

	p.appendStmt(ir.MakeNullStmt{
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planBoolean(b ast.Boolean, iter planiter) error {

	target := p.newLocal()

	p.appendStmt(ir.MakeBooleanStmt{
		Value:  bool(b),
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planNumber(num ast.Number, iter planiter) error {

	i, ok := num.Int()
	if !ok {
		return fmt.Errorf("float values not implemented")
	}

	i64 := int64(i)
	target := p.newLocal()

	p.appendStmt(ir.MakeNumberIntStmt{
		Value:  i64,
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planString(str ast.String, iter planiter) error {

	index := p.appendStringConst(string(str))
	target := p.newLocal()

	p.appendStmt(ir.MakeStringStmt{
		Index:  index,
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planVar(v ast.Var, iter planiter) error {
	if _, ok := p.vars[v]; !ok {
		p.vars[v] = p.newLocal()
	}
	p.ltarget = p.vars[v]
	return iter()
}

func (p *Planner) planArray(arr ast.Array, iter planiter) error {

	larr := p.newLocal()

	p.appendStmt(ir.MakeArrayStmt{
		Capacity: int32(len(arr)),
		Target:   larr,
	})

	return p.planArrayRec(arr, 0, larr, iter)
}

func (p *Planner) planArrayRec(arr ast.Array, index int, larr ir.Local, iter planiter) error {
	if index == len(arr) {
		return iter()
	}

	return p.planTerm(arr[index], func() error {

		p.appendStmt(ir.ArrayAppendStmt{
			Value: p.ltarget,
			Array: larr,
		})

		return p.planArrayRec(arr, index+1, larr, iter)
	})
}

func (p *Planner) planObject(obj ast.Object, iter planiter) error {

	lobj := p.newLocal()

	p.appendStmt(ir.MakeObjectStmt{
		Target: lobj,
	})

	return p.planObjectRec(obj, 0, obj.Keys(), lobj, iter)
}

func (p *Planner) planObjectRec(obj ast.Object, index int, keys []*ast.Term, lobj ir.Local, iter planiter) error {
	if index == len(keys) {
		return iter()
	}

	return p.planTerm(keys[index], func() error {
		lkey := p.ltarget

		return p.planTerm(obj.Get(keys[index]), func() error {
			lval := p.ltarget
			p.appendStmt(ir.ObjectInsertStmt{
				Key:    lkey,
				Value:  lval,
				Object: lobj,
			})

			return p.planObjectRec(obj, index+1, keys, lobj, iter)
		})
	})
}

func (p *Planner) planRef(ref ast.Ref, iter planiter) error {

	if !ref[0].Equal(ast.InputRootDocument) {
		return fmt.Errorf("%v root document not implemented", ref[0])
	}

	p.ltarget = p.vars[ast.InputRootDocument.Value.(ast.Var)]

	return p.planRefRec(ref, 1, iter)
}

func (p *Planner) planRefRec(ref ast.Ref, index int, iter planiter) error {

	if len(ref) == index {
		return iter()
	}

	switch v := ref[index].Value.(type) {

	case ast.Null, ast.Boolean, ast.Number, ast.String:
		source := p.ltarget
		return p.planTerm(ref[index], func() error {
			key := p.ltarget
			target := p.newLocal()
			p.appendStmt(ir.DotStmt{
				Source: source,
				Key:    key,
				Target: target,
			})
			p.ltarget = target
			return p.planRefRec(ref, index+1, iter)
		})

	case ast.Var:
		if _, ok := p.vars[v]; !ok {
			return p.planScan(ref, index, func() error {
				return p.planRefRec(ref, index+1, iter)
			})
		}
		p.ltarget = p.vars[v]
		return p.planRefRec(ref, index+1, iter)

	default:
		return fmt.Errorf("%v reference operand not implemented", ast.TypeName(ref[index].Value))
	}
}

func (p *Planner) planScan(ref ast.Ref, index int, iter planiter) error {

	source := p.ltarget

	return p.planVar(ref[index].Value.(ast.Var), func() error {

		key := p.ltarget
		cond := p.newLocal()
		value := p.newLocal()

		p.appendStmt(ir.MakeBooleanStmt{
			Value:  false,
			Target: cond,
		})

		scan := ir.ScanStmt{
			Source: source,
			Key:    key,
			Value:  value,
		}

		prev := p.curr
		p.curr = &scan.Block
		p.ltarget = value

		if err := iter(); err != nil {
			return err
		}

		p.appendStmt(ir.AssignBooleanStmt{
			Value:  true,
			Target: cond,
		})

		p.curr = prev
		p.appendStmt(scan)

		truth := p.newLocal()

		p.appendStmt(ir.MakeBooleanStmt{
			Value:  true,
			Target: truth,
		})

		p.appendStmt(ir.EqualStmt{
			A: cond,
			B: truth,
		})

		return nil
	})
}

func (p *Planner) appendStmt(s ir.Stmt) {
	p.curr.Stmts = append(p.curr.Stmts, s)
}

func (p *Planner) appendStringConst(s string) int {
	index := len(p.strings)
	p.strings = append(p.strings, ir.StringConst{
		Value: s,
	})
	return index
}

func (p *Planner) newLocal() ir.Local {
	x := p.lcurr
	p.lcurr++
	return x
}
