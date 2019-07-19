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
	queries []ast.Body           // input query to plan
	modules []*ast.Module        // input modules to support queries
	strings []*ir.StringConst    // planned (global) string constants
	blocks  []*ir.Block          // planned blocks
	funcs   *functrie            // planned functions to support blocks
	curr    *ir.Block            // in-progress query block
	vars    map[ast.Var]ir.Local // in-scope variables
	ltarget ir.Local             // target variable of last planned statement
	lcurr   ir.Local             // next variable to use
}

// New returns a new Planner object.
func New() *Planner {
	return &Planner{
		lcurr: ir.Input + 1,
		vars: map[ast.Var]ir.Local{
			ast.InputRootDocument.Value.(ast.Var): ir.Input,
		},
		funcs: newFunctrie(),
	}
}

// WithQueries sets the query set to generate a plan for.
func (p *Planner) WithQueries(queries []ast.Body) *Planner {
	p.queries = queries
	return p
}

// WithModules sets the module set that contains query dependencies.
func (p *Planner) WithModules(modules []*ast.Module) *Planner {
	p.modules = modules
	return p
}

// Plan returns a IR plan for the policy query.
func (p *Planner) Plan() (*ir.Policy, error) {

	if err := p.planModules(); err != nil {
		return nil, err
	}

	if err := p.planQueries(); err != nil {
		return nil, err
	}

	policy := &ir.Policy{
		Static: &ir.Static{
			Strings: p.strings,
		},
		Plan: &ir.Plan{
			Blocks: p.blocks,
		},
		Funcs: &ir.Funcs{
			Funcs: p.funcs.FuncMap(),
		},
	}

	return policy, nil
}

func (p *Planner) planModules() error {

	// Build a set of all the rulesets to plan.
	funcs := map[*functrieValue]struct{}{}

	for _, module := range p.modules {

		// Create functrie node for empty packages so that extent queries return
		// empty objects. For example:
		//
		// package x.y
		//
		// Query: data.x
		//
		// Expected result: {"y": {}}
		if len(module.Rules) == 0 {
			_ = p.funcs.LookupOrInsert(module.Package.Path, nil)
			continue
		}

		for _, rule := range module.Rules {
			val := p.funcs.LookupOrInsert(rule.Path(), &functrieValue{})
			val.Rules = append(val.Rules, rule)
			funcs[val] = struct{}{}
		}
	}

	for val := range funcs {
		if err := p.planRules(val); err != nil {
			return err
		}
	}

	return nil
}

