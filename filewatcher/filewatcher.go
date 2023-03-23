// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package filewatcher

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/ast"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
)

type OnReload func(context.Context, storage.Transaction, time.Duration, storage.Store, *initload.LoadPathsResult, error)

type FileWatcher struct {
	paths      []string
	filter     loader.Filter
	bundleMode bool
	store      storage.Store
	onReload   OnReload
	logger     logging.Logger
}

func NewFileWatcher(paths []string, filter loader.Filter, bundleMode bool, store storage.Store, onReload OnReload, logger logging.Logger) *FileWatcher {
	return &FileWatcher{
		paths:      paths,
		filter:     filter,
		bundleMode: bundleMode,
		store:      store,
		onReload:   onReload,
		logger:     logger,
	}
}

func (w *FileWatcher) Start(ctx context.Context) error {
	watcher, err := w.getWatcher(w.paths)
	if err != nil {
		return err
	}
	go w.readWatcher(ctx, watcher)
	return nil
}

func (w *FileWatcher) getWatcher(rootPaths []string) (*fsnotify.Watcher, error) {
	watchPaths, err := getWatchPaths(rootPaths)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, path := range watchPaths {
		w.logger.WithFields(map[string]interface{}{"path": path}).Debug("watching path")
		if err := watcher.Add(path); err != nil {
			return nil, err
		}
	}

	return watcher, nil
}

func (w *FileWatcher) readWatcher(ctx context.Context, watcher *fsnotify.Watcher) {
	for evt := range watcher.Events {
		removalMask := fsnotify.Remove | fsnotify.Rename
		mask := fsnotify.Create | fsnotify.Write | removalMask
		if (evt.Op & mask) != 0 {
			w.logger.WithFields(map[string]interface{}{
				"event": evt.String(),
			}).Debug("Registered file event.")
			removed := ""
			if (evt.Op & removalMask) != 0 {
				removed = evt.Name
			}
			w.processWatcherUpdate(ctx, w.paths, removed)
		}
	}
}

func (w *FileWatcher) processWatcherUpdate(ctx context.Context, paths []string, removed string) {
	t0 := time.Now()

	loaded, err := initload.LoadPaths(paths, w.filter, w.bundleMode, nil, true, false, nil)
	if err != nil {
		w.onReload(ctx, nil, time.Since(t0), w.store, nil, err)
		return
	}

	removed = loader.CleanPath(removed)

	err = storage.Txn(ctx, w.store, storage.WriteParams, func(txn storage.Transaction) error {
		if !w.bundleMode {
			ids, err := w.store.ListPolicies(ctx, txn)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if id == removed {
					if err := w.store.DeletePolicy(ctx, txn, id); err != nil {
						return err
					}
				} else if _, exists := loaded.Files.Modules[id]; !exists {
					// This branch get hit in two cases.
					// 1. Another piece of code has access to the store and inserts
					//    a policy out-of-band.
					// 2. In between FS notification and loader.Filtered() call above, a
					//    policy is removed from disk.
					bs, err := w.store.GetPolicy(ctx, txn, id)
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

		// It's up to onReload to update the store with loaded content
		w.onReload(ctx, txn, time.Since(t0), w.store, loaded, err)
		return nil
	})

	if err != nil {
		w.onReload(ctx, nil, time.Since(t0), w.store, nil, err)
	}
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
