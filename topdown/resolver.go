// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/resolver"
)

type resolverTrie struct {
	r        resolver.Resolver
	children map[ast.Value]*resolverTrie
}

func newResolverTrie() *resolverTrie {
	return &resolverTrie{children: map[ast.Value]*resolverTrie{}}
}

func (t *resolverTrie) Put(ref ast.Ref, r resolver.Resolver) {
	node := t
	for _, t := range ref {
		child, ok := node.children[t.Value]
		if !ok {
			child = &resolverTrie{children: map[ast.Value]*resolverTrie{}}
			node.children[t.Value] = child
		}
		node = child
	}
	node.r = r
}

func (t *resolverTrie) Resolve(ctx context.Context, ref ast.Ref, input *ast.Term) (ast.Value, error) {
	in := resolver.Input{
		Ref:   ref,
		Input: input,
	}
	node := t
	for i, t := range ref {
		child, ok := node.children[t.Value]
		if !ok {
			return nil, nil
		}
		node = child
		if node.r != nil {
			result, err := node.r.Eval(ctx, in)
			if err != nil {
				return nil, err
			}
			return result.Value.Find(ref[i+1:])
		}
	}
	return node.mktree(ctx, in)
}

func (t *resolverTrie) mktree(ctx context.Context, in resolver.Input) (ast.Value, error) {
	if t.r != nil {
		result, err := t.r.Eval(ctx, in)
		if err != nil {
			return nil, err
		}
		return result.Value, nil
	}
	obj := ast.NewObject()
	for k, child := range t.children {
		v, err := child.mktree(ctx, resolver.Input{Ref: append(in.Ref, ast.NewTerm(k)), Input: in.Input})
		if err != nil {
			return nil, err
		}
		obj.Insert(ast.NewTerm(k), ast.NewTerm(v))
	}
	return obj, nil
}
