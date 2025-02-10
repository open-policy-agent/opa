// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// Error defines the structure of errors returned by the oracle.
type Error struct {
	Code string `json:"code"`
}

func (e Error) Error() string {
	return e.Code
}

// Oracle implements different queries over ASTs, e.g., find definition.
type Oracle struct {
	compiler *ast.Compiler
}

// New returns a new Oracle object.
func New() *Oracle {
	return &Oracle{}
}

// DefinitionQuery defines a Rego definition query.
type DefinitionQuery struct {
	Modules  map[string]*ast.Module // workspace modules; buffer may shadow a file inside the workspace
	Filename string                 // name of file to search for position inside of
	Buffer   []byte                 // buffer that overrides module with filename
	Pos      int                    // position to search for
}

var (
	// ErrNoDefinitionFound indicates the position was valid but no matching definition was found.
	ErrNoDefinitionFound = Error{Code: "oracle_no_definition_found"}

	// ErrNoMatchFound indicates the position was invalid.
	ErrNoMatchFound = Error{Code: "oracle_no_match_found"}
)

// DefinitionQueryResult defines output of a definition query.
type DefinitionQueryResult struct {
	Result *ast.Location `json:"result"`
}

// WithCompiler sets the compiler to use for the oracle. If not set, a new ast.Compiler
// will be created when needed.
func (o *Oracle) WithCompiler(compiler *ast.Compiler) *Oracle {
	o.compiler = compiler

	return o
}

// FindDefinition returns the location of the definition referred to by the symbol
// at the position in q.
func (o *Oracle) FindDefinition(q DefinitionQuery) (*DefinitionQueryResult, error) {

	// TODO(tsandall): how can we cache the results of compilation and parsing so that
	// multiple queries can be executed without having to re-compute the same values?
	// Ditto for caching across runs. Avoid repeating the same work.

	// NOTE(sr): "SetRuleTree" because it's needed for compiler.GetRulesExact() below
	compiler, parsed, err := o.compileUpto("SetRuleTree", q.Modules, q.Buffer, q.Filename)
	if err != nil {
		return nil, err
	}
	mod, ok := compiler.Modules[q.Filename]
	if !ok {
		return nil, ErrNoMatchFound
	}
	stack := findContainingNodeStack(mod, q.Pos)
	if len(stack) == 0 {
		return nil, ErrNoMatchFound
	}

	// Walk outwards from the match location, attempting to find the definition via
	// references to imports or other rules. This handles intra-module, intra-package,
	// and inter-package references.
	for i := len(stack) - 1; i >= 0; i-- {
		if term, ok := stack[i].(*ast.Term); ok {
			if ref, ok := term.Value.(ast.Ref); ok {
				prefix := ref.ConstantPrefix()
				if rules := compiler.GetRulesExact(prefix); len(rules) > 0 {
					return &DefinitionQueryResult{rules[0].Location}, nil
				}
				for _, imp := range parsed.Imports {
					if path, ok := imp.Path.Value.(ast.Ref); ok {
						if prefix.HasPrefix(path) {
							return &DefinitionQueryResult{imp.Path.Location}, nil
						}
					}
				}
			}
		}
	}

	// If the match is a variable, walk inward to find the first occurrence of the variable
	// in function arguments or the body.
	top := stack[len(stack)-1]
	if term, ok := top.(*ast.Term); ok {
		if name, ok := term.Value.(ast.Var); ok {
			for i := range stack {
				switch node := stack[i].(type) {
				case *ast.Rule:
					if match := walkToFirstOccurrence(node.Head.Args, name); match != nil {
						return &DefinitionQueryResult{match.Location}, nil
					}
				case ast.Body:
					if match := walkToFirstOccurrence(node, name); match != nil {
						return &DefinitionQueryResult{match.Location}, nil
					}
				}
			}
		}
	}

	return nil, ErrNoDefinitionFound
}

func (o *Oracle) compileUpto(stage string, modules map[string]*ast.Module, bs []byte, filename string) (*ast.Compiler, *ast.Module, error) {
	var compiler *ast.Compiler
	if o.compiler != nil {
		compiler = o.compiler
	} else {
		compiler = ast.NewCompiler()
	}

	compiler = compiler.WithStageAfter(stage, ast.CompilerStageDefinition{
		Name: "halt",
		Stage: func(c *ast.Compiler) *ast.Error {
			return &ast.Error{
				Code: "halt",
			}
		},
	})

	var module *ast.Module

	if len(bs) > 0 {
		var err error
		module, err = ast.ParseModule(filename, util.ByteSliceToString(bs))
		if err != nil {
			return nil, nil, err
		}
	} else {
		module = modules[filename]
	}

	if modules == nil {
		modules = map[string]*ast.Module{}
	}

	if len(bs) > 0 {
		modules[filename] = module
	}

	compiler.Compile(modules)

	if stage != "" {
		if err := halted(compiler); err != nil {
			return nil, nil, err
		}
	}

	return compiler, module, nil
}

func halted(c *ast.Compiler) error {
	if c.Failed() && len(c.Errors) == 1 && c.Errors[0].Code == "halt" {
		return nil
	} else if len(c.Errors) > 0 {
		return c.Errors
	}
	// NOTE(tsandall): this indicate an internal error in the compiler and should
	// not be reachable.
	return errors.New("unreachable: did not halt")
}

func walkToFirstOccurrence(node ast.Node, needle ast.Var) (match *ast.Term) {
	ast.WalkNodes(node, func(x ast.Node) bool {
		if match == nil {
			switch x := x.(type) {
			case *ast.SomeDecl:
				// NOTE(tsandall): The visitor doesn't traverse into some decl terms
				// so special case here.
				for i := range x.Symbols {
					if x.Symbols[i].Value.Compare(needle) == 0 {
						match = x.Symbols[i]
						break
					}
				}
			case *ast.Term:
				if x.Value.Compare(needle) == 0 {
					match = x
				}
			}
		}
		return match != nil
	})
	return match
}

func findContainingNodeStack(module *ast.Module, pos int) []ast.Node {

	var matches []ast.Node

	ast.WalkNodes(module, func(x ast.Node) bool {

		minLoc, maxLoc := getLocMinMax(x)

		if pos < minLoc || pos >= maxLoc {
			return true
		}

		matches = append(matches, x)
		return false
	})

	return matches
}

func getLocMinMax(x ast.Node) (int, int) {

	if x.Loc() == nil {
		return -1, -1
	}

	loc := x.Loc()
	minOff := loc.Offset

	// Special case bodies because location text is only for the first expr.
	if body, ok := x.(ast.Body); ok {
		last := findLastExpr(body)
		extraLoc := last.Loc()
		if extraLoc == nil {
			return -1, -1
		}
		return minOff, extraLoc.Offset + len(extraLoc.Text)
	}

	return minOff, minOff + len(loc.Text)
}

// findLastExpr returns the last expression in an ast.Body that has not been generated
// by the compiler. It's used to cope with the fact that a compiler stage before SetRuleTree
// has rewritten the rule bodies slightly. By ignoring appended generated body expressions,
// we can still use the "circling in on the variable" logic based on node locations.
func findLastExpr(body ast.Body) *ast.Expr {
	for i := len(body) - 1; i >= 0; i-- {
		if !body[i].Generated {
			return body[i]
		}
	}
	// NOTE(sr): I believe this shouldn't happen -- we only ever start circling in on a node
	// inside a body if there's something in that body. A body that only consists of generated
	// expressions should not appear here. Either way, the caller deals with `nil` returned by
	// this helper.
	return nil
}
