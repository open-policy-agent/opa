// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	internal_tracing "github.com/open-policy-agent/opa/internal/distributedtracing"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/topdown/cache"
	prom "github.com/prometheus/client_golang/prometheus"
)

func TestManagerCacheTriggers(t *testing.T) {
	m, err := New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	l1Called := false
	m.RegisterCacheTrigger(func(*cache.Config) {
		l1Called = true
	})

	if m.registeredCacheTriggers[0] == nil {
		t.Fatal("First listener failed to register")
	}

	l2Called := false
	m.RegisterCacheTrigger(func(*cache.Config) {
		l2Called = true
	})

	if m.registeredCacheTriggers[0] == nil || m.registeredCacheTriggers[1] == nil {
		t.Fatal("Second listener failed to register")
	}

	if l1Called == true || l2Called == true {
		t.Fatal("Listeners should not be called yet")
	}

	err = m.Reconfigure(m.Config)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if l1Called == false || l2Called == false {
		t.Fatal("Listeners should hav been called")
	}
}

func TestManagerNDCacheTriggers(t *testing.T) {
	m, err := New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	l1Called := false
	m.RegisterNDCacheTrigger(func(bool) {
		l1Called = true
	})

	if m.registeredNDCacheTriggers[0] == nil {
		t.Fatal("First listener failed to register")
	}

	l2Called := false
	m.RegisterNDCacheTrigger(func(bool) {
		l2Called = true
	})

	if m.registeredNDCacheTriggers[0] == nil || m.registeredNDCacheTriggers[1] == nil {
		t.Fatal("Second listener failed to register")
	}

	if l1Called == true || l2Called == true {
		t.Fatal("Listeners should not be called yet")
	}

	err = m.Reconfigure(m.Config)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if l1Called == false || l2Called == false {
		t.Fatal("Listeners should hav been called")
	}
}

func TestManagerPluginStatusListener(t *testing.T) {
	m, err := New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// Start by registering a single listener and validate that it was registered correctly
	var l1Status map[string]*Status
	m.RegisterPluginStatusListener("l1", func(status map[string]*Status) {
		l1Status = status
	})
	if len(m.pluginStatusListeners) != 1 || m.pluginStatusListeners["l1"] == nil {
		t.Fatalf("Expected a single listener named 'l1' got: %+v", m.pluginStatusListeners)
	}

	// Register a second one, validate both are there
	var l2Status map[string]*Status
	m.RegisterPluginStatusListener("l2", func(status map[string]*Status) {
		l2Status = status
	})
	if len(m.pluginStatusListeners) != 2 || m.pluginStatusListeners["l2"] == nil {
		t.Fatalf("Expected a two listeners named 'l1' and 'l2' got: %+v", m.pluginStatusListeners)
	}

	// Ensure starting statuses are empty by default
	currentStatus := m.PluginStatus()
	if len(currentStatus) != 0 {
		t.Fatalf("Expected 0 statuses in current plugin status map, got: %+v", currentStatus)
	}

	// Push an update to a plugin, ensure current status is reflected and listeners were called
	const message = "foo"
	m.UpdatePluginStatus("p1", &Status{State: StateOK, Message: message})
	currentStatus = m.PluginStatus()
	if len(currentStatus) != 1 || currentStatus["p1"].State != StateOK || currentStatus["p1"].Message != message {
		t.Fatalf("Expected 1 statuses in current plugin status map with state OK and message 'foo', got: %+v", currentStatus)
	}
	if !reflect.DeepEqual(currentStatus, l1Status) || !reflect.DeepEqual(l1Status, l2Status) {
		t.Fatalf("Unexpected status in updates:\n\n\texpecting: %+v\n\n\tgot: l1: %+v  l2: %+v\n", currentStatus, l1Status, l2Status)
	}

	// Unregister the first listener, ensure it is removed
	m.UnregisterPluginStatusListener("l1")
	if len(m.pluginStatusListeners) != 1 || m.pluginStatusListeners["l2"] == nil {
		t.Fatalf("Expected a single listeners named 'l2' got: %+v", m.pluginStatusListeners)
	}

	// Send another update, ensure the status is ok and the remaining listener is still called
	m.UpdatePluginStatus("p2", &Status{State: StateErr})
	currentStatus = m.PluginStatus()
	if len(currentStatus) != 2 || currentStatus["p1"].State != StateOK || currentStatus["p1"].Message != message || currentStatus["p2"].State != StateErr {
		t.Fatalf("Unexpected current plugin status, got: %+v", currentStatus)
	}
	if !reflect.DeepEqual(currentStatus, l2Status) {
		t.Fatalf("Unexpected status in updates:\n\n\texpecting: %+v\n\n\tgot: %+v\n", currentStatus, l2Status)
	}

	// Unregister the last listener
	m.UnregisterPluginStatusListener("l2")
	if len(m.pluginStatusListeners) != 0 {
		t.Fatalf("Expected zero listeners got: %+v", m.pluginStatusListeners)
	}

	// Ensure updates can still be sent with no listeners
	m.UpdatePluginStatus("p2", &Status{State: StateOK})
	currentStatus = m.PluginStatus()
	if len(currentStatus) != 2 || currentStatus["p1"].State != StateOK || currentStatus["p1"].Message != message || currentStatus["p2"].State != StateOK {
		t.Fatalf("Unexpected current plugin status, got: %+v", currentStatus)
	}
}