func (p *Planner) planRules(trieNode *functrieValue) error {

	rules := trieNode.Rules

	// Create function definition for rules.
	fn := &ir.Func{
		Name:   rules[0].Path().String(),
		Params: []ir.Local{p.newLocal()},
		Return: p.newLocal(),
	}

	trieNode.Fn = fn

	// Initialize parameters for functions.
	for i := 0; i < len(rules[0].Head.Args); i++ {
		fn.Params = append(fn.Params, p.newLocal())
	}

	params := fn.Params[1:]

	// Initialize return value for partial set/object rules. Complete docs do
	// not require their return value to be initialized.
	if rules[0].Head.DocKind() == ast.PartialObjectDoc {
		fn.Blocks = append(fn.Blocks, &ir.Block{
			Stmts: []ir.Stmt{
				&ir.MakeObjectStmt{
					Target: fn.Return,
				},
			},
		})
	} else if rules[0].Head.DocKind() == ast.PartialSetDoc {
		fn.Blocks = append(fn.Blocks, &ir.Block{
			Stmts: []ir.Stmt{
				&ir.MakeSetStmt{
					Target: fn.Return,
				},
			},
		})
	}

	// Save current state of planner.
	//
	// TODO(tsandall): perhaps we would be better off using stacks here or
	// splitting block planner into separate struct that could be instantiated
	// for rule and comprehension bodies.
	currVars := p.vars
	currBlock := p.curr

	var defaultRule *ast.Rule

	// Generate function blocks for rules.
	for i := range rules {

		// Save default rule for the end.
		if rules[i].Default {
			defaultRule = rules[i]
			continue
		}

		// Ordered rules are nested inside an additional block so that execution
		// can short-circuit. For unordered rules blocks can be added directly
		// to the function.
		var blocks *[]*ir.Block

		if rules[i].Else == nil {
			blocks = &fn.Blocks
		} else {
			stmt := &ir.BlockStmt{}
			block := &ir.Block{Stmts: []ir.Stmt{stmt}}
			fn.Blocks = append(fn.Blocks, block)
			blocks = &stmt.Blocks
		}

		// Unordered rules are treated as a special case of ordered rules.
		for rule := rules[i]; rule != nil; rule = rule.Else {

			// Setup planner for block.
			p.vars = map[ast.Var]ir.Local{
				ast.InputRootDocument.Value.(ast.Var): fn.Params[0],
			}

			curr := &ir.Block{}
			*blocks = append(*blocks, curr)
			p.curr = curr

			// Complete and partial rules are treated as special cases of
			// functions. If there are args, the first step is a no-op.
			err := p.planFuncParams(params, rule.Head.Args, 0, func() error {

				// Run planner on the rule body.
				err := p.planQuery(rule.Body, 0, func() error {

					// Run planner on the result.
					switch rule.Head.DocKind() {
					case ast.CompleteDoc:
						return p.planTerm(rule.Head.Value, func() error {
							p.appendStmt(&ir.AssignVarOnceStmt{
								Target: fn.Return,
								Source: p.ltarget,
							})
							return nil
						})
					case ast.PartialSetDoc:
						return p.planTerm(rule.Head.Key, func() error {
							p.appendStmt(&ir.SetAddStmt{
								Set:   fn.Return,
								Value: p.ltarget,
							})
							return nil
						})
					case ast.PartialObjectDoc:
						return p.planTerm(rule.Head.Key, func() error {
							key := p.ltarget
							return p.planTerm(rule.Head.Value, func() error {
								value := p.ltarget
								p.appendStmt(&ir.ObjectInsertOnceStmt{
									Object: fn.Return,
									Key:    key,
									Value:  value,
								})
								return nil
							})
						})
					default:
						return fmt.Errorf("illegal rule kind")
					}
				})

				if err != nil {
					return err
				}

				// Ordered rules are handled by short circuiting execution. The
				// plan will jump out to the extra block that was planned above.
				if rule.Else != nil {
					p.appendStmt(&ir.IsDefinedStmt{Source: fn.Return})
					p.appendStmt(&ir.BreakStmt{Index: 1})
				}

				return nil
			})

			if err != nil {
				return err
			}
		}
	}

	// Default rules execute if the return is undefined.
	if defaultRule != nil {

		fn.Blocks = append(fn.Blocks, &ir.Block{
			Stmts: []ir.Stmt{
				&ir.IsUndefinedStmt{Source: fn.Return},
			},
		})

		p.curr = fn.Blocks[len(fn.Blocks)-1]

		if err := p.planTerm(defaultRule.Head.Value, func() error {
			p.appendStmt(&ir.AssignVarStmt{
				Target: fn.Return,
				Source: p.ltarget,
			})
			return nil
		}); err != nil {
			return err
		}
	}

	// All rules return a value.
	fn.Blocks = append(fn.Blocks, &ir.Block{
		Stmts: []ir.Stmt{
			&ir.ReturnLocalStmt{
				Source: fn.Return,
			},
		},
	})

	// Restore the state of the planner.
	p.vars = currVars
	p.curr = currBlock

	return nil
}

func (p *Planner) planFuncParams(params []ir.Local, args ast.Args, idx int, iter planiter) error {
	if idx >= len(args) {
		return iter()
	}
	return p.planUnifyLocal(params[idx], args[idx], func() error {
		return p.planFuncParams(params, args, idx+1, iter)
	})
}

