// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	testLog "github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/logs"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	sdktest "github.com/open-policy-agent/opa/v1/sdk/test"
	"github.com/open-policy-agent/opa/v1/storage"
)

func newConfigReloadRuntime(t *testing.T, config string) (*Runtime, string) {
	t.Helper()

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configFile, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = testLog.New()

	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if err := rt.Manager.Start(ctx); err != nil {
		t.Fatalf("start plugins: %v", err)
	}
	t.Cleanup(func() { rt.Manager.Stop(ctx) })

	return rt, configFile
}

func writeConfig(t *testing.T, path, config string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestReloadConfigNoChange(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	// Same content: not a change.
	writeConfig(t, configFile, `labels:
  region: west
`)

	changed, err := rt.reloadConfig(t.Context())
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if changed {
		t.Error("expected no change to be reported")
	}
}

func TestReloadConfigStartsAndReconfiguresPlugins(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	if p := logs.Lookup(rt.Manager); p != nil {
		t.Fatal("expected no decision log plugin before reload")
	}

	writeConfig(t, configFile, `labels:
  region: west
  team: infra
decision_logs:
  console: true
`)

	changed, err := rt.reloadConfig(t.Context())
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !changed {
		t.Fatal("expected change to be reported")
	}

	if p := logs.Lookup(rt.Manager); p == nil {
		t.Error("expected decision log plugin to be started")
	}
	if labels := rt.Manager.GetConfig().Labels; labels["team"] != "infra" {
		t.Errorf("expected label team=infra, got %v", labels)
	}

	// Already running: reconfigured, not started again.
	writeConfig(t, configFile, `labels:
  region: west
  team: infra
decision_logs:
  console: true
  mask_decision: /system/log/mask
`)

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	p := logs.Lookup(rt.Manager)
	if p == nil {
		t.Fatal("expected decision log plugin to still be registered")
	}
	if got := p.Config().MaskDecision; got == nil || *got != "/system/log/mask" {
		t.Errorf("expected mask decision to be reconfigured, got %v", got)
	}
}

func TestReloadConfigRejectsNonReloadableChanges(t *testing.T) {
	for _, tc := range []struct {
		note   string
		config string
		key    string
	}{
		{
			note: "server",
			config: `labels:
  region: west
server:
  decoding:
    max_length: 42
`,
			key: "server",
		},
		{
			note: "storage",
			config: `labels:
  region: west
storage:
  disk:
    directory: /tmp/opa
`,
			key: "storage",
		},
		{
			note: "persistence_directory",
			config: `labels:
  region: west
persistence_directory: /var/opa
`,
			key: "persistence_directory",
		},
		{
			note: "default_decision",
			config: `labels:
  region: west
default_decision: /example/allow
`,
			key: "default_decision",
		},
		{
			note: "several at once",
			config: `labels:
  region: west
server:
  decoding:
    max_length: 42
storage:
  disk:
    directory: /tmp/opa
`,
			key: "server, storage",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

			writeConfig(t, configFile, tc.config)

			changed, err := rt.reloadConfig(t.Context())
			if err == nil {
				t.Fatal("expected error")
			}
			if !changed {
				t.Error("expected the on-disk change to be reported")
			}
			if exp := fmt.Sprintf("changes to %s require a restart", tc.key); err.Error() != exp {
				t.Errorf("expected error %q, got %q", exp, err.Error())
			}

			// Reported once, not on every event that follows.
			changed, err = rt.reloadConfig(t.Context())
			if err != nil {
				t.Errorf("expected the rejected config to be reported only once, got %v", err)
			}
			if changed {
				t.Error("expected no further change to be reported")
			}

			// Reverting is a change again, so it is accepted.
			writeConfig(t, configFile, `labels:
  region: west
`)
			if _, err := rt.reloadConfig(t.Context()); err != nil {
				t.Errorf("expected the revert to be accepted, got %v", err)
			}
		})
	}
}

