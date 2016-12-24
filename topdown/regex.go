// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"regexp"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/pkg/errors"
)

var regexpCacheLock = sync.Mutex{}
var regexpCache map[string]*regexp.Regexp

func builtinRegexMatch(a, b ast.Value) error {
	s1, err := builtins.StringOperand(a, 1)
	if err != nil {
		return err
	}
	s2, err := builtins.StringOperand(b, 2)
	if err != nil {
		return err
	}
	re, err := getRegexp(string(s1))
	if err != nil {
		return err
	}
	if re.Match([]byte(s2)) {
		return nil
	}
	return BuiltinEmpty{}
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
	RegisterFunctionalBuiltinVoid2(ast.RegexMatch.Name, builtinRegexMatch)
}
