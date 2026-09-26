// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/v1/logging"
	testLog "github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/util/test"
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

// restartRequested reports whether a restart is pending, consuming it so a
// later check sees only what happened since.
func restartRequested(rt *Runtime) bool {
	select {
	case <-rt.restartc:
		return true
	default:
		return false
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
	if restartRequested(rt) {
		t.Error("expected no restart to be requested")
	}
}

// TestReloadConfigRequestsRestart covers the options that used to be refused
// outright. Each is now applied by restarting, so the reload only has to accept
// them and ask for one.
func TestReloadConfigRequestsRestart(t *testing.T) {
	for _, tc := range []struct {
		note   string
		config string
	}{
		{
			note: "a start-up only option",
			config: `labels:
  region: west
server:
  decoding:
    max_length: 42
`,
		},
		{
			note: "storage",
			config: `labels:
  region: west
persistence_directory: /tmp/opa-reload
`,
		},
		{
			note: "a changed label",
			config: `labels:
  region: east
`,
		},
		{
			note: "a removed label",
			config: `labels: {}
`,
		},
		{
			note:   "a dropped section",
			config: "{}\n",
		},
		{
			note: "a new plugin",
			config: `labels:
  region: west
decision_logs:
  console: true
`,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
status:
  console: true
`)

			writeConfig(t, configFile, tc.config)

			changed, err := rt.reloadConfig(t.Context())
			if err != nil {
				t.Fatalf("reload config: %v", err)
			}
			if !changed {
				t.Error("expected the on-disk change to be reported")
			}
			if !restartRequested(rt) {
				t.Error("expected a restart to be requested")
			}
		})
	}
}

// TestReloadConfigRejectsInvalid covers what can be known without tearing
// anything down. A configuration OPA could not come up under must not cost the
// one that is serving.
func TestReloadConfigRejectsInvalid(t *testing.T) {
	for _, tc := range []struct {
		note   string
		config string
		expErr string
	}{
		{
			note: "a plugin option of the wrong type",
			config: `decision_logs:
  reporting:
    max_delay_seconds: "soon"
`,
			expErr: "max_delay_seconds",
		},
		{
			note: "a reference to a service that does not exist",
			config: `bundles:
  b:
    service: nowhere
`,
			expErr: "nowhere",
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
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Errorf("expected error mentioning %q, got %q", tc.expErr, err.Error())
			}
			if restartRequested(rt) {
				t.Error("expected no restart to be requested for a configuration OPA cannot run")
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

// TestReloadConfigUnreadable covers a file that cannot be read at all, which
// fails before OPA has anything to compare against and so keeps being reported.
func TestReloadConfigUnreadable(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	writeConfig(t, configFile, "labels: [\n")

	if _, err := rt.reloadConfig(t.Context()); err == nil {
		t.Fatal("expected error")
	}
	if restartRequested(rt) {
		t.Error("expected no restart to be requested")
	}
}

func TestReloadConfigReappliesCLIOverrides(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)
	rt.Params.ConfigOverrides = []string{"labels.team=infra"}

	writeConfig(t, configFile, `labels:
  region: west
  extra: yes
`)

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if !strings.Contains(string(rt.appliedConfig), "infra") {
		t.Errorf("expected the CLI override to be re-applied, got %s", rt.appliedConfig)
	}
}

func TestConfigWatcherRequestsRestartOnChange(t *testing.T) {
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
		t.Fatal("timed out waiting for the config file change to be picked up")
	}

	if !restartRequested(rt) {
		t.Error("expected a restart to be requested")
	}

	select {
	case err := <-reloads:
		t.Errorf("unexpected second reload (err: %v)", err)
	default:
	}
}

// TestConfigWatcherCoalescesTruncateAndWrite guards against reading a file a
// writer has emptied but not yet filled in.
func TestConfigWatcherCoalescesTruncateAndWrite(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `nd_builtin_cache: true
labels:
  region: west
`)

	reloads := make(chan []byte, 4)
	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {
		rt.configMtx.Lock()
		defer rt.configMtx.Unlock()
		reloads <- rt.appliedConfig
	}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}

	f, err := os.OpenFile(configFile, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("truncate config: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := f.WriteString("nd_builtin_cache: true\nlabels:\n  region: east\n"); err != nil {
		t.Fatalf("write config: %v", err)
	}
	f.Close()

	select {
	case applied := <-reloads:
		if !strings.Contains(string(applied), "nd_builtin_cache") {
			t.Errorf("expected the settled file to be read, got %s", applied)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the config file change to be picked up")
	}

	select {
	case applied := <-reloads:
		t.Errorf("unexpected second reload: %s", applied)
	default:
	}
}

func TestConfigWatcherIgnoresOtherFiles(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	events := make(chan fsnotify.Event)
	errs := make(chan error)
	done := make(chan struct{})

	rt.configWatcherStop = make(chan struct{})
	go func() {
		defer close(done)
		rt.watchConfigEvents(t.Context(), events, errs, func(time.Duration, error) {
			t.Error("unexpected reload")
		})
	}()

	events <- fsnotify.Event{Name: filepath.Join(filepath.Dir(configFile), "other.yaml"), Op: fsnotify.Write}

	time.Sleep(2 * configCoalesceWindow)
	close(rt.configWatcherStop)
	<-done
}

func TestConfigWatcherNotStartedWithoutConfigFile(t *testing.T) {
	params := NewParams()
	params.Output = io.Discard
	params.Logger = testLog.New()

	rt, err := NewRuntime(t.Context(), params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}
	if rt.configWatcherStop != nil {
		t.Error("expected no watcher to be started without a configuration file")
	}
}

func TestConfigWatcherNotStartedWithDiscovery(t *testing.T) {
	logger := testLog.New()
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, `services:
  acme:
    url: https://example.com
discovery:
  resource: /bundles/discovery.tar.gz
`)

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = logger

	rt, err := NewRuntime(t.Context(), params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}
	if rt.configWatcherStop != nil {
		t.Error("expected no watcher to be started when discovery is enabled")
	}

	var warned bool
	for _, e := range logger.Entries() {
		if strings.Contains(e.Message, "discovery is enabled") {
			warned = true
		}
	}
	if !warned {
		t.Error("expected a warning that the configuration file is not watched")
	}
}

// TestReloadConfigAcceptsNewServiceAndDependants covers the most common reload
// there is: a service arriving in the same change as what uses it. Validation
// runs before the service is registered, so it has to read the names from the
// configuration rather than from the running manager.
func TestReloadConfigAcceptsNewServiceAndDependants(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)

	writeConfig(t, configFile, `services:
  acme:
    url: https://example.com
bundles:
  b1:
    service: acme
decision_logs:
  service: acme
status:
  service: acme
`)

	if _, err := rt.reloadConfig(t.Context()); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !restartRequested(rt) {
		t.Error("expected a restart to be requested")
	}
}

// TestRouterSharedBetweenManagerAndServer guards the mux OPA registers its
// routes on. The manager hands it to plugins and the server registers the API on
// it, so the two must be the same one -- and a restart must give both a new one,
// since registering a pattern twice panics.
func TestRouterSharedBetweenManagerAndServer(t *testing.T) {
	rt, _ := newConfigReloadRuntime(t, `labels:
  region: west
`)
	rt.Params.Addrs = &[]string{"localhost:0"}
	rt.Params.DiagnosticAddrs = &[]string{}

	if _, err := rt.buildServer(t.Context()); err != nil {
		t.Fatalf("build server: %v", err)
	}
	if rt.Manager.GetRouter() != rt.Params.Router {
		t.Errorf("manager router %p is not the one served %p", rt.Manager.GetRouter(), rt.Params.Router)
	}

	// A restart builds both afresh, and they still have to match.
	before := rt.Params.Router
	if err := rt.configure(t.Context(), rt.appliedConfig, rt.metrics); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if rt.Params.Router == before {
		t.Error("expected a restart to get a fresh router")
	}
	if _, err := rt.buildServer(t.Context()); err != nil {
		t.Fatalf("build server: %v", err)
	}
	if rt.Manager.GetRouter() != rt.Params.Router {
		t.Errorf("after a restart, manager router %p is not the one served %p", rt.Manager.GetRouter(), rt.Params.Router)
	}
}

func TestWatchDoesNotImplyWatchConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, `server:
  decoding:
    max_length: 64
`)

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = testLog.New()
	params.Addrs = &[]string{"localhost:0"}
	params.Watch = true
	params.GracefulShutdownPeriod = 1

	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	go func() { _ = rt.Serve(ctx) }()
	if !test.Eventually(t, 10*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("timed out waiting for the server to start")
	}

	before := rt.Addrs()[0]

	writeConfig(t, configFile, `server:
  decoding:
    max_length: 4096
`)

	// Nothing should come of it. Long enough for the coalesce window and a
	// restart to have run had one been asked for.
	time.Sleep(20 * configCoalesceWindow)

	if restartRequested(rt) {
		t.Error("expected no restart to be requested when only --watch is set")
	}
	if after := rt.Addrs(); len(after) != 1 || after[0] != before {
		t.Errorf("expected the listener to be left alone, was %s now %v", before, after)
	}
}

func TestStartConfigWatcherReconcilesOnStart(t *testing.T) {
	rt, configFile := newConfigReloadRuntime(t, `labels:
  region: west
`)
	rt.Params.WatchConfig = true

	writeConfig(t, configFile, `labels:
  region: east
`)

	if err := rt.startConfigWatcher(t.Context(), func(time.Duration, error) {}); err != nil {
		t.Fatalf("start config watcher: %v", err)
	}
	t.Cleanup(rt.stopConfigWatcher)

	if !restartRequested(rt) {
		t.Error("expected the change written before the watch was added to be picked up")
	}
}

func TestServeRestartsWhenShutdownOverruns(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, `server:
  decoding:
    max_length: 64
`)

	logger := testLog.New()
	logger.SetLevel(logging.Debug)

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = logger
	params.Addrs = &[]string{"localhost:0"}
	params.WatchConfig = true
	params.GracefulShutdownPeriod = 1

	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	served := make(chan error, 1)
	go func() { served <- rt.Serve(ctx) }()
	if !test.Eventually(t, 10*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("timed out waiting for the server to start")
	}

	before := rt.Addrs()[0]

	// A request whose body never arrives holds the connection open, so it is
	// never idle and the graceful shutdown runs out of time on it.
	stalled, err := net.Dial("tcp", before)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer stalled.Close()
	if _, err := io.WriteString(stalled, "POST /v1/data HTTP/1.1\r\nHost: opa\r\nContent-Type: application/json\r\nContent-Length: 32\r\n\r\n{"); err != nil {
		t.Fatalf("write stalled request: %v", err)
	}

	writeConfig(t, configFile, `server:
  decoding:
    max_length: 4096
`)

	body := fmt.Sprintf(`{"input": {"pad": %q}}`, strings.Repeat("a", 128))
	var lastCode int
	var lastErr error
	if !test.Eventually(t, 30*time.Second, func() bool {
		addrs := rt.Addrs()
		if len(addrs) == 0 {
			lastCode, lastErr = 0, errors.New("no listener")
			return false
		}
		resp, err := http.Post("http://"+addrs[0]+"/v1/data", "application/json", strings.NewReader(body))
		if err != nil {
			lastCode, lastErr = 0, err
			return false
		}
		resp.Body.Close()
		lastCode, lastErr = resp.StatusCode, nil
		return resp.StatusCode == http.StatusOK
	}) {
		reportServeState(t, rt, served, before, lastCode, lastErr)
		t.Fatal("the overrunning shutdown cost the configuration that asked for the restart")
	}
}

// TestServeRestartsOnConfigChange is the end-to-end proof: a change to
// server.decoding, which cannot be applied to a running handler chain, takes
// effect because the serve routine comes back up under it.
func TestServeRestartsOnConfigChange(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, configFile, `server:
  decoding:
    max_length: 64
`)

	// Debug level, and reported on failure below: every way this test can time
	// out looks the same from the outside -- the reload was rejected, the
	// restart fell back to the previous configuration, or the file event never
	// arrived -- and only the runtime's own log tells them apart.
	logger := testLog.New()
	logger.SetLevel(logging.Debug)

	params := NewParams()
	params.ConfigFile = configFile
	params.Output = io.Discard
	params.Logger = logger
	params.Addrs = &[]string{"localhost:0"}
	params.WatchConfig = true
	params.GracefulShutdownPeriod = 1

	ctx := t.Context()
	rt, err := NewRuntime(ctx, params)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	served := make(chan error, 1)
	go func() { served <- rt.Serve(ctx) }()
	if !test.Eventually(t, 10*time.Second, func() bool {
		return rt.ServerStatus() == ServerInitialized && len(rt.Addrs()) > 0
	}) {
		t.Fatal("timed out waiting for the server to start")
	}

	body := fmt.Sprintf(`{"input": {"pad": %q}}`, strings.Repeat("a", 128))
	post := func() (int, error) {
		addrs := rt.Addrs()
		if len(addrs) == 0 {
			return 0, errors.New("no listener")
		}
		resp, err := http.Post("http://"+addrs[0]+"/v1/data", "application/json", strings.NewReader(body))
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}

	code, err := post()
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if code != http.StatusBadRequest {
		t.Fatalf("expected the body to exceed server.decoding.max_length, got %d", code)
	}

	before := rt.Addrs()[0]

	writeConfig(t, configFile, `server:
  decoding:
    max_length: 4096
`)

	// The listener is rebound, so both the address and the limit change.
	var lastCode int
	var lastErr error
	if !test.Eventually(t, 30*time.Second, func() bool {
		lastCode, lastErr = post()
		return lastErr == nil && lastCode == http.StatusOK
	}) {
		reportServeState(t, rt, served, before, lastCode, lastErr)
		t.Fatal("timed out waiting for the new decoding limit to take effect")
	}

	if after := rt.Addrs()[0]; after == before {
		t.Errorf("expected the listener to be rebound, still on %s", after)
	}
}

// reportServeState describes what the serve routine ended up doing, so a
// timeout in TestServeRestartsOnConfigChange says which step did not happen
// rather than only that none of them did. An address unchanged from before
// means no restart ran at all; a new one means a restart ran and came up under
// the wrong configuration.
func reportServeState(t *testing.T, rt *Runtime, served <-chan error, before string, code int, err error) {
	t.Helper()

	t.Logf("last response: code=%d err=%v", code, err)
	t.Logf("addresses: before=%s now=%v status=%d", before, rt.Addrs(), rt.ServerStatus())

	select {
	case err := <-served:
		t.Logf("serve returned: %v", err)
	default:
		t.Log("serve still running")
	}

	// Everything worth seeing -- the file event, the reload, the restart -- is
	// logged in the first moments after the write, and the rest of the wait is
	// the handler logging one rejected request per poll. Drop those and keep
	// the head.
	var entries []testLog.LogEntry
	for _, e := range rt.Params.Logger.(*testLog.Logger).Entries() {
		if _, request := e.Fields["req_id"]; request {
			continue
		}
		entries = append(entries, e)
		if len(entries) == maxReportedLogEntries {
			break
		}
	}
	for _, e := range entries {
		t.Logf("[%v] %s %v", e.Level, e.Message, e.Fields)
	}
}

// Enough to cover the reload and the restart, without pasting a whole run into
// the failure output.
const maxReportedLogEntries = 200
