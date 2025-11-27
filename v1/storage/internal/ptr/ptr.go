// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package ptr provides utilities for pointer operations using storage layer paths.
package ptr

import (
	"strconv"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/internal/errors"
)

func Ptr(data any, path storage.Path) (any, error) {
	node := data
	for i := range path {
		key := path[i]
		switch curr := node.(type) {
		case map[string]any:
			var ok bool
			if node, ok = curr[key]; !ok {
				return nil, errors.NotFoundErr
			}
		case []any:
			pos, err := ValidateArrayIndex(curr, key, path)
			if err != nil {
				return nil, err
			}
			node = curr[pos]
		default:
			return nil, errors.NotFoundErr
		}
	}

	return node, nil
}

func ValuePtr(data ast.Value, path storage.Path) (ast.Value, error) {
	var keyTerm *ast.Term

	defer func() {
		if keyTerm != nil {
			ast.TermPtrPool.Put(keyTerm)
		}
	}()

	node := data
	for i := range path {
		key := path[i]
		switch curr := node.(type) {
		case ast.Object:
			// Note(anders):
			// This term is only created for the lookup, which is not great — especially
			// considering the path likely was converted from a ref, where we had all
			// the terms available already! Without chaging the storage API, our options
			// for performant lookups are limitied to using interning or a pool. Prefer
			// interning when possible, as that is zero alloc. Using the pool avoids at
			// least allocating a new term for every lookup, but still requires an alloc
			// for the string Value.
			if ast.HasInternedValue(key) {
				if val := curr.Get(ast.InternedTerm(key)); val != nil {
					node = val.Value
				} else {
					return nil, errors.NotFoundErr
				}
			} else {
				if keyTerm == nil {
					keyTerm = ast.TermPtrPool.Get()
				}
				// 1 alloc
				keyTerm.Value = ast.String(key)
				if val := curr.Get(keyTerm); val != nil {
					node = val.Value
				} else {
					return nil, errors.NotFoundErr
				}
			}
		case *ast.Array:
			pos, err := ValidateASTArrayIndex(curr, key, path)
			if err != nil {
				return nil, err
			}
			node = curr.Elem(pos).Value
		default:
			return nil, errors.NotFoundErr
		}
	}

	return node, nil
}

func ValidateArrayIndex(arr []any, s string, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, errors.NewNotFoundErrorWithHint(path, errors.ArrayIndexTypeMsg)
	}
	return inRange(idx, arr, path)
}

func ValidateASTArrayIndex(arr *ast.Array, s string, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, errors.NewNotFoundErrorWithHint(path, errors.ArrayIndexTypeMsg)
	}
	return inRange(idx, arr, path)
}

// ValidateArrayIndexForWrite also checks that `s` is a valid way to address an
// array element like `ValidateArrayIndex`, but returns a `resource_conflict` error
// if it is not.
func ValidateArrayIndexForWrite(arr []any, s string, i int, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, errors.NewWriteConflictError(path[:i-1])
	}
	return inRange(idx, arr, path)
}

func isInt(s string) (int, bool) {
	idx, err := strconv.Atoi(s)
	return idx, err == nil
}

func inRange(i int, arr any, path storage.Path) (int, error) {

	var arrLen int

	switch v := arr.(type) {
	case []any:
		arrLen = len(v)
	case *ast.Array:
		arrLen = v.Len()
	}

	if i < 0 || i >= arrLen {
		return 0, errors.NewNotFoundErrorWithHint(path, errors.OutOfRangeMsg)
	}
	return i, nil
}
