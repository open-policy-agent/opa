// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package manifest implements helper functions for the stored manifest.
package manifest

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

var bundlePath = storage.MustParsePath("/system/bundle")
var manifestPath = storage.MustParsePath("/system/bundle/manifest")
var revisionPath = storage.MustParsePath("/system/bundle/manifest/revision")
var rootsPath = storage.MustParsePath("/system/bundle/manifest/roots")

// Write the manifest into the storage. This function is called when
// the bundle is activated.
func Write(ctx context.Context, store storage.Store, txn storage.Transaction, m bundle.Manifest) error {

	var value interface{} = m

	if err := util.RoundTrip(&value); err != nil {
		return err
	}

	if err := storage.MakeDir(ctx, store, txn, bundlePath); err != nil {
		return err
	}

	return store.Write(ctx, txn, storage.AddOp, manifestPath, value)
}

// ReadBundleRoots returns the roots specified in the currently
// activated bundle. If there is no activated bundle, this function
// will return storage NotFound error.
func ReadBundleRoots(ctx context.Context, store storage.Store, txn storage.Transaction) ([]string, error) {

	value, err := store.Read(ctx, txn, rootsPath)
	if err != nil {
		return nil, err
	}

	sl, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("corrupt manifest roots")
	}

	roots := make([]string, len(sl))

	for i := range sl {
		roots[i], ok = sl[i].(string)
		if !ok {
			return nil, fmt.Errorf("corrupt manifest root")
		}
	}

	return roots, nil
}

// ReadBundleRevision returns the revision in the currently activated
// bundle. If there is no activated bundle, ths function will return
// storage NotFound error.
func ReadBundleRevision(ctx context.Context, store storage.Store, txn storage.Transaction) (string, error) {

	value, err := store.Read(ctx, txn, revisionPath)
	if err != nil {
		return "", err
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("corrupt manifest revision")
	}

	return str, nil
}
