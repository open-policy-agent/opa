// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

// MakeRequest returns a request value for the given key/value pairs. Assumes
// keys are valid import paths.
func MakeRequest(pairs [][2]*ast.Term) (ast.Value, error) {

	// Fast-path for the root case.
	if len(pairs) == 1 && len(pairs[0][0].Value.(ast.Ref)) == 0 {
		return pairs[0][1].Value, nil
	}

	var request ast.Object

	for _, pair := range pairs {

		if err := ast.IsValidImportPath(pair[0].Value); err != nil {
			return nil, errors.Wrapf(err, "invalid request path")
		}

		k := pair[0].Value.(ast.Ref)
		obj := makeTree(k[1:], pair[1])
		var ok bool
		request, ok = request.Merge(obj)

		if !ok {
			return nil, fmt.Errorf("conflicting request value %v: check request parameters", k)
		}
	}

	return request, nil
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