func TestReloadConfigRevertAfterFailedApply(t *testing.T) {
	const good = `labels:
  region: west
`

	rt, configFile := newConfigReloadRuntime(t, good)

	if rt.Manager.GetConfig().NDBuiltinCacheEnabled() {
		t.Fatal("expected the ND builtin cache to start out disabled")
	}

	// nd_builtin_cache applies before status is validated, and the service named
	// here doesn't exist.
	writeConfig(t, configFile, `labels:
  region: west
nd_builtin_cache: true
status:
  service: nonexistent
`)

	if _, err := rt.reloadConfig(t.Context()); err == nil {
		t.Fatal("expected error")
	}
	if !rt.Manager.GetConfig().NDBuiltinCacheEnabled() {
		t.Error("expected the ND builtin cache to have been applied before the failure")
	}

	writeConfig(t, configFile, good)

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if rt.Manager.GetConfig().NDBuiltinCacheEnabled() {
		t.Error("expected the revert to undo the partially applied configuration")
	}
}

func TestReloadConfigLabels(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	writeConfig(t, configFile, `labels:
  region: west
  team: infra
`)
	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("expected an added label to be accepted, got %v", err)
	}
	if got := rt.Manager.GetConfig().Labels["team"]; got != "infra" {
		t.Errorf("expected label team=infra, got %q", got)
	}

	writeConfig(t, configFile, `labels:
  region: east
  team: infra
`)
	_, err := rt.reloadConfig(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if exp := "changing or removing labels (region) requires a restart"; err.Error() != exp {
		t.Errorf("expected error %q, got %q", exp, err.Error())
	}
	if got := rt.Manager.GetConfig().Labels["region"]; got != "west" {
		t.Errorf("expected label region to stay west, got %q", got)
	}
}

func TestReloadConfigRejectsPluginRemoval(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `decision_logs:
  console: true
`)

	if logs.Lookup(rt.Manager) == nil {
		t.Fatal("expected decision log plugin at boot")
	}

	writeConfig(t, configFile, `labels:
  region: west
`)

	_, err := rt.reloadConfig(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if exp := "removing decision_logs requires a restart"; err.Error() != exp {
		t.Errorf("expected error %q, got %q", exp, err.Error())
	}

	// The plugin keeps running either way, so the reported configuration has to
	// keep saying so.
	if logs.Lookup(rt.Manager) == nil {
		t.Error("expected decision log plugin to still be registered")
	}
	if rt.Manager.GetConfig().DecisionLogs == nil {
		t.Error("expected decision_logs to remain in the reported configuration")
	}
}

func TestReloadConfigRejectsCustomPluginRemoval(t *testing.T) {
	RegisterPlugin("reload_test", Factory{})

	rt, configFile := newConfigReloadRuntime(t, `plugins:
  reload_test: {}
`)

	if rt.Manager.Plugin("reload_test") == nil {
		t.Fatal("expected custom plugin at boot")
	}

	writeConfig(t, configFile, `labels:
  region: west
`)

	_, err := rt.reloadConfig(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if exp := "removing plugins.reload_test requires a restart"; err.Error() != exp {
		t.Errorf("expected error %q, got %q", exp, err.Error())
	}
}

// A section that is still there but no longer enables its plugin is the same
// removal as dropping it, and has to be rejected the same way.
func TestReloadConfigRejectsDisablingPlugins(t *testing.T) {
	for _, tc := range []struct {
		note    string
		start   string
		next    string
		message string
	}{
		{
			note:    "decision_logs emptied",
			start:   "decision_logs:\n  console: true\n",
			next:    "decision_logs: {}\n",
			message: "disabling decision_logs requires a restart",
		},
		{
			note:    "decision_logs nulled",
			start:   "decision_logs:\n  console: true\n",
			next:    "decision_logs:\n",
			message: "disabling decision_logs requires a restart",
		},
		{
			note:    "decision_logs console turned off",
			start:   "decision_logs:\n  console: true\n",
			next:    "decision_logs:\n  console: false\n",
			message: "disabling decision_logs requires a restart",
		},
		{
			note:    "status emptied",
			start:   "status:\n  console: true\n",
			next:    "status: {}\n",
			message: "disabling status requires a restart",
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			rt, configFile := newConfigReloadRuntime(t, tc.start)

			writeConfig(t, configFile, tc.next)

			_, err := rt.reloadConfig(t.Context())
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tc.message {
				t.Errorf("expected error %q, got %q", tc.message, err.Error())
			}
			if logs.Lookup(rt.Manager) == nil && status.Lookup(rt.Manager) == nil {
				t.Error("expected the plugin to still be registered")
			}
		})
	}
}

