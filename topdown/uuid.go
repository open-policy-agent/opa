// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"crypto/rand"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/uuid"
)

type uuidCachingKey string

func builtinUUID(bctx BuiltinContext, args []*ast.Term, iter func(*ast.Term) error) error {
	var cachingKey = uuidCachingKey("UUID-" + args[0].Value.String())

	id, ok := bctx.Cache.Get(cachingKey)

	var uuidv4 *ast.Term

	if !ok {
		var err error
		var newUUID string

		newUUID, err = uuid.New(rand.Reader)
		if err != nil {
			return err
		}

		uuidv4 = ast.NewTerm(ast.String(newUUID))
		bctx.Cache.Put(cachingKey, uuidv4)
	} else {
		uuidv4 = id.(*ast.Term)
	}

	return iter(uuidv4)
}

func init() {
	RegisterBuiltinFunc(ast.UUID.Name, builtinUUID)
}
