// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/internal/config"
	"github.com/open-policy-agent/opa/internal/pluginset"
	opa_config "github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/hooks"
)

// How long to wait for the file to stop changing before reading it. Writers that
// truncate before writing leave it briefly empty, and reading that would apply
// an empty configuration.
const configCoalesceWindow = 200 * time.Millisecond

// startConfigWatcher asks the serve loop to restart when the configuration file
// changes. A no-op if no configuration file was given.
func (rt *Runtime) startConfigWatcher(ctx context.Context, onReload func(time.Duration, error)) error {
	if rt.Params.ConfigFile == "" {
		return nil
	}

	// Discovery owns the plugin configuration once enabled, so a reload here
	// would fight with it.
	if rt.discoveryEnabled() {
		rt.logger.Warn("Configuration file changes will not be reloaded because discovery is enabled.")
		return nil
	}

	// A restart registers OPA's routes afresh, which needs a router OPA owns:
	// registering the same pattern twice on a ServeMux panics.
	if !rt.ownRouter {
		rt.logger.Warn("Configuration file changes will not be reloaded because a router was supplied by the caller.")
		return nil
	}

	watcher, err := rt.getConfigWatcher(rt.Params.ConfigFile)
	if err != nil {
		return err
	}

	rt.configWatcherStop = make(chan struct{})
	rt.configWatcherDone = make(chan struct{})

	go rt.readConfigWatcher(ctx, watcher, onReload)

	// The watch only covers what happens from here on, and the server has been
	// accepting traffic since before it was added. Reconcile once so a change
	// written in that gap is not lost until the next one.
	t0 := time.Now()
	if changed, err := rt.reloadConfig(ctx); changed || err != nil {
		onReload(time.Since(t0), err)
	}

	return nil
}

// stopConfigWatcher waits for a reload in flight, so no restart is requested
// after the serve loop has stopped listening for one. Idempotent: a reload that
// turns discovery on stops the watcher before the deferred stop in Serve runs.
func (rt *Runtime) stopConfigWatcher() {
	if rt.configWatcherStop == nil {
		return
	}

	close(rt.configWatcherStop)
	<-rt.configWatcherDone
	rt.configWatcherStop = nil
}

// discoveryEnabled reports whether the configuration in effect hands the plugin
// configuration to the discovery plugin.
func (rt *Runtime) discoveryEnabled() bool {
	return rt.manager().GetConfig().Discovery != nil
}

// getConfigWatcher watches the directory holding path, not the file: editors and
// ConfigMap volumes replace the file by rename, which drops a watch on it.
func (rt *Runtime) getConfigWatcher(path string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	rt.logger.WithFields(map[string]any{"path": dir}).Debug("Watching directory of configuration file.")
	return watcher, nil
}

// isConfigFileEvent reports whether an event concerns the configuration file. A
// ConfigMap volume update only swaps the "..data" symlink, so that rename is the
// only signal there.
func (rt *Runtime) isConfigFileEvent(name string) bool {
	base := filepath.Base(name)
	return base == filepath.Base(rt.Params.ConfigFile) || base == "..data"
}

func (rt *Runtime) readConfigWatcher(ctx context.Context, watcher *fsnotify.Watcher, onReload func(time.Duration, error)) {
	defer close(rt.configWatcherDone)
	defer watcher.Close()

	rt.watchConfigEvents(ctx, watcher.Events, watcher.Errors, onReload)
}

// watchConfigEvents is split from the watcher's lifecycle so tests can drive it:
// sending into a live watcher's channel races with the goroutine owning it.
func (rt *Runtime) watchConfigEvents(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error, onReload func(time.Duration, error)) {
	mask := fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename

	// Armed only while a change settles.
	settle := time.NewTimer(configCoalesceWindow)
	settle.Stop()
	defer settle.Stop()

	for {
		select {
		case evt := <-events:
			if (evt.Op&mask) == 0 || !rt.isConfigFileEvent(evt.Name) {
				continue
			}

			rt.logger.WithFields(map[string]any{
				"event": evt.String(),
			}).Debug("Registered configuration file event.")

			settle.Reset(configCoalesceWindow)
		case <-settle.C:
			// Re-checked on each pass, not just at start-up: a reload can turn
			// discovery on, and from then on the discovered configuration is what
			// the plugins follow.
			if rt.discoveryEnabled() {
				rt.logger.Warn("Further configuration file changes will not be reloaded because discovery is now enabled.")
				return
			}

			t0 := time.Now()
			changed, err := rt.reloadConfig(ctx)
			if changed || err != nil {
				onReload(time.Since(t0), err)
			}
		case err := <-errs:
			rt.logger.WithFields(map[string]any{"err": err}).Error("Configuration file watcher error.")
		case <-rt.configWatcherStop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// reloadConfig re-reads the configuration file and any --set overrides, and asks
// the serve loop to restart under them. Reports whether the file differed from
// the one previously read.
//
// The configuration is validated here, before anything is torn down, so that a
// file OPA cannot run with leaves the current one serving.
func (rt *Runtime) reloadConfig(ctx context.Context) (bool, error) {
	bs, err := config.Load(rt.Params.ConfigFile, rt.Params.ConfigOverrides, rt.Params.ConfigOverrideFiles)
	if err != nil {
		return false, err
	}

	rt.configMtx.Lock()
	defer rt.configMtx.Unlock()

	// Nothing new, including a change already read and rejected: report once.
	if bytes.Equal(bs, rt.lastConfig) {
		return false, nil
	}
	rt.lastConfig = bs

	if err := rt.validateConfig(ctx, bs); err != nil {
		return true, err
	}

	rt.previousConfig = rt.appliedConfig
	rt.appliedConfig = bs
	rt.requestRestart()

	return true, nil
}

// validateConfig checks that a configuration is one OPA could start under,
// without touching anything that is running. It covers what can be known
// statically: the core schema, the hooks, and every plugin's own configuration.
// What cannot -- a port that will not bind, a directory that cannot be written
// -- surfaces when the restart runs.
func (rt *Runtime) validateConfig(ctx context.Context, bs []byte) error {
	parsed, err := opa_config.ParseConfig(bs, rt.Params.ID)
	if err != nil {
		return err
	}

	rt.Params.Hooks.Each(func(h hooks.Hook) {
		if f, ok := h.(hooks.ConfigHook); ok {
			if c, e := f.OnConfig(ctx, parsed); e != nil {
				err = errors.Join(err, e)
			} else {
				parsed = c
			}
		}
	})
	if err != nil {
		return err
	}

	for _, w := range parsed.Warnings {
		rt.logger.Warn("%s", w)
	}

	registeredPluginsMux.Lock()
	factories := maps.Clone(registeredPlugins)
	registeredPluginsMux.Unlock()

	// Parses and validates every plugin section without touching the manager.
	_, err = pluginset.Parse(factories, rt.manager(), parsed, rt.metricsProvider(), rt.logger, nil)
	return err
}

// requestRestart asks the serve loop to come back up under rt.appliedConfig.
// Non-blocking: a request already pending will pick up this configuration too,
// since the loop reads the file's latest contents when it restarts.
func (rt *Runtime) requestRestart() {
	select {
	case rt.restartc <- struct{}{}:
	default:
	}
}

func (rt *Runtime) onConfigReloadLogger(d time.Duration, err error) {
	if err != nil {
		rt.logger.WithFields(map[string]any{
			"duration": d,
			"err":      err,
		}).Error("Failed to reload configuration.")
		return
	}

	rt.logger.WithFields(map[string]any{
		"duration": d,
	}).Info("Reloading configuration.")
}
