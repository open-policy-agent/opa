// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package init is an internal package with helpers for data and policy loading during initialization.
package init

import (
	"context"

	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	storedversion "github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
)

// InsertAndCompileOptions contains the input for the operation.
type InsertAndCompileOptions struct {
	Store     storage.Store
	Txn       storage.Transaction
	Files     loader.Result
	Bundles   map[string]*bundle.Bundle
	MaxErrors int
}

// InsertAndCompileResult contains the output of the operation.
type InsertAndCompileResult struct {
	Compiler *ast.Compiler
	Metrics  metrics.Metrics
}

// InsertAndCompile writes data and policy into the store and returns a compiler for the
// store contents.
func InsertAndCompile(ctx context.Context, opts InsertAndCompileOptions) (*InsertAndCompileResult, error) {

	if len(opts.Files.Documents) > 0 {
		if err := opts.Store.Write(ctx, opts.Txn, storage.AddOp, storage.Path{}, opts.Files.Documents); err != nil {
			return nil, errors.Wrap(err, "storage error")
		}
	}

	policies := make(map[string]*ast.Module, len(opts.Files.Modules))

	for id, parsed := range opts.Files.Modules {
		policies[id] = parsed.Parsed
	}

	compiler := ast.NewCompiler().SetErrorLimit(opts.MaxErrors).WithPathConflictsCheck(storage.NonEmpty(ctx, opts.Store, opts.Txn))
	m := metrics.New()

	activation := &bundle.ActivateOpts{
		Ctx:          ctx,
		Store:        opts.Store,
		Txn:          opts.Txn,
		Compiler:     compiler,
		Metrics:      m,
		Bundles:      opts.Bundles,
		ExtraModules: policies,
	}

	err := bundle.Activate(activation)
	if err != nil {
		return nil, err
	}

	// Policies in bundles will have already been added to the store, but
	// modules loaded outside of bundles will need to be added manually.
	for id, parsed := range opts.Files.Modules {
		if err := opts.Store.UpsertPolicy(ctx, opts.Txn, id, parsed.Raw); err != nil {
			return nil, errors.Wrap(err, "storage error")
		}
	}

	// Set the version in the store last to prevent data files from overwriting.
	if err := storedversion.Write(ctx, opts.Store, opts.Txn); err != nil {
		return nil, errors.Wrap(err, "storage error")
	}

	return &InsertAndCompileResult{Compiler: compiler, Metrics: m}, nil
}

// LoadPathsResult contains the output loading a set of paths.
type LoadPathsResult struct {
	Bundles map[string]*bundle.Bundle
	Files   loader.Result
}

// LoadPaths reads data and policy from the given paths and returns a set of bundles or
// raw loader file results.
func LoadPaths(paths []string, filter loader.Filter, asBundle bool) (*LoadPathsResult, error) {

	var result LoadPathsResult
	var err error

	if asBundle {
		result.Bundles = make(map[string]*bundle.Bundle, len(paths))
		for _, path := range paths {
			result.Bundles[path], err = loader.NewFileLoader().AsBundle(path)
			if err != nil {
				return nil, err
			}
		}
		return &result, nil
	}

	files, err := loader.NewFileLoader().Filtered(paths, filter)
	if err != nil {
		return nil, err
	}

	result.Files = *files

	return &result, nil
}
