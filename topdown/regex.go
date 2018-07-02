// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/gobwas/glob"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/yashtewari/glob-intersection"
	"regexp"
	"sync"
)

var regexpCacheLock = sync.Mutex{}
var regexpCache map[string]*regexp.Regexp

var globCacheLock = sync.Mutex{}
var globCache map[string]glob.Glob

func builtinRegexMatch(a, b ast.Value) (ast.Value, error) {
	s1, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	s2, err := builtins.StringOperand(b, 2)
	if err != nil {
		return nil, err
	}
	re, err := getRegexp(string(s1))
	if err != nil {
		return nil, err
	}
	return ast.Boolean(re.Match([]byte(s2))), nil
}

func getRegexp(pat string) (*regexp.Regexp, error) {
	regexpCacheLock.Lock()
	defer regexpCacheLock.Unlock()
	re, ok := regexpCache[pat]
	if !ok {
		var err error
		re, err = regexp.Compile(string(pat))
		if err != nil {
			return nil, err
		}
		regexpCache[pat] = re
	}
	return re, nil
}

func builtinGlobMatch(a, b ast.Value) (ast.Value, error) {
	pattern, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	input, err := builtins.StringOperand(b, 2)
	if err != nil {
		return nil, err
	}
	glb, err := getGlob(string(pattern))
	if err != nil {
		return nil, err
	}
	return ast.Boolean(glb.Match(string(input))), nil
}

func getGlob(pat string) (glob.Glob, error) {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	glb, ok := globCache[pat]
	if !ok {
		var err error
		glb, err = glob.Compile(pat)
		if err != nil {
			return nil, err
		}
		globCache[pat] = glb
	}
	return glb, nil
}

func builtinGlobIntersect(a, b ast.Value) (ast.Value, error) {
	s1, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	s2, err := builtins.StringOperand(b, 2)
	if err != nil {
		return nil, err
	}
	ne, err := gintersect.NonEmpty(string(s1), string(s2))
	if err != nil {
		return nil, err
	}
	return ast.Boolean(ne), nil
}

func init() {
	regexpCache = map[string]*regexp.Regexp{}
	globCache = map[string]glob.Glob{}
	// Backwards compatibility
	RegisterFunctionalBuiltin2(ast.RegexMatchDeprecated.Name, builtinRegexMatch)
	RegisterFunctionalBuiltin2(ast.GlobsMatchDeprecated.Name, builtinGlobIntersect)
	// New functions
	RegisterFunctionalBuiltin2(ast.RegexMatch.Name, builtinRegexMatch)
	RegisterFunctionalBuiltin2(ast.GlobMatch.Name, builtinGlobMatch)
	RegisterFunctionalBuiltin2(ast.GlobIntersect.Name, builtinGlobIntersect)
}
