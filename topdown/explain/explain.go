// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package explain

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

// Truth implements post-processing on raw traces. The goal of the
// post-processing is to produce a filtered version of the trace that shows why
// the top-level query was true.
func Truth(compiler *ast.Compiler, trace []*topdown.Event) ([]*topdown.Event, error) {

	truth := &truth{
		compiler: compiler,
		source:   nil,
		byTime:   nil,
		byQuery:  map[uint64][]*node{},
		allPaths: map[uint64]struct{}{},
	}

	// Process each event in the trace, updating the state stored on the truth
	// struct. Once all events have been processed, return the answer.
	for _, event := range trace {
		if err := truth.Update(event); err != nil {
			return nil, err
		}
	}

	return truth.Answer(), nil
}

// truth contains state used to perform post-processing on traces.
type truth struct {
	compiler *ast.Compiler
	source   *node
	byTime   *node
	byQuery  map[uint64][]*node
	allPaths map[uint64]struct{}
}

func (t *truth) Update(event *topdown.Event) error {

	n := &node{event: event}
	qid := event.QueryID

	// First event initializes time and source of graph.
	if t.source == nil {
		t.source = n
		t.byTime = n
		t.byQuery[qid] = append(t.byQuery[qid], n)
		return nil
	}

	// Check if all paths are required. If all paths are required or this event
	// does not represent a branch in the search, just link the node to the
	// previous event in time.
	//
	// TODO(tsandall): it's possible that we could perform more filtering on
	// child queries and avoid showing all paths. Need to consider what users
	// need to see for negation and full evaluation cases...
	allPaths := t.checkAndSetAllPaths(event)
	if event.Op != topdown.RedoOp || allPaths {
		t.byTime.AddEdge(n)
		t.byTime = n
		t.byQuery[qid] = append(t.byQuery[qid], n)
		return nil
	}

	// Handle branch in search.
	switch event.Node.(type) {
	case *ast.Rule:
		return t.updateRedoRule(n)
	case *ast.Expr:
		return t.updateRedoExpr(n)
	}

	return nil
}

// Answer returns the filtered trace by performing a depth-first traversal on
// the graph. The traversal goes from source to sink, where a sink is one of the
// exits events of the top-level query.
func (t *truth) Answer() (result []*topdown.Event) {

	byQuery := t.byQuery[t.source.event.QueryID]
	var sink *node
	for _, node := range byQuery {
		if node.event.Op == topdown.ExitOp {
			sink = node
			break
		}
	}

	if sink == nil {
		return nil
	}

	traversal := newTruthTraversal(t)
	nodes := util.DFS(traversal, t.source, sink)

	for _, n := range nodes {
		node := n.(*node)
		result = append(result, node.event)
	}

	return result
}

// checkAndSetAllPaths returns true if all search paths should be included for
// this query. All search paths are included for negated expressions, full
// references to partial definitions of objects and sets, and comprehensions.
func (t *truth) checkAndSetAllPaths(event *topdown.Event) bool {

	_, ok := t.allPaths[event.QueryID]
	if ok {
		return ok
	}

	_, ok = t.allPaths[event.ParentID]
	if ok {
		t.allPaths[event.QueryID] = struct{}{}
		return ok
	}

	if event.Op != topdown.EnterOp {
		return false
	}

	prevQuery := t.byQuery[event.ParentID]
	prev := prevQuery[len(prevQuery)-1]
	prevExpr := prev.event.Node.(*ast.Expr)

	switch node := event.Node.(type) {
	case *ast.Rule:
		if node.DocKind() == ast.PartialObjectDoc || node.DocKind() == ast.PartialSetDoc {
			plugged := topdown.PlugExpr(prevExpr, prev.event.Locals.Get)
			found := false
			ast.WalkRefs(plugged, func(r ast.Ref) bool {
				rules := t.compiler.GetRulesWithPrefix(r)
				for _, rule := range rules {
					if rule.Equal(node) {
						found = true
						return true
					}
				}
				return false
			})
			if found {
				t.allPaths[event.QueryID] = struct{}{}
				return true
			}
		}
	case ast.Body:
		if prevExpr.Negated {
			t.allPaths[event.QueryID] = struct{}{}
		} else {
			found := false
			ast.WalkClosures(prevExpr, func(x interface{}) bool {
				if ac, ok := x.(*ast.ArrayComprehension); ok {
					if ac.Body.Equal(node) {
						found = true
						return true
					}
				}
				return false
			})
			if found {
				t.allPaths[event.QueryID] = struct{}{}
				return true
			}
		}
	}
	return false
}

// updateRedoRule will link the node to the most recent expression in the parent
// query. This represents a branch in the search.
func (t *truth) updateRedoRule(n *node) error {
	qid := n.event.QueryID
	byQuery := t.byQuery[n.event.ParentID]
	byQuery[len(byQuery)-1].AddEdge(n)
	t.byTime = n
	t.byQuery[qid] = append(t.byQuery[qid], n)
	return nil
}

// updateRedoExpr will link the node to the previous node in the query *before*
// the restart, i.e., the previous expression or the previous enter/redo of the
// rule/body. This represents a branch in the search.
func (t *truth) updateRedoExpr(n *node) error {

	qid := n.event.QueryID
	byQuery := t.byQuery[qid]
	expr := n.event.Node.(*ast.Expr)

	var prev *node

	if expr.Index == 0 {
		prev = t.findQueryRestart(byQuery)
	} else {
		prev = t.findExprRestart(byQuery, expr.Index)
	}

	if prev == nil {
		return fmt.Errorf("cannot add %v to graph, restart not found", n)
	}

	prev.AddEdge(n)
	t.byTime = n
	t.byQuery[qid] = append(byQuery, n)

	return nil
}

func (t *truth) findQueryRestart(byQuery []*node) *node {
	for i := len(byQuery) - 1; i >= 0; i-- {
		_, isBody := byQuery[i].event.Node.(ast.Body)
		_, isRule := byQuery[i].event.Node.(*ast.Rule)
		if isBody || isRule {
			return byQuery[i]
		}
	}
	return nil
}

func (t *truth) findExprRestart(byQuery []*node, index int) *node {
	for i := len(byQuery) - 1; i >= 0; i-- {
		prev, ok := byQuery[i].event.Node.(*ast.Expr)
		if ok && prev.Index == (index-1) {
			return byQuery[i]
		}
	}
	return nil
}

type node struct {
	event *topdown.Event
	edges []*node
}

func (n *node) String() string {
	return fmt.Sprintf("%v", n.event)
}

func (n *node) AddEdge(other *node) {
	n.edges = append(n.edges, other)
}

type truthTraversal struct {
	truth   *truth
	visited map[*node]struct{}
}

func newTruthTraversal(truth *truth) *truthTraversal {
	return &truthTraversal{
		truth:   truth,
		visited: map[*node]struct{}{},
	}
}

func (t *truthTraversal) Edges(u util.T) []util.T {
	un := u.(*node)
	r := make([]util.T, len(un.edges))
	for i := range un.edges {
		r[i] = un.edges[i]
	}
	return r
}

func (t *truthTraversal) Equals(u util.T, v util.T) bool {
	un := u.(*node)
	vn := v.(*node)
	return un == vn
}

func (t *truthTraversal) Visited(u util.T) bool {
	un := u.(*node)
	_, ok := t.visited[un]
	if ok {
		return true
	}
	t.visited[un] = struct{}{}
	return false
}
