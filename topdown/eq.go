// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

func evalEq(ctx *Context, expr *ast.Expr, iter Iterator) error {

	operands := expr.Terms.([]*ast.Term)
	a := operands[1].Value
	b := operands[2].Value

	undo, err := evalEqUnify(ctx, a, b, nil, iter)
	ctx.Unbind(undo)
	return err
}

func evalEqGround(ctx *Context, a ast.Value, b ast.Value, iter Iterator) error {
	av, err := ValueToInterface(a, ctx)
	if err != nil {
		return err
	}
	bv, err := ValueToInterface(b, ctx)
	if err != nil {
		return err
	}
	if util.Compare(av, bv) == 0 {
		return iter(ctx)
	}
	return nil
}

// evalEqUnify is the top level of the unification implementation.
//
// When evaluating equality expressions, OPA tries to unify variables
// with values or other variables in the expression.
//
// The simplest case for unification is an expression of the form "<var> = ???".
// In this case, the variable is unified/bound to the other side the expression
// and evaluation continues to the next expression.
//
// In cases involving composites, OPA tries to unify elements in the same position
// of collections. For example, given an expression "[1,2,3] = [1,x,y]", OPA will
// unify variables x and y with the numbers 2 and 3. This process happens recursively,
// such that unification can happen on deeply embedded values.
//
// In cases involving references, OPA assumes that the references are ground at this stage.
// As a result, references are just special cases of the normal scalar/composite unification.
func evalEqUnify(ctx *Context, a ast.Value, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {

	// Plug bindings into both terms because this will be called recursively and there may be
	// new bindings that have been made as part of unification.
	a = PlugValue(a, ctx)
	b = PlugValue(b, ctx)

	switch a := a.(type) {
	case ast.Var:
		return evalEqUnifyVar(ctx, a, b, prev, iter)
	case ast.Object:
		return evalEqUnifyObject(ctx, a, b, prev, iter)
	case ast.Array:
		return evalEqUnifyArray(ctx, a, b, prev, iter)
	default:
		switch b := b.(type) {
		case ast.Var:
			return evalEqUnifyVar(ctx, b, a, prev, iter)
		case ast.Array:
			return evalEqUnifyArray(ctx, b, a, prev, iter)
		case ast.Object:
			return evalEqUnifyObject(ctx, b, a, prev, iter)
		default:
			return prev, evalEqGround(ctx, a, b, iter)
		}
	}

}

func evalEqUnifyArray(ctx *Context, a ast.Array, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	switch b := b.(type) {
	case ast.Var:
		return evalEqUnifyVar(ctx, b, a, prev, iter)
	case ast.Ref:
		return evalEqUnifyArrayRef(ctx, a, b, prev, iter)
	case ast.Array:
		return evalEqUnifyArrays(ctx, a, b, prev, iter)
	default:
		return prev, nil
	}
}

func evalEqUnifyArrayRef(ctx *Context, a ast.Array, b ast.Ref, prev *Undo, iter Iterator) (*Undo, error) {

	// TODO(tsandall): should not be accessing txn here?
	r, err := ctx.Store.Read(ctx.txn, b)
	if err != nil {
		return prev, err
	}

	slice, ok := r.([]interface{})
	if !ok {
		return prev, nil
	}

	if len(a) != len(slice) {
		return prev, nil
	}

	for i := range a {
		var tmp *Context
		child := make(ast.Ref, len(b), len(b)+1)
		copy(child, b)
		child = append(child, ast.NumberTerm(float64(i)))
		p, err := evalEqUnify(ctx, a[i].Value, child, prev, func(ctx *Context) error {
			tmp = ctx
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		ctx = tmp
	}
	return prev, iter(ctx)
}

func evalEqUnifyArrays(ctx *Context, a ast.Array, b ast.Array, prev *Undo, iter Iterator) (*Undo, error) {
	aLen := len(a)
	bLen := len(b)
	if aLen != bLen {
		return nil, nil
	}
	for i := 0; i < aLen; i++ {
		ai := a[i].Value
		bi := b[i].Value
		var tmp *Context
		p, err := evalEqUnify(ctx, ai, bi, prev, func(ctx *Context) error {
			tmp = ctx
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		ctx = tmp
	}
	return prev, iter(ctx)
}

// evalEqUnifyObject attempts to unify the object "a" with some other value "b".
// TODO(tsandal): unification of object keys (or unordered sets in general) is not
// supported because it would be too expensive. We may revisit this in the future.
func evalEqUnifyObject(ctx *Context, a ast.Object, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	switch b := b.(type) {
	case ast.Var:
		return evalEqUnifyVar(ctx, b, a, prev, iter)
	case ast.Ref:
		return evalEqUnifyObjectRef(ctx, a, b, prev, iter)
	case ast.Object:
		return evalEqUnifyObjects(ctx, a, b, prev, iter)
	default:
		return nil, nil
	}
}

func evalEqUnifyObjectRef(ctx *Context, a ast.Object, b ast.Ref, prev *Undo, iter Iterator) (*Undo, error) {

	// TODO(tsandall): should not be accessing txn here?
	r, err := ctx.Store.Read(ctx.txn, b)

	if err != nil {
		return prev, err
	}

	for i := range a {
		if !a[i][0].IsGround() {
			return prev, fmt.Errorf("illegal variable object key: %v", a[i][0])
		}
	}

	obj, ok := r.(map[string]interface{})
	if !ok {
		return prev, nil
	}

	if len(obj) != len(a) {
		return prev, nil
	}

	for i := range a {
		// TODO(tsandall): support non-string keys in storage.
		k, ok := a[i][0].Value.(ast.String)
		if !ok {
			return prev, fmt.Errorf("illegal object key type %T: %v", a[i][0], a[i][0])
		}

		_, ok = obj[string(k)]
		if !ok {
			return nil, nil
		}

		child := make(ast.Ref, len(b), len(b)+1)
		copy(child, b)
		child = append(child, a[i][0])
		var tmp *Context
		p, err := evalEqUnify(ctx, a[i][1].Value, child, prev, func(ctx *Context) error {
			tmp = ctx
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		ctx = tmp
	}
	return prev, iter(ctx)
}

func evalEqUnifyObjects(ctx *Context, a ast.Object, b ast.Object, prev *Undo, iter Iterator) (*Undo, error) {

	if len(a) != len(b) {
		return nil, nil
	}

	for i := range a {
		if !a[i][0].IsGround() {
			return prev, fmt.Errorf("illegal variable object key: %v", a[i][0])
		}
		if !b[i][0].IsGround() {
			return prev, fmt.Errorf("illegal variable object key: %v", b[i][0])
		}
	}

	for i := range a {
		var tmp *Context
		for j := range b {
			if b[j][0].Equal(a[i][0]) {
				p, err := evalEqUnify(ctx, a[i][1].Value, b[j][1].Value, prev, func(ctx *Context) error {
					tmp = ctx
					return nil
				})
				prev = p
				if err != nil {
					return nil, err
				}
				if tmp == nil {
					break
				}
			}
		}
		if tmp == nil {
			return nil, nil
		}
		ctx = tmp
	}

	return prev, iter(ctx)
}

func evalEqUnifyVar(ctx *Context, a ast.Var, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	undo := ctx.Bind(a, b, prev)
	err := iter(ctx)
	return undo, err
}
