// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"regexp"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

var regexpCacheLock = sync.Mutex{}
var regexpCache map[string]*regexp.Regexp

func evalRegexMatch(ctx *Context, expr *ast.Expr, iter Iterator) error {
	ops := expr.Terms.([]*ast.Term)
	pat, err := ValueToString(ops[1].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "re_match: pattern value must be a string")
	}
	input, err := ValueToString(ops[2].Value, ctx)
	if err != nil {
		return errors.Wrapf(err, "re_match: input value must be a string")
	}
	re, err := getRegexp(pat)
	if err != nil {
		return err
	}
	if re.Match([]byte(input)) {
		return iter(ctx)
	}
	return nil
}

func getRegexp(pat string) (*regexp.Regexp, error) {
	regexpCacheLock.Lock()
	defer regexpCacheLock.Unlock()
	re, ok := regexpCache[pat]
	if !ok {
		var err error
		re, err = regexp.Compile(string(pat))
		if err != nil {
			return nil, errors.Wrapf(err, "re_match")
		}
		regexpCache[pat] = re
	}
	return re, nil
}

func init() {
	regexpCache = map[string]*regexp.Regexp{}
}