func (p *Planner) planQueries() error {

	for _, q := range p.queries {
		p.curr = &ir.Block{}
		defined := false

		if err := p.planQuery(q, 0, func() error {
			p.appendStmt(&ir.ReturnStmt{
				Code: ir.Defined,
			})
			defined = true
			return nil
		}); err != nil {
			return err
		}

		if defined {
			p.blocks = append(p.blocks, p.curr)
		}
	}

	p.blocks = append(p.blocks, &ir.Block{
		Stmts: []ir.Stmt{
			&ir.ReturnStmt{
				Code: ir.Undefined,
			},
		},
	})

	return nil
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

	p.appendStmt(&ir.MakeBooleanStmt{
		Value:  true,
		Target: cond,
	})

	not := &ir.NotStmt{
		Cond:  cond,
		Block: &ir.Block{},
	}

	prev := p.curr
	p.curr = not.Block

	if err := p.planExpr(e.Complement(), func() error {
		p.appendStmt(&ir.AssignBooleanStmt{
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

	p.appendStmt(&ir.MakeBooleanStmt{
		Value:  true,
		Target: truth,
	})

	p.appendStmt(&ir.EqualStmt{
		A: cond,
		B: truth,
	})

	return iter()
}

func (p *Planner) planExprTerm(e *ast.Expr, iter planiter) error {
	return p.planTerm(e.Terms.(*ast.Term), func() error {
		falsy := p.newLocal()
		p.appendStmt(&ir.MakeBooleanStmt{
			Value:  false,
			Target: falsy,
		})
		p.appendStmt(&ir.NotEqualStmt{
			A: p.ltarget,
			B: falsy,
		})
		return iter()
	})
}

func (p *Planner) planExprCall(e *ast.Expr, iter planiter) error {
	operator := e.Operator().String()
	switch operator {
	case ast.Equality.Name:
		return p.planUnify(e.Operand(0), e.Operand(1), iter)
	case ast.Equal.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.EqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.LessThan.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.LessThanStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.LessThanEq.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.LessThanEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.GreaterThan.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.GreaterThanStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.GreaterThanEq.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.GreaterThanEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	case ast.NotEqual.Name:
		return p.planBinaryExpr(e, func(a, b ir.Local) error {
			p.appendStmt(&ir.NotEqualStmt{
				A: a,
				B: b,
			})
			return iter()
		})
	default:
		trieNode := p.funcs.Lookup(e.Operator())
		if trieNode == nil {
			return fmt.Errorf("illegal call: unknown operator %v", operator)
		}

		arity := trieNode.Arity()
		operands := e.Operands()

		args := []ir.Local{
			p.vars[ast.InputRootDocument.Value.(ast.Var)],
		}

		if len(operands) == arity {
			// rule: f(x) = x { ... }
			// call: f(x) == 1 or f(x) # result not captured
			return p.planCallArgs(operands, 0, args, func(args []ir.Local) error {
				p.ltarget = p.newLocal()
				p.appendStmt(&ir.CallStmt{
					Func:   operator,
					Args:   args,
					Result: p.ltarget,
				})
				return iter()
			})
		} else if len(operands) == arity+1 {
			// rule: f(x) = x { ... }
			// call: f(x, 1)  # caller captures result
			return p.planCallArgs(operands[:len(operands)-1], 0, args, func(args []ir.Local) error {
				result := p.newLocal()
				p.appendStmt(&ir.CallStmt{
					Func:   operator,
					Args:   args,
					Result: result,
				})
				return p.planUnifyLocal(result, operands[len(operands)-1], iter)
			})
		}

		return fmt.Errorf("illegal call: wrong number of operands: got %v, want %v)", len(operands), arity)
	}
}

func (p *Planner) planCallArgs(terms []*ast.Term, idx int, args []ir.Local, iter func([]ir.Local) error) error {
	if idx >= len(terms) {
		return iter(args)
	}
	return p.planTerm(terms[idx], func() error {
		args = append(args, p.ltarget)
		return p.planCallArgs(terms, idx+1, args, iter)
	})
}

func (p *Planner) planUnify(a, b *ast.Term, iter planiter) error {

	switch va := a.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Ref, ast.Set:
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
		p.appendStmt(&ir.AssignVarStmt{
			Source: p.ltarget,
			Target: target,
		})
		return iter()
	})
}

func (p *Planner) planUnifyLocal(a ir.Local, b *ast.Term, iter planiter) error {
	switch vb := b.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Ref, ast.Set:
		return p.planTerm(b, func() error {
			p.appendStmt(&ir.EqualStmt{
				A: a,
				B: p.ltarget,
			})
			return iter()
		})
	case ast.Var:
		if lv, ok := p.vars[vb]; ok {
			p.appendStmt(&ir.EqualStmt{
				A: a,
				B: lv,
			})
			return iter()
		}
		lv := p.newLocal()
		p.vars[vb] = lv
		p.appendStmt(&ir.AssignVarStmt{
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
	p.appendStmt(&ir.IsArrayStmt{
		Source: a,
	})

	blen := p.newLocal()
	alen := p.newLocal()

	p.appendStmt(&ir.LenStmt{
		Source: a,
		Target: alen,
	})

	p.appendStmt(&ir.MakeNumberIntStmt{
		Value:  int64(len(b)),
		Target: blen,
	})

	p.appendStmt(&ir.EqualStmt{
		A: alen,
		B: blen,
	})

	lkey := p.newLocal()

	p.appendStmt(&ir.MakeNumberIntStmt{
		Target: lkey,
	})

	lval := p.newLocal()

	return p.planUnifyLocalArrayRec(a, 0, b, lkey, lval, iter)
}

func (p *Planner) planUnifyLocalArrayRec(a ir.Local, index int, b ast.Array, lkey, lval ir.Local, iter planiter) error {
	if len(b) == index {
		return iter()
	}

	p.appendStmt(&ir.AssignIntStmt{
		Value:  int64(index),
		Target: lkey,
	})

	p.appendStmt(&ir.DotStmt{
		Source: a,
		Key:    lkey,
		Target: lval,
	})

	return p.planUnifyLocal(lval, b[index], func() error {
		return p.planUnifyLocalArrayRec(a, index+1, b, lkey, lval, iter)
	})
}

func (p *Planner) planUnifyLocalObject(a ir.Local, b ast.Object, iter planiter) error {
	p.appendStmt(&ir.IsObjectStmt{
		Source: a,
	})

	blen := p.newLocal()
	alen := p.newLocal()

	p.appendStmt(&ir.LenStmt{
		Source: a,
		Target: alen,
	})

	p.appendStmt(&ir.MakeNumberIntStmt{
		Value:  int64(b.Len()),
		Target: blen,
	})

	p.appendStmt(&ir.EqualStmt{
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
		p.appendStmt(&ir.AssignVarStmt{
			Source: p.ltarget,
			Target: lkey,
		})
		p.appendStmt(&ir.DotStmt{
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
	case ast.Set:
		return p.planSet(v, iter)
	default:
		return fmt.Errorf("%v term not implemented", ast.TypeName(v))
	}
}

func (p *Planner) planNull(null ast.Null, iter planiter) error {

	target := p.newLocal()

	p.appendStmt(&ir.MakeNullStmt{
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planBoolean(b ast.Boolean, iter planiter) error {

	target := p.newLocal()

	p.appendStmt(&ir.MakeBooleanStmt{
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

	p.appendStmt(&ir.MakeNumberIntStmt{
		Value:  i64,
		Target: target,
	})

	p.ltarget = target

	return iter()
}

func (p *Planner) planString(str ast.String, iter planiter) error {

	index := p.appendStringConst(string(str))
	target := p.newLocal()

	p.appendStmt(&ir.MakeStringStmt{
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

	p.appendStmt(&ir.MakeArrayStmt{
		Capacity: int32(len(arr)),
		Target:   larr,
	})

	return p.planArrayRec(arr, 0, larr, iter)
}

func (p *Planner) planArrayRec(arr ast.Array, index int, larr ir.Local, iter planiter) error {
	if index == len(arr) {
		p.ltarget = larr
		return iter()
	}

	return p.planTerm(arr[index], func() error {

		p.appendStmt(&ir.ArrayAppendStmt{
			Value: p.ltarget,
			Array: larr,
		})

		return p.planArrayRec(arr, index+1, larr, iter)
	})
}

func (p *Planner) planObject(obj ast.Object, iter planiter) error {

	lobj := p.newLocal()

	p.appendStmt(&ir.MakeObjectStmt{
		Target: lobj,
	})

	return p.planObjectRec(obj, 0, obj.Keys(), lobj, iter)
}

func (p *Planner) planObjectRec(obj ast.Object, index int, keys []*ast.Term, lobj ir.Local, iter planiter) error {
	if index == len(keys) {
		p.ltarget = lobj
		return iter()
	}

	return p.planTerm(keys[index], func() error {
		lkey := p.ltarget

		return p.planTerm(obj.Get(keys[index]), func() error {
			lval := p.ltarget
			p.appendStmt(&ir.ObjectInsertStmt{
				Key:    lkey,
				Value:  lval,
				Object: lobj,
			})

			return p.planObjectRec(obj, index+1, keys, lobj, iter)
		})
	})
}

func (p *Planner) planSet(set ast.Set, iter planiter) error {
	lset := p.newLocal()

	p.appendStmt(&ir.MakeSetStmt{
		Target: lset,
	})

	return p.planSetRec(set, 0, set.Slice(), lset, iter)
}

func (p *Planner) planSetRec(set ast.Set, index int, elems []*ast.Term, lset ir.Local, iter planiter) error {
	if index == len(elems) {
		p.ltarget = lset
		return iter()
	}

	return p.planTerm(elems[index], func() error {
		p.appendStmt(&ir.SetAddStmt{
			Value: p.ltarget,
			Set:   lset,
		})
		return p.planSetRec(set, index+1, elems, lset, iter)
	})
}

func (p *Planner) planRef(ref ast.Ref, iter planiter) error {

	head, ok := ref[0].Value.(ast.Var)
	if !ok {
		return fmt.Errorf("illegal ref: non-var head")
	}

	if head.Compare(ast.DefaultRootDocument.Value) == 0 {
		return p.planRefData(p.funcs, ref, 0, iter)
	}

	p.ltarget, ok = p.vars[head]
	if !ok {
		return fmt.Errorf("illegal ref: unsafe head")
	}

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
			p.appendStmt(&ir.DotStmt{
				Source: source,
				Key:    key,
				Target: target,
			})
			p.ltarget = target
			return p.planRefRec(ref, index+1, iter)
		})

	case ast.Var:
		if _, ok := p.vars[v]; !ok {
			return p.planRefScan(ref, index, func() error {
				return p.planRefRec(ref, index+1, iter)
			})
		}
		target := p.newLocal()
		p.appendStmt(&ir.DotStmt{
			Source: p.ltarget,
			Key:    p.vars[v],
			Target: target,
		})
		p.ltarget = target
		return p.planRefRec(ref, index+1, iter)

	default:
		return fmt.Errorf("%v reference operand not implemented", ast.TypeName(ref[index].Value))
	}
}

func (p *Planner) planRefScan(ref ast.Ref, index int, iter planiter) error {

	source := p.ltarget

	return p.planVar(ref[index].Value.(ast.Var), func() error {

		key := p.ltarget
		cond := p.newLocal()
		value := p.newLocal()

		p.appendStmt(&ir.MakeBooleanStmt{
			Value:  false,
			Target: cond,
		})

		scan := &ir.ScanStmt{
			Source: source,
			Key:    key,
			Value:  value,
			Block:  &ir.Block{},
		}

		prev := p.curr
		p.curr = scan.Block
		p.ltarget = value

		if err := iter(); err != nil {
			return err
		}

		p.appendStmt(&ir.AssignBooleanStmt{
			Value:  true,
			Target: cond,
		})

		p.curr = prev
		p.appendStmt(scan)

		truth := p.newLocal()

		p.appendStmt(&ir.MakeBooleanStmt{
			Value:  true,
			Target: truth,
		})

		p.appendStmt(&ir.EqualStmt{
			A: cond,
			B: truth,
		})

		return nil
	})
}

func (p *Planner) planRefData(node *functrie, ref ast.Ref, idx int, iter planiter) error {

	if idx >= len(ref) {
		return p.planRefDataVirtualExtent(node, iter)
	}

	term := ref[idx]

	if _, ok := term.Value.(ast.String); !ok && idx > 0 {
		return fmt.Errorf("not implemented: refs with non-string operands")
	}

	child, ok := node.children[term.Value]
	if !ok {
		return nil
	}

	if child.val == nil {
		return p.planRefData(child, ref, idx+1, iter)
	}

	p.ltarget = p.newLocal()

	p.appendStmt(&ir.CallStmt{
		Func:   ref[:idx+1].String(),
		Args:   []ir.Local{p.vars[ast.InputRootDocument.Value.(ast.Var)]},
		Result: p.ltarget,
	})

	return p.planRefRec(ref, idx+1, iter)
}

func (p *Planner) planRefDataVirtualExtent(node *functrie, iter planiter) error {

	// Create a new object document. The target is not set until the planner
	// recurses so that we can build the hierarchy depth-first.
	target := p.newLocal()

	p.appendStmt(&ir.MakeObjectStmt{
		Target: target,
	})

	for key, child := range node.children {

		// Skip functions.
		if child.val != nil && child.val.Arity() > 0 {
			continue
		}

		lkey := p.newLocal()
		idx := p.appendStringConst(string(key.(ast.String)))
		p.appendStmt(&ir.MakeStringStmt{
			Index:  idx,
			Target: lkey,
		})

		// Build object hierarchy depth-first.
		if child.val == nil {
			err := p.planRefDataVirtualExtent(child, func() error {
				p.appendStmt(&ir.ObjectInsertStmt{
					Object: target,
					Key:    lkey,
					Value:  p.ltarget,
				})
				return nil
			})
			if err != nil {
				return err
			}
			continue
		}

		// Generate virtual document for leaf.
		lvalue := p.newLocal()

		// Add leaf to object if defined.
		p.appendStmt(&ir.BlockStmt{
			Blocks: []*ir.Block{
				&ir.Block{
					Stmts: []ir.Stmt{
						&ir.CallStmt{
							Func:   child.val.Rules[0].Path().String(),
							Args:   []ir.Local{p.vars[ast.InputRootDocument.Value.(ast.Var)]},
							Result: lvalue,
						},
						&ir.ObjectInsertStmt{
							Object: target,
							Key:    lkey,
							Value:  lvalue,
						},
					},
				},
			},
		})

	}

	// Set target to object and recurse.
	p.ltarget = target
	return iter()
}

func (p *Planner) appendStmt(s ir.Stmt) {
	p.curr.Stmts = append(p.curr.Stmts, s)
}

func (p *Planner) appendStringConst(s string) int {
	index := len(p.strings)
	p.strings = append(p.strings, &ir.StringConst{
		Value: s,
	})
	return index
}

func (p *Planner) newLocal() ir.Local {
	x := p.lcurr
	p.lcurr++
	return x
}
