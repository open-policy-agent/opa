// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

var (
	errBadPath        = errors.New("bad document path")
	errBadArrayIndex  = errors.New("bad document path: array index must be an integer within the bounds of a known array")
	errNumberNotArray = errors.New("bad document path: number path elements may only index into a known array")
)

func mergeTermWithValues(exist *ast.Term, pairs [][2]*ast.Term) (*ast.Term, error) {
	var result *ast.Term
	var init bool

	for i, pair := range pairs {
		if err := validWithTargetPath(pair[0].Value); err != nil {
			return nil, err
		}

		target := pair[0].Value.(ast.Ref)

		// Copy the value if subsequent pairs in the slice would modify it.
		for j := i + 1; j < len(pairs); j++ {
			other := pairs[j][0].Value.(ast.Ref)
			if len(other) > len(target) && other.HasPrefix(target) {
				pair[1] = pair[1].Copy()
				break
			}
		}

		if len(target) == 1 {
			result = pair[1]
			init = true
			continue
		}

		if !init {
			result = exist.Copy()
			init = true
		}

		var err error
		result, err = setPath(result, target[1:], pair[1])
		if err != nil {
			return nil, err
		}
	}

	if !init {
		result = exist
	}

	return result, nil
}

// setPath sets the value v at path in the document doc and returns the
// (possibly replaced) document. Objects along the path are updated in place
// and missing or non-object intermediate values are replaced by objects.
// Number path elements index into arrays: the array must already exist at that
// point in the document and the index must be an integer within its bounds,
// as the length of the array cannot be changed by a with statement.
func setPath(doc *ast.Term, path ast.Ref, v *ast.Term) (*ast.Term, error) {
	if doc == nil || !canDescend(doc, path[0]) {
		return makeTreeChecked(path, v)
	}

	node := doc
	for i := range path {
		key := path[i]
		last := i == len(path)-1

		switch x := node.Value.(type) {
		case ast.Object:
			if !last {
				if child := x.Get(key); child != nil && canDescend(child, path[i+1]) {
					node = child
					continue
				}
				sub, err := makeTreeChecked(path[i+1:], v)
				if err != nil {
					return nil, err
				}
				x.Insert(key, sub)
				return doc, nil
			}
			x.Insert(key, v)
			return doc, nil

		case *ast.Array:
			idx, err := arrayIndex(x, key)
			if err != nil {
				return nil, err
			}
			if !last {
				if child := x.Elem(idx); canDescend(child, path[i+1]) {
					node = child
					continue
				}
				sub, err := makeTreeChecked(path[i+1:], v)
				if err != nil {
					return nil, err
				}
				x.Set(idx, sub)
				return doc, nil
			}
			x.Set(idx, v)
			return doc, nil
		}
	}

	// Unreachable: canDescend guarantees node is an object or array.
	return nil, errBadPath
}

// canDescend returns true if the next path element key can be applied to the
// existing value of node rather than replacing it. Strings descend into
// objects and numbers into arrays. A number on anything other than an array
// is reported as an error by makeTreeChecked.
func canDescend(node *ast.Term, key *ast.Term) bool {
	switch node.Value.(type) {
	case ast.Object:
		_, ok := key.Value.(ast.String)
		return ok
	case *ast.Array:
		_, ok := key.Value.(ast.Number)
		return ok
	}
	return false
}

func arrayIndex(arr *ast.Array, key *ast.Term) (int, error) {
	num, ok := key.Value.(ast.Number)
	if !ok {
		return 0, errBadPath
	}
	idx, ok := num.Int()
	if !ok || idx < 0 || idx >= arr.Len() {
		return 0, errBadArrayIndex
	}
	return idx, nil
}

// makeTreeChecked is like makeTree but refuses paths with number elements, as
// those can only be used to index into an existing array.
func makeTreeChecked(k ast.Ref, v *ast.Term) (*ast.Term, error) {
	for _, x := range k {
		if _, ok := x.Value.(ast.Number); ok {
			return nil, errNumberNotArray
		}
	}
	return makeTree(k, v), nil
}

// validWithTargetPath returns an error if v is not a valid with target path
// for the input or data document: it must be input, data, or a ref rooted in
// one of them where every other element is a string or a number.
func validWithTargetPath(v ast.Value) error {
	switch v := v.(type) {
	case ast.Var:
		if !v.Equal(ast.DefaultRootDocument.Value) && !v.Equal(ast.InputRootDocument.Value) {
			return errBadPath
		}
	case ast.Ref:
		if err := validWithTargetPath(v[0].Value); err != nil {
			return err
		}
		for _, x := range v[1:] {
			switch x.Value.(type) {
			case ast.String, ast.Number:
			default:
				return errBadPath
			}
		}
	default:
		return errBadPath
	}
	return nil
}

// makeTree returns an object that represents a document where the value v is
// the leaf and elements in k represent intermediate objects.
func makeTree(k ast.Ref, v *ast.Term) *ast.Term {
	for _, v0 := range slices.Backward(k) {
		v = ast.ObjectTerm(ast.Item(v0, v))
	}
	return v
}
