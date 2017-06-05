// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/open-policy-agent/opa/ast"
)

type nowKeyID string

var nowKey = nowKeyID("time.now_ns")

func builtinTimeNowNanos(t *Topdown, expr *ast.Expr, iter Iterator) error {

	operands := expr.Terms.([]*ast.Term)

	var now ast.Number
	exist, ok := t.builtins.Get(nowKey)

	if !ok {
		curr := time.Now()
		now = ast.Number(json.Number(strconv.FormatInt(curr.UnixNano(), 10)))
		t.builtins.Put(nowKey, now)
	} else {
		now = exist.(ast.Number)
	}

	return unifyAndContinue(t, iter, now, operands[1].Value)
}

func init() {
	RegisterBuiltinFunc(ast.NowNanos.Name, builtinTimeNowNanos)
}
