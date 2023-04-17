// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package refactor implements different refactoring operations over Rego modules.
package refactor

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// Error defines the structure of errors returned by refactor.
type Error struct {
	Message  string        `json:"message"`
	Location *ast.Location `json:"location,omitempty"`
}

func (e Error) Error() string {

	if e.Location != nil {
		return e.Location.Format(e.Message)
	}

	return e.Message
}

// Refactor implements different refactoring operations over Rego modules eg. renaming packages.
type Refactor struct {
}

// New returns a new Refactor object.
func New() *Refactor {
	return &Refactor{}
}

// MoveQuery holds the set of Rego modules whose package paths and other references are to be rewritten
// as per the mapping defined in SrcDstMapping.
// If validate is true, the moved modules will be compiled to ensure they are valid.
type MoveQuery struct {
	Modules       map[string]*ast.Module
	SrcDstMapping map[string]string
	validate      bool
}

// WithValidation controls whether to compile moved modules to ensure they are valid.
func (mq MoveQuery) WithValidation(v bool) MoveQuery {
	mq.validate = v
	return mq
}

// MoveQueryResult defines the output of a move query and holds the rewritten modules with updated packages paths
// and references.
type MoveQueryResult struct {
	Result map[string]*ast.Module `json:"result"`
}

// validate validates moved modules by compiling them.
func (mqr *MoveQueryResult) validate() error {
	compiler := ast.NewCompiler()
	compiler.Compile(mqr.Result)

	if compiler.Failed() {
		return compiler.Errors
	}
	return nil
}

// Move rewrites Rego code by updating package paths and other references in q's modules as per
// the mapping specified in q.
func (r *Refactor) Move(q MoveQuery) (*MoveQueryResult, error) {

	for _, module := range q.Modules {
		t := ast.NewGenericTransformer(func(x interface{}) (interface{}, error) {
			if s, ok := x.(ast.Ref); ok {
				for k, v := range q.SrcDstMapping {
					other, err := ast.ParseRef(k)
					if err != nil {
						return nil, err
					}

					if s.HasPrefix(other) {
						newRef, err := ast.ParseRef(v)
						if err != nil {
							return nil, err
						}
						return newRef.Concat(s[len(other):]), nil
					}

					// check if a reference in the policy is a prefix of a source reference
					// example: policy_reference = data.foo
					//          mapping: {"data.foo.bar": "data.baz"}
					// In this scenario, we can relocate data.foo.bar but everything under data.foo
					// (e.g., data.foo.baz, data.foo.qux, etc.) can't be relocated
					r := s.ConstantPrefix()
					if len(r) != 0 && other.HasPrefix(r) {
						msg := fmt.Sprintf("cannot rewrite `%v`: constant prefix `%v` of `%v` is too short", s, r, s)
						x := Error{Message: msg, Location: s[len(s)-1].Loc()}
						return nil, x
					}
				}
			}
			return x, nil
		})
		_, err := ast.Transform(t, module)
		if err != nil {
			switch err.(type) {
			case Error:
				return nil, err
			default:
				return nil, Error{Message: err.Error()}
			}
		}
	}

	result := &MoveQueryResult{Result: q.Modules}

	if q.validate {
		if err := result.validate(); err != nil {
			return nil, err
		}
	}
	return result, nil
}
