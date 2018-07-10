// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !linux,!darwin !cgo

package runtime

// Contains parts of the runtime package that do not use the plugin package
import (
	"context"

	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// NewRuntime returns a new Runtime object initialized with params.
func NewRuntime(ctx context.Context, params Params) (*Runtime, error) {

	if params.ID == "" {
		var err error
		params.ID, err = generateInstanceID()
		if err != nil {
			return nil, err
		}
	}

	loaded, err := loader.Filtered(params.Paths, params.Filter)
	if err != nil {
		return nil, err
	}

	store := inmem.New()

	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return nil, err
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.Path{}, loaded.Documents); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrapf(err, "storage error")
	}

	if err := compileAndStoreInputs(ctx, store, txn, loaded.Modules, params.ErrorLimit); err != nil {
		store.Abort(ctx, txn)
		return nil, errors.Wrapf(err, "compile error")
	}

	if err := store.Commit(ctx, txn); err != nil {
		return nil, errors.Wrapf(err, "storage error")
	}

	m, plugins, err := initPlugins(params.ID, store, params.ConfigFile)
	if err != nil {
		return nil, err
	}

	var decisionLogger func(context.Context, *server.Info)

	if p, ok := plugins["decision_logs"]; ok {
		decisionLogger = p.(*logs.Plugin).Log

		if params.DecisionIDFactory == nil {
			params.DecisionIDFactory = generateDecisionID
		}
	}

	rt := &Runtime{
		Store:          store,
		Manager:        m,
		Params:         params,
		decisionLogger: decisionLogger,
	}

	return rt, nil
}