// A rejected reload must not leave a plugin registered but never started: the
// next reload would reconfigure it, and a plugin that isn't running never reads
// from its reconfigure channel.
func TestReloadConfigRejectedReloadStartsNothing(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `decision_logs:
  console: true
`)

	// Adds status, and disables decision_logs in the same write.
	writeConfig(t, configFile, `decision_logs: {}
status:
  console: true
`)

	if _, err := rt.reloadConfig(t.Context()); err == nil {
		t.Fatal("expected error")
	}
	if status.Lookup(rt.Manager) != nil {
		t.Fatal("expected the status plugin not to have been registered by a rejected reload")
	}

	// Reverting leaves both plugins usable.
	writeConfig(t, configFile, `decision_logs:
  console: true
status:
  console: true
`)

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if status.Lookup(rt.Manager) == nil {
		t.Error("expected the status plugin to be started")
	}
}

// An empty section still enables the plugin where it names the only configured
// service, exactly as it would at start-up.
func TestReloadConfigEmptyDecisionLogsPicksUpTheOnlyService(t *testing.T) {
	server := sdktest.MustNewServer()
	defer server.Stop()

	rt, configFile := newConfigReloadRuntime(t, fmt.Sprintf(`services:
  acme:
    url: %q
decision_logs:
  console: true
`, server.URL()))

	writeConfig(t, configFile, fmt.Sprintf(`services:
  acme:
    url: %q
decision_logs: {}
`, server.URL()))

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	p := logs.Lookup(rt.Manager)
	if p == nil {
		t.Fatal("expected decision log plugin to still be registered")
	}
	if got := p.Config().Service; got != "acme" {
		t.Errorf("expected the plugin to default to service acme, got %q", got)
	}
}

// The bundle plugin takes an empty "bundles" as "no bundles", so unlike the
// other plugins it can be emptied without a restart.
func TestReloadConfigEmptyBundlesDropsBundles(t *testing.T) {
	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/b.tar.gz", map[string]string{
			"data.json": `{"reload": {"which": "b"}}`,
		}),
	)
	defer server.Stop()

	rt, configFile := newConfigReloadRuntime(t, fmt.Sprintf(`services:
  acme:
    url: %q
bundles:
  b:
    resource: /bundles/b.tar.gz
`, server.URL()))

	writeConfig(t, configFile, fmt.Sprintf(`services:
  acme:
    url: %q
bundles: {}
`, server.URL()))

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	p := bundle.Lookup(rt.Manager)
	if p == nil {
		t.Fatal("expected bundle plugin to still be registered")
	}
	if got := len(p.Config().Bundles); got != 0 {
		t.Errorf("expected no bundles to be left configured, got %d", got)
	}
}

func TestReloadConfigAppliesBundleChanges(t *testing.T) {
	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/first.tar.gz", map[string]string{
			"data.json": `{"reload": {"which": "first"}}`,
		}),
		sdktest.MockBundle("/bundles/second.tar.gz", map[string]string{
			"data.json": `{"reload": {"which": "second"}}`,
		}),
	)
	defer server.Stop()

	config := func(resource string) string {
		return fmt.Sprintf(`services:
  acme:
    url: %q
bundles:
  b:
    resource: %s
`, server.URL(), resource)
	}

	rt, configFile := newConfigReloadRuntime(t, config("/bundles/first.tar.gz"))

	waitForBundle(t, rt, "first")

	writeConfig(t, configFile, config("/bundles/second.tar.gz"))

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if got := bundle.Lookup(rt.Manager).Config().Bundles["b"].Resource; got != "/bundles/second.tar.gz" {
		t.Fatalf("expected the bundle plugin to be reconfigured, got resource %q", got)
	}

	waitForBundle(t, rt, "second")
}