func TestPluginStatusUpdateOnStartAndStop(t *testing.T) {
	m, err := New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	m.Register("p1", &testPlugin{m})

	err = m.Start(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	m.Stop(context.Background())
}

func TestManagerWithOPATelemetryUpdateLoop(t *testing.T) {
	// test server
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	versions := []string{}
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, req *http.Request) {
		var data map[string]string

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}

		err = json.Unmarshal(body, &data)
		if err != nil {
			t.Fatal(err)
		}

		versions = append(versions, data["min_compatible_version"])

		w.WriteHeader(http.StatusOK)
		bs, _ := json.Marshal(map[string]string{"foo": "bar"}) // dummy data
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bs) // ignore error
	})
	defer ts.Close()

	t.Setenv("OPA_TELEMETRY_SERVICE_URL", ts.URL)

	ctx := context.Background()

	m, err := New([]byte{}, "test", inmem.New(), WithEnableTelemetry(true))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	defaultUploadIntervalSec = int64(1)

	err = m.Start(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// add a policy to the store to trigger a telemetry update
	module := `package x
				p { array.reverse([1,2,3]) }`

	err = storage.Txn(ctx, m.Store, storage.WriteParams, func(txn storage.Transaction) error {
		return m.Store.UpsertPolicy(ctx, txn, "policy.rego", []byte(module))
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(2 * time.Second)

	// add data to the store and verify there is no trigger for a telemetry update
	err = storage.Txn(ctx, m.Store, storage.WriteParams, func(txn storage.Transaction) error {
		return m.Store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/a"), `[2,1,3]`)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// add a bundle with some policy to trigger a telemetry update
	txn := storage.NewTransactionOrDie(ctx, m.Store, storage.WriteParams)

	var archiveFiles = map[string]string{
		"/a/b/c/data.json":   "[1,2,3]",
		"/policy.rego":       "package foo\n import future.keywords.every",
		"/roles/policy.rego": "package bar\n import future.keywords.if\n p.a.b.c.d if { true }",
	}

	files := make([][2]string, 0, len(archiveFiles))
	for name, content := range archiveFiles {
		files = append(files, [2]string{name, content})
	}

	buf := archive.MustWriteTarGz(files)
	b, err := bundle.NewReader(buf).WithLazyLoadingMode(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	iterator := bundle.NewIterator(b.Raw)

	params := storage.WriteParams
	params.BasePaths = []string{""}

	err = m.Store.Truncate(ctx, txn, params, iterator)
	if err != nil {
		t.Fatalf("Unexpected truncate error: %v", err)
	}

	if err := m.Store.Commit(ctx, txn); err != nil {
		t.Fatalf("Unexpected commit error: %v", err)
	}

	time.Sleep(2 * time.Second)

	m.Stop(ctx)

	exp := 2
	if len(versions) != exp {
		t.Fatalf("Expected number of server calls: %+v but got: %+v", exp, len(versions))
	}

	expVers := []string{"0.36.0", "0.46.0"}
	if !reflect.DeepEqual(expVers, versions) {
		t.Fatalf("Expected OPA versions: %+v but got: %+v", expVers, versions)
	}
}

type testPlugin struct {
	m *Manager
}

func (p *testPlugin) Start(context.Context) error {
	p.m.UpdatePluginStatus("p1", &Status{State: StateOK})
	return nil
}

func (p *testPlugin) Stop(context.Context) {
	p.m.UpdatePluginStatus("p1", &Status{State: StateNotReady})
}

func (p *testPlugin) Reconfigure(context.Context, interface{}) {
	p.m.UpdatePluginStatus("p1", &Status{State: StateNotReady})
}

func TestPluginManagerLazyInitBeforePluginStart(t *testing.T) {

	m, err := New([]byte(`{"plugins": {"someplugin": {"enabled": true}}}`), "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockForInitStartOrdering{Manager: m}

	m.Register("someplugin", mock)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !mock.Started {
		t.Fatal("expected plugin to be started")
	}

}

func TestPluginManagerInitBeforePluginStart(t *testing.T) {

	m, err := New([]byte(`{"plugins": {"someplugin": {}}}`), "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	mock := &mockForInitStartOrdering{Manager: m}

	m.Register("someplugin", mock)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !mock.Started {
		t.Fatal("expected plugin to be started")
	}

}

func TestPluginManagerInitIdempotence(t *testing.T) {

	mockStore := mock.New()

	m, err := New([]byte(`{"plugins": {"someplugin": {}}}`), "test", mockStore)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	exp := len(mockStore.Transactions)

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if len(mockStore.Transactions) != exp {
		t.Fatal("expected num txns to be:", exp, "but got:", len(mockStore.Transactions))
	}

}

func TestManagerWithCachingConfig(t *testing.T) {
	m, err := New([]byte(`{"caching": {"inter_query_builtin_cache": {"max_size_bytes": 100}}}`), "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	expected, _ := cache.ParseCachingConfig(nil)
	limit := int64(100)
	expected.InterQueryBuiltinCache.MaxSizeBytes = &limit

	if !reflect.DeepEqual(m.InterQueryBuiltinCacheConfig(), expected) {
		t.Fatalf("want %+v got %+v", expected, m.interQueryBuiltinCacheConfig)
	}

	// config error
	_, err = New([]byte(`{"caching": {"inter_query_builtin_cache": {"max_size_bytes": "100"}}}`), "test", inmem.New())
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestManagerWithNDCachingConfig(t *testing.T) {
	m, err := New([]byte(`{"nd_builtin_cache": true}`), "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	expected := true
	if !m.Config.NDBuiltinCache == expected {
		t.Fatalf("want %+v got %+v", expected, m.Config.NDBuiltinCache)
	}

	// config error
	_, err = New([]byte(`{"nd_builtin_cache": "x"}`), "test", inmem.New())
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

type mockForInitStartOrdering struct {
	Manager *Manager
	Started bool
}

func (m *mockForInitStartOrdering) Start(ctx context.Context) error {
	m.Started = true
	if m.Manager.initialized {
		return nil
	}
	return fmt.Errorf("expected manager to be initialized")
}

func (*mockForInitStartOrdering) Stop(context.Context)                     {}
func (*mockForInitStartOrdering) Reconfigure(context.Context, interface{}) {}

func TestPluginManagerAuthPlugin(t *testing.T) {
	m, err := New([]byte(`{"plugins": {"someplugin": {}}}`), "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	mock := &myAuthPluginMock{}

	m.Register("someplugin", mock)

	authPlugin := m.AuthPlugin("someplugin")

	if authPlugin == nil {
		t.Fatal("expected to receive HTTPAuthPlugin")
	}

	switch authPlugin.(type) {
	case *myAuthPluginMock:
		return
	default:
		t.Fatal("expected HTTPAuthPlugin to be myAuthPluginMock")
	}
}

func TestPluginManagerLogger(t *testing.T) {

	logger := logging.Get().WithFields(map[string]interface{}{"context": "myloggincontext"})

	m, err := New([]byte(`{}`), "test", inmem.New(), Logger(logger))
	if err != nil {
		t.Fatal(err)
	}

	if m.Logger() != logger {
		t.Fatal("Logger was not configured on plugin manager")
	}
}

func TestPluginManagerConsoleLogger(t *testing.T) {
	consoleLogger := test.New()

	mgr, err := New([]byte(`{}`), "", inmem.New(), ConsoleLogger(consoleLogger))
	if err != nil {
		t.Fatal(err)
	}

	const fieldKey = "foo"
	const fieldValue = "bar"
	mgr.ConsoleLogger().WithFields(map[string]interface{}{fieldKey: fieldValue}).Info("Some message")

	entries := consoleLogger.Entries()

	exp := []test.LogEntry{
		{
			Level:   logging.Info,
			Fields:  map[string]interface{}{fieldKey: fieldValue},
			Message: "Some message",
		},
	}

	if !reflect.DeepEqual(exp, entries) {
		t.Fatalf("want %v but got %v", exp, entries)
	}
}

func TestPluginManagerPrometheusRegister(t *testing.T) {
	register := prometheusRegisterMock{Collectors: map[prom.Collector]bool{}}
	mgr, err := New([]byte(`{}`), "", inmem.New(), WithPrometheusRegister(register))
	if err != nil {
		t.Fatal(err)
	}

	counter := prom.NewCounter(prom.CounterOpts{})
	if err := mgr.PrometheusRegister().Register(counter); err != nil {
		t.Fatal(err)
	}
	if register.Collectors[counter] != true {
		t.Fatalf("Counter metric was not registered on prometheus")
	}
}

func TestPluginManagerTracerProvider(t *testing.T) {
	_, tracerProvider, err := internal_tracing.Init(context.TODO(), []byte(`{ "distributed_tracing": { "type": "grpc" } }`), "test")
	if err != nil {
		t.Fatal(err)
	}
	m, err := New([]byte(`{}`), "test", inmem.New(), WithTracerProvider(tracerProvider))
	if err != nil {
		t.Fatal(err)
	}

	if m.TracerProvider() != tracerProvider {
		t.Fatal("TracerProvider was not configured on plugin manager")
	}
}
func TestPluginManagerServerInitialized(t *testing.T) {
	// Verify that ServerInitializedChannel is closed when
	// ServerInitialized is called.
	m1, err := New([]byte{}, "test1", inmem.New())
	if err != nil {
		t.Fatal(err)
	}
	initChannel1 := m1.ServerInitializedChannel()
	m1.ServerInitialized()
	// Verify that ServerInitialized is idempotent and will not panic
	m1.ServerInitialized()
	select {
	case <-initChannel1:
		break
	default:
		t.Fatal("expected ServerInitializedChannel to be closed")
	}

	// Verify that ServerInitializedChannel is open when
	// ServerInitialized is not called.
	m2, err := New([]byte{}, "test2", inmem.New())
	if err != nil {
		t.Fatal(err)
	}
	initChannel2 := m2.ServerInitializedChannel()
	select {
	case <-initChannel2:
		t.Fatal("expected ServerInitializedChannel to be open and have no messages")
	default:
		break
	}
}

type myAuthPluginMock struct{}

func (m *myAuthPluginMock) NewClient(c rest.Config) (*http.Client, error) {
	tlsConfig, err := rest.DefaultTLSConfig(c)
	if err != nil {
		return nil, err
	}
	return rest.DefaultRoundTripperClient(
		tlsConfig,
		10,
	), nil
}
func (*myAuthPluginMock) Prepare(*http.Request) error {
	return nil
}
func (*myAuthPluginMock) Start(context.Context) error {
	return nil
}
func (*myAuthPluginMock) Stop(context.Context) {
}
func (*myAuthPluginMock) Reconfigure(context.Context, interface{}) {
}

type prometheusRegisterMock struct {
	Collectors map[prom.Collector]bool
}

func (p prometheusRegisterMock) Register(collector prom.Collector) error {
	p.Collectors[collector] = true
	return nil
}

func (p prometheusRegisterMock) MustRegister(collector ...prom.Collector) {
	for _, c := range collector {
		p.Collectors[c] = true
	}
}

func (p prometheusRegisterMock) Unregister(collector prom.Collector) bool {
	delete(p.Collectors, collector)
	return true
}
