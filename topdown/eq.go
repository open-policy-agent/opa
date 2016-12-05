// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

func evalEq(t *Topdown, expr *ast.Expr, iter Iterator) error {

	operands := expr.Terms.([]*ast.Term)
	a := operands[1].Value
	b := operands[2].Value

	undo, err := evalEqUnify(t, a, b, nil, iter)
	t.Unbind(undo)
	return err
}

func evalEqGround(t *Topdown, a ast.Value, b ast.Value, iter Iterator) error {
	a, err := ResolveRefs(a, t)
	if err != nil {
		return err
	}
	b, err = ResolveRefs(b, t)
	if err != nil {
		return err
	}
	if ast.Compare(a, b) == 0 {
		return iter(t)
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
func evalEqUnify(t *Topdown, a ast.Value, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {

	// Plug bindings into both terms because this will be called recursively and there may be
	// new bindings that have been made as part of unification.
	a = PlugValue(a, t.Binding)
	b = PlugValue(b, t.Binding)

	switch a := a.(type) {
	case ast.Var:
		return evalEqUnifyVar(t, a, b, prev, iter)
	case ast.Object:
		return evalEqUnifyObject(t, a, b, prev, iter)
	case ast.Array:
		return evalEqUnifyArray(t, a, b, prev, iter)
	case *ast.Set:
		return evalEqUnifySet(t, a, b, prev, iter)
	default:
		switch b := b.(type) {
		case ast.Var:
			return evalEqUnifyVar(t, b, a, prev, iter)
		case ast.Array:
			return evalEqUnifyArray(t, b, a, prev, iter)
		case ast.Object:
			return evalEqUnifyObject(t, b, a, prev, iter)
		case *ast.Set:
			return evalEqUnifySet(t, b, a, prev, iter)
		default:
			return prev, evalEqGround(t, a, b, iter)
		}
	}

}

func evalEqUnifyArray(t *Topdown, a ast.Array, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	switch b := b.(type) {
	case ast.Var:
		return evalEqUnifyVar(t, b, a, prev, iter)
	case ast.Ref:
		return evalEqUnifyArrayRef(t, a, b, prev, iter)
	case ast.Array:
		return evalEqUnifyArrays(t, a, b, prev, iter)
	default:
		return prev, nil
	}
}

func evalEqUnifyArrayRef(t *Topdown, a ast.Array, b ast.Ref, prev *Undo, iter Iterator) (*Undo, error) {

	r, err := t.Resolve(b)
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
		var tmp *Topdown
		child := make(ast.Ref, len(b), len(b)+1)
		copy(child, b)
		child = append(child, ast.IntNumberTerm(i))
		p, err := evalEqUnify(t, a[i].Value, child, prev, func(t *Topdown) error {
			tmp = t
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		t = tmp
	}
	return prev, iter(t)
}

func evalEqUnifyArrays(t *Topdown, a ast.Array, b ast.Array, prev *Undo, iter Iterator) (*Undo, error) {
	aLen := len(a)
	bLen := len(b)
	if aLen != bLen {
		return nil, nil
	}
	for i := 0; i < aLen; i++ {
		ai := a[i].Value
		bi := b[i].Value
		var tmp *Topdown
		p, err := evalEqUnify(t, ai, bi, prev, func(t *Topdown) error {
			tmp = t
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		t = tmp
	}
	return prev, iter(t)
}

// evalEqUnifyObject attempts to unify the object "a" with some other value "b".
// TODO(tsandal): unification of object keys (or unordered sets in general) is not
// supported because it would be too expensive. We may revisit this in the future.
func evalEqUnifyObject(t *Topdown, a ast.Object, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	switch b := b.(type) {
	case ast.Var:
		return evalEqUnifyVar(t, b, a, prev, iter)
	case ast.Ref:
		return evalEqUnifyObjectRef(t, a, b, prev, iter)
	case ast.Object:
		return evalEqUnifyObjects(t, a, b, prev, iter)
	default:
		return nil, nil
	}
}

func evalEqUnifyObjectRef(t *Topdown, a ast.Object, b ast.Ref, prev *Undo, iter Iterator) (*Undo, error) {

	r, err := t.Resolve(b)

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
		var tmp *Topdown
		p, err := evalEqUnify(t, a[i][1].Value, child, prev, func(t *Topdown) error {
			tmp = t
			return nil
		})
		prev = p
		if err != nil {
			return nil, err
		}
		if tmp == nil {
			return nil, nil
		}
		t = tmp
	}
	return prev, iter(t)
}

func evalEqUnifyObjects(t *Topdown, a ast.Object, b ast.Object, prev *Undo, iter Iterator) (*Undo, error) {

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
		var tmp *Topdown
		for j := range b {
			if b[j][0].Equal(a[i][0]) {
				p, err := evalEqUnify(t, a[i][1].Value, b[j][1].Value, prev, func(t *Topdown) error {
					tmp = t
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
		t = tmp
	}

	return prev, iter(t)
}

func evalEqUnifySet(t *Topdown, a *ast.Set, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	switch b := b.(type) {
	case *ast.Set:
		return evalEqSets(t, a, b, prev, iter)
	case ast.Var:
		return evalEqUnifyVar(t, b, a, prev, iter)
	default:
		return prev, nil
	}
}

func evalEqSets(t *Topdown, a *ast.Set, b *ast.Set, prev *Undo, iter Iterator) (*Undo, error) {

	x, err := ResolveRefs(a, t)
	if err != nil {
		return prev, err
	}

	a = x.(*ast.Set)

	y, err := ResolveRefs(b, t)
	if err != nil {
		return prev, err
	}

	b = y.(*ast.Set)

	if a.Equal(b) {
		return prev, iter(t)
	}

	return prev, nil
}

func evalEqUnifyVar(t *Topdown, a ast.Var, b ast.Value, prev *Undo, iter Iterator) (*Undo, error) {
	undo := t.Bind(a, b, prev)
	err := iter(t)
	return undo, err
}
