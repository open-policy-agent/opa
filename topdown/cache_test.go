// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestVirtualCacheInvalidate(t *testing.T) {
	cache := newVirtualCache()
	cache.Push()
	cache.Put(ast.MustParseRef("data.x.p"), ast.BooleanTerm(true))
	cache.Pop()
	result := cache.Get(ast.MustParseRef("data.x.p"))
	if result != nil {
		t.Fatal("Expected nil result but got:", result)
	}
}
