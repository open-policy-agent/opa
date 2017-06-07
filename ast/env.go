// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/types"
)

// TypeEnv contains type info for static analysis such as type checking.
type TypeEnv struct {
	funcs map[String][]types.Type
	tree  *typeTreeNode
	next  *TypeEnv
}

// NewTypeEnv returns an empty TypeEnv.
func NewTypeEnv() *TypeEnv {
	return &TypeEnv{
		funcs: map[String][]types.Type{},
		tree:  newTypeTree(),
	}
}

// GetFunc returns the type array corresponding to the arguments of the function
// referred to by name. GetFunc returns nil if there is no function matching that
// name.
func (env *TypeEnv) GetFunc(name String) []types.Type {
	tps, ok := env.funcs[name]
	if !ok && env.next != nil {
		return env.next.GetFunc(name)
	}
	return tps
}

// PutFunc inserts the type information for the function referred to by name into
// this TypeEnv.
func (env *TypeEnv) PutFunc(name String, args []types.Type) {
	env.funcs[name] = args
}

// Get returns the type of x.
func (env *TypeEnv) Get(x interface{}) types.Type {

	if term, ok := x.(*Term); ok {
		x = term.Value
	}

	switch x := x.(type) {

	// Scalars.
	case Null:
		return types.NewNull()
	case Boolean:
		return types.NewBoolean()
	case Number:
		return types.NewNumber()
	case String:
		return types.NewString()

	// Composites.
	case Array:
		static := make([]types.Type, len(x))
		for i := range static {
			tpe := env.Get(x[i].Value)
			static[i] = tpe
		}

		var dynamic types.Type
		if len(static) == 0 {
			dynamic = types.A
		}

		return types.NewArray(static, dynamic)

	case Object:
		static := []*types.Property{}
		// TODO(tsandall): handle non-string keys?
		for _, pair := range x {
			if k, ok := pair[0].Value.(String); ok {
				tpe := env.Get(pair[1].Value)
				static = append(static, types.NewProperty(string(k), tpe))
			}
		}

		var dynamic types.Type
		if len(static) == 0 {
			dynamic = types.A
		}

		return types.NewObject(static, dynamic)

	case *Set:
		var tpe types.Type
		x.Iter(func(elem *Term) bool {
			other := env.Get(elem.Value)
			tpe = types.Or(tpe, other)
			return false
		})
		if tpe == nil {
			tpe = types.A
		}
		return types.NewSet(tpe)

	// Comprehensions.
	case *ArrayComprehension:

		checker := newTypeChecker()
		cpy, errs := checker.CheckBody(env, x.Body)
		if len(errs) == 0 {
			return types.NewArray(nil, cpy.Get(x.Term))
		}

		return nil

	// Refs.
	case Ref:
		return env.getRef(x)

	// Vars.
	case Var:
		if node := env.tree.Child(x); node != nil {
			return node.Value()
		}
		if env.next != nil {
			return env.next.Get(x)
		}
		return nil

	default:
		panic("unreachable")
	}
}

func (env *TypeEnv) getRef(ref Ref) types.Type {

	node := env.tree.Child(ref[0].Value)
	if node == nil {
		return env.getRefFallback(ref)
	}

	return env.getRefRec(node, ref, ref[1:])
}

func (env *TypeEnv) getRefFallback(ref Ref) types.Type {

	if env.next != nil {
		return env.next.Get(ref)
	}

	if RootDocumentNames.Contains(ref[0]) {
		return types.A
	}

	return nil
}

func (env *TypeEnv) getRefRec(node *typeTreeNode, ref, tail Ref) types.Type {
	if len(tail) == 0 {
		return env.getRefRecExtent(node)
	}

	if node.Leaf() {
		return selectRef(node.Value(), tail)
	}

	if !IsConstant(tail[0].Value) {
		return selectRef(env.getRefRecExtent(node), tail)
	}

	child := node.Child(tail[0].Value)
	if child == nil {
		return env.getRefFallback(ref)
	}

	return env.getRefRec(child, ref, tail[1:])
}

func (env *TypeEnv) getRefRecExtent(node *typeTreeNode) types.Type {

	if node.Leaf() {
		return node.Value()
	}

	children := []*types.Property{}

	for key, child := range node.Children() {
		tpe := env.getRefRecExtent(child)
		// TODO(tsandall): handle non-string keys?
		if s, ok := key.(String); ok {
			children = append(children, types.NewProperty(string(s), tpe))
		}
	}

	// TODO(tsandall): for now, these objects can have any dynamic properties
	// because we don't have schema for base docs. Once schemas are supported
	// we can improve this.
	return types.NewObject(children, types.A)
}

func (env *TypeEnv) wrap() *TypeEnv {
	cpy := *env
	cpy.next = env
	cpy.tree = newTypeTree()
	return &cpy
}

// typeTreeNode is used to store type information in a tree.
type typeTreeNode struct {
	key      Value
	value    types.Type
	children map[Value]*typeTreeNode
}

func newTypeTree() *typeTreeNode {
	return &typeTreeNode{
		key:      nil,
		value:    nil,
		children: map[Value]*typeTreeNode{},
	}
}

func (n *typeTreeNode) Child(key Value) *typeTreeNode {
	return n.children[key]
}

func (n *typeTreeNode) Children() map[Value]*typeTreeNode {
	return n.children
}

func (n *typeTreeNode) Get(path Ref) types.Type {
	curr := n
	for _, term := range path {
		child, ok := curr.children[term.Value]
		if !ok {
			return nil
		}
		curr = child
	}
	return curr.Value()
}

func (n *typeTreeNode) Leaf() bool {
	return n.value != nil
}

func (n *typeTreeNode) PutOne(key Value, tpe types.Type) {
	child, ok := n.children[key]
	if !ok {
		child = newTypeTree()
		child.key = key
		n.children[key] = child
	}
	child.value = tpe
}

func (n *typeTreeNode) Put(path Ref, tpe types.Type) {
	curr := n
	for _, term := range path {
		child, ok := curr.children[term.Value]
		if !ok {
			child = newTypeTree()
			child.key = term.Value
			curr.children[child.key] = child
		}
		curr = child
	}
	curr.value = tpe
}

func (n *typeTreeNode) Value() types.Type {
	return n.value
}

// selectConstant returns the attribute of the type referred to by the term. If
// the attribute type cannot be determined, nil is returned.
func selectConstant(tpe types.Type, term *Term) types.Type {
	switch v := term.Value.(type) {
	case String:
		return types.Select(tpe, string(v))
	case Number:
		return types.Select(tpe, json.Number(v))
	case Boolean:
		return types.Select(tpe, bool(v))
	case Null:
		return types.Select(tpe, nil)
	default:
		return nil
	}
}

// selectRef returns the type of the nested attribute referred to by ref. If
// the attribute type cannot be determined, nil is returned. If the ref
// contains vars or refs, then the returned type will be a union of the
// possible types.
func selectRef(tpe types.Type, ref Ref) types.Type {

	if tpe == nil || len(ref) == 0 {
		return tpe
	}

	head, tail := ref[0], ref[1:]

	switch head.Value.(type) {
	case Var, Ref:
		return selectRef(types.Values(tpe), tail)
	default:
		return selectRef(selectConstant(tpe, head), tail)
	}
}