// waitForBundle blocks until data.reload.which has the expected value, which is
// how far the bundle has got through downloading and activating.
func waitForBundle(t *testing.T, rt *Runtime, exp string) {
	t.Helper()

	ctx := t.Context()
	deadline := time.Now().Add(10 * time.Second)

	for {
		value, err := storage.ReadOne(ctx, rt.Store, storage.MustParsePath("/reload/which"))
		if err == nil && value == exp {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for data.reload.which to become %q (last: %v, err: %v)", exp, value, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReloadConfigWarnsOnDroppedService(t *testing.T) {
	server := sdktest.MustNewServer()
	defer server.Stop()

	logger := testLog.New()
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, fmt.Sprintf(`services:
  acme:
    url: %q
  other:
    url: %q
`, server.URL(), server.URL()))

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = logger

	rt, err := NewRuntime(t.Context(), params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	writeConfig(t, configFile, fmt.Sprintf(`services:
  acme:
    url: %q
`, server.URL()))

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	found := slices.ContainsFunc(logger.Entries(), func(e testLog.LogEntry) bool {
		return strings.Contains(e.Message, "Entries removed from services")
	})
	if !found {
		t.Errorf("expected a warning about the dropped service, got %v", logger.Entries())
	}

	// Still there, which is what the warning is about.
	if !slices.Contains(rt.Manager.Services(), "other") {
		t.Error("expected the dropped service to stay registered")
	}
}

func TestReloadConfigReappliesCLIOverrides(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, `labels:
  region: west
`)

	params := NewParams()
	params.ConfigFile = configFile
	params.ConfigOverrides = []string{"default_decision=/example/allow"}
	params.Output = io.Discard
	params.Logger = testLog.New()

	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	writeConfig(t, configFile, `labels:
  region: west
  team: infra
`)

	if _, err := rt.reloadConfig(ctx); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if got := *rt.Manager.GetConfig().DefaultDecision; got != "/example/allow" {
		t.Errorf("expected --set override to survive the reload, got %q", got)
	}
}

func TestReloadConfigInvalid(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	writeConfig(t, configFile, `status:
  service: nonexistent
`)

	if _, err := rt.reloadConfig(t.Context()); err == nil {
		t.Fatal("expected error")
	}

	if p := status.Lookup(rt.Manager); p != nil {
		t.Error("expected no status plugin to be registered for an invalid config")
	}
}

func TestConfigWatcherAppliesChanges(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	reloads := make(chan error, 4)
	if err := rt.startConfigWatcher(t.Context(), func(_ time.Duration, err error) {
		reloads <- err
	}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}

	// Watched directory, but not the config file.
	writeConfig(t, filepath.Join(filepath.Dir(configFile), "unrelated.txt"), "hello")

	writeConfig(t, configFile, `labels:
  region: west
decision_logs:
  console: true
`)

	select {
	case err := <-reloads:
		if err != nil {
			t.Fatalf("reload config: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the config file change to be applied")
	}

	if p := logs.Lookup(rt.Manager); p == nil {
		t.Error("expected decision log plugin to be started")
	}

	select {
	case err := <-reloads:
		t.Errorf("unexpected second reload (err: %v)", err)
	default:
	}
}

func TestConfigWatcherCoalescesTruncateAndWrite(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `nd_builtin_cache: true
labels:
  region: west
`)

	if !rt.Manager.GetConfig().NDBuiltinCacheEnabled() {
		t.Fatal("expected the ND builtin cache to start out enabled")
	}

	// A read of the empty file would show the ND cache turned off.
	type reload struct {
		ndCache bool
		err     error
	}
	reloads := make(chan reload, 8)

	// Stands in for inotify's truncate-then-write on Linux; macOS reports the
	// truncate as a Chmod, which the watcher ignores.
	events := make(chan fsnotify.Event)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go rt.watchConfigEvents(ctx, events, nil, func(_ time.Duration, err error) {
		reloads <- reload{ndCache: rt.Manager.GetConfig().NDBuiltinCacheEnabled(), err: err}
	})

	evt := fsnotify.Event{Name: configFile, Op: fsnotify.Write}

	f, err := os.Create(configFile) // truncates in place
	if err != nil {
		t.Fatalf("truncate config: %v", err)
	}
	events <- evt

	// Inside configCoalesceWindow, but long enough for a watcher that doesn't wait.
	time.Sleep(25 * time.Millisecond)

	if _, err := f.WriteString(`nd_builtin_cache: true
labels:
  region: west
  team: infra
`); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close config: %v", err)
	}
	events <- evt

	select {
	case r := <-reloads:
		if r.err != nil {
			t.Fatalf("reload config: %v", r.err)
		}
		if !r.ndCache {
			t.Error("expected the ND builtin cache to stay enabled; the empty file was read")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the config file change to be applied")
	}

	if labels := rt.Manager.GetConfig().Labels; labels["team"] != "infra" {
		t.Errorf("expected label team=infra, got %v", labels)
	}

	// One logical change, so one reload.
	select {
	case r := <-reloads:
		t.Errorf("unexpected second reload (nd_builtin_cache: %v, err: %v)", r.ndCache, r.err)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestConfigWatcherIgnoresOtherFiles(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `nd_builtin_cache: true
labels:
  region: west
`)

	reloads := make(chan error, 4)
	events := make(chan fsnotify.Event)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go rt.watchConfigEvents(ctx, events, nil, func(_ time.Duration, err error) {
		reloads <- err
	})

	// Truncated, as a writer that hasn't finished would leave it.
	f, err := os.Create(configFile)
	if err != nil {
		t.Fatalf("truncate config: %v", err)
	}
	defer f.Close()

	events <- fsnotify.Event{
		Name: filepath.Join(filepath.Dir(configFile), "unrelated.txt"),
		Op:   fsnotify.Create,
	}

	select {
	case err := <-reloads:
		t.Errorf("unexpected reload for an unrelated file (err: %v)", err)
	case <-time.After(configCoalesceWindow + 500*time.Millisecond):
	}

	if !rt.Manager.GetConfig().NDBuiltinCacheEnabled() {
		t.Error("expected the ND builtin cache to stay enabled; the truncated file was read")
	}
}

func TestConfigWatcherNotStartedWithoutConfigFile(t *testing.T) {
	params := NewParams()
	params.Output = io.Discard
	params.Logger = testLog.New()

	rt, err := NewRuntime(t.Context(), params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {
		t.Error("unexpected reload")
	}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}
}

func TestConfigWatcherNotStartedWithDiscovery(t *testing.T) {
	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/discovery.tar.gz", map[string]string{
			"main.rego": "package config\n",
		}),
	)
	defer server.Stop()

	logger := testLog.New()
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, fmt.Sprintf(`services:
  test:
    url: %q
discovery:
  decision: config
  resource: /bundles/discovery.tar.gz
`, server.URL()))

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = logger

	rt, err := NewRuntime(t.Context(), params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {
		t.Error("unexpected reload")
	}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}

	found := slices.ContainsFunc(logger.Entries(), func(e testLog.LogEntry) bool {
		return strings.Contains(e.Message, "discovery is enabled")
	})
	if !found {
		t.Errorf("expected a warning about discovery, got %v", logger.Entries())
	}
}

