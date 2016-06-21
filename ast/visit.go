// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// Visitor defines the interface for iterating AST elements.
// The Visit function can return a Visitor w which will be
// used to visit the children of the AST element v. If the
// Visit function returns nil, the children will not be visited.
type Visitor interface {
	Visit(v interface{}) (w Visitor)
}

// Walk iterates the AST by calling the Visit function on the Visitor
// v for x before recursing.
func Walk(v Visitor, x interface{}) {
	if t, ok := x.(*Term); ok {
		Walk(v, t.Value)
		return
	}
	w := v.Visit(x)
	if w == nil {
		return
	}
	switch x := x.(type) {
	case *Module:
		Walk(w, x.Package)
		for _, i := range x.Imports {
			Walk(w, i)
		}
		for _, r := range x.Rules {
			Walk(w, r)
		}
	case *Package:
		Walk(w, x.Path)
	case *Import:
		Walk(w, x.Path.Value)
		Walk(w, x.Alias)
	case *Rule:
		Walk(w, x.Name)
		if x.Key != nil {
			Walk(w, x.Key.Value)
		}
		if x.Value != nil {
			Walk(w, x.Value.Value)
		}
		Walk(w, x.Body)
	case Body:
		for _, e := range x {
			Walk(w, e)
		}
	case *Expr:
		switch ts := x.Terms.(type) {
		case []*Term:
			for _, t := range ts {
				Walk(w, t.Value)
			}
		case *Term:
			Walk(w, ts.Value)
		}
	case Ref:
		for _, t := range x {
			Walk(w, t.Value)
		}
	case Object:
		for _, t := range x {
			Walk(w, t[0].Value)
			Walk(w, t[1].Value)
		}
	case Array:
		for _, t := range x {
			Walk(w, t.Value)
		}
	case *ArrayComprehension:
		Walk(w, x.Term)
		Walk(w, x.Body)
	}
}

// WalkClosures calls the function f on all closures under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkClosures(x interface{}, f func(interface{}) bool) {
	vis := &GenericVisitor{func(x interface{}) bool {
		switch x.(type) {
		case *ArrayComprehension:
			return f(x)
		}
		return false
	}}
	Walk(vis, x)
}

// WalkRefs calls the function f on all references under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkRefs(x interface{}, f func(Ref) bool) {
	vis := &GenericVisitor{func(x interface{}) bool {
		if r, ok := x.(Ref); ok {
			return f(r)
		}
		return false
	}}
	Walk(vis, x)
}

// GenericVisitor implements the Visitor interface to provide
// a utility to walk over AST nodes using a closure. If the closure
// returns true, the visitor will not walk over AST nodes under x.
type GenericVisitor struct {
	f func(x interface{}) bool
}

// Visit calls the function f on the GenericVisitor.
func (vis *GenericVisitor) Visit(x interface{}) Visitor {
	if vis.f(x) {
		return nil
	}
	return vis
}
