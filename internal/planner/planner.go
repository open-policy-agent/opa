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

		p.blocks = append(p.blocks, ir.Block{})
		p.curr = &p.blocks[len(p.blocks)-1]

		if err := p.planQuery(q, 0); err != nil {
			return nil, err
		}

		p.appendStmt(ir.ReturnStmt{
			Code: ir.Defined,
		})
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

func (p *Planner) planQuery(q ast.Body, index int) error {

	if index >= len(q) {
		return nil
	}

	return p.planExpr(q[index], func() error {
		return p.planQuery(q, index+1)
	})
}

// TODO(tsandall): improve errors to include locaiton information.
func (p *Planner) planExpr(e *ast.Expr, iter planiter) error {

	if e.Negated {
		return fmt.Errorf("not keyword not implemented")
	}

	if len(e.With) > 0 {
		return fmt.Errorf("with keyword not implemented")
	}

	if e.IsCall() {
		return p.planExprCall(e, iter)
	}

	return p.planExprTerm(e, iter)
}

func (p *Planner) planExprTerm(e *ast.Expr, iter planiter) error {
	return p.planTerm(e.Terms.(*ast.Term), iter)
}

func (p *Planner) planExprCall(e *ast.Expr, iter planiter) error {

	switch e.Operator().String() {
	case ast.Equality.Name:
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
	case ast.Number:
		return p.planNumber(v, iter)
	case ast.String:
		return p.planString(v, iter)
	case ast.Var:
		return p.planVar(v, iter)
	case ast.Ref:
		return p.planRef(v, iter)
	default:
		return fmt.Errorf("%v term not implemented", ast.TypeName(v))
	}
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
			return p.planLoop(ref, index, iter)
		}
		p.ltarget = p.vars[v]
		return p.planRefRec(ref, index+1, iter)

	default:
		return fmt.Errorf("%v reference operand not implemented", ast.TypeName(ref[index].Value))
	}
}

func (p *Planner) planLoop(ref ast.Ref, index int, iter planiter) error {

	source := p.ltarget

	return p.planVar(ref[index].Value.(ast.Var), func() error {

		key := p.ltarget
		cond := p.newLocal()
		value := p.newLocal()

		p.appendStmt(ir.MakeBooleanStmt{
			Value:  false,
			Target: cond,
		})

		loop := ir.LoopStmt{
			Source: source,
			Key:    key,
			Value:  value,
			Cond:   cond,
		}

		prev := p.curr
		p.curr = &loop.Block
		p.ltarget = value

		if err := iter(); err != nil {
			return err
		}

		p.appendStmt(ir.AssignStmt{
			Value: ir.BooleanConst{
				Value: true,
			},
			Target: cond,
		})

		p.curr = prev
		p.appendStmt(loop)

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