func TestChangedConfigKeys(t *testing.T) {
	tests := []struct {
		note     string
		old, new string
		exp      []string
	}{
		{
			note: "no change",
			old:  "server:\n  decoding:\n    max_length: 42\n",
			new:  "server:\n  decoding:\n    max_length: 42\n",
		},
		{
			note: "change in a watched key",
			old:  "server:\n  decoding:\n    max_length: 42\n",
			new:  "server:\n  decoding:\n    max_length: 43\n",
			exp:  []string{"server"},
		},
		{
			note: "watched key added",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: west\nstorage:\n  disk:\n    directory: /tmp/opa\n",
			exp:  []string{"storage"},
		},
		{
			note: "watched key removed",
			old:  "storage:\n  disk:\n    directory: /tmp/opa\n",
			new:  "labels:\n  region: west\n",
			exp:  []string{"storage"},
		},
		{
			note: "change outside the watched keys",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: east\ndecision_logs:\n  console: true\n",
		},
		{
			note: "key order is not a change",
			old:  "server:\n  encoding:\n    gzip:\n      min_length: 1\n  decoding:\n    max_length: 42\n",
			new:  "server:\n  decoding:\n    max_length: 42\n  encoding:\n    gzip:\n      min_length: 1\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			oldConf, err := rawConfigMap([]byte(tc.old))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			newConf, err := rawConfigMap([]byte(tc.new))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			changed := changedKeys(oldConf, newConf, nonReloadableConfigKeys)
			if !slices.Equal(changed, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, changed)
			}
		})
	}
}

