// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import "sort"

// Visitor defines the interface for visiting IR nodes.
type Visitor interface {
	Before(x interface{})
	Visit(x interface{}) (Visitor, error)
	After(x interface{})
}

// Walk invokes the visitor for nodes under x.
func Walk(vis Visitor, x interface{}) error {
	impl := walkerImpl{
		vis: vis,
	}
	impl.walk(x)
	return impl.err
}

type walkerImpl struct {
	vis Visitor
	err error
}

func (w *walkerImpl) walk(x interface{}) {

	if x == nil {
		return
	}

	prev := w.vis
	w.vis.Before(x)
	defer w.vis.After(x)
	w.vis, w.err = w.vis.Visit(x)
	if w.err != nil {
		return
	} else if w.vis == nil {
		w.vis = prev
		return
	}

	switch x := x.(type) {
	case *Policy:
		w.walk(x.Static)
		w.walk(x.Plan)
		w.walk(x.Funcs)
	case *Static:
		for _, s := range x.Strings {
			w.walk(s)
		}
	case *Funcs:
		keys := make([]string, 0, len(x.Funcs))
		for k := range x.Funcs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			w.walk(x.Funcs[k])
		}
	case *Func:
		for _, b := range x.Blocks {
			w.walk(b)
		}
	case *Plan:
		for _, b := range x.Blocks {
			w.walk(b)
		}
	case *Block:
		for _, s := range x.Stmts {
			w.walk(s)
		}
	case *BlockStmt:
		for _, b := range x.Blocks {
			w.walk(b)
		}
	case *ScanStmt:
		w.walk(x.Block)
	case *NotStmt:
		w.walk(x.Block)
	}
}
