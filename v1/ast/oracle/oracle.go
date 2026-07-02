// Copyright 2025 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
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

	target := findTarget(stack)
	if target == nil {
		return nil, ErrNoDefinitionFound
	}

	location := findDefinition(target, stack, compiler, parsed)
	if location == nil {
		return nil, ErrNoDefinitionFound
	}

	return &DefinitionQueryResult{Result: location}, nil
}

func (o *Oracle) compileUpto(stage ast.StageID, modules map[string]*ast.Module, bs []byte, filename string) (*ast.Compiler, *ast.Module, error) {
	var compiler *ast.Compiler
	if o.compiler != nil {
		compiler = o.compiler
	} else {
		compiler = ast.NewCompiler()
	}

	if stage != "" {
		compiler = compiler.WithOnlyStagesUpTo(stage)
	}

	if modules == nil {
		modules = map[string]*ast.Module{}
	}

	var module *ast.Module
	var err error

	if len(bs) > 0 {
		module, err = ast.ParseModule(filename, util.ByteSliceToString(bs))
		if err != nil {
			return nil, nil, err
		}
		modules[filename] = module
	} else {
		module = modules[filename]
	}

	compiler.Compile(modules)
	if compiler.Failed() {
		return nil, nil, compiler.Errors
	}

	return compiler, module, nil
}
