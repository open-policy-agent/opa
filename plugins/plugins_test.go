// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package plugins

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown/cache"
)

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
	m.UpdatePluginStatus("p1", &Status{State: StateOK})
	currentStatus = m.PluginStatus()
	if len(currentStatus) != 1 || currentStatus["p1"].State != StateOK {
		t.Fatalf("Expected 1 statuses in current plugin status map with state OK, got: %+v", currentStatus)
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
	if len(currentStatus) != 2 || currentStatus["p1"].State != StateOK || currentStatus["p2"].State != StateErr {
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
	if len(currentStatus) != 2 || currentStatus["p1"].State != StateOK || currentStatus["p2"].State != StateOK {
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

type testPlugin struct {
	m *Manager
}

func (p *testPlugin) Start(ctx context.Context) error {
	p.m.UpdatePluginStatus("p1", &Status{State: StateOK})
	return nil
}

func (p *testPlugin) Stop(ctx context.Context) {
	p.m.UpdatePluginStatus("p1", &Status{State: StateNotReady})
}

func (p *testPlugin) Reconfigure(ctx context.Context, config interface{}) {
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

func (m *mockForInitStartOrdering) Stop(ctx context.Context)                            { return }
func (m *mockForInitStartOrdering) Reconfigure(ctx context.Context, config interface{}) { return }