func TestRemovedPlugins(t *testing.T) {
	tests := []struct {
		note    string
		running []string
		custom  []string
		new     string
		exp     []string
	}{
		{
			note: "nothing running",
			new:  "labels:\n  region: east\n",
		},
		{
			note:    "retained",
			running: []string{logs.Name},
			new:     "decision_logs:\n  console: false\n",
		},
		{
			note:    "removed",
			running: []string{logs.Name},
			new:     "labels:\n  region: west\n",
			exp:     []string{"decision_logs"},
		},
		{
			note:    "an empty section is still a section",
			running: []string{logs.Name},
			new:     "decision_logs:\n",
			// pluginset reports this one, once the section has been parsed.
		},
		{
			note: "a section that was never running is not a removal",
			new:  "labels:\n  region: west\n",
		},
		{
			note:    "several, reported in a stable order",
			running: []string{bundle.Name, logs.Name, status.Name},
			new:     "labels:\n  region: west\n",
			exp:     []string{"bundles", "decision_logs", "status"},
		},
		{
			note:    "the deprecated bundle key keeps the bundle plugin",
			running: []string{bundle.Name},
			new:     "bundle:\n  name: b\n  service: s\n",
		},
		{
			note:    "the whole bundle group removed",
			running: []string{bundle.Name},
			new:     "labels:\n  region: west\n",
			exp:     []string{"bundles"},
		},
		{
			note:    "custom plugin removed",
			running: []string{"foo", "bar"},
			custom:  []string{"foo", "bar"},
			new:     "plugins:\n  foo: {}\n",
			exp:     []string{"plugins.bar"},
		},
		{
			note:    "all custom plugins removed",
			running: []string{"foo"},
			custom:  []string{"foo"},
			new:     "labels:\n  region: west\n",
			exp:     []string{"plugins.foo"},
		},
		{
			note:   "a registered custom plugin that never ran is not a removal",
			custom: []string{"foo"},
			new:    "labels:\n  region: west\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			newConf, err := rawConfigMap([]byte(tc.new))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			running := func(name string) bool { return slices.Contains(tc.running, name) }
			if removed := removedPlugins(running, tc.custom, newConf); !slices.Equal(removed, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, removed)
			}
		})
	}
}

func TestDroppedKeys(t *testing.T) {
	tests := []struct {
		note     string
		old, new string
		exp      []string
	}{
		{
			note: "nothing to drop",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: east\n",
		},
		{
			note: "service retained",
			old:  "services:\n  s:\n    url: http://localhost\n",
			new:  "services:\n  s:\n    url: http://elsewhere\n",
		},
		{
			note: "service dropped",
			old:  "services:\n  s:\n    url: http://localhost\n  t:\n    url: http://localhost\n",
			new:  "services:\n  s:\n    url: http://localhost\n",
			exp:  []string{"services"},
		},
		{
			note: "all services dropped",
			old:  "services:\n  s:\n    url: http://localhost\n",
			new:  "labels:\n  region: west\n",
			exp:  []string{"services"},
		},
		{
			note: "both",
			old:  "services:\n  s:\n    url: http://localhost\nkeys:\n  k:\n    key: secret\n",
			new:  "labels:\n  region: west\n",
			exp:  []string{"services", "keys"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			oldConf, err := rawConfigMap([]byte(tc.old))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			newConf, err := rawConfigMap([]byte(tc.new))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if dropped := droppedKeys(oldConf, newConf, "services", "keys"); !slices.Equal(dropped, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, dropped)
			}
		})
	}
}

func TestChangedLabels(t *testing.T) {
	tests := []struct {
		note     string
		old, new string
		exp      []string
	}{
		{
			note: "no labels at all",
			old:  "decision_logs:\n  console: true\n",
			new:  "decision_logs:\n  console: false\n",
		},
		{
			note: "unchanged",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: west\n",
		},
		{
			note: "addition is allowed",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: west\n  team: infra\n",
		},
		{
			note: "labels added where there were none",
			old:  "decision_logs:\n  console: true\n",
			new:  "labels:\n  region: west\n",
		},
		{
			note: "changed value",
			old:  "labels:\n  region: west\n",
			new:  "labels:\n  region: east\n",
			exp:  []string{"region"},
		},
		{
			note: "removed label",
			old:  "labels:\n  region: west\n  team: infra\n",
			new:  "labels:\n  region: west\n",
			exp:  []string{"team"},
		},
		{
			note: "whole section removed",
			old:  "labels:\n  region: west\n",
			new:  "decision_logs:\n  console: true\n",
			exp:  []string{"region"},
		},
		{
			note: "reported in a stable order",
			old:  "labels:\n  region: west\n  team: infra\n  zone: a\n",
			new:  "labels:\n  region: east\n  team: platform\n  zone: b\n",
			exp:  []string{"region", "team", "zone"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			oldConf, err := rawConfigMap([]byte(tc.old))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			newConf, err := rawConfigMap([]byte(tc.new))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if changed := changedLabels(oldConf, newConf); !slices.Equal(changed, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, changed)
			}
		})
	}
}
