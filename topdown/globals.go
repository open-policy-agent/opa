// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// MakeGlobals returns a globals mapping for the given key-value pairs.
func MakeGlobals(pairs [][2]*ast.Term) (*ast.ValueMap, error) {
	result := ast.NewValueMap()
	for _, pair := range pairs {
		k, v := pair[0], pair[1]
		switch k := k.Value.(type) {
		case ast.Ref:
			obj := makeTree(k[1:], v)
			switch b := result.Get(k[0].Value).(type) {
			case nil:
				result.Put(k[0].Value, obj)
			case ast.Object:
				m, ok := b.Merge(obj)
				if !ok {
					return nil, globalConflictErr(k)
				}
				result.Put(k[0].Value, m)
			default:
				return nil, globalConflictErr(k)
			}
		case ast.Var:
			if result.Get(k) != nil {
				return nil, globalConflictErr(k)
			}
			result.Put(k, v.Value)
		default:
			return nil, fmt.Errorf("invalid global: %v: path must be a variable or a reference", k)
		}
	}
	return result, nil
}

func globalConflictErr(k ast.Value) error {
	return fmt.Errorf("conflicting global: %v: check global arguments", k)
}

// makeTree returns an object that represents a document where the value v is the
// leaf and elements in k represent intermediate objects.
func makeTree(k ast.Ref, v *ast.Term) ast.Object {
	var obj ast.Object
	for i := len(k) - 1; i >= 1; i-- {
		obj = ast.Object{ast.Item(k[i], v)}
		v = &ast.Term{Value: obj}
		obj = ast.Object{}
	}
	obj = ast.Object{ast.Item(k[0], v)}
	return obj
}
