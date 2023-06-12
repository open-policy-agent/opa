// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package pathwatcher provides helper functions for creating file and directory watchers
package pathwatcher

import (
	"context"

	"github.com/fsnotify/fsnotify"
	"github.com/open-policy-agent/opa/ast"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/storage"
)

// CreatePathWatcher creates watchers to monitor for path changes
func CreatePathWatcher(rootPaths []string) (*fsnotify.Watcher, error) {
	watchPaths, err := getWatchPaths(rootPaths)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, path := range watchPaths {
		if err := watcher.Add(path); err != nil {
			return nil, err
		}
	}

	return watcher, nil
}

// ProcessWatcherUpdate handles an occurrence of a watcher event
func ProcessWatcherUpdate(ctx context.Context, paths []string, removed string, store storage.Store, filter loader.Filter, asBundle bool,
	f func(context.Context, storage.Transaction, *initload.LoadPathsResult) error) error {
	loaded, err := initload.LoadPaths(paths, filter, asBundle, nil, true, false, nil, nil)
	if err != nil {
		return err
	}

	removed = loader.CleanPath(removed)

	return storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		if !asBundle {
			ids, err := store.ListPolicies(ctx, txn)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if id == removed {
					if err := store.DeletePolicy(ctx, txn, id); err != nil {
						return err
					}
				} else if _, exists := loaded.Files.Modules[id]; !exists {
					// This branch get hit in two cases.
					// 1. Another piece of code has access to the store and inserts
					//    a policy out-of-band.
					// 2. In between FS notification and loader.Filtered() call above, a
					//    policy is removed from disk.
					bs, err := store.GetPolicy(ctx, txn, id)
					if err != nil {
						return err
					}
					module, err := ast.ParseModule(id, string(bs))
					if err != nil {
						return err
					}
					loaded.Files.Modules[id] = &loader.RegoFile{
						Name:   id,
						Raw:    bs,
						Parsed: module,
					}
				}
			}
		}

		return f(ctx, txn, loaded)
	})
}

func getWatchPaths(rootPaths []string) ([]string, error) {
	paths := []string{}

	for _, path := range rootPaths {

		_, path = loader.SplitPrefix(path)
		result, err := loader.Paths(path, true)
		if err != nil {
			return nil, err
		}

		paths = append(paths, loader.Dirs(result)...)
	}

	return paths, nil
}
