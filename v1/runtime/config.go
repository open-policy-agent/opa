// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/internal/config"
	"github.com/open-policy-agent/opa/internal/pluginset"
	opa_config "github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/logs"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/util"
)

// Top-level keys OPA only reads at start-up. A reload that changes one is
// rejected rather than applied in part.
//
// None of these are inherently unreloadable; they are consumed once, while the
// runtime is being built, by something that has no way to be handed a new
// configuration afterwards. "storage" and "persistence_directory" pick the store
// the compiler and every plugin already hold a reference to; "server",
// "default_decision" and "default_authorization_decision" are baked into the
// server and its routes; "distributed_tracing" and "metrics_export" construct a
// tracer and a meter provider that are wired into the server and into every REST
// client at creation, and neither is torn back down; "discovery" would hand
// ownership of the plugin configuration to the discovery plugin, which is why
// the watcher does not even start when it is set.
var nonReloadableConfigKeys = []string{
	"default_authorization_decision",
	"default_decision",
	"discovery",
	"distributed_tracing",
	"metrics_export",
	"persistence_directory",
	"server",
	"storage",
}

// Configuration sections that enable a plugin, grouped where more than one
// drives the same plugin: "bundle" is the deprecated spelling of "bundles", and
// either keeps the bundle plugin running. The manager cannot unregister a
// plugin, so dropping a whole group would leave it running with its old settings
// while disappearing from the reported configuration.
//
// This catches a section going away outright, before anything is applied. A
// section that is still there but no longer enables its plugin -- an empty
// "decision_logs", say -- only shows up once the plugin configuration is parsed,
// and is caught by pluginset.Configs.Orphaned instead.
var pluginConfigKeys = []struct {
	plugin string
	keys   []string
}{
	{bundle.Name, []string{"bundle", "bundles"}},
	{logs.Name, []string{"decision_logs"}},
	{status.Name, []string{"status"}},
}

// How long to wait for the file to stop changing before reading it. Writers that
// truncate before writing leave it briefly empty, and reading that would apply
// an empty configuration.
const configCoalesceWindow = 200 * time.Millisecond

// startConfigWatcher applies configuration file changes to the running plugins.
// A no-op if no configuration file was given.
func (rt *Runtime) startConfigWatcher(ctx context.Context, onReload func(time.Duration, error)) error {
	if rt.Params.ConfigFile == "" {
		return nil
	}

	// Discovery owns the plugin configuration once enabled, so a reload here
	// would fight with it.
	if rt.Manager.GetConfig().Discovery != nil {
		rt.logger.Warn("Configuration file changes will not be reloaded because discovery is enabled.")
		return nil
	}

	watcher, err := rt.getConfigWatcher(rt.Params.ConfigFile)
	if err != nil {
		return err
	}

	rt.configWatcherStop = make(chan struct{})
	rt.configWatcherDone = make(chan struct{})

	go rt.readConfigWatcher(ctx, watcher, onReload)
	return nil
}

// stopConfigWatcher waits for a reload in flight, so no plugin is started after
// the manager has stopped.
func (rt *Runtime) stopConfigWatcher() {
	if rt.configWatcherStop == nil {
		return
	}

	close(rt.configWatcherStop)
	<-rt.configWatcherDone
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

// reloadConfig re-reads the configuration file and any --set overrides and
// applies them. Reports whether the file differed from the one previously read.
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

	// Diffed against the running configuration, not the last read, so reverting
	// a change that failed undoes whatever part of it took effect.
	if err := rt.applyConfig(ctx, bs); err != nil {
		return true, err
	}

	rt.appliedConfig = bs
	return true, nil
}

// applyConfig hands a new configuration to the plugin manager. Plugins it no
// longer enables keep running: as with discovery, plugins can be added and
// reconfigured but not removed, so a configuration that would drop one is
// rejected. Validating the plugin sections needs the new services registered
// first, so a failure there leaves the manager holding the new services, keys
// and caching.
func (rt *Runtime) applyConfig(ctx context.Context, bs []byte) error {
	oldConf, err := rawConfigMap(rt.appliedConfig)
	if err != nil {
		return err
	}
	newConf, err := rawConfigMap(bs)
	if err != nil {
		return err
	}

	if changed := changedKeys(oldConf, newConf, nonReloadableConfigKeys); len(changed) > 0 {
		return fmt.Errorf("changes to %s require a restart", strings.Join(changed, ", "))
	}
	if relabelled := changedLabels(oldConf, newConf); len(relabelled) > 0 {
		return fmt.Errorf("changing or removing labels (%s) requires a restart", strings.Join(relabelled, ", "))
	}
	if removed := removedPlugins(rt.pluginRunning, registeredPluginNames(), newConf); len(removed) > 0 {
		return fmt.Errorf("removing %s requires a restart", strings.Join(removed, ", "))
	}

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

	if dropped := droppedKeys(oldConf, newConf, "services", "keys"); len(dropped) > 0 {
		rt.logger.Warn("Entries removed from %s stay registered until OPA restarts.", strings.Join(dropped, " and "))
	}

	if err := rt.Manager.Reconfigure(parsed); err != nil {
		return err
	}

	registeredPluginsMux.Lock()
	factories := maps.Clone(registeredPlugins)
	registeredPluginsMux.Unlock()

	// Parsed before anything is registered, so that rejecting below leaves no
	// half-built plugin behind.
	configs, err := pluginset.Parse(factories, rt.Manager, parsed, rt.metrics, rt.logger, nil)
	if err != nil {
		return err
	}

	// A section that is still present but no longer enables its plugin; the
	// removedPlugins check above only sees one that is gone outright.
	if len(configs.Orphaned) > 0 {
		return fmt.Errorf("disabling %s requires a restart", strings.Join(configs.Orphaned, ", "))
	}

	return configs.Set(rt.Manager).Apply(ctx)
}

// rawConfigMap decodes a configuration as written, before defaults such as the
// "id" and "version" labels are injected.
func rawConfigMap(bs []byte) (map[string]any, error) {
	var conf map[string]any
	if err := util.Unmarshal(bs, &conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// changedKeys returns the keys whose value differs between two configurations.
func changedKeys(oldConf, newConf map[string]any, keys []string) []string {
	var changed []string
	for _, k := range keys {
		if !reflect.DeepEqual(oldConf[k], newConf[k]) {
			changed = append(changed, k)
		}
	}

	return changed
}

// droppedKeys returns the given keys that held entries the new configuration no
// longer lists. The manager merges these rather than replacing them, so the
// entries stay registered. Only the map form is inspected; "services" written as
// an array is left alone.
func droppedKeys(oldConf, newConf map[string]any, keys ...string) []string {
	var dropped []string
	for _, k := range keys {
		oldEntries, _ := oldConf[k].(map[string]any)
		newEntries, _ := newConf[k].(map[string]any)
		for name := range oldEntries {
			if _, ok := newEntries[name]; !ok {
				dropped = append(dropped, k)
				break
			}
		}
	}

	return dropped
}

// pluginRunning reports whether a plugin is registered under the given name.
func (rt *Runtime) pluginRunning(name string) bool {
	return rt.Manager.Plugin(name) != nil
}

// changedLabels returns the labels the new configuration changes or drops.
// Additions are left out because those do take effect; the manager restores the
// labels captured at start-up over anything else, which would be silent.
func changedLabels(oldConf, newConf map[string]any) []string {
	oldLabels, _ := oldConf["labels"].(map[string]any)
	newLabels, _ := newConf["labels"].(map[string]any)

	var changed []string
	for k, v := range oldLabels {
		if nv, ok := newLabels[k]; !ok || !reflect.DeepEqual(nv, v) {
			changed = append(changed, k)
		}
	}
	slices.Sort(changed)

	return changed
}

// removedPlugins returns the configuration sections of running plugins that the
// new configuration drops outright. running reports whether a plugin of that
// name is registered, so a section that never enabled anything is not mistaken
// for a removal; custom names the plugins RegisterPlugin knows about.
//
// A section that is still present, however empty, is left alone here: the bundle
// plugin reads an empty "bundles" as "no bundles" and tears its downloaders
// down, and for the others pluginset reports the plugin as orphaned once the
// section has been parsed.
func removedPlugins(running func(name string) bool, custom []string, newConf map[string]any) []string {
	var removed []string
	for _, group := range pluginConfigKeys {
		if !running(group.plugin) {
			continue
		}
		if !slices.ContainsFunc(group.keys, func(k string) bool { _, ok := newConf[k]; return ok }) {
			removed = append(removed, group.keys[len(group.keys)-1])
		}
	}

	newCustom, _ := newConf["plugins"].(map[string]any)
	for _, name := range custom {
		if _, ok := newCustom[name]; !ok && running(name) {
			removed = append(removed, "plugins."+name)
		}
	}
	slices.Sort(removed)

	return removed
}

// registeredPluginNames returns the names custom plugins have been registered
// under with RegisterPlugin.
func registeredPluginNames() []string {
	registeredPluginsMux.Lock()
	defer registeredPluginsMux.Unlock()

	return slices.Collect(maps.Keys(registeredPlugins))
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
	}).Info("Reloaded configuration.")
}
