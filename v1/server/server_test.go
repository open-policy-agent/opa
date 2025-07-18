// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/open-policy-agent/opa/internal/distributedtracing"
	"github.com/open-policy-agent/opa/internal/prometheus"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/config"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	pluginBundle "github.com/open-policy-agent/opa/v1/plugins/bundle"
	pluginStatus "github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/server/authorizer"
	"github.com/open-policy-agent/opa/v1/server/identifier"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/disk"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
	"github.com/open-policy-agent/opa/v1/version"
	prom "github.com/prometheus/client_golang/prometheus"
)

type tr struct {
	method string
	path   string
	body   string
	code   int
	resp   string
}

func TestUnversionedGetHealth(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqUnversioned(http.MethodGet, "/health", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)
}

func TestUnversionedGetHealthBundleNoBundleSet(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)
}

func TestUnversionedGetHealthCheckOnlyBundlePlugin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	// Initialize the server as if a bundle plugin was
	// configured on the manager.
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateNotReady})

	// The bundle hasn't been activated yet, expect the health check to fail
	req := newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 500, `{"error":"one or more bundles are not activated"}`)

	// Set the bundle to be activated.
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateOK})

	// The heath check should now respond as healthy
	req = newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)
}

func TestUnversionedGetHealthCheckDiscoveryWithBundle(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	// Initialize the server as if a discovery bundle is configured
	f.server.manager.UpdatePluginStatus("discovery", &plugins.Status{State: plugins.StateNotReady})

	// The discovery bundle hasn't been activated yet, expect the health check to fail
	req := newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 500, `{"error":"one or more bundles are not activated"}`)

	// Set the bundle to be not ready (plugin configured and created, but hasn't activated all bundles yet).
	f.server.manager.UpdatePluginStatus("discovery", &plugins.Status{State: plugins.StateOK})
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateNotReady})

	// The discovery bundle is OK, but the newly configured bundle hasn't been activated yet, expect the health check to fail
	req = newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 500, `{"error":"one or more bundles are not activated"}`)

	// Set the bundle to be activated.
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateOK})

	// The heath check should now respond as healthy
	req = newReqUnversioned(http.MethodGet, "/health?bundles=true", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)
}

func TestUnversionedGetHealthCheckBundleActivationSingleLegacy(t *testing.T) {
	t.Parallel()

	// Initialize the server as if there is no bundle plugin

	f := newFixture(t)

	ctx := context.Background()

	// The server doesn't know about any bundles, so return a healthy status
	req := newReqUnversioned(http.MethodGet, "/health?bundle=true", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)

	err := storage.Txn(ctx, f.server.store, storage.WriteParams, func(txn storage.Transaction) error {
		return bundle.LegacyWriteManifestToStore(ctx, f.server.store, txn, bundle.Manifest{
			Revision: "a",
		})
	})

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// The heath check still respond as healthy with a legacy bundle found in storage
	req = newReqUnversioned(http.MethodGet, "/health?bundle=true", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)
}

func TestBundlesReady(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		status map[string]*plugins.Status
		ready  bool
	}{
		{
			note:   "nil status",
			status: nil,
			ready:  true,
		},
		{
			note:   "empty status",
			status: map[string]*plugins.Status{},
			ready:  true,
		},
		{
			note: "discovery not ready - bundle missing",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateNotReady},
			},
			ready: false,
		},
		{
			note: "discovery ok - bundle missing",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateOK},
			},
			ready: true, // bundles aren't enabled, only discovery plugin configured
		},
		{
			note: "discovery missing - bundle not ready",
			status: map[string]*plugins.Status{
				"bundle": {State: plugins.StateNotReady},
			},
			ready: false,
		},
		{
			note: "discovery missing - bundle ok",
			status: map[string]*plugins.Status{
				"bundle": {State: plugins.StateOK},
			},
			ready: true, // discovery isn't enabled, only bundle plugin configured
		},
		{
			note: "discovery not ready - bundle not ready",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateNotReady},
				"bundle":    {State: plugins.StateNotReady},
			},
			ready: false,
		},
		{
			note: "discovery ok - bundle not ready",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateOK},
				"bundle":    {State: plugins.StateNotReady},
			},
			ready: false,
		},
		{
			note: "discovery not ready - bundle ok",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateNotReady},
				"bundle":    {State: plugins.StateOK},
			},
			ready: false,
		},
		{
			note: "discovery ok - bundle ok",
			status: map[string]*plugins.Status{
				"discovery": {State: plugins.StateOK},
				"bundle":    {State: plugins.StateOK},
			},
			ready: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			f := newFixture(t)

			actual := f.server.bundlesReady(tc.status)
			if actual != tc.ready {
				t.Errorf("Expected %t got %t", tc.ready, actual)
			}
		})
	}
}

func TestUnversionedGetHealthCheckDiscoveryWithPlugins(t *testing.T) {
	t.Parallel()

	// Use the same server through the cases, the status updates apply incrementally to it.
	f := newFixture(t)

	cases := []struct {
		note          string
		statusUpdates map[string]*plugins.Status
		exp           int
		expBody       string
	}{
		{
			note:          "no plugins configured",
			statusUpdates: nil,
			exp:           200,
			expBody:       `{}`,
		},
		{
			note: "one plugin configured - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "one plugin configured - ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "one plugin configured - error state",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateErr},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "one plugin configured - recovered from error",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "add second plugin - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "add third plugin - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateNotReady},
				"p3": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "mixed states - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateErr},
				"p3": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "mixed states - still not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateErr},
				"p3": {State: plugins.StateOK},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "all plugins ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateOK},
				"p3": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "one plugins fails",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateErr},
				"p2": {State: plugins.StateOK},
				"p3": {State: plugins.StateOK},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "all plugins ready - recovery",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateOK},
				"p3": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "nil plugin status",
			statusUpdates: map[string]*plugins.Status{
				"p1": nil,
			},
			exp:     200,
			expBody: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			for name, status := range tc.statusUpdates {
				f.server.manager.UpdatePluginStatus(name, status)
			}

			req := newReqUnversioned(http.MethodGet, "/health?plugins", "")
			validateDiagnosticRequest(t, f, req, tc.exp, tc.expBody)
		})
	}
}

func TestUnversionedGetHealthCheckDiscoveryWithPluginsAndExclude(t *testing.T) {
	t.Parallel()

	// Use the same server through the cases, the status updates apply incrementally to it.
	f := newFixture(t)

	cases := []struct {
		note          string
		statusUpdates map[string]*plugins.Status
		exp           int
		expBody       string
	}{
		{
			note:          "no plugins configured",
			statusUpdates: nil,
			exp:           200,
			expBody:       `{}`,
		},
		{
			note: "one plugin configured - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "one plugin configured - ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "one plugin configured - error state",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateErr},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "one plugin configured - recovered from error",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "add excluded plugin - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateNotReady},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "add another excluded plugin - not ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateNotReady},
				"p3": {State: plugins.StateNotReady},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "excluded plugin - error",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateErr},
				"p3": {State: plugins.StateErr},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "first plugin - error",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateErr},
				"p2": {State: plugins.StateErr},
				"p3": {State: plugins.StateErr},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "all plugins ready",
			statusUpdates: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
				"p2": {State: plugins.StateOK},
				"p3": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			for name, status := range tc.statusUpdates {
				f.server.manager.UpdatePluginStatus(name, status)
			}

			req := newReqUnversioned(http.MethodGet, "/health?plugins&exclude-plugin=p2&exclude-plugin=p3", "")
			validateDiagnosticRequest(t, f, req, tc.exp, tc.expBody)
		})
	}
}

func TestUnversionedGetHealthCheckBundleAndPlugins(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note     string
		statuses map[string]*plugins.Status
		exp      int
		expBody  string
	}{
		{
			note:     "no plugins configured",
			statuses: nil,
			exp:      200,
			expBody:  `{}`,
		},
		{
			note: "only bundle plugin configured - not ready",
			statuses: map[string]*plugins.Status{
				"bundle": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more bundles are not activated"}`,
		},
		{
			note: "only bundle plugin configured - ok",
			statuses: map[string]*plugins.Status{
				"bundle": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "only custom plugin configured - not ready",
			statuses: map[string]*plugins.Status{
				"p1": {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "only custom plugin configured - ok",
			statuses: map[string]*plugins.Status{
				"p1": {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
		{
			note: "both configured - bundle not ready",
			statuses: map[string]*plugins.Status{
				"bundle": {State: plugins.StateNotReady},
				"p1":     {State: plugins.StateOK},
			},
			exp:     500,
			expBody: `{"error": "one or more bundles are not activated"}`,
		},
		{
			note: "both configured - custom plugin not ready",
			statuses: map[string]*plugins.Status{
				"bundle": {State: plugins.StateOK},
				"p1":     {State: plugins.StateNotReady},
			},
			exp:     500,
			expBody: `{"error": "one or more plugins are not up"}`,
		},
		{
			note: "both configured - both ready",
			statuses: map[string]*plugins.Status{
				"bundle": {State: plugins.StateOK},
				"p1":     {State: plugins.StateOK},
			},
			exp:     200,
			expBody: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			f := newFixture(t)

			for name, status := range tc.statuses {
				f.server.manager.UpdatePluginStatus(name, status)
			}

			req := newReqUnversioned(http.MethodGet, "/health?plugins&bundles", "")
			validateDiagnosticRequest(t, f, req, tc.exp, tc.expBody)
		})
	}
}

func TestUnversionedGetHealthWithPolicyMissing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, req, 500, `{"error":"health check (data.system.health.live) was undefined"}`)
}

func TestUnversionedGetHealthWithPolicyUpdates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	healthPolicy := `package system.health

  live := true
  `

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(healthPolicy)); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	f := newFixtureWithStore(t, store)
	req := newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, req, 200, `{}`)

	// update health policy to set live to false
	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	healthPolicy = `package system.health

  live := false
  `

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(healthPolicy)); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	req = newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, req, 500, `{"error": "health check (data.system.health.live) returned unexpected value"}`)
}

func TestUnversionedGetHealthWithPolicyUsingPlugins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	healthPolicy := `package system.health
  import rego.v1

  default live = false

  live if {
    input.plugin_state.bundle == "OK"
  }

  default ready = false

  ready if {
    input.plugins_ready
  }
  `

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(healthPolicy)); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	// plugins start out as not ready
	f := newFixtureWithStore(t, store)
	f.server.manager.UpdatePluginStatus("discovery", &plugins.Status{State: plugins.StateNotReady})
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateNotReady})

	// make sure live and ready are failing, as expected
	liveReq := newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, liveReq, 500, `{"error": "health check (data.system.health.live) returned unexpected value"}`)

	readyReq := newReqUnversioned(http.MethodGet, "/health/ready", "")
	validateDiagnosticRequest(t, f, readyReq, 500, `{"error": "health check (data.system.health.ready) returned unexpected value"}`)

	// all plugins are reporting OK
	f.server.manager.UpdatePluginStatus("discovery", &plugins.Status{State: plugins.StateOK})
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateOK})

	// make sure live and ready are now passing, as expected
	liveReq = newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, liveReq, 200, `{}`)

	readyReq = newReqUnversioned(http.MethodGet, "/health/ready", "")
	validateDiagnosticRequest(t, f, readyReq, 200, `{}`)

	// bundle is now not ready again
	f.server.manager.UpdatePluginStatus("bundle", &plugins.Status{State: plugins.StateNotReady})

	// the live rule should fail, but the ready rule should still succeed, because plugins_ready stays true once set
	liveReq = newReqUnversioned(http.MethodGet, "/health/live", "")
	validateDiagnosticRequest(t, f, liveReq, 500, `{"error": "health check (data.system.health.live) returned unexpected value"}`)

	readyReq = newReqUnversioned(http.MethodGet, "/health/ready", "")
	validateDiagnosticRequest(t, f, readyReq, 200, `{}`)
}

func TestDataV0(t *testing.T) {
	t.Parallel()

	testMod1 := `package test
	import rego.v1

	p = "hello"

	q = {
		"foo": [1,2,3,4]
	} if {
		input.flag = true
	}
	`
	pretty := `{
          "p": "hello",
          "q": {
            "foo": [
              1,
              2,
              3,
              4
            ]
          }
        }`

	f := newFixture(t)

	if err := f.v1(http.MethodPut, "/policies/test", testMod1, 200, ""); err != nil {
		t.Fatalf("Unexpected error while creating policy: %v", err)
	}

	if err := f.v0(http.MethodPost, "/data/test/p", "", 200, `"hello"`); err != nil {
		t.Fatalf("Expected response hello but got: %v", err)
	}

	if err := f.v0(http.MethodPost, "/data/test/q/foo", `{"flag": true}`, 200, `[1,2,3,4]`); err != nil {
		t.Fatalf("Expected response [1,2,3,4] but got: %v", err)
	}

	if err := f.v0(http.MethodPost, "/data/test?pretty=true", `{"flag": true}`, 200, pretty); err != nil {
		t.Fatalf("Expected response %v but got: %v", pretty, err)
	}

	req := newReqV0(http.MethodPost, "/data/test/q", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 404 {
		t.Fatalf("Expected HTTP 404 but got: %v", f.recorder)
	}

	var resp types.ErrorV1
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("Unexpected error while deserializing response: %v", err)
	}

	if resp.Code != types.CodeUndefinedDocument {
		t.Fatalf("Expected undefiend code but got: %v", resp)
	}
}

// Tests that the responses for (theoretically) valid resources but with forbidden methods return the proper status code
func Test405StatusCodev1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note string
		reqs []tr
	}{
		{"v1 data one level 405", []tr{
			{http.MethodHead, "/data/lvl1", "", 405, ""},
			{http.MethodConnect, "/data/lvl1", "", 405, ""},
			{http.MethodOptions, "/data/lvl1", "", 405, ""},
			{http.MethodTrace, "/data/lvl1", "", 405, ""},
		}},
		{"v1 data 405", []tr{
			{http.MethodHead, "/data", "", 405, ""},
			{http.MethodConnect, "/data", "", 405, ""},
			{http.MethodOptions, "/data", "", 405, ""},
			{http.MethodTrace, "/data", "", 405, ""},
			{http.MethodDelete, "/data", "", 405, ""},
		}},
		{"v1 policies 405", []tr{
			{http.MethodHead, "/policies", "", 405, ""},
			{http.MethodConnect, "/policies", "", 405, ""},
			{http.MethodDelete, "/policies", "", 405, ""},
			{http.MethodOptions, "/policies", "", 405, ""},
			{http.MethodTrace, "/policies", "", 405, ""},
			{http.MethodPost, "/policies", "", 405, ""},
			{http.MethodPut, "/policies", "", 405, ""},
			{http.MethodPatch, "/policies", "", 405, ""},
		}},
		{"v1 policies one level 405", []tr{
			{http.MethodHead, "/policies/lvl1", "", 405, ""},
			{http.MethodConnect, "/policies/lvl1", "", 405, ""},
			{http.MethodOptions, "/policies/lvl1", "", 405, ""},
			{http.MethodTrace, "/policies/lvl1", "", 405, ""},
			{http.MethodPost, "/policies/lvl1", "", 405, ""},
		}},
		{"v1 query one level 405", []tr{
			{http.MethodHead, "/query/lvl1", "", 405, ""},
			{http.MethodConnect, "/query/lvl1", "", 405, ""},
			{http.MethodDelete, "/query/lvl1", "", 405, ""},
			{http.MethodOptions, "/query/lvl1", "", 405, ""},
			{http.MethodTrace, "/query/lvl1", "", 405, ""},
			{http.MethodPost, "/query/lvl1", "", 405, ""},
			{http.MethodPut, "/query/lvl1", "", 405, ""},
			{http.MethodPatch, "/query/lvl1", "", 405, ""},
		}},
		{"v1 query 405", []tr{
			{http.MethodHead, "/query", "", 405, ""},
			{http.MethodConnect, "/query", "", 405, ""},
			{http.MethodDelete, "/query", "", 405, ""},
			{http.MethodOptions, "/query", "", 405, ""},
			{http.MethodTrace, "/query", "", 405, ""},
			{http.MethodPut, "/query", "", 405, ""},
			{http.MethodPatch, "/query", "", 405, ""},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			executeRequests(t, tc.reqs)
		})
	}
}

// Tests that the responses for (theoretically) valid resources but with forbidden methods return the proper status code
func Test405StatusCodev0(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note string
		reqs []tr
	}{
		{"v0 data one levels 405", []tr{
			{http.MethodHead, "/data/lvl2", "", 405, ""},
			{http.MethodConnect, "/data/lvl2", "", 405, ""},
			{http.MethodDelete, "/data/lvl2", "", 405, ""},
			{http.MethodOptions, "/data/lvl2", "", 405, ""},
			{http.MethodTrace, "/data/lvl2", "", 405, ""},
			{http.MethodGet, "/data/lvl2", "", 405, ""},
			{http.MethodPatch, "/data/lvl2", "", 405, ""},
			{http.MethodPut, "/data/lvl2", "", 405, ""},
		}},
		{"v0 data 405", []tr{
			{http.MethodHead, "/data", "", 405, ""},
			{http.MethodConnect, "/data", "", 405, ""},
			{http.MethodDelete, "/data", "", 405, ""},
			{http.MethodOptions, "/data", "", 405, ""},
			{http.MethodTrace, "/data", "", 405, ""},
			{http.MethodGet, "/data", "", 405, ""},
			{http.MethodPatch, "/data", "", 405, ""},
			{http.MethodPut, "/data", "", 405, ""},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			executeRequestsv0(t, tc.reqs)
		})
	}
}

func TestCompileV1(t *testing.T) {
	t.Parallel()

	v0mod := `package test

	p {
		input.x = 1
	}

	q {
		data.a[i] = input.x
	}

	default r = true

	r { input.x = 1 }

	custom_func(x) { data.a[i] == x }

	s { custom_func(input.x) }
	`

	v1mod := `package test

	p if {
		input.x = 1
	}

	q if {
		data.a[i] = input.x
	}

	default r = true

	r if { input.x = 1 }

	custom_func(x) if { data.a[i] == x }

	s if { custom_func(input.x) }
	`

	v0v1mod := `package test
	import rego.v1

	p if {
		input.x = 1
	}

	q if {
		data.a[i] = input.x
	}

	default r = true

	r if { input.x = 1 }

	custom_func(x) if { data.a[i] == x }

	s if { custom_func(input.x) }
	`

	expQuery := func(s string) string {
		return fmt.Sprintf(`{"result": {"queries": [%v]}}`, string(util.MustMarshalJSON(ast.MustParseBody(s))))
	}

	expError := func(s string) string {
		return fmt.Sprintf(`{
          "code": "invalid_parameter",
          "errors": [
            %s
          ],
          "message": "error(s) occurred while compiling module(s)"
        }`, s)
	}

	expQueryAndSupport := func(q string, m string, rv ast.RegoVersion) string {
		opts := ast.ParserOptions{RegoVersion: rv}
		return fmt.Sprintf(`{"result": {"queries": [%v], "support": [%v]}}`,
			string(util.MustMarshalJSON(ast.MustParseBodyWithOpts(q, opts))),
			string(util.MustMarshalJSON(ast.MustParseModuleWithOpts(m, opts))))
	}

	tests := []struct {
		note        string
		trs         []tr
		regoVersion ast.RegoVersion
	}{
		{
			note: "v1 keyword in query",
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"unknowns": ["input"],
					"query": "42 in input.x"
				}`, 200, expQuery("42 in input.x")},
			},
		},
		{
			note:        "v1 keyword in query (v0 rego-version)",
			regoVersion: ast.RegoV0,
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"unknowns": ["input"],
					"query": "42 in input.x"
				}`, 400, expError(fmt.Sprintf(`{
					"code": "rego_unsafe_var_error",
					"location": {
					"col": 4,
					"file": "",
					"row": 1
					},
					"message": "%s"
				}`, "var in is unsafe (hint: `import future.keywords.in` to import a future keyword)"))},
			},
		},
		{
			note: "basic",
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["input"],
					"query": "data.test.p = true"
				}`, 200, expQuery("input.x = 1")},
			},
		},
		{
			note: "subtree",
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"unknowns": ["input.x"],
					"input": {"y": 1},
					"query": "input.x > input.y"
				}`, 200, expQuery("input.x > 1")},
			},
		},
		{
			note: "data",
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["data.a"],
					"input": {
						"x": 1
					},
					"query": "data.test.q = true"
				}`, 200, expQuery("1 = data.a[i1]")},
			},
		},
		{
			note: "escaped string",
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"query": "input[\"x\"] = 1"
				}`, 200, expQuery("input.x = 1")},
			},
		},
		{
			note: "support",
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"query": "data.test.r = true"
				}`, 200, expQueryAndSupport(
					`data.partial.test.r = true`,
					`package partial.test

					r if { input.x = 1 }
					default r = true
					`,
					ast.DefaultRegoVersion)},
			},
		},
		{
			note:        "support (v0 rego-version)",
			regoVersion: ast.RegoV0,
			trs: []tr{
				{http.MethodPut, "/policies/test", v0mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"query": "data.test.r = true"
				}`, 200, expQueryAndSupport(
					`data.partial.test.r = true`,
					`package partial.test

					r { input.x = 1 }
					default r = true
					`,
					ast.RegoV0)},
			},
		},
		{
			note:        "support (v1 rego-version)",
			regoVersion: ast.RegoV1,
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"query": "data.test.r = true"
				}`, 200, expQueryAndSupport(
					`data.partial.test.r = true`,
					`package partial.test

					r if { input.x = 1 }
					default r = true
					`,
					ast.RegoV1)},
			},
		},
		{
			note:        "support (import rego.v1)",
			regoVersion: ast.RegoV0,
			trs: []tr{
				{http.MethodPut, "/policies/test", v0v1mod, 200, ""},
				// NOTE: v0 support rules don't get the rego.v1 import applied
				{http.MethodPost, "/compile", `{
					"query": "data.test.r = true"
				}`, 200, expQueryAndSupport(
					`data.partial.test.r = true`,
					`package partial.test

					r { input.x = 1 }
					default r = true
					`,
					ast.RegoV0)},
			},
		},
		{
			note: "function without disableInlining",
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["data.a"],
					"query": "data.test.s = true",
					"input": { "x": 1 }
				}`, 200, expQuery("data.a[i2] = 1")},
			},
		},
		{
			note: "function with disableInlining",
			trs: []tr{
				{http.MethodPut, "/policies/test", v1mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["data.a"],
					"query": "data.test.s = true",
					"options": { "disableInlining": ["data.test"] },
					"input": { "x": 1 }
				}`, 200, expQueryAndSupport(
					`data.partial.test.s = true`,
					`package partial.test

					s if { data.partial.test.custom_func(1) }
					custom_func(__local0__2) if { data.a[i2] = __local0__2 }
					`,
					ast.DefaultRegoVersion)},
			},
		},
		{
			note: "empty unknowns",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "input.x > 1", "unknowns": []}`, 200, `{"result": {}}`},
			},
		},
		{
			note: "never defined",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "1 = 2"}`, 200, `{"result": {}}`},
			},
		},
		{
			note: "always defined",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "1 = 1"}`, 200, `{"result": {"queries": [[]]}}`},
			},
		},
		{
			note: "error: bad request",
			trs:  []tr{{http.MethodPost, "/compile", `{"input": [{]}`, 400, ``}},
		},
		{
			note: "error: empty query",
			trs:  []tr{{http.MethodPost, "/compile", `{}`, 400, ""}},
		},
		{
			note: "error: bad query",
			trs:  []tr{{http.MethodPost, "/compile", `{"query": "x %!> 9"}`, 400, ""}},
		},
		{
			note: "error: bad unknown",
			trs:  []tr{{http.MethodPost, "/compile", `{"unknowns": ["input."], "query": "true"}`, 400, ""}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if tc.regoVersion != ast.RegoUndefined {
				executeRequests(t, tc.trs, variant{
					name: tc.regoVersion.String(),
					opts: []any{plugins.WithParserOptions(ast.ParserOptions{RegoVersion: tc.regoVersion})},
				})
			} else {
				executeRequests(t, tc.trs)
			}
		})
	}
}

func TestCompileV1Observability(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	test.WithTempFS(nil, func(root string) {
		disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close(ctx)
		f := newFixtureWithStore(t, disk)

		err = f.v1(http.MethodPut, "/policies/test", `package test
	import rego.v1

	p if { input.x = 1 }`, 200, "")
		if err != nil {
			t.Fatal(err)
		}

		compileReq := newReqV1(http.MethodPost, "/compile?metrics&explain=full", `{
		"query": "data.test.p = true"
	}`)

		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, compileReq)

		var response types.CompileResponseV1
		if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}

		if len(response.Explanation) == 0 {
			t.Fatal("Expected non-empty explanation")
		}

		assertMetricsExist(t, response.Metrics, []string{
			"timer_rego_partial_eval_ns",
			"timer_rego_query_compile_ns",
			"timer_rego_query_parse_ns",
			"timer_server_handler_ns",
			"counter_disk_read_keys",
			"timer_disk_read_ns",
		})
	})
}

func TestCompileV1UnsafeBuiltin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	query := `{"query": "http.send({\"method\": \"get\", \"url\": \"foo.com\"}, x)"}`
	expResp := `{
  "code": "invalid_parameter",
  "message": "error(s) occurred while compiling module(s)",
  "errors": [
    {
      "code": "rego_type_error",
      "message": "unsafe built-in function calls in expression: http.send",
      "location": {
        "file": "",
        "row": 1,
        "col": 1
      }
    }
  ]
}`

	if err := f.v1(http.MethodPost, `/compile`, query, 400, expResp); err != nil {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}
}

func TestDataV1Redirection(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// Testing redirect at the root level
	if err := f.v1(http.MethodPut, "/data/", `{"foo": [1,2,3]}`, 301, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	locHdr := f.recorder.Header().Get("Location")
	if strings.Compare(locHdr, "/v1/data") != 0 {
		t.Fatalf("Unexpected error Location header value: %v", locHdr)
	}
	RedirectedPath := strings.SplitAfter(locHdr, "/v1")[1]
	if err := f.v1(http.MethodPut, RedirectedPath, `{"foo": [1,2,3]}`, 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	if err := f.v1(http.MethodGet, RedirectedPath, "", 200, `{"result": {"foo": [1,2,3]}}`); err != nil {
		t.Fatalf("Unexpected error from GET: %v", err)
	}
	// Now we test redirection a few levels down
	if err := f.v1(http.MethodPut, "/data/a/b/c/", `{"foo": [1,2,3]}`, 301, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	locHdrLv := f.recorder.Header().Get("Location")
	if strings.Compare(locHdrLv, "/v1/data/a/b/c") != 0 {
		t.Fatalf("Unexpected error Location header value: %v", locHdrLv)
	}
	RedirectedPathLvl := strings.SplitAfter(locHdrLv, "/v1")[1]
	if err := f.v1(http.MethodPut, RedirectedPathLvl, `{"foo": [1,2,3]}`, 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	if err := f.v1(http.MethodGet, RedirectedPathLvl, "", 200, `{"result": {"foo": [1,2,3]}}`); err != nil {
		t.Fatalf("Unexpected error from GET: %v", err)
	}
}

func TestDataV1(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testMod1 := `package testmod

import rego.v1
import input.req1
import input.req2 as reqx
import input.req3.attr1

p contains x if { q[x]; not r[x] }
q contains x if { data.x.y[i] = x }
r contains x if { data.x.z[i] = x }
g = true if { req1.a[0] = 1; reqx.b[i] = 1 }
h = true if { attr1[i] > 1 }
gt1 = true if { req1 > 1 }
arr = [1, 2, 3, 4] if { true }
undef = true if { false }`

	testMod2 := `package testmod
import rego.v1

p = [1, 2, 3, 4] if { true }
q = {"a": 1, "b": 2} if { true }`

	testMod4 := `package testmod
import rego.v1

p = true if { true }
p = false if { true }`

	testMod5 := `package testmod.empty.mod`
	testMod6 := `package testmod.all.undefined
import rego.v1

p = true if { false }`

	tests := []struct {
		note string
		reqs []tr
	}{
		{"add root", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {"a": 1}}]`, 204, ""},
			{http.MethodGet, "/data/x/a", "", 200, `{"result": 1}`},
		}},
		{"append array", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": []}]`, 204, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "-", "value": {"a": 1}}]`, 204, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "-", "value": {"a": 2}}]`, 204, ""},
			{http.MethodGet, "/data/x/0/a", "", 200, `{"result": 1}`},
			{http.MethodGet, "/data/x/1/a", "", 200, `{"result": 2}`},
		}},
		{"append array one-shot", []tr{
			{http.MethodPatch, "/data/x", `[
                {"op": "add", "path": "/", "value": []},
                {"op": "add", "path": "-", "value": {"a": 1}},
                {"op": "add", "path": "-", "value": {"a": 2}}
            ]`, 204, ""},
			{http.MethodGet, "/data/x/1/a", "", 200, `{"result": 2}`},
		}},
		{"insert array", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {
                "y": [
                    {"z": [1,2,3]},
                    {"z": [4,5,6]}
                ]
            }}]`, 204, ""},
			{http.MethodGet, "/data/x/y/1/z/2", "", 200, `{"result": 6}`},
			{http.MethodPatch, "/data/x/y/1", `[{"op": "add", "path": "/z/1", "value": 100}]`, 204, ""},
			{http.MethodGet, "/data/x/y/1/z", "", 200, `{"result": [4, 100, 5, 6]}`},
		}},
		{"patch root", []tr{
			{http.MethodPatch, "/data", `[
				{
					"op": "add",
					"path": "/",
					"value": {"a": 1, "b": 2}
				}
			]`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"a": 1, "b": 2}}`},
		}},
		{"patch root invalid", []tr{
			{http.MethodPatch, "/data", `[
				{
					"op": "add",
					"path": "/",
					"value": [1,2,3]
				}
			]`, 400, ""},
		}},
		{"patch invalid", []tr{
			{http.MethodPatch, "/data", `[
				{
					"op": "remove",
					"path": "/"
				}
			]`, 400, ""},
		}},
		{"patch abort", []tr{
			{http.MethodPatch, "/data", `[
				{"op": "add", "path": "/foo", "value": "hello"},
				{"op": "add", "path": "/bar", "value": "world"},
				{"op": "add", "path": "/foo/bad", "value": "deadbeef"}
			]`, 404, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {}}`},
		}},
		{"put root", []tr{
			{http.MethodPut, "/data", `{"foo": [1,2,3]}`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"foo": [1,2,3]}}`},
		}},
		{"put deep makedir", []tr{
			{http.MethodPut, "/data/a/b/c/d", `1`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 1}}`},
		}},
		{"put deep makedir partial", []tr{
			{http.MethodPut, "/data/a/b", `{}`, 204, ""},
			{http.MethodPut, "/data/a/b/c/d", `0`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 0}}`},
		}},
		{"put exists overwrite", []tr{
			{http.MethodPut, "/data/a/b/c", `"hello"`, 204, ""},
			{http.MethodPut, "/data/a/b", `"goodbye"`, 204, ""},
			{http.MethodGet, "/data/a", "", 200, `{"result": {"b": "goodbye"}}`},
		}},
		{"put base write conflict", []tr{
			{http.MethodPut, "/data/a/b", `[1,2,3,4]`, 204, ""},
			{http.MethodPut, "/data/a/b/c/d", "0", 404, `{
				"code": "resource_conflict",
				"message": "storage_write_conflict_error: /a/b"
			}`},
		}},
		{"put base/virtual conflict", []tr{
			{http.MethodPut, "/policies/testmod", "package x.y\np = 1\nq = 2", 200, ""},
			{http.MethodPut, "/data/x", `{"y": {"p": "xxx"}}`, 400, `{
              "code": "invalid_parameter",
              "message": "1 error occurred: testmod:2: rego_compile_error: conflicting rule for data path x/y/p found"
            }`},
			{http.MethodPut, "/data/x/y", `{"p": "xxx"}`, 400, ``},
			{http.MethodPut, "/data/x/y/p", `"xxx"`, 400, ``},
			{http.MethodPut, "/data/x/y/p/a", `1`, 400, ``},
			{http.MethodDelete, "/policies/testmod", "", 200, ""},
			{http.MethodPut, "/data/x/y/p/a", `1`, 204, ``},
			{http.MethodPut, "/policies/testmod", "package x.y\np = 1\nq = 2", 400, `{
              "code": "invalid_parameter",
              "message": "error(s) occurred while compiling module(s)",
              "errors": [
                {
                  "code": "rego_compile_error",
                  "message": "conflicting rule for data path x/y/p found",
                  "location": {
                    "file": "testmod",
                    "row": 2,
                    "col": 1
                  }
                }
              ]
            }`},
		}},
		{"get virtual", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {"y": [1,2,3,4], "z": [3,4,5,6]}}]`, 204, ""},
			{http.MethodGet, "/data/testmod/p", "", 200, `{"result": [1,2]}`},
		}},
		{"get with input", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/g?input=%7B%22req1%22%3A%7B%22a%22%3A%5B1%5D%7D%2C+%22req2%22%3A%7B%22b%22%3A%5B0%2C1%5D%7D%7D", "", 200, `{"result": true}`},
		}},
		{"get with input (missing input value)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/g?input=%7B%22req1%22%3A%7B%22a%22%3A%5B1%5D%7D%7D", "", 200, "{}"}, // req2 not specified
		}},
		{"get with input (namespaced)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/h?input=%7B%22req3%22%3A%7B%22attr1%22%3A%5B4%2C3%2C2%2C1%5D%7D%7D", "", 200, `{"result": true}`},
		}},
		{"get with input (root)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, `/data/testmod/gt1?input={"req1":2}`, "", 200, `{"result": true}`},
		}},
		{"get with input (bad format)", []tr{
			{http.MethodGet, "/data/deadbeef?input", "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: EOF"
			}`},
			{http.MethodGet, "/data/deadbeef?input=", "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: EOF"
			}`},
			{http.MethodGet, `/data/deadbeef?input="foo`, "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: unexpected EOF"
			}`},
		}},
		{"get with input (path error)", []tr{
			{http.MethodGet, `/data/deadbeef?input={"foo:1}`, "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: unexpected EOF"
			}`},
		}},
		{"get empty and undefined", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPut, "/policies/test2", testMod5, 200, ""},
			{http.MethodPut, "/policies/test3", testMod6, 200, ""},
			{http.MethodGet, "/data/testmod/undef", "", 200, "{}"},
			{http.MethodGet, "/data/doesnot/exist", "", 200, "{}"},
			{http.MethodGet, "/data/testmod/empty/mod", "", 200, `{
				"result": {}
			}`},
			{http.MethodGet, "/data/testmod/all/undefined", "", 200, `{
				"result": {}
			}`},
		}},
		{"get root", []tr{
			{http.MethodPut, "/policies/test", testMod2, 200, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"testmod": {"p": [1,2,3,4], "q": {"a":1, "b": 2}}, "x": [1,2,3,4]}}`},
		}},
		{"post root", []tr{
			{http.MethodPost, "/data", "", 200, `{
				"result": {},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
			{http.MethodPut, "/policies/test", testMod2, 200, ""},
			{http.MethodPost, "/data", "", 200, `{
				"result": {
					"testmod": {
						"p": [1,2,3,4],
						"q": {"b": 2, "a": 1}
					}
				},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
		}},
		{"post input", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPost, "/data/testmod/gt1", `{"input": {"req1": 2}}`, 200, `{"result": true}`},
		}},
		{"post malformed input", []tr{
			{http.MethodPost, "/data/deadbeef", `{"input": @}`, 400, `{
				"code": "invalid_parameter",
				"message": "body contains malformed input document: invalid character '@' looking for beginning of value"
			}`},
		}},
		{"post empty object", []tr{
			{http.MethodPost, "/data", `{}`, 200, `{
				"result": {},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
		}},
		{"evaluation conflict", []tr{
			{http.MethodPut, "/policies/test", testMod4, 200, ""},
			{http.MethodPost, "/data/testmod/p", "", 500, `{
    		  "code": "internal_error",
    		  "errors": [
    		    {
    		      "code": "eval_conflict_error",
    		      "location": {
    		        "col": 1,
    		        "file": "test",
    		        "row": 5
    		      },
    		      "message": "complete rules must not produce multiple outputs"
    		    }
    		  ],
    		  "message": "error(s) occurred while evaluating query"
    		}`},
		}},
		{"query wildcards omitted", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			{http.MethodGet, "/query?q=data.x[_]%20=%20x", "", 200, `{"result": [{"x": 1}, {"x": 2}, {"x": 3}, {"x": 4}]}`},
		}},
		{"query undefined", []tr{
			{http.MethodGet, "/query?q=a=1%3Bb=2%3Ba=b", "", 200, `{}`},
		}},
		{"query compiler error", []tr{
			{http.MethodGet, "/query?q=x", "", 400, ""},
			// Subsequent query should not fail.
			{http.MethodGet, "/query?q=x=1", "", 200, `{"result": [{"x": 1}]}`},
		}},
		{"delete and check", []tr{
			{http.MethodDelete, "/data/a/b", "", 404, ""},
			{http.MethodPut, "/data/a/b/c/d", `1`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 1}}`},
			{http.MethodDelete, "/data/a/b", "", 204, ""},
			{http.MethodGet, "/data/a/b/c/d", "", 200, `{}`},
			{http.MethodGet, "/data/a", "", 200, `{"result": {}}`},
			{http.MethodGet, "/data/a/b/c", "", 200, `{}`},
		}},
		{"escaped paths", []tr{
			{http.MethodPut, "/data/a%2Fb", `{"c/d": 1}`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"a/b": {"c/d": 1}}}`},
			{http.MethodGet, "/data/a%2Fb/c%2Fd", "", 200, `{"result": 1}`},
			{http.MethodGet, "/data/a/b", "", 200, `{}`},
			{http.MethodPost, "/data/a%2Fb/c%2Fd", "", 200, `{
				"result": 1,
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
			{http.MethodPost, "/data/a/b", "", 200, `{
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
			{http.MethodPatch, "/data/a%2Fb", `[{"op": "add", "path": "/e%2Ff", "value": 2}]`, 204, ""},
			{http.MethodPost, "/data", "", 200, `{
				"result": {
					"a/b": {
						"c/d": 1,
						"e/f": 2
					}
				},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
		}},
		{"strict-builtin-errors", []tr{
			{http.MethodPut, "/policies/test", `
				package test
				import rego.v1

				default p = false

				p if { 1/0 }
			`, 200, ""},
			{http.MethodGet, "/data/test/p", "", 200, `{"result": false}`},
			{http.MethodGet, "/data/test/p?strict-builtin-errors", "", 500, `{
				"code": "internal_error",
				"message": "error(s) occurred while evaluating query",
				"errors": [
				  {
					"code": "eval_builtin_error",
					"message": "div: divide by zero",
					"location": {
					  "file": "test",
					  "row": 7,
					  "col": 12
					}
				  }
				]
			  }`},
			{http.MethodPost, "/data/test/p", "", 200, `{
				"result": false,
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
			{http.MethodPost, "/data/test/p?strict-builtin-errors", "", 500, `{
				"code": "internal_error",
				"message": "error(s) occurred while evaluating query",
				"errors": [
				  {
					"code": "eval_builtin_error",
					"message": "div: divide by zero",
					"location": {
					  "file": "test",
					  "row": 7,
					  "col": 12
					}
				  }
				]
			  }`},
		}},
		{"post api usage warning", []tr{
			{http.MethodPost, "/data", "", 200, `{
				"result": {},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				}
			}`},
			{http.MethodPost, "/data", `{"input": {}}`, 200, `{"result": {}}`},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(nil, func(root string) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
				if err != nil {
					t.Fatal(err)
				}
				defer disk.Close(ctx)
				executeRequests(t, tc.reqs,
					variant{"inmem", nil},
					variant{"disk", []any{
						func(s *Server) {
							s.WithStore(disk)
						},
					}},
				)
			})
		})
	}
}

func TestDataV1Metrics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	test.WithTempFS(nil, func(root string) {
		disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close(ctx)

		f := newFixtureWithStore(t, disk)
		put := newReqV1(http.MethodPut, `/data?metrics`, `{"foo":"bar"}`)
		f.server.Handler.ServeHTTP(f.recorder, put)

		if f.recorder.Code != 200 {
			t.Fatalf("Expected success but got %v", f.recorder)
		}

		var result types.DataResponseV1
		err = util.UnmarshalJSON(f.recorder.Body.Bytes(), &result)
		if err != nil {
			t.Fatalf("Unexpected error while unmarshalling result: %v", err)
		}

		assertMetricsExist(t, result.Metrics, []string{
			"counter_disk_read_keys",
			"counter_disk_deleted_keys",
			"counter_disk_written_keys",
			"counter_disk_read_bytes",
			"timer_rego_input_parse_ns",
			"timer_server_handler_ns",
			"timer_disk_read_ns",
			"timer_disk_write_ns",
			"timer_disk_commit_ns",
		})
	})
}

func TestConfigV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	c := []byte(`{"services": {
			"acmecorp": {
				"url": "https://example.com/control-plane-api/v1",
				"credentials": {"bearer": {"token": "test"}}
			}
		},
		"labels": {
			"region": "west"
		},
		"keys": {
			"global_key": {
				"algorithm": HS256,
				"key": "secret"
			}
		}}`)

	conf, err := config.ParseConfig(c, "foo")
	if err != nil {
		t.Fatal(err)
	}

	f.server.manager.Config = conf

	expected := map[string]any{
		"result": map[string]any{
			"labels":                         map[string]any{"id": "foo", "version": version.Version, "region": "west"},
			"keys":                           map[string]any{"global_key": map[string]any{"algorithm": "HS256"}},
			"services":                       map[string]any{"acmecorp": map[string]any{"url": "https://example.com/control-plane-api/v1"}},
			"default_authorization_decision": "/system/authz/allow",
			"default_decision":               "/system/main",
		},
	}
	bs, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.v1(http.MethodGet, "/config", "", 200, string(bs)); err != nil {
		t.Fatal(err)
	}

	badServicesConfig := []byte(`{
		"services": {
			"acmecorp": ["foo"]
		}
	}`)

	conf, err = config.ParseConfig(badServicesConfig, "foo")
	if err != nil {
		t.Fatal(err)
	}

	f.server.manager.Config = conf

	if err := f.v1(http.MethodGet, "/config", "", 500, `{
				"code": "internal_error",
				"message": "type assertion error"}`); err != nil {
		t.Fatal(err)
	}
}

func TestDataYAML(t *testing.T) {
	t.Parallel()

	testMod1 := `package testmod
import rego.v1
import input.req1
gt1 = true if { req1 > 1 }`

	inputYaml1 := `
---
input:
  req1: 2`

	inputYaml2 := `
---
req1: 2`

	f := newFixture(t)

	if err := f.v1(http.MethodPut, "/policies/test", testMod1, 200, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	// First JSON and then later yaml to make sure both work
	if err := f.v1(http.MethodPost, "/data/testmod/gt1", `{"input": {"req1": 2}}`, 200, `{"result": true}`); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	req := newReqV1(http.MethodPost, "/data/testmod/gt1", inputYaml1)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `{"result": true}`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

	req = newReqV0(http.MethodPost, "/data/testmod/gt1", inputYaml2)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `true`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

	if err := f.v1(http.MethodPut, "/policies/test2", `package system
main = data.testmod.gt1`, 200, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	req = newReqUnversioned(http.MethodPost, "/", inputYaml2)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `true`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

}

func TestDataPutV1IfNoneMatch(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	if err := f.v1(http.MethodPut, "/data/a/b/c", "0", 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /data/a/b/c: %v", err)
	}
	req := newReqV1(http.MethodPut, "/data/a/b/c", "1")
	req.Header.Set("If-None-Match", "*")
	if err := f.executeRequest(req, 304, ""); err != nil {
		t.Fatalf("Unexpected error from PUT with If-None-Match=*: %v", err)
	}
}

// Ensure JSON payload is compressed with gzip.
func mustGZIPPayload(payload []byte) []byte {
	var compressedPayload bytes.Buffer
	gz := gzip.NewWriter(&compressedPayload)
	if _, err := gz.Write(payload); err != nil {
		panic(fmt.Errorf("Error writing to gzip writer: %w", err))
	}
	if err := gz.Close(); err != nil {
		panic(fmt.Errorf("Error closing gzip writer: %w", err))
	}
	return compressedPayload.Bytes()
}

// generateJSONBenchmarkData returns a map of `k` keys and `v` key/value pairs.
// Taken from topdown/topdown_bench_test.go
func generateJSONBenchmarkData(k, v int) map[string]any {
	// create array of null values that can be iterated over
	keys := make([]any, k)
	for i := range keys {
		keys[i] = nil
	}

	// create large JSON object value (100,000 entries is about 2MB on disk)
	values := map[string]any{}
	for i := range v {
		values[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	return map[string]any{
		"input": map[string]any{
			"keys":   keys,
			"values": values,
		},
	}
}

// Ref: https://github.com/open-policy-agent/opa/issues/6804
func TestDataGetV1CompressedRequestWithAuthorizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note                  string
		payload               []byte
		forcePayloadSizeField uint32 // Size to manually set the payload field for the gzip blob.
		expRespHTTPStatus     int
		expErrorMsg           string
	}{
		{
			note:              "empty message",
			payload:           mustGZIPPayload([]byte{}),
			expRespHTTPStatus: 401,
		},
		{
			note:              "empty object",
			payload:           mustGZIPPayload([]byte(`{}`)),
			expRespHTTPStatus: 401,
		},
		{
			note:              "basic authz - fail",
			payload:           mustGZIPPayload([]byte(`{"user": "bob"}`)),
			expRespHTTPStatus: 401,
		},
		{
			note:              "basic authz - pass",
			payload:           mustGZIPPayload([]byte(`{"user": "alice"}`)),
			expRespHTTPStatus: 200,
		},
		{
			note:                  "basic authz - malicious size field",
			payload:               mustGZIPPayload([]byte(`{"user": "alice"}`)),
			expRespHTTPStatus:     400,
			forcePayloadSizeField: 134217728, // 128 MB
			expErrorMsg:           "gzip: invalid checksum",
		},
		{
			note:              "basic authz - huge zip",
			payload:           mustGZIPPayload(util.MustMarshalJSON(generateJSONBenchmarkData(100, 100))),
			expRespHTTPStatus: 401,
		},
	}

	for _, test := range tests {
		t.Run(test.note, func(t *testing.T) {
			ctx := context.Background()
			store := inmem.New()
			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
			authzPolicy := `package system.authz

import rego.v1

default allow := false # Reject requests by default.

allow if {
	# Logic to authorize request goes here.
	input.body.user == "alice"
}
`

			if err := store.UpsertPolicy(ctx, txn, "test", []byte(authzPolicy)); err != nil {
				panic(err)
			}

			if err := store.Commit(ctx, txn); err != nil {
				panic(err)
			}

			opts := [](func(*Server)){
				func(s *Server) {
					s.WithStore(store)
				},
				func(s *Server) {
					s.WithAuthorization(AuthorizationBasic)
				},
			}

			f := newFixtureWithConfig(t, fmt.Sprintf(`{"server":{"decision_logs": %t}}`, true), opts...)

			// Forcibly replace the size trailer field for the gzip blob.
			// Byte order is little-endian, field is a uint32.
			if test.forcePayloadSizeField != 0 {
				binary.LittleEndian.PutUint32(test.payload[len(test.payload)-4:], test.forcePayloadSizeField)
			}

			// execute the request
			req := newReqV1(http.MethodPost, "/data/test", string(test.payload))
			req.Header.Set("Content-Encoding", "gzip")
			f.reset()
			f.server.Handler.ServeHTTP(f.recorder, req)
			if f.recorder.Code != test.expRespHTTPStatus {
				t.Fatalf("Unexpected HTTP status code, (exp,got): %d, %d", test.expRespHTTPStatus, f.recorder.Code)
			}
			if test.expErrorMsg != "" {
				var serverErr types.ErrorV1
				if err := json.Unmarshal(f.recorder.Body.Bytes(), &serverErr); err != nil {
					t.Fatalf("Could not deserialize error message: %s", err.Error())
				}
				if serverErr.Message != test.expErrorMsg {
					t.Fatalf("Expected error message to have message '%s', got message: '%s'", test.expErrorMsg, serverErr.Message)
				}
			}
		})
	}
}

// Tests to ensure the body size limits work, for compressed requests.
func TestDataPostV1CompressedDecodingLimits(t *testing.T) {
	t.Parallel()

	defaultMaxLen := int64(1024)
	defaultGzipMaxLen := int64(1024)

	tests := []struct {
		note                  string
		wantGzip              bool
		wantChunkedEncoding   bool
		payload               []byte
		forceContentLen       int64  // Size to manually set the Content-Length header to.
		forcePayloadSizeField uint32 // Size to manually set the payload field for the gzip blob.
		expRespHTTPStatus     int
		expWarningMsg         string
		expErrorMsg           string
		maxLen                int64
		gzipMaxLen            int64
	}{
		{
			note:              "empty message",
			payload:           []byte{},
			expRespHTTPStatus: 200,
			expWarningMsg:     "'input' key missing from the request",
		},
		{
			note:              "empty message, gzip",
			wantGzip:          true,
			payload:           mustGZIPPayload([]byte{}),
			expRespHTTPStatus: 200,
			expWarningMsg:     "'input' key missing from the request",
		},
		{
			note:              "empty message, malicious Content-Length",
			payload:           []byte{},
			forceContentLen:   2048, // Server should ignore this header entirely.
			expRespHTTPStatus: 400,
			expErrorMsg:       "request body too large",
		},
		{
			note:              "empty message, gzip, malicious Content-Length",
			wantGzip:          true,
			payload:           mustGZIPPayload([]byte{}),
			forceContentLen:   2048, // Server should ignore this header entirely.
			expRespHTTPStatus: 400,
			expErrorMsg:       "request body too large",
		},
		{
			note:                  "basic - malicious size field, expect reject on gzip payload length",
			wantGzip:              true,
			payload:               mustGZIPPayload([]byte(`{"input": {"user": "alice"}}`)),
			expRespHTTPStatus:     400,
			forcePayloadSizeField: 134217728, // 128 MB
			expErrorMsg:           "gzip payload too large",
			gzipMaxLen:            1024,
		},
		{
			note:                  "basic - malicious size field, expect reject on gzip payload length, chunked encoding",
			wantGzip:              true,
			wantChunkedEncoding:   true,
			payload:               mustGZIPPayload([]byte(`{"input": {"user": "alice"}}`)),
			expRespHTTPStatus:     400,
			forcePayloadSizeField: 134217728, // 128 MB
			expErrorMsg:           "gzip payload too large",
			gzipMaxLen:            1024,
		},
		{
			note:              "basic, large payload",
			payload:           util.MustMarshalJSON(generateJSONBenchmarkData(100, 100)),
			expRespHTTPStatus: 200,
			maxLen:            134217728,
		},
		{
			note:              "basic, large payload, expect reject on Content-Length",
			payload:           util.MustMarshalJSON(generateJSONBenchmarkData(100, 100)),
			expRespHTTPStatus: 400,
			maxLen:            512,
			expErrorMsg:       "request body too large",
		},
		{
			note:                "basic, large payload, expect reject on Content-Length, chunked encoding",
			wantChunkedEncoding: true,
			payload:             util.MustMarshalJSON(generateJSONBenchmarkData(100, 100)),
			expRespHTTPStatus:   200,
			maxLen:              134217728,
		},
		{
			note:              "basic, gzip, large payload",
			wantGzip:          true,
			payload:           mustGZIPPayload(util.MustMarshalJSON(generateJSONBenchmarkData(100, 100))),
			expRespHTTPStatus: 200,
			maxLen:            1024,
			gzipMaxLen:        134217728,
		},
		{
			note:              "basic, gzip, large payload, expect reject on gzip payload length",
			wantGzip:          true,
			payload:           mustGZIPPayload(util.MustMarshalJSON(generateJSONBenchmarkData(100, 100))),
			expRespHTTPStatus: 400,
			maxLen:            1024,
			gzipMaxLen:        10,
			expErrorMsg:       "gzip payload too large",
		},
	}

	for _, test := range tests {
		t.Run(test.note, func(t *testing.T) {
			ctx := context.Background()
			store := inmem.New()
			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
			examplePolicy := `package example.authz

import rego.v1

default allow := false # Reject requests by default.

allow if {
	# Logic to authorize request goes here.
	input.body.user == "alice"
}
`

			if err := store.UpsertPolicy(ctx, txn, "test", []byte(examplePolicy)); err != nil {
				panic(err)
			}

			if err := store.Commit(ctx, txn); err != nil {
				panic(err)
			}

			opts := [](func(*Server)){
				func(s *Server) {
					s.WithStore(store)
				},
			}

			// Set defaults for max_length configs, if not specified in the test case.
			if test.maxLen == 0 {
				test.maxLen = defaultMaxLen
			}
			if test.gzipMaxLen == 0 {
				test.gzipMaxLen = defaultGzipMaxLen
			}

			f := newFixtureWithConfig(t, fmt.Sprintf(`{"server":{"decision_logs": %t, "decoding":{"max_length": %d, "gzip": {"max_length": %d}}}}`, true, test.maxLen, test.gzipMaxLen), opts...)

			// Forcibly replace the size trailer field for the gzip blob.
			// Byte order is little-endian, field is a uint32.
			if test.forcePayloadSizeField != 0 {
				binary.LittleEndian.PutUint32(test.payload[len(test.payload)-4:], test.forcePayloadSizeField)
			}

			// execute the request
			req := newReqV1(http.MethodPost, "/data/test", string(test.payload))
			if test.wantGzip {
				req.Header.Set("Content-Encoding", "gzip")
			}
			if test.wantChunkedEncoding {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
				req.Header.Set("Transfer-Encoding", "chunked")
			}
			if test.forceContentLen > 0 {
				req.ContentLength = test.forceContentLen
				req.Header.Set("Content-Length", strconv.FormatInt(test.forceContentLen, 10))
			}
			f.reset()
			f.server.Handler.ServeHTTP(f.recorder, req)
			if f.recorder.Code != test.expRespHTTPStatus {
				t.Fatalf("Unexpected HTTP status code, (exp,got): %d, %d, response body: %s", test.expRespHTTPStatus, f.recorder.Code, f.recorder.Body.Bytes())
			}
			if test.expErrorMsg != "" {
				var serverErr types.ErrorV1
				if err := json.Unmarshal(f.recorder.Body.Bytes(), &serverErr); err != nil {
					t.Fatalf("Could not deserialize error message: %s, message was: %s", err.Error(), f.recorder.Body.Bytes())
				}
				if !strings.Contains(serverErr.Message, test.expErrorMsg) {
					t.Fatalf("Expected error message to have message '%s', got message: '%s'", test.expErrorMsg, serverErr.Message)
				}
			} else {
				var resp types.DataResponseV1
				if err := json.Unmarshal(f.recorder.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Could not deserialize response: %s, message was: %s", err.Error(), f.recorder.Body.Bytes())
				}
				if test.expWarningMsg != "" {
					if !strings.Contains(resp.Warning.Message, test.expWarningMsg) {
						t.Fatalf("Expected warning message to have message '%s', got message: '%s'", test.expWarningMsg, resp.Warning.Message)
					}
				} else if resp.Warning != nil {
					// Error on unexpected warnings. Something is wrong.
					t.Fatalf("Unexpected warning: code: %s, message: %s", resp.Warning.Code, resp.Warning.Message)
				}
			}
		})
	}
}

func TestDataPostV0CompressedResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gzipMinLength      int
		compressedResponse bool
	}{
		{
			gzipMinLength:      3,
			compressedResponse: true,
		},
		{
			gzipMinLength:      1400,
			compressedResponse: false,
		},
	}

	for _, test := range tests {
		f := newFixtureWithConfig(t, fmt.Sprintf(`{"server":{"encoding":{"gzip":{"min_length": %d}}}}`, test.gzipMinLength))
		// create the policy
		err := f.v1(http.MethodPut, "/policies/test", `package opa.examples
import rego.v1
import input.example.flag
allow_request if { flag == true }
`, 200, "")
		if err != nil {
			t.Fatal(err)
		}

		// execute the request
		req := newReqV0(http.MethodPost, "/data/opa/examples/allow_request", `{"example": {"flag": true}}`)
		req.Header.Set("Accept-Encoding", "gzip")
		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, req)

		// check for content encoding
		expectedEncoding := "gzip"
		if !test.compressedResponse {
			expectedEncoding = ""
		}
		receivedEncodingHeaderValue := f.recorder.Header().Get("Content-Encoding")
		if receivedEncodingHeaderValue != expectedEncoding {
			t.Fatalf("Expected Content-Encoding %v but got: %v", expectedEncoding, receivedEncodingHeaderValue)
		}

		var plainOutput []byte
		if test.compressedResponse {
			// unzip the response
			gzReader, err := gzip.NewReader(f.recorder.Body)
			if err != nil {
				t.Fatalf("Unexpected gzip error: %v", err)
			}
			plainOutput, err = io.ReadAll(gzReader)
			if err != nil {
				t.Fatalf("Unexpected error on reading the response: %v", err)
			}
		} else {
			plainOutput = f.recorder.Body.Bytes()
		}

		expected := "true"
		result := strings.TrimSuffix(string(plainOutput), "\n")
		if plainOutput == nil || result != expected {
			t.Fatalf("Expected %v but got: %v", expected, result)
		}
	}
}

func TestDataPostV1CompressedResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gzipMinLength      int
		compressedResponse bool
	}{
		{
			gzipMinLength:      3,
			compressedResponse: true,
		},
		{
			gzipMinLength:      1400,
			compressedResponse: false,
		},
	}

	for _, test := range tests {
		f := newFixtureWithConfig(t, fmt.Sprintf(`{"server":{"encoding":{"gzip":{"min_length": %d}}}}`, test.gzipMinLength))
		// create the policy
		err := f.v1(http.MethodPut, "/policies/test", `package test
import rego.v1
default hello := false
hello if {
	input.message == "world"
}
`, 200, "")
		if err != nil {
			t.Fatal(err)
		}

		// execute the request
		req := newReqV1(http.MethodPost, "/data/test", `{"input": {"message": "world"}}`)
		req.Header.Set("Accept-Encoding", "gzip")
		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, req)

		var result types.DataResponseV1

		// check for content encoding
		expectedEncoding := "gzip"
		if !test.compressedResponse {
			expectedEncoding = ""
		}
		receivedEncodingHeaderValue := f.recorder.Header().Get("Content-Encoding")
		if receivedEncodingHeaderValue != expectedEncoding {
			t.Fatalf("Expected Content-Encoding %v but got: %v", expectedEncoding, receivedEncodingHeaderValue)
		}

		if test.compressedResponse {
			// unzip and unmarshall the response
			gzReader, err := gzip.NewReader(f.recorder.Body)
			if err != nil {
				t.Fatalf("Unexpected gzip error: %v", err)
			}
			if err := util.NewJSONDecoder(gzReader).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
		} else {
			// unmarshall the response
			if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
		}

		var expected any
		if err := util.UnmarshalJSON([]byte(`{"hello": true}`), &expected); err != nil {
			panic(err)
		}
		if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
			t.Fatalf("Expected %v but got: %v", expected, *result.Result)
		}
	}
}

func TestCompileV1CompressedResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gzipMinLength      int
		compressedResponse bool
	}{
		{
			gzipMinLength:      3,
			compressedResponse: true,
		},
		{
			gzipMinLength:      1400,
			compressedResponse: false,
		},
	}

	for _, test := range tests {
		f := newFixtureWithConfig(t, fmt.Sprintf(`{"server":{"encoding":{"gzip":{"min_length": %d}}}}`, test.gzipMinLength))

		// create the policy
		mod := `package test
	import rego.v1

	p if {
		input.x = 1
	}

	q if {
		data.a[i] = input.x
	}

	default r = true

	r if { input.x = 1 }

	custom_func(x) if { data.a[i] == x }

	s if { custom_func(input.x) }
	`
		err := f.v1(http.MethodPut, "/policies/test", mod, 200, "")
		if err != nil {
			t.Fatal(err)
		}

		// execute the request
		req := newReqV1(http.MethodPost, "/compile", `{"unknowns": ["input"], "query": "data.test.p = true"}`)
		req.Header.Set("Accept-Encoding", "gzip")
		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, req)

		var result types.CompileResponseV1

		// check for content encoding
		expectedEncoding := "gzip"
		if !test.compressedResponse {
			expectedEncoding = ""
		}
		receivedEncodingHeaderValue := f.recorder.Header().Get("Content-Encoding")
		if receivedEncodingHeaderValue != expectedEncoding {
			t.Fatalf("Expected Content-Encoding %v but got: %v", expectedEncoding, receivedEncodingHeaderValue)
		}

		if test.compressedResponse {
			// unzip and unmarshall the response
			gzReader, err := gzip.NewReader(f.recorder.Body)
			if err != nil {
				t.Fatalf("Unexpected gzip error: %v", err)
			}
			if err := util.NewJSONDecoder(gzReader).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
		} else {
			// unmarshall the response
			if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
		}

		var expected any
		expectedStr := fmt.Sprintf(`{"queries": [%v]}`, string(util.MustMarshalJSON(ast.MustParseBody("input.x = 1"))))
		if err := util.UnmarshalJSON([]byte(expectedStr), &expected); err != nil {
			panic(err)
		}

		if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
			t.Fatalf("Expected %v but got: %v", expected, *result.Result)
		}
	}
}

func TestDataPostV0CompressedRequest(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// create the policy
	err := f.v1(http.MethodPut, "/policies/test", `package opa.examples
import rego.v1
import input.example.flag
allow_request if { flag == true }
`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	// execute the request
	compressedBoy := zipString(`{"example": {"flag": true}}`)
	req := newStreamedReqV0(http.MethodPost, "/data/opa/examples/allow_request", bytes.NewReader(compressedBoy))
	req.Header.Set("Content-Encoding", "gzip")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	expected := "true"
	result := strings.TrimSuffix(f.recorder.Body.String(), "\n")
	if result != expected {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestDataPostV1CompressedRequest(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// create the policy
	err := f.v1(http.MethodPut, "/policies/test", `package test
import rego.v1
default hello := false
hello if {
	input.message == "world"
}
`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	// execute the request
	compressedBoy := zipString(`{"input": {"message": "world"}}`)
	req := newStreamedReqV1(http.MethodPost, "/data/test", bytes.NewReader(compressedBoy))
	req.Header.Set("Content-Encoding", "gzip")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	// unmarshall the response
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	var expected any
	if err := util.UnmarshalJSON([]byte(`{"hello": true}`), &expected); err != nil {
		panic(err)
	}
	if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, *result.Result)
	}
}

func TestCompileV1CompressedRequest(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// create the policy
	mod := `package test
	import rego.v1

	p if {
		input.x = 1
	}

	q if {
		data.a[i] = input.x
	}

	default r = true

	r if { input.x = 1 }

	custom_func(x) if { data.a[i] == x }

	s if { custom_func(input.x) }
	`
	err := f.v1(http.MethodPut, "/policies/test", mod, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	// execute the request
	compressedBoy := zipString(`{"unknowns": ["input"], "query": "data.test.p = true"}`)
	req := newStreamedReqV1(http.MethodPost, "/compile", bytes.NewReader(compressedBoy))
	req.Header.Set("Content-Encoding", "gzip")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.CompileResponseV1

	// unmarshall the response
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	var expected any
	expectedStr := fmt.Sprintf(`{"queries": [%v]}`, string(util.MustMarshalJSON(ast.MustParseBody("input.x = 1"))))
	if err := util.UnmarshalJSON([]byte(expectedStr), &expected); err != nil {
		panic(err)
	}

	if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, *result.Result)
	}
}

func TestBundleScope(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	test.WithTempFS(nil, func(root string) {
		disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close(ctx)

		for _, v := range []variant{
			{"inmem", nil},
			{"disk", []any{func(s *Server) { s.WithStore(disk) }}},
		} {
			t.Run(v.name, func(t *testing.T) {
				f := newFixture(t, v.opts...)

				txn := storage.NewTransactionOrDie(ctx, f.server.store, storage.WriteParams)

				if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle", bundle.Manifest{
					Revision: "AAAAA",
					Roots:    &[]string{"a/b/c", "x/y", "foobar"},
				}); err != nil {
					t.Fatal(err)
				}

				if err := f.server.store.UpsertPolicy(ctx, txn, "someid", []byte(`package x.y.z`)); err != nil {
					t.Fatal(err)
				}

				if err := f.server.store.Commit(ctx, txn); err != nil {
					t.Fatal(err)
				}

				cases := []tr{
					{
						method: "PUT",
						path:   "/data/a/b",
						body:   "1",
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a/b is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/data/a/b/c",
						body:   "1",
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a/b/c is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/data/a/b/c/d",
						body:   "1",
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a/b/c/d is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/data/a/b/d",
						body:   "1",
						code:   http.StatusNoContent,
					},
					{
						method: "PATCH",
						path:   "/data/a",
						body:   `[{"path": "/b/c", "op": "add", "value": 1}]`,
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a/b/c is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "DELETE",
						path:   "/data/a",
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/policies/test1",
						body:   `package a.b`,
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path a/b is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/policies/someid",
						body:   `package other.path`,
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path x/y/z is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "DELETE",
						path:   "/policies/someid",
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "path x/y/z is owned by bundle \"test-bundle\""}`,
					},
					{
						method: "PUT",
						path:   "/data/foo/bar",
						body:   "1",
						code:   http.StatusNoContent,
					},
					{
						method: "PUT",
						path:   "/data/foo",
						body:   "1",
						code:   http.StatusNoContent,
					},
					{
						method: "PUT",
						path:   "/data",
						body:   `{"a": "b"}`,
						code:   http.StatusBadRequest,
						resp:   `{"code": "invalid_parameter", "message": "can't write to document root with bundle roots configured"}`,
					},
				}

				if err := f.v1TestRequests(cases); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}

func TestBundleScopeMultiBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	f := newFixture(t)

	txn := storage.NewTransactionOrDie(ctx, f.server.store, storage.WriteParams)

	if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle1", bundle.Manifest{
		Revision: "AAAAA",
		Roots:    &[]string{"a/b/c", "x/y"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle2", bundle.Manifest{
		Revision: "AAAAA",
		Roots:    &[]string{"a/b/d"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle3", bundle.Manifest{
		Revision: "AAAAA",
		Roots:    &[]string{"a/b/e", "a/b/f"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.UpsertPolicy(ctx, txn, "someid", []byte(`package x.y.z`)); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	cases := []tr{
		{
			method: "PUT",
			path:   "/data/x/y",
			body:   "1",
			code:   http.StatusBadRequest,
			resp:   `{"code": "invalid_parameter", "message": "path x/y is owned by bundle \"test-bundle1\""}`,
		},
		{
			method: "PUT",
			path:   "/data/a/b/d",
			body:   "1",
			code:   http.StatusBadRequest,
			resp:   `{"code": "invalid_parameter", "message": "path a/b/d is owned by bundle \"test-bundle2\""}`,
		},
		{
			method: "PUT",
			path:   "/data/foo/bar",
			body:   "1",
			code:   http.StatusNoContent,
		},
	}

	if err := f.v1TestRequests(cases); err != nil {
		t.Fatal(err)
	}
}

func TestBundleNoRoots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	f := newFixture(t)

	txn := storage.NewTransactionOrDie(ctx, f.server.store, storage.WriteParams)

	if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle", bundle.Manifest{
		Revision: "AAAAA",
		// No Roots provided
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.UpsertPolicy(ctx, txn, "someid", []byte(`package x.y.z`)); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	cases := []tr{
		{
			method: "PUT",
			path:   "/data/a/b",
			body:   "1",
			code:   http.StatusBadRequest,
			resp:   `{"code": "invalid_parameter", "message": "all paths owned by bundle \"test-bundle\""}`,
		},
	}

	if err := f.v1TestRequests(cases); err != nil {
		t.Fatal(err)
	}

	txn = storage.NewTransactionOrDie(ctx, f.server.store, storage.WriteParams)

	if err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "test-bundle", bundle.Manifest{
		Revision: "AAAAA",
		// Roots provided but contains empty string
		Roots: &[]string{"", "does/not/matter"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.UpsertPolicy(ctx, txn, "someid", []byte(`package x.y.z`)); err != nil {
		t.Fatal(err)
	}

	if err := f.server.store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	cases = []tr{
		{
			method: "PUT",
			path:   "/data/a/b",
			body:   "1",
			code:   http.StatusBadRequest,
			resp:   `{"code": "invalid_parameter", "message": "all paths owned by bundle \"test-bundle\""}`,
		},
	}

	if err := f.v1TestRequests(cases); err != nil {
		t.Fatal(err)
	}
}

func TestDataUpdate(t *testing.T) {
	tests := []struct {
		note    string
		readAst bool
	}{
		{
			note: "read raw data",
		},
		{
			note:    "read ast data",
			readAst: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			f := newFixtureWithStore(t, inmem.NewWithOpts(inmem.OptReturnASTValuesOnRead(tc.readAst)))

			// PUT data

			putData := `{"a":1,"b":2, "c": 3}`
			err := f.v1(http.MethodPut, "/data/x", putData, 204, "")
			if err != nil {
				t.Fatal(err)
			}

			req := newReqV1(http.MethodGet, "/data/x", "")
			f.reset()
			f.server.Handler.ServeHTTP(f.recorder, req)

			var result types.DataResponseV1

			if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}

			var expected any
			if err := util.UnmarshalJSON([]byte(putData), &expected); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
			if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
				t.Fatalf("Expected %v but got: %v", expected, *result.Result)
			}

			// DELETE data

			if err := f.v1(http.MethodDelete, "/data/x/b", "", 204, ""); err != nil {
				t.Fatal("Unexpected error:", err)
			}

			req = newReqV1(http.MethodGet, "/data/x", "")
			f.reset()
			f.server.Handler.ServeHTTP(f.recorder, req)

			if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}

			if err := util.UnmarshalJSON([]byte(`{"a":1,"c": 3}`), &expected); err != nil {
				t.Fatalf("Unexpected JSON decode error: %v", err)
			}
			if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
				t.Fatalf("Expected %v but got: %v", expected, *result.Result)
			}
		})
	}
}

func TestDataGetExplainFull(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	err := f.v1(http.MethodPut, "/data/x", `{"a":1,"b":2}`, 204, "")
	if err != nil {
		t.Fatal(err)
	}

	req := newReqV1(http.MethodGet, "/data/x?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain := mustUnmarshalTrace(result.Explanation)
	nexpect := 5
	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d events but got %d", nexpect, len(explain))
	}

	exitEvent := -1
	for i := 0; i < len(explain) && exitEvent < 0; i++ {
		if explain[i].Op == "exit" {
			exitEvent = i
		}
	}
	if exitEvent < 0 {
		t.Fatalf("Expected one exit node but found none")
	}

	_, ok := explain[exitEvent].Node.(ast.Body)
	if !ok {
		t.Fatalf("Expected body for node but got: %v", explain[exitEvent].Node)
	}

	if len(explain[exitEvent].Locals) != 1 {
		t.Fatalf("Expected one binding but got: %v", explain[exitEvent].Locals)
	}

	req = newReqV1(http.MethodGet, "/data/deadbeef?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if f.recorder.Code != 200 {
		t.Fatalf("Expected status code to be 200 but got: %v", f.recorder.Code)
	}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain = mustUnmarshalTrace(result.Explanation)
	nexpect = 3
	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d events but got %d", nexpect, len(explain))
	}

	lastEvent := len(explain) - 1
	if explain[lastEvent].Op != "fail" {
		t.Fatalf("Expected last event to be 'fail' but got: %v", explain[lastEvent])
	}

	req = newReqV1(http.MethodGet, "/data/x?explain=full&pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	exp := []any{
		`query:1     Enter data.x = _`,
		`query:1     | Eval data.x = _`,
		`query:1     | Exit data.x = _`,
		`query:1     Redo data.x = _`,
		`query:1     | Redo data.x = _`}
	actual := util.MustUnmarshalJSON(result.Explanation).([]any)
	if !reflect.DeepEqual(actual, exp) {
		t.Fatalf(`Expected pretty explanation to be %v, got %v`, exp, actual)
	}
}

func TestDataPostWithActiveStoreWriteTxn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	err := f.v1(http.MethodPut, "/policies/test", `package test
import rego.v1

p = [1, 2, 3, 4] if { true }`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	// open write transaction on the store and execute a query.
	// Then check the query is processed
	ctx := context.Background()
	_ = storage.NewTransactionOrDie(ctx, f.server.store, storage.WriteParams)

	req := newReqV1(http.MethodPost, "/data/test/p", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	var expected any

	if err := util.UnmarshalJSON([]byte(`[1,2,3,4]`), &expected); err != nil {
		panic(err)
	}

	if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result.Result)
	}
}

func TestDataPostExplain(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	err := f.v1(http.MethodPut, "/policies/test", `package test
import rego.v1

p = [1, 2, 3, 4] if { true }`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	req := newReqV1(http.MethodPost, "/data/test/p?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain := mustUnmarshalTrace(result.Explanation)
	nexpect := 11

	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d events but got %d", nexpect, len(explain))
	}

	var expected any

	if err := util.UnmarshalJSON([]byte(`[1,2,3,4]`), &expected); err != nil {
		panic(err)
	}

	if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result.Result)
	}

}

func TestDataPostExplainNotes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	err := f.v1(http.MethodPut, "/policies/test", `
		package test
		import rego.v1

		p if {
			data.a[i] = x; x > 1
			trace(sprintf("found x = %d", [x]))
		}`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	err = f.v1(http.MethodPut, "/data/a", `[1,2,3]`, 204, "")
	if err != nil {
		t.Fatal(err)
	}
	f.reset()

	req := newReqV1(http.MethodPost, "/data/test/p?explain=notes", "")
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode err: %v", err)
	}

	var trace types.TraceV1Raw

	if err := trace.UnmarshalJSON(result.Explanation); err != nil {
		t.Fatal(err)
	}

	if len(trace) != 3 || trace[2].Op != "note" {
		t.Logf("Found %d events in trace", len(trace))
		for i := range trace {
			t.Logf("Event #%d: %v\n", i, trace[i])
		}
		t.Fatal("Unexpected trace")
	}
}

// Warning(philipc): This test modifies package variables in the version
// package, which means it cannot be run in parallel with other tests.
func TestDataProvenanceSingleBundle(t *testing.T) {
	f := newFixture(t)

	// Dummy up since we are not using ld...
	// Note:  No bundle 'revision'...
	version.Version = "0.10.7"
	version.Vcs = "ac23eb45"
	version.Timestamp = "today"
	version.Hostname = "foo.bar.com"

	// Initialize as if a bundle plugin is running
	bp := pluginBundle.New(&pluginBundle.Config{Name: "b1"}, f.server.manager)
	f.server.manager.Register(pluginBundle.Name, bp)

	req := newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance := &types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}

	ctx := context.Background()

	// Update bundle revision and request again
	err := storage.Txn(ctx, f.server.store, storage.WriteParams, func(txn storage.Transaction) error {
		return bundle.LegacyWriteManifestToStore(ctx, f.server.store, txn, bundle.Manifest{Revision: "r1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	req = newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance.Revision = "r1"
	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}
}

// Warning(philipc): This test modifies package variables in the version
// package, which means it cannot be run in parallel with other tests.
func TestDataProvenanceSingleFileBundle(t *testing.T) {
	f := newFixture(t)

	// Dummy up since we are not using ld...
	// Note:  No bundle 'revision'...
	version.Version = "0.10.7"
	version.Vcs = "ac23eb45"
	version.Timestamp = "today"
	version.Hostname = "foo.bar.com"

	// No bundle plugin initialized, just a legacy revision set
	ctx := context.Background()

	err := storage.Txn(ctx, f.server.store, storage.WriteParams, func(txn storage.Transaction) error {
		return bundle.LegacyWriteManifestToStore(ctx, f.server.store, txn, bundle.Manifest{Revision: "r1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	req := newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result := types.DataResponseV1{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance := &types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
	}

	expectedProvenance.Revision = "r1"
	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}
}

// Warning(philipc): This test modifies package variables in the version
// package, which means it cannot be run in parallel with other tests.
func TestDataProvenanceMultiBundle(t *testing.T) {
	f := newFixture(t)

	// Dummy up since we are not using ld...
	version.Version = "0.10.7"
	version.Vcs = "ac23eb45"
	version.Timestamp = "today"
	version.Hostname = "foo.bar.com"

	// Initialize as if a bundle plugin is running with 2 bundles
	bp := pluginBundle.New(&pluginBundle.Config{Bundles: map[string]*pluginBundle.Source{
		"b1": {Service: "s1", Resource: "bundle.tar.gz"},
		"b2": {Service: "s2", Resource: "bundle.tar.gz"},
	}}, f.server.manager)
	f.server.manager.Register(pluginBundle.Name, bp)

	req := newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance := &types.ProvenanceV1{
		Version:   version.Version,
		Vcs:       version.Vcs,
		Timestamp: version.Timestamp,
		Hostname:  version.Hostname,
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}

	// Update bundle revision for a single bundle and make the request again
	ctx := context.Background()

	err := storage.Txn(ctx, f.server.store, storage.WriteParams, func(txn storage.Transaction) error {
		return bundle.WriteManifestToStore(ctx, f.server.store, txn, "b1", bundle.Manifest{Revision: "r1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	req = newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance.Bundles = map[string]types.ProvenanceBundleV1{
		"b1": {Revision: "r1"},
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}

	// Update both and check again
	err = storage.Txn(ctx, f.server.store, storage.WriteParams, func(txn storage.Transaction) error {
		err := bundle.WriteManifestToStore(ctx, f.server.store, txn, "b1", bundle.Manifest{Revision: "r2"})
		if err != nil {
			return err
		}
		return bundle.WriteManifestToStore(ctx, f.server.store, txn, "b2", bundle.Manifest{Revision: "r1"})
	})
	if err != nil {
		t.Fatal(err)
	}

	req = newReqV1(http.MethodPost, "/data?provenance", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if result.Provenance == nil {
		t.Fatalf("Expected non-nil provenance: %v", result.Provenance)
	}

	expectedProvenance.Bundles = map[string]types.ProvenanceBundleV1{
		"b1": {Revision: "r2"},
		"b2": {Revision: "r1"},
	}

	if !reflect.DeepEqual(result.Provenance, expectedProvenance) {
		t.Errorf("Unexpected provenance data: \n\n%+v\n\nExpected:\n%+v\n\n", result.Provenance, expectedProvenance)
	}
}

func TestDataMetricsEval(t *testing.T) {
	t.Parallel()

	// These tests all use the /v1/data API with ?metrics appended.
	// We're setting up the disk store because that injects a few extra metrics,
	// which storage/inmem does not.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	test.WithTempFS(nil, func(root string) {
		disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
		if err != nil {
			t.Fatal(err)
		}
		defer disk.Close(ctx)

		f := newFixtureWithStore(t, disk)

		// Make a request to evaluate `data`
		testDataMetrics(t, f, http.MethodPost, "/data?metrics", "", []string{
			"counter_server_query_cache_hit",
			"counter_disk_read_keys",
			"counter_disk_read_bytes",
			"timer_rego_input_parse_ns",
			"timer_rego_query_compile_ns",
			"timer_rego_query_eval_ns",
			"timer_server_handler_ns",
			"timer_disk_read_ns",
			"timer_rego_external_resolve_ns",
		})

		// Repeat previous request, expect to have hit the query cache
		// so fewer timers should have been reported.
		testDataMetrics(t, f, http.MethodPost, "/data?metrics", "", []string{
			"counter_server_query_cache_hit",
			"counter_disk_read_keys",
			"counter_disk_read_bytes",
			"timer_disk_read_ns",
			"timer_rego_external_resolve_ns",
			"timer_rego_input_parse_ns",
			"timer_rego_query_eval_ns",
			"timer_server_handler_ns",
		})

		// Exercise the PUT, PATCH, and DELETE endpoints.
		testDataMetrics(t, f, http.MethodPut, "/data/example?metrics", "{}", []string{
			"counter_disk_read_keys",
			"counter_disk_written_keys",
			"timer_disk_commit_ns",
			"timer_disk_read_ns",
			"timer_disk_write_ns",
			"timer_rego_input_parse_ns",
			"timer_server_handler_ns",
		})

		testDataMetrics(t, f, http.MethodPatch, "/data/example?metrics", "[]", []string{
			"timer_disk_commit_ns",
			"timer_rego_input_parse_ns",
			"timer_server_handler_ns",
		})

		testDataMetrics(t, f, http.MethodDelete, "/data/example?metrics", "{}", []string{
			"counter_disk_deleted_keys",
			"counter_disk_read_keys",
			"counter_disk_read_bytes",
			"timer_disk_commit_ns",
			"timer_disk_read_ns",
			"timer_disk_write_ns",
			"timer_server_handler_ns",
		})
	})
}

func testDataMetrics(t *testing.T, f *fixture, method string, url string, payload string, expected []string) {
	t.Helper()
	f.reset()
	req := newReqV1(method, url, payload)
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}
	assertMetricsExist(t, result.Metrics, expected)
}

func assertMetricsExist(t *testing.T, metrics types.MetricsV1, expected []string) {
	t.Helper()

	for _, key := range expected {
		v, ok := metrics[key]
		if !ok {
			t.Errorf("Missing expected metric: %s", key)
		} else if v == nil {
			t.Errorf("Expected non-nil value for metric: %s", key)
		}

	}

	if len(expected) != len(metrics) {
		t.Errorf("Expected %d metrics, got %d\n\n\tValues: %+v", len(expected), len(metrics), metrics)
	}
}

func TestV1Pretty(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	err := f.v1(http.MethodPatch, "/data/x", `[{"op": "add", "path":"/", "value": [1,2,3,4]}]`, 204, "")
	if err != nil {
		t.Fatal(err)
	}

	req := newReqV1(http.MethodGet, "/data/x?pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines := strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 9 {
		t.Errorf("Expected 8 lines in output but got %d:\n%v", len(lines), lines)
	}

	req = newReqV1(http.MethodGet, "/query?q=data.x[i]&pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines = strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 17 {
		t.Errorf("Expected 16 lines of output but got %d:\n%v", len(lines), lines)
	}
}

func TestPoliciesPutV1(t *testing.T) {
	t.Parallel()

	v0Module := `package a.b.c

import data.x.y as z
import data.p

q[x] { p[x]; not r[x] }
r[x] { z[x] = 4 }`

	v1Module := `package a.b.c

import data.x.y as z
import data.p

q contains x if { p[x]; not r[x] }
r contains x if { z[x] = 4 }`

	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		module      string
		expErrs     []string
	}{
		{
			note:        "v0 server, v0 module",
			regoVersion: ast.RegoV0,
			module:      v0Module,
		},
		{
			note:        "v0 server, v1 module",
			regoVersion: ast.RegoV0,
			module:      v1Module,
			expErrs:     []string{"var cannot be used for rule name"},
		},
		{
			note:        "v1 server, v1 module",
			regoVersion: ast.RegoV1,
			module:      v1Module,
		},
		{
			note:        "v1 server, v0 module",
			regoVersion: ast.RegoV1,
			module:      v0Module,
			expErrs: []string{
				"`if` keyword is required before rule body",
				"`contains` keyword is required for partial set rules",
			},
		},
	}

	for i, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			f := newFixture(t, plugins.WithParserOptions(ast.ParserOptions{
				RegoVersion: tc.regoVersion,
			}))
			req := newReqV1(http.MethodPut, fmt.Sprintf("/policies/%d", i), tc.module)

			f.server.Handler.ServeHTTP(f.recorder, req)

			var response map[string]any
			if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
				t.Fatalf("Unexpected error while unmarshalling response: %v", err)
			}

			if len(tc.expErrs) > 0 {
				if f.recorder.Code != 400 {
					t.Fatalf("Expected bad request but got %v", f.recorder)
				}

				var errs []string
				if errors, ok := response["errors"].([]any); ok {
					for _, err := range errors {
						errs = append(errs, err.(map[string]any)["message"].(string))
					}
				}

				for _, expErr := range tc.expErrs {
					found := false
					for _, err := range errs {
						if strings.Contains(err, expErr) {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("Expected error containing %q but got: %v", expErr, errs)
					}
				}
			} else {
				if f.recorder.Code != 200 {
					t.Fatalf("Expected success but got %v", f.recorder)
				}

				if len(response) != 0 {
					t.Fatalf("Expected empty wrapper object")
				}
			}
		})
	}
}

func TestPoliciesPutV1Empty(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}
}

func TestPoliciesPutV1ParseError(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/test", `
    package a.b.c

    p ;- true
    `)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	response := map[string]any{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if !reflect.DeepEqual(response["code"], types.CodeInvalidParameter) {
		t.Fatalf("Expected code %v but got: %v", types.CodeInvalidParameter, response)
	}

	v := ast.MustInterfaceToValue(response)

	name, err := v.Find(ast.MustParseRef("_.errors[0].location.file")[1:])
	if err != nil {
		t.Fatalf("Expecfted to find name in errors but: %v", err)
	}

	if name.Compare(ast.String("test")) != 0 {
		t.Fatalf("Expected name ot equal test but got: %v", name)
	}

	req = newReqV1(http.MethodPut, "/policies/test", ``)
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)
	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	req = newReqV1(http.MethodPut, "/policies/test", `
	package a.b.c

	p = true`)
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected ok but got %v", f.recorder)
	}

}

func TestPoliciesPutV1CompileError(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/test", `package a.b.c

p[x] { q[x] }
q[x] { p[x] }`,
	)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	response := map[string]any{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if !reflect.DeepEqual(response["code"], types.CodeInvalidParameter) {
		t.Fatalf("Expected code %v but got: %v", types.CodeInvalidParameter, response)
	}

	v := ast.MustInterfaceToValue(response)

	name, err := v.Find(ast.MustParseRef("_.errors[0].location.file")[1:])
	if err != nil {
		t.Fatalf("Expecfted to find name in errors but: %v", err)
	}

	if name.Compare(ast.String("test")) != 0 {
		t.Fatalf("Expected name ot equal test but got: %v", name)
	}
}

func TestPoliciesPutV1Noop(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	err := f.v1("PUT", "/policies/test?metrics", `package foo`, 200, "")
	if err != nil {
		t.Fatal(err)
	}
	f.reset()
	err = f.v1("PUT", "/policies/test?metrics", `package foo`, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	var resp types.PolicyPutResponseV1
	if err := json.NewDecoder(f.recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	exp := []string{"timer_server_read_bytes_ns"}

	// Sort the metric keys and compare to expected value. We're assuming the
	// server skips parsing if the bytes are equal.
	result := util.KeysSorted(resp.Metrics)

	if !reflect.DeepEqual(exp, result) {
		t.Fatalf("Expected %v but got %v", exp, result)
	}

	f.reset()

	// Ensure subsequent update with changed policy parses the body.
	err = f.v1("PUT", "/policies/test?metrics", "package foo\np = 1", 200, "")
	if err != nil {
		t.Fatal(err)
	}

	var resp2 types.PolicyPutResponseV1
	if err := json.NewDecoder(f.recorder.Body).Decode(&resp2); err != nil {
		t.Fatal(err)
	}

	if _, ok := resp2.Metrics["timer_rego_module_parse_ns"]; !ok {
		t.Fatalf("Expected parse module metric in response but got %v", resp2)
	}

}

func TestPoliciesListV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	putPolicy(t, f, testMod)

	expected := []types.PolicyV1{
		newPolicy("1", testMod),
	}

	assertListPolicy(t, f, expected)
}

func putPolicy(t *testing.T, f *fixture, mod string) {
	t.Helper()
	put := newReqV1(http.MethodPut, "/policies/1", mod)
	f.server.Handler.ServeHTTP(f.recorder, put)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}
	f.reset()
}

func assertListPolicy(t *testing.T, f *fixture, expected []types.PolicyV1) {
	t.Helper()

	list := newReqV1(http.MethodGet, "/policies", "")
	f.server.Handler.ServeHTTP(f.recorder, list)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	// var policies []*PolicyV1
	var response types.PolicyListResponseV1

	err := util.NewJSONDecoder(f.recorder.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Expected policy list but got error: %v with response body:\n\n%v\n", err, f.recorder)
	}

	if len(expected) != len(response.Result) {
		t.Fatalf("Expected %d policies but got: %v", len(expected), response.Result)
	}
	for i := range expected {
		if !expected[i].Equal(response.Result[i]) {
			t.Fatalf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%+v\n", expected, response.Result)
		}
	}

	f.reset()
}

func TestPoliciesGetV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	put := newReqV1(http.MethodPut, "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	f.reset()
	get := newReqV1(http.MethodGet, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var response types.PolicyGetResponseV1
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := newPolicy("1", testMod)

	if !expected.Equal(response.Result) {
		t.Errorf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected, response.Result)
	}
}

func TestPoliciesDeleteV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	put := newReqV1(http.MethodPut, "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	f.reset()
	del := newReqV1(http.MethodDelete, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, del)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var response map[string]any
	if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected unmarshal error: %v", err)
	}

	if len(response) > 0 {
		t.Fatalf("Expected empty response but got: %v", response)
	}

	f.reset()
	get := newReqV1(http.MethodGet, "/policies/1", "")
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found but got %v", f.recorder)
	}
}

func TestPoliciesPathSlashes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	if err := f.v1(http.MethodPut, "/policies/a/b/c.rego", testMod, 200, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := f.v1(http.MethodGet, "/policies/a/b/c.rego", testMod, 200, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestPoliciesUrlEncoded(t *testing.T) {
	t.Parallel()

	const expectedPolicyID = "/a policy/another-component"
	var urlEscapedPolicyID = url.PathEscape(expectedPolicyID)
	f := newFixture(t)

	// PUT policy with URL encoded ID
	put := newReqV1(http.MethodPut, "/policies/"+urlEscapedPolicyID, testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	// end PUT policy with URL encoded ID
	f.reset()
	// GET policy with URL encoded ID

	get := newReqV1(http.MethodGet, "/policies/"+urlEscapedPolicyID, "")
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}
	var getResponse types.PolicyGetResponseV1
	if err := json.NewDecoder(f.recorder.Body).Decode(&getResponse); err != nil {
		t.Fatalf("Unexpected unmarshal error: %v", err)
	}

	if getResponse.Result.ID != expectedPolicyID {
		t.Fatalf(`Expected policy ID to be "%s" but got "%s"`, expectedPolicyID, getResponse.Result.ID)
	}

	// end GET policy with URL encoded ID
	f.reset()
	// DELETE policy with URL encoded ID

	deleteRequest := newReqV1(http.MethodDelete, "/policies/"+urlEscapedPolicyID, "")
	f.server.Handler.ServeHTTP(f.recorder, deleteRequest)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}
}

func TestStatusV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	// Expect HTTP 500 before status plugin is registered
	req := newReqV1(http.MethodGet, "/status", "")
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusInternalServerError {
		t.Fatal("expected internal error")
	}

	// Expect HTTP 200 after status plus is registered
	manual := plugins.TriggerManual
	bs := pluginStatus.New(&pluginStatus.Config{
		Trigger: &manual,
		PrometheusConfig: &pluginStatus.PrometheusConfig{
			Collectors: &pluginStatus.Collectors{
				BundleLoadDurationNanoseconds: &pluginStatus.BundleLoadDurationNanoseconds{
					Buckets: prom.ExponentialBuckets(1000, 2, 20),
				},
			},
		},
	}, f.server.manager)
	err := bs.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	f.server.manager.Register(pluginStatus.Name, bs)

	// Fetch the status info, wait for status plugin to be ok
	t0 := time.Now()
	ok := false
	for !ok && time.Since(t0) < time.Second {
		req = newReqV1(http.MethodGet, "/status", "")
		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, req)
		if f.recorder.Result().StatusCode != http.StatusOK {
			t.Fatal("expected ok")
		}

		var resp1 struct {
			Result struct {
				Plugins struct {
					Status struct {
						State string
					}
				}
			}
		}
		if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp1); err != nil {
			t.Fatal(err)
		}
		if resp1.Result.Plugins.Status.State == "OK" {
			ok = true
		} else {
			t.Log("expected plugin state for status to be 'OK' but got:", resp1)
		}
	}

	// Expect HTTP 200 and updated status after bundle update occurs
	bs.BulkUpdateBundleStatus(map[string]*pluginBundle.Status{
		"test": {
			Name:     "test",
			HTTPCode: "403",
		},
	})

	req = newReqV1(http.MethodGet, "/status", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("expected ok")
	}

	var resp2 struct {
		Result struct {
			Bundles struct {
				Test struct {
					Name     string
					HTTPCode json.Number `json:"http_code"`
				}
			}
		}
	}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp2); err != nil {
		t.Fatal(err)
	}
	if resp2.Result.Bundles.Test.Name != "test" {
		t.Fatal("expected bundle to exist in status response but got:", resp2)
	}
	if resp2.Result.Bundles.Test.HTTPCode != "403" {
		t.Fatal("expected HTTPCode to equal 403 but got:", resp2)
	}
}

func TestStatusV1MetricsWithSystemAuthzPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Add the authz policy
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	authzPolicy := `package system.authz
	import rego.v1

	default allow = false
	allow if {
		input.path = ["v1", "status"]
	}`

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(authzPolicy)); err != nil {
		t.Fatal(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	// Add Prometheus Registerer to be used by plugins
	inner := metrics.New()

	logger := func(logger logging.Logger) func(attrs map[string]any, f string, a ...any) {
		return func(attrs map[string]any, f string, a ...any) {
			logger.WithFields(attrs).Error(f, a...)
		}
	}(logging.NewNoOpLogger())

	prom := prometheus.New(inner, logger, []float64{1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1})
	serverOpts := []any{func(s *Server) { s.WithAuthorization(AuthorizationBasic) }, func(s *Server) { s.WithMetrics(prom) }}

	f := newFixtureWithStore(t, store, serverOpts...)

	// Expect HTTP 500 before status plugin is registered
	req := newReqV1(http.MethodGet, "/status", "")
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusInternalServerError {
		t.Fatal("expected internal error")
	}

	// Register Status plugin
	manual := plugins.TriggerManual
	bs := pluginStatus.New(&pluginStatus.Config{
		Trigger:    &manual,
		Prometheus: true,
		PrometheusConfig: &pluginStatus.PrometheusConfig{
			Collectors: &pluginStatus.Collectors{
				BundleLoadDurationNanoseconds: &pluginStatus.BundleLoadDurationNanoseconds{
					Buckets: []float64{1, 1000, 10_000, 1e8},
				},
			},
		},
	}, f.server.manager).WithMetrics(prom)
	err := bs.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	f.server.manager.Register(pluginStatus.Name, bs)

	// Fetch the status info, wait for status plugin to be ok
	t0 := time.Now()
	ok := false
	for !ok && time.Since(t0) < time.Second {
		req = newReqV1(http.MethodGet, "/status", "")
		f.reset()
		f.server.Handler.ServeHTTP(f.recorder, req)
		if f.recorder.Result().StatusCode != http.StatusOK {
			t.Fatal("expected ok")
		}

		var resp1 struct {
			Result struct {
				Plugins struct {
					Status struct {
						State string
					}
				}
			}
		}
		if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp1); err != nil {
			t.Fatal(err)
		}
		if resp1.Result.Plugins.Status.State == "OK" {
			ok = true
		} else {
			t.Log("expected plugin state for status to be 'OK' but got:", resp1)
		}
	}

	// Make requests that should get denied
	req = newReqV1(http.MethodGet, "/policies", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	req = newReqV1(http.MethodGet, "/data", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	// Check Prometheus status metrics in the Status API

	req = newReqV1(http.MethodGet, "/status", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Result().StatusCode != http.StatusOK {
		t.Fatal("expected ok")
	}

	var resp struct {
		Result struct {
			Plugins struct {
				Status struct {
					State string
				}
			}
			Metrics map[string]any
		}
	}
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	} else if resp.Result.Plugins.Status.State != "OK" {
		t.Fatal("expected plugin state for status to be 'OK' but got:", resp)
	}

	met, ok := resp.Result.Metrics["prometheus"]
	if !ok {
		t.Fatal("expected prometheus metrics to be present in status")
	}

	promMet, ok := met.(map[string]any)
	if !ok {
		t.Fatal("expected prometheus metrics to be a map")
	}

	httpMet, ok := promMet["http_request_duration_seconds"].(map[string]any)
	if !ok {
		t.Fatal("expected http_request_duration_seconds metric to be a map")
	}

	innerMet, ok := httpMet["metric"].([]any)
	if !ok {
		t.Fatal("expected http_request_duration_seconds histogram metric to be a list")
	}

	expected := []any{map[string]any{"name": "code", "value": "401"},
		map[string]any{"name": "handler", "value": "authz"},
		map[string]any{"name": "method", "value": "get"}}

	found := false
	for _, m := range innerMet {
		item, ok := m.(map[string]any)
		if ok {
			if reflect.DeepEqual(item["label"].([]any), expected) {
				found = true
				break
			}
		} else {
			t.Fatal("expected each http_request_duration_seconds histogram metric element to be a map")
		}
	}

	if !found {
		t.Fatalf("expected to find metrics %v but found no match", expected)
	}
}

func TestQueryPostBasic(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.server, _ = New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(f.server.store).
		WithManager(f.server.manager).
		Init(context.Background())

	setup := []tr{
		{http.MethodPost, "/query", `{"query": "a=data.k.x with data.k as {\"x\" : 7}"}`, 200, `{"result":[{"a":7}]}`},
		{http.MethodPost, "/query", `{"query": "input=x", "input": 7}`, 200, `{"result":[{"x":7}]}`},
		{http.MethodPost, "/query", `{"query": "input=x", "input": @}`, 400, ``},
	}

	for _, tr := range setup {
		req := newReqV1(tr.method, tr.path, tr.body)
		req.RemoteAddr = "testaddr"

		if err := f.executeRequest(req, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDecisionIDs(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	ids := []string{}
	ctr := 0

	f.server = f.server.WithDecisionLoggerWithErr(func(_ context.Context, info *Info) error {
		ids = append(ids, info.DecisionID)
		return nil
	}).WithDecisionIDFactory(func() string {
		ctr++
		return strconv.Itoa(ctr)
	})

	if err := f.v1("GET", "/data/undefined", "", 200, `{"decision_id": "1"}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("POST", "/data/undefined", "", 200, `{
		"decision_id": "2",
		"warning": {
			"code": "api_usage_warning",
			"message": "'input' key missing from the request"
		}
	}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("GET", "/data", "", 200, `{"decision_id": "3", "result": {}}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("POST", "/data", "", 200, `{
		"decision_id": "4",
		"result": {},
		"warning": {
			"code": "api_usage_warning",
			"message": "'input' key missing from the request"
		}
	}`); err != nil {
		t.Fatal(err)
	}

	exp := []string{"1", "2", "3", "4"}

	if !reflect.DeepEqual(ids, exp) {
		t.Fatalf("Expected %v but got %v", exp, ids)
	}
}

func TestDecisionLoggingWithHTTPRequestContext(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	decisions := []*Info{}

	var nextID int

	f.server = f.server.WithDecisionIDFactory(func() string {
		nextID++
		return strconv.Itoa(nextID)
	}).WithDecisionLoggerWithErr(func(_ context.Context, info *Info) error {
		decisions = append(decisions, info)
		return nil
	})

	req := newReqV1("POST", "/data/nonexistent", `{"input": {"foo": 1}}`)
	req.Header.Set("foo", "bar")
	req.Header.Set("foo2", "bar2")
	req.Header.Add("foo2", "bar3")

	httpRctx := logging.HTTPRequestContext{Header: req.Header.Clone()}

	req = req.WithContext(logging.WithHTTPRequestContext(req.Context(), &httpRctx))

	if err := f.executeRequest(req, http.StatusOK, `{"decision_id": "1"}`); err != nil {
		t.Fatal(err)
	}

	if len(decisions) != 1 {
		t.Fatalf("Expected exactly 1 decision but got: %d", len(decisions))
	}

	expHeaders := http.Header{}
	expHeaders.Set("foo", "bar")
	expHeaders.Add("foo2", "bar2")
	expHeaders.Add("foo2", "bar3")

	exp := logging.HTTPRequestContext{Header: expHeaders}

	if !reflect.DeepEqual(decisions[0].HTTPRequestContext, exp) {
		t.Fatalf("Expected HTTP request context %v but got: %v", exp, decisions[0].HTTPRequestContext)
	}
}

func TestDecisionLogging(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	decisions := []*Info{}

	var nextID int

	f.server = f.server.WithDecisionIDFactory(func() string {
		nextID++
		return strconv.Itoa(nextID)
	}).WithDecisionLoggerWithErr(func(_ context.Context, info *Info) error {
		if info.Path == "fail_closed/decision_logger_err" {
			return errors.New("some error")
		}
		decisions = append(decisions, info)
		return nil
	})

	reqs := []struct {
		raw      *http.Request
		v0       bool
		method   string
		path     string
		body     string
		code     int
		response string
	}{
		{
			method:   "PUT",
			path:     "/policies/test",
			body:     "package system\nmain=true",
			response: "{}",
		},
		{
			method: "POST",
			path:   "/data",
			response: `{
				"result": {},
				"warning": {
					"code": "api_usage_warning",
					"message": "'input' key missing from the request"
				},
				"decision_id": "1"
			}`,
		},
		{
			method:   "GET",
			path:     "/data",
			response: `{"result": {}, "decision_id": "2"}`,
		},
		{
			method:   "POST",
			path:     "/data/nonexistent",
			body:     `{"input": {"foo": 1}}`,
			response: `{"decision_id": "3"}`,
		},
		{
			method:   "POST",
			v0:       true,
			path:     "/data",
			response: `{}`,
		},
		{
			raw:      newReqUnversioned("POST", "/", ""),
			response: "true",
		},
		{
			method:   "GET",
			path:     "/query?q=data=x",
			response: `{"result": [{"x": {}}]}`,
		},
		{
			method:   "POST",
			path:     "/query",
			body:     `{"query": "data=x"}`,
			response: `{"result": [{"x": {}}]}`,
		},
		{
			method: "PUT",
			path:   "/policies/test2",
			body: `package foo
			import rego.v1
			p if { {k: v | k = ["a", "a"][_]; v = [1, 2][_]} }`,
			response: `{}`,
		},
		{
			method: "PUT",
			path:   "/policies/test",
			body: `package system
			import rego.v1
			main if { data.foo.p }`,
			response: `{}`,
		},
		{
			method: "POST",
			path:   "/data",
			code:   500,
		},
		{
			method: "GET",
			path:   "/data",
			code:   500,
		},
		{
			raw:  newReqUnversioned("POST", "/", ""),
			code: 500,
		},
		{
			method: "POST",
			path:   "/data/fail_closed/decision_logger_err",
			code:   500,
		},
		{
			method: "POST",
			v0:     true,
			path:   "/data/test",
			code:   404,
			response: `{
				"code": "undefined_document",
				"message": "document missing: data.test"
			  }`,
		},
	}

	for _, r := range reqs {
		code := r.code
		if code == 0 {
			code = http.StatusOK
		}
		if r.raw != nil {
			if err := f.executeRequest(r.raw, code, r.response); err != nil {
				t.Fatal(err)
			}
		} else if r.v0 {
			if err := f.v0(r.method, r.path, r.body, code, r.response); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := f.v1(r.method, r.path, r.body, code, r.response); err != nil {
				t.Fatal(err)
			}
		}
	}

	exp := []struct {
		input   string
		path    string
		query   string
		wantErr bool
	}{
		{path: ""},
		{path: ""},
		{path: "nonexistent", input: `{"foo": 1}`},
		{path: ""},
		{path: "system/main"},
		{query: "data = x"},
		{query: "data = x"},
		{path: "", wantErr: true},
		{path: "", wantErr: true},
		{path: "system/main", wantErr: true},
		{path: `test`, wantErr: true},
	}

	if len(decisions) != len(exp) {
		t.Fatalf("Expected exactly %d decisions but got: %d", len(exp), len(decisions))
	}

	for i, d := range decisions {
		if d.DecisionID == "" {
			t.Fatalf("Expected decision ID on decision %d but got: %v", i, d)
		}
		if d.Metrics.Timer(metrics.ServerHandler).Value() == 0 {
			t.Fatalf("Expected server handler timer to be started on decision %d but got %v", i, d)
		}
		if exp[i].path != d.Path || exp[i].query != d.Query {
			t.Fatalf("Unexpected path or query on %d, want: %v but got: %v", i, exp[i], d)
		}
		if exp[i].wantErr && d.Error == nil || !exp[i].wantErr && d.Error != nil {
			t.Fatalf("Unexpected error on %d, wantErr: %v, got: %v", i, exp[i].wantErr, d)
		}
		if exp[i].input != "" {
			input := util.MustUnmarshalJSON([]byte(exp[i].input))
			if d.Input == nil || !reflect.DeepEqual(input, *d.Input) {
				t.Fatalf("Unexpected input on %d, want: %v, but got: %v", i, exp[i], d)
			}
		}
	}

}

func TestDecisionLogErrorMessage(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	f.server.WithDecisionLoggerWithErr(func(context.Context, *Info) error {
		return errors.New("xxx")
	})

	if err := f.v1(http.MethodPost, "/data", "", 500, `{
		"code": "internal_error",
		"message": "decision_logs: xxx"
	}`); err != nil {
		t.Fatal(err)
	}
}

func TestQueryV1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		query       string
		expErr      bool
	}{
		{
			note:        "v0",
			regoVersion: ast.RegoV0,
			query:       "a=[1,2,3]%3Ba[i]=x",
		},
		{
			note:        "v0, v1 keywords in query",
			regoVersion: ast.RegoV0,
			query:       "a=[1,2,3]%3Bsome+i,+x+in+a",
			expErr:      true,
		},
		{
			note:        "v1",
			regoVersion: ast.RegoV1,
			query:       "a=[1,2,3]%3Bsome+i,+x+in+a",
		},
		{
			note:  "default rego-version", // v1
			query: "a=[1,2,3]%3Bsome+i,+x+in+a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			test.WithTempFS(nil, func(root string) {
				disk, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: root})
				if err != nil {
					t.Fatal(err)
				}
				defer disk.Close(ctx)

				var opts []any
				if tc.regoVersion != ast.RegoUndefined {
					opts = append(opts, plugins.WithParserOptions(ast.ParserOptions{RegoVersion: tc.regoVersion}))
				}

				f := newFixtureWithStore(t, disk, opts...)
				get := newReqV1(http.MethodGet, fmt.Sprintf(`/query?q=%s&metrics`, tc.query), "")
				f.server.Handler.ServeHTTP(f.recorder, get)

				if tc.expErr {
					if f.recorder.Code != 400 {
						t.Fatalf("Expected error but got %v", f.recorder)
					}
				} else {
					if f.recorder.Code != 200 {
						t.Fatalf("Expected success but got %v", f.recorder)
					}

					var expected types.QueryResponseV1
					err = util.UnmarshalJSON([]byte(`{
		"result": [{"a":[1,2,3],"i":0,"x":1},{"a":[1,2,3],"i":1,"x":2},{"a":[1,2,3],"i":2,"x":3}]
	}`), &expected)
					if err != nil {
						panic(err)
					}

					var result types.QueryResponseV1
					err = util.UnmarshalJSON(f.recorder.Body.Bytes(), &result)
					if err != nil {
						t.Fatalf("Unexpected error while unmarshalling result: %v", err)
					}

					assertMetricsExist(t, result.Metrics, []string{
						"counter_disk_read_keys",
						"timer_rego_query_compile_ns",
						"timer_rego_query_eval_ns",
						// "timer_server_handler_ns", // TODO(sr): we're not consistent about timing this?
						"timer_disk_read_ns",
					})

					result.Metrics = nil
					if !reflect.DeepEqual(result, expected) {
						t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", expected, result)
					}
				}
			})
		})
	}
}

func TestBadQueryV1(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	expectedErr := `{
  "code": "invalid_parameter",
  "message": "error(s) occurred while parsing query",
  "errors": [
    {
      "code": "rego_parse_error",
      "message": "illegal token",
      "location": {
        "file": "",
        "row": 1,
        "col": 1
      },
      "details": {
        "line": "^ -i",
        "idx": 0
      }
    }
  ]
}`

	if err := f.v1(http.MethodGet, `/query?q=^ -i`, "", 400, expectedErr); err != nil {
		recvErr := f.recorder.Body.String()
		t.Fatalf(`Expected %v but got: %v`, expectedErr, recvErr)
	}
}

func TestQueryV1UnsafeBuiltin(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	query := `/query?q=http.send({"method": "get", "url": "foo.com"}, x)`

	expected := `{
  "code": "invalid_parameter",
  "message": "error(s) occurred while compiling query",
  "errors": [
    {
      "code": "rego_type_error",
      "message": "unsafe built-in function calls in expression: http.send",
      "location": {
        "file": "",
        "row": 1,
        "col": 1
      }
    }
  ]
}`

	if err := f.v1(http.MethodGet, query, "", 400, expected); err != nil {
		t.Fatalf(`Expected %v but got: %v`, expected, f.recorder.Body.String())
	}
}

func TestUnversionedPost(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	post := func() *http.Request {
		return newReqUnversioned(http.MethodPost, "/", `
		{
			"foo": {
				"bar": [1,2,3]
			}
		}`)
	}

	f.server.Handler.ServeHTTP(f.recorder, post())

	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found before policy added but got %v", f.recorder)
	}

	expectedBody := `{
  "code": "undefined_document",
  "message": "document missing: data.system.main"
}
`
	if f.recorder.Body.String() != expectedBody {
		t.Errorf("Expected %s got %s", expectedBody, f.recorder.Body.String())
	}

	module := `
	package system.main
	import rego.v1

	agg = x if {
		sum(input.foo.bar, x)
	}
	`

	if err := f.v1("PUT", "/policies/test", module, 200, ""); err != nil {
		t.Fatal(err)
	}

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, post())

	expected := "{\"agg\":6}\n"
	if f.recorder.Code != 200 || f.recorder.Body.String() != expected {
		t.Fatalf(`Expected HTTP 200 / %v but got: %v`, expected, f.recorder)
	}

	module = `
	package system
	import rego.v1

	main if {
		input.foo == "bar"
	}
	`

	if err := f.v1("PUT", "/policies/test", module, 200, ""); err != nil {
		t.Fatal(err)
	}

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, func() *http.Request {
		return newReqUnversioned(http.MethodPost, "/", `{"input": {"foo": "bar"}}`)
	}())

	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found before policy added but got %v", f.recorder)
	}

	expectedBody = `{
  "code": "undefined_document",
  "message": "document undefined: data.system.main"
}
`
	if f.recorder.Body.String() != expectedBody {
		t.Errorf("Expected %s got %s", expectedBody, f.recorder.Body.String())
	}

	// update the default decision path
	s := "http/authz"
	f.server.manager.Config.DefaultDecision = &s

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, post())

	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found before policy added but got %v", f.recorder)
	}

	expectedBody = `{
  "code": "undefined_document",
  "message": "document missing: data.http.authz"
}
`
	if f.recorder.Body.String() != expectedBody {
		t.Fatalf("Expected %s got %s", expectedBody, f.recorder.Body.String())
	}

	module = `
	package http.authz
	import rego.v1

	agg = x if {
		sum(input.foo.bar, x)
	}
	`

	if err := f.v1("PUT", "/policies/test", module, 200, ""); err != nil {
		t.Fatal(err)
	}

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, post())

	expected = "{\"agg\":6}\n"
	if f.recorder.Code != 200 || f.recorder.Body.String() != expected {
		t.Fatalf(`Expected HTTP 200 / %v but got: %v`, expected, f.recorder)
	}
}

func TestQueryV1Explain(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	get := newReqV1(http.MethodGet, `/query?q=a=[1,2,3]%3Ba[i]=x&explain=debug`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected 200 but got: %v", f.recorder)
	}

	var result types.QueryResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	nexpect := 21
	explain := mustUnmarshalTrace(result.Explanation)
	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d trace events for full query but got %d", nexpect, len(explain))
	}
}

func TestAuthorization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.New()
	m, err := plugins.New([]byte{}, "test", store)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	authzPolicy := `package system.authz

		import rego.v1
		import input.identity

		default allow = false

		allow if {
			identity = "bob"
		}
		`

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(authzPolicy)); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	server, err := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(store).
		WithManager(m).
		WithAuthorization(AuthorizationBasic).
		Init(ctx)

	if err != nil {
		panic(err)
	}

	// Test that bob can do stuff.
	req1, err := http.NewRequest(http.MethodGet, "http://localhost:8182/health", nil)
	if err != nil {
		panic(err)
	}

	req1 = identifier.SetIdentity(req1, "bob")

	validateAuthorizedRequest(t, server, req1, http.StatusOK)

	// Test that alice can't do stuff.
	req2, err := http.NewRequest(http.MethodGet, "http://localhost:8182/health", nil)
	if err != nil {
		panic(err)
	}

	req2 = identifier.SetIdentity(req2, "alice")

	validateAuthorizedRequest(t, server, req2, http.StatusUnauthorized)

	// Reverse the policy.
	update := identifier.SetIdentity(newReqV1(http.MethodPut, "/policies/test", `
		package system.authz

		import rego.v1
		import input.identity

		default allow = false

		allow if {
			identity = "alice"
		}
	`), "bob")

	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, update)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected policy update to succeed but got: %v", recorder)
	}

	// Try alice again.
	server.Handler.ServeHTTP(recorder, req2)
	validateAuthorizedRequest(t, server, req2, http.StatusOK)

	// Try bob again.
	server.Handler.ServeHTTP(recorder, req1)
	validateAuthorizedRequest(t, server, req1, http.StatusUnauthorized)

	// Try to query for "data" as alice (allowed)
	req3, err := http.NewRequest(http.MethodPost, "http://localhost:8182/v1/data", bytes.NewBufferString(`{"input": {"foo": "bar"}}`))
	if err != nil {
		panic(err)
	}

	req3 = identifier.SetIdentity(req3, "alice")
	recorder = httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, req3)
	if recorder.Code != http.StatusOK {
		t.Fatal("expected successful response for data")
	}

	// Try to query for "data" as bob (denied)
	req4, err := http.NewRequest(http.MethodPost, "http://localhost:8182/v1/data", bytes.NewBufferString(`{"input": {"foo": "bar"}}`))
	if err != nil {
		panic(err)
	}

	req4 = identifier.SetIdentity(req4, "bob")
	recorder = httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, req4)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatal("expected unauthorized response for data")
	}
}

func TestAuthorizationUsesInterQueryCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := inmem.New()
	m, err := plugins.New([]byte{}, "test", store)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	var c uint64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddUint64(&c, 1)
		fmt.Fprintf(w, `{"count": %d}`, c)
	}))

	authzPolicy := fmt.Sprintf(`package system.authz
import rego.v1

default allow := false

allow if {
	resp := http.send({
		"method": "GET", "url": "%[1]s/foo",
		"force_cache": true,
		"force_json_decode": true,
		"force_cache_duration_seconds": 60,
	})

	resp.body.count == 1
}
`, ts.URL)
	t.Log(authzPolicy)

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(authzPolicy)); err != nil {
		t.Fatal(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		t.Fatal(err)
	}

	server, err := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(store).
		WithManager(m).
		WithAuthorization(AuthorizationBasic).
		Init(ctx)

	if err != nil {
		t.Fatal(err)
	}

	for range 5 {
		req1, err := http.NewRequest(http.MethodGet, "http://localhost:8182/health", nil)
		if err != nil {
			t.Fatal(err)
		}

		validateAuthorizedRequest(t, server, req1, http.StatusOK)
	}
}

func validateAuthorizedRequest(t *testing.T, s *Server, req *http.Request, exp int) {
	t.Helper()

	r := httptest.NewRecorder()

	// First check the main router
	s.Handler.ServeHTTP(r, req)
	if r.Code != exp {
		t.Errorf("(Default Handler) Expected %v but got: %v", exp, r)
	}

	r = httptest.NewRecorder()

	// Ensure that auth happens for the diagnostic handler as well
	s.DiagnosticHandler.ServeHTTP(r, req)
	if r.Code != exp {
		t.Errorf("(Diagnostic Handler) Expected %v but got: %v", exp, r)
	}
}

func TestServerUsesAuthorizerParsedBody(t *testing.T) {
	t.Parallel()

	// Construct a request w/ a different message body (this should never happen.)
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8182/v1/data/test/echo", bytes.NewBufferString(`{"foo": "bad"}`))
	if err != nil {
		t.Fatal(err)
	}

	// Set the authorizer's parsed input to the expected message body.
	ctx := authorizer.SetBodyOnContext(req.Context(), map[string]any{
		"input": map[string]any{
			"foo": "good",
		},
	})

	// Check that v1 reader function behaves correctly.
	inp, goInp, err := readInputPostV1(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	exp := ast.MustParseTerm(`{"foo": "good"}`)

	if exp.Value.Compare(inp) != 0 {
		t.Fatalf("expected %v but got %v", exp, inp)
	}

	if exp.Value.Compare(ast.MustInterfaceToValue(*goInp)) != 0 {
		t.Fatalf("expected %v but got %v", exp, *goInp)
	}

	// Check that v0 reader function behaves correctly.
	ctx = authorizer.SetBodyOnContext(req.Context(), map[string]any{
		"foo": "good",
	})

	inp, goInp, err = readInputV0(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	if exp.Value.Compare(inp) != 0 {
		t.Fatalf("expected %v but got %v", exp, inp)
	}

	if exp.Value.Compare(ast.MustInterfaceToValue(*goInp)) != 0 {
		t.Fatalf("expected %v but got %v", exp, *goInp)
	}
}

func TestServerReloadTrigger(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	store := f.server.store
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	if err := store.UpsertPolicy(ctx, txn, "test", []byte("package test\np = 1")); err != nil {
		panic(err)
	}
	if err := f.v1(http.MethodGet, "/data/test", "", 200, `{}`); err != nil {
		t.Fatalf("Unexpected error from server: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}
	if err := f.v1(http.MethodGet, "/data/test", "", 200, `{"result": {"p": 1}}`); err != nil {
		t.Fatalf("Unexpected error from server: %v", err)
	}
}

func TestServerClearsCompilerConflictCheck(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	store := f.server.store
	ctx := context.Background()

	// Make a new transaction
	params := storage.WriteParams
	params.Context = storage.NewContext()
	txn := storage.NewTransactionOrDie(ctx, store, params)

	// Fresh compiler we will swap on the manager
	c := ast.NewCompiler()

	// Add the policy we want to use
	c.Compile(map[string]*ast.Module{"test": ast.MustParseModule("package test\np=1")})
	if len(c.Errors) > 0 {
		t.Fatalf("Unexpected compile errors: %v", c.Errors)
	}

	// Add in a "bad" conflict check
	c = c.WithPathConflictsCheck(func(_ []string) (bool, error) {
		t.Fatal("Conflict check should not have been called")
		return false, nil
	})

	// Set the compiler on the transaction context and commit to trigger listeners
	plugins.SetCompilerOnContext(params.Context, c)

	if err := store.UpsertPolicy(ctx, txn, "test", []byte("package test\np = 1")); err != nil {
		panic(err)
	}
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	// internal helpers should now give the new compiler back
	if f.server.getCompiler() != c {
		t.Fatalf("Expected to get the updated compiler")
	}
}

type queryBindingErrStore struct {
	storage.WritesNotSupported
	storage.PolicyNotSupported
}

func (*queryBindingErrStore) Read(_ context.Context, _ storage.Transaction, _ storage.Path) (any, error) {
	return nil, errors.New("expected error")
}

func (*queryBindingErrStore) ListPolicies(_ context.Context, _ storage.Transaction) ([]string, error) {
	return nil, nil
}

func (queryBindingErrStore) NewTransaction(_ context.Context, _ ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (queryBindingErrStore) Commit(_ context.Context, _ storage.Transaction) error {
	return nil
}

func (queryBindingErrStore) Abort(_ context.Context, _ storage.Transaction) {

}

func (queryBindingErrStore) Truncate(context.Context, storage.Transaction, storage.TransactionParams, storage.Iterator) error {
	return nil
}

func (queryBindingErrStore) Register(context.Context, storage.Transaction, storage.TriggerConfig) (storage.TriggerHandle, error) {
	return nil, nil
}

func (queryBindingErrStore) Unregister(context.Context, storage.Transaction, string) {

}

func TestQueryBindingIterationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &queryBindingErrStore{}
	m, err := plugins.New([]byte{}, "test", mock)
	if err != nil {
		panic(err)
	}

	server, err := New().WithStore(mock).WithManager(m).WithAddresses([]string{":8182"}).Init(ctx)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()

	f := &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}

	get := newReqV1(http.MethodGet, `/query?q=a=data.foo.bar`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 500 {
		t.Fatalf("Expected 500 error due to unknown storage error but got: %v", f.recorder)
	}

	var resultErr types.ErrorV1

	if jsonErr := json.NewDecoder(f.recorder.Body).Decode(&resultErr); jsonErr != nil {
		t.Fatal(jsonErr)
	}

	if resultErr.Code != types.CodeInternal || resultErr.Message != "expected error" {
		t.Fatal("unexpected response:", resultErr)
	}
}

const (
	testMod = `package a.b.c

import rego.v1
import data.x.y as z
import data.p

q contains x if { p[x]; not r[x] }
r contains x if { z[x] = 4 }`
)

type fixture struct {
	server   *Server
	recorder *httptest.ResponseRecorder
	t        *testing.T
}

func newFixture(t *testing.T, opts ...any) *fixture {
	ctx := context.Background()
	server := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(inmem.New()) // potentially overridden via opts

	for _, opt := range opts {
		if opt, ok := opt.(func(*Server)); ok {
			opt(server)
		}
	}

	var mOpts []func(*plugins.Manager)
	for _, opt := range opts {
		if opt, ok := opt.(func(*plugins.Manager)); ok {
			mOpts = append(mOpts, opt)
		}
	}

	m, err := plugins.New([]byte{}, "test", server.store, mOpts...)
	if err != nil {
		t.Fatal(err)
	}
	server = server.WithManager(m)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	server, err = server.Init(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()

	return &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}
}

func newFixtureWithConfig(t *testing.T, config string, opts ...func(*Server)) *fixture {
	ctx := context.Background()
	server := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(inmem.New()) // potentially overridden via opts
	for _, opt := range opts {
		opt(server)
	}

	m, err := plugins.New([]byte(config), "test", server.store)
	if err != nil {
		t.Fatal(err)
	}
	server = server.WithManager(m)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	server, err = server.Init(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()

	return &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}
}

func newFixtureWithStore(t *testing.T, store storage.Store, opts ...any) *fixture {
	ctx := context.Background()

	var mOpts []func(*plugins.Manager)
	for _, opt := range opts {
		if opt, ok := opt.(func(*plugins.Manager)); ok {
			mOpts = append(mOpts, opt)
		}
	}

	m, err := plugins.New([]byte{}, "test", store, mOpts...)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	server := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(store).
		WithManager(m)

	for _, opt := range opts {
		if opt, ok := opt.(func(*Server)); ok {
			opt(server)
		}
	}

	server, err = server.Init(ctx)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()

	return &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}
}

func (f *fixture) v1TestRequests(trs []tr) error {
	for i, tr := range trs {
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			return fmt.Errorf("error on test request #%d: %w", i+1, err)
		}
	}
	return nil
}

func (f *fixture) v1(method string, path string, body string, code int, resp string) error {
	// All v1 API's should 404 for the diagnostic handler
	if err := f.executeDiagnosticRequest(newReqV1(method, path, body), 404, ""); err != nil {
		return err
	}

	return f.executeRequest(newReqV1(method, path, body), code, resp)
}

func (f *fixture) v0(method string, path string, body string, code int, resp string) error {
	// All v0 API's should 404 for the diagnostic handler
	if err := f.executeDiagnosticRequest(newReqV0(method, path, body), 404, ""); err != nil {
		return err
	}

	return f.executeRequest(newReqV0(method, path, body), code, resp)
}

func (f *fixture) executeRequestForHandler(h http.Handler, req *http.Request, code int, resp string) error {
	f.reset()
	h.ServeHTTP(f.recorder, req)
	if f.recorder.Code != code {
		return fmt.Errorf("Expected code %v from %v %v but got: %+v", code, req.Method, req.URL, f.recorder)
	}
	if resp != "" {
		body := f.recorder.Body.String()
		if resp == body {
			// Early return on exact match as we can avoid the cost of uunmarshalling
			// both the expected and actual response in that case. This is particularly
			// useful for benchmarks where you only want to measure server-sider handling.
			return nil
		}

		var result any
		if err := util.UnmarshalJSON(f.recorder.Body.Bytes(), &result); err != nil {
			return fmt.Errorf("Expected JSON response from %v %v but got: %v", req.Method, req.URL, f.recorder)
		}
		var expected any
		if err := util.UnmarshalJSON([]byte(resp), &expected); err != nil {
			panic(err)
		}
		if !reflect.DeepEqual(result, expected) {
			a, err := json.MarshalIndent(expected, "", "  ")
			if err != nil {
				panic(err)
			}
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				panic(err)
			}
			return fmt.Errorf("Expected JSON response from %v %v to equal:\n\n%s\n\nGot:\n\n%s", req.Method, req.URL, a, b)
		}
	}
	return nil
}

func (f *fixture) executeRequest(req *http.Request, code int, resp string) error {
	return f.executeRequestForHandler(f.server.Handler, req, code, resp)
}

func (f *fixture) executeDiagnosticRequest(req *http.Request, code int, resp string) error {
	return f.executeRequestForHandler(f.server.DiagnosticHandler, req, code, resp)
}

func (f *fixture) reset() {
	f.recorder = httptest.NewRecorder()
}

type variant struct {
	name string
	opts []any
}

func executeRequests(t *testing.T, reqs []tr, variants ...variant) {
	t.Helper()

	if len(variants) == 0 {
		f := newFixture(t)
		for i, req := range reqs {
			if err := f.v1(req.method, req.path, req.body, req.code, req.resp); err != nil {
				t.Errorf("Unexpected response on request %d: %v", i+1, err)
			}
		}
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := newFixture(t, v.opts...)
			for i, req := range reqs {
				if err := f.v1(req.method, req.path, req.body, req.code, req.resp); err != nil {
					t.Errorf("Unexpected response on request %d: %v", i+1, err)
				}
			}
		})
	}
}

// Runs through an array of test cases against the v0 REST API tree
func executeRequestsv0(t *testing.T, reqs []tr) {
	t.Helper()
	f := newFixture(t)
	for i, req := range reqs {
		if err := f.v0(req.method, req.path, req.body, req.code, req.resp); err != nil {
			t.Errorf("Unexpected response on request %d: %v", i+1, err)
		}
	}
}

func validateDiagnosticRequest(t *testing.T, f *fixture, req *http.Request, code int, resp string) {
	t.Helper()
	// diagnostic requests need to be available on both the normal handler and diagnostic handler
	if err := f.executeRequest(req, code, resp); err != nil {
		t.Errorf("Unexpected error for request %v: %s", req, err)
	}
	if err := f.executeDiagnosticRequest(req, code, resp); err != nil {
		t.Errorf("Unexpected error for request %v: %s", req, err)
	}
}

func newPolicy(id, s string) types.PolicyV1 {
	compiler := ast.NewCompiler()
	parsed := ast.MustParseModule(s)
	if compiler.Compile(map[string]*ast.Module{"": parsed}); compiler.Failed() {
		panic(compiler.Errors)
	}
	mod := compiler.Modules[""]
	return types.PolicyV1{ID: id, AST: mod, Raw: s}
}

func newReqV1(method string, path string, body string) *http.Request {
	return newReq(1, method, path, body)
}

func newReqV0(method string, path string, body string) *http.Request {
	return newReq(0, method, path, body)
}

func newReq(version int, method, path, body string) *http.Request {
	return newReqUnversioned(method, fmt.Sprintf("/v%d", version)+path, body)
}

func newReqUnversioned(method, path, body string) *http.Request {
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	return req
}

func newStreamedReqV0(method string, path string, body io.Reader) *http.Request {
	return newStreamedReq(0, method, path, body)
}

func newStreamedReqV1(method string, path string, body io.Reader) *http.Request {
	return newStreamedReq(1, method, path, body)
}

func newStreamedReq(version int, method string, path string, body io.Reader) *http.Request {
	return newStreamedReqUnversioned(method, fmt.Sprintf("/v%d", version)+path, body)
}

func newStreamedReqUnversioned(method string, path string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		panic(err)
	}
	return req
}

func mustUnmarshalTrace(t types.TraceV1) (trace types.TraceV1Raw) {
	if err := json.Unmarshal(t, &trace); err != nil {
		panic("not reached")
	}
	return trace
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	f := newFixture(t, func(s *Server) {
		s.WithDiagnosticAddresses([]string{":8443"})
	})
	loops, err := f.server.Listeners()
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	errc := make(chan error)
	for _, loop := range loops {
		go func(serverLoop func() error) {
			errc <- serverLoop()
		}(loop)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	err = f.server.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected error shutting down server: %s", err.Error())
	}
}

func TestShutdownError(t *testing.T) {
	t.Parallel()

	f := newFixture(t, func(s *Server) {
		s.WithDiagnosticAddresses([]string{":8443"})
	})

	errMsg := "failed to shutdown"

	// Add a mock httpListener to the server
	m := &mockHTTPListener{
		shutdownHook: func() error {
			return errors.New(errMsg)
		},
	}
	f.server.httpListeners = []httpListener{m}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	err := f.server.Shutdown(ctx)
	if err == nil {
		t.Error("expected an error shutting down server but err==nil")
	} else if !strings.Contains(err.Error(), errMsg) {
		t.Errorf("unexpected error shutting down server: %s", err.Error())
	}
}

func TestShutdownMultipleErrors(t *testing.T) {
	t.Parallel()

	f := newFixture(t, func(s *Server) {
		s.WithDiagnosticAddresses([]string{":8443"})
	})

	shutdownErrs := []error{errors.New("err1"), nil, errors.New("err3")}

	// Add mock httpListeners to the server
	for _, err := range shutdownErrs {
		m := &mockHTTPListener{}
		if err != nil {
			retVal := errors.New(err.Error())
			m.shutdownHook = func() error {
				return retVal
			}
		}
		f.server.httpListeners = append(f.server.httpListeners, m)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	err := f.server.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected an error shutting down server but err==nil")
	}

	for _, expectedErr := range shutdownErrs {
		if expectedErr != nil && !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Errorf("expected error message to contain '%s', full message: '%s'", expectedErr.Error(), err.Error())
		}
	}
}

func TestAddrsNoListeners(t *testing.T) {
	t.Parallel()

	s := New()
	a := s.Addrs()
	if len(a) != 0 {
		t.Errorf("expected an empty list of addresses, got: %+v", a)
	}
}

func TestAddrsWithEmptyListenAddr(t *testing.T) {
	t.Parallel()

	s := New()
	s.httpListeners = []httpListener{&mockHTTPListener{}}
	a := s.Addrs()
	if len(a) != 0 {
		t.Errorf("expected an empty list of addresses, got: %+v", a)
	}
}

func TestAddrsWithListenAddr(t *testing.T) {
	t.Parallel()

	s := New()
	s.httpListeners = []httpListener{&mockHTTPListener{addrs: ":8181"}}
	a := s.Addrs()
	if len(a) != 1 || a[0] != ":8181" {
		t.Errorf("expected only an ':8181' address, got: %+v", a)
	}
}

func TestAddrsWithMixedListenerAddr(t *testing.T) {
	t.Parallel()

	s := New()
	addrs := []string{":8181", "", "unix:///var/tmp/foo.sock"}
	expected := []string{":8181", "unix:///var/tmp/foo.sock"}

	s.httpListeners = []httpListener{}
	for _, addr := range addrs {
		s.httpListeners = append(s.httpListeners, &mockHTTPListener{addrs: addr, t: defaultListenerType})
	}

	a := s.Addrs()
	if len(a) != 2 {
		t.Errorf("expected 2 addresses, got: %+v", a)
	}

	for _, expectedAddr := range expected {
		if !slices.Contains(a, expectedAddr) {
			t.Errorf("expected %q in address list, got: %+v", expectedAddr, a)
		}
	}
}

func TestDiagnosticAddrsNoListeners(t *testing.T) {
	t.Parallel()

	s := New()
	a := s.DiagnosticAddrs()
	if len(a) != 0 {
		t.Errorf("expected an empty list of addresses, got: %+v", a)
	}
}

func TestDiagnosticAddrsWithEmptyListenAddr(t *testing.T) {
	t.Parallel()

	s := New()
	s.httpListeners = []httpListener{&mockHTTPListener{t: diagnosticListenerType}}
	a := s.DiagnosticAddrs()
	if len(a) != 0 {
		t.Errorf("expected an empty list of addresses, got: %+v", a)
	}
}

func TestDiagnosticAddrsWithListenAddr(t *testing.T) {
	t.Parallel()

	s := New()
	s.httpListeners = []httpListener{&mockHTTPListener{addrs: ":8181", t: diagnosticListenerType}}
	a := s.DiagnosticAddrs()
	if len(a) != 1 || a[0] != ":8181" {
		t.Errorf("expected only an ':8181' address, got: %+v", a)
	}
}

func TestDiagnosticAddrsWithMixedListenerAddr(t *testing.T) {
	t.Parallel()

	s := New()
	addrs := []string{":8181", "", "unix:///var/tmp/foo.sock"}
	expected := []string{":8181", "unix:///var/tmp/foo.sock"}

	s.httpListeners = []httpListener{}
	for _, addr := range addrs {
		s.httpListeners = append(s.httpListeners, &mockHTTPListener{addrs: addr, t: diagnosticListenerType})
	}

	a := s.DiagnosticAddrs()
	if len(a) != 2 {
		t.Errorf("expected 2 addresses, got: %+v", a)
	}

	for _, expectedAddr := range expected {
		if !slices.Contains(a, expectedAddr) {
			t.Errorf("expected %q in address list, got: %+v", expectedAddr, a)
		}
	}
}

func TestMixedAddrTypes(t *testing.T) {
	t.Parallel()

	s := New()

	s.httpListeners = []httpListener{}

	addrs := map[string]struct{}{"localhost:8181": {}, "localhost:1234": {}, "unix:///var/tmp/foo.sock": {}}
	for addr := range addrs {
		s.httpListeners = append(s.httpListeners, &mockHTTPListener{addrs: addr, t: defaultListenerType})
	}

	diagAddrs := map[string]struct{}{":8181": {}, "https://127.0.0.1": {}}
	for addr := range diagAddrs {
		s.httpListeners = append(s.httpListeners, &mockHTTPListener{addrs: addr, t: diagnosticListenerType})
	}

	actualAddrs := s.Addrs()
	if len(actualAddrs) != len(addrs) {
		t.Errorf("expected %d addresses, got: %+v", len(addrs), actualAddrs)
	}

	for _, addr := range actualAddrs {
		if _, ok := addrs[addr]; !ok {
			t.Errorf("Unexpected address %v", addr)
		}
	}

	actualDiagAddrs := s.DiagnosticAddrs()
	if len(actualDiagAddrs) != len(diagAddrs) {
		t.Errorf("expected %d addresses, got: %+v", len(diagAddrs), actualDiagAddrs)
	}

	for _, addr := range actualDiagAddrs {
		if _, ok := diagAddrs[addr]; !ok {
			t.Errorf("Unexpected diagnostic address %v", addr)
		}
	}
}

func TestCustomRoute(t *testing.T) {
	t.Parallel()

	router := http.NewServeMux()
	router.HandleFunc("GET /customEndpoint", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"myCustomResponse": true}`)) // ignore error
	})
	f := newFixture(t, func(server *Server) {
		server.WithRouter(router)
	})

	if err := f.v1(http.MethodGet, "/data", "", 200, `{"result":{}}`); err != nil {
		t.Fatalf("Unexpected response for default server route: %v", err)
	}
	r, err := http.NewRequest(http.MethodGet, "/customEndpoint", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := f.executeRequest(r, http.StatusOK, `{"myCustomResponse": true}`); err != nil {
		t.Fatalf("Request to custom endpoint failed: %s", err)
	}
}

func TestCustomRouteWithMetrics(t *testing.T) {
	t.Parallel()

	// Add Prometheus Registerer to be used by plugins
	inner := metrics.New()
	prom := prometheus.New(inner, nil, []float64{1})
	f := newFixture(t,
		func(m *plugins.Manager) {
			m.ExtraRoute("GET /v1/foo", "v1/foo", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, `{"foo": "bar"}`)
			})
		},
		func(s *Server) {
			s.WithMetrics(prom)
		})

	{ // existing APIs still work fine
		if err := f.v1(http.MethodGet, "/data", "", 200, `{"result":{}}`); err != nil {
			t.Fatalf("Unexpected response for default server route: %v", err)
		}
	}
	{ // new endpoint is wired up
		r, err := http.NewRequest(http.MethodGet, "/v1/foo", nil)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if err := f.executeRequest(r, http.StatusOK, `{"foo": "bar"}`); err != nil {
			t.Fatalf("Request to custom endpoint failed: %s", err)
		}
	}
	{ // metrics are recorded for special endpoint
		r, err := http.NewRequest(http.MethodGet, "/metrics", nil)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		f.reset()
		f.server.DiagnosticHandler.ServeHTTP(f.recorder, r)
		resp := f.recorder.Result()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		resp.Body.Close()

		for _, want := range []string{
			`http_request_duration_seconds_count{code="200",handler="v1/data",method="get"} 1`, // default http handler
			`http_request_duration_seconds_count{code="200",handler="v1/foo",method="get"} 1`,  // added handler
			`http_request_duration_seconds_bucket{code="200",handler="v1/foo",method="get",le="1"} 1`,
			`http_request_duration_seconds_bucket{code="200",handler="v1/foo",method="get",le="+Inf"} 1`,
		} {
			if !strings.Contains(string(body), want) {
				t.Errorf("expected response to contain metric %q, but it did not.\nBody:\n%s", want, string(body))
			}
		}
	}
}

func TestDiagnosticRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path      string
		should404 bool
	}{
		{"/health", false},
		{"/metrics", false},
		{"/debug/pprof/", true},
		{"/v0/data", true},
		{"/v0/data/foo", true},
		{"/v1/data/", true},
		{"/v1/data/foo", true},
		{"/v1/policies", true},
		{"/v1/policies/foo", true},
		{"/v1/query", true},
		{"/v1/compile", true},
		{"/", true},
	}

	f := newFixture(t, func(s *Server) {
		s.WithPprofEnabled(true)
		s.WithMetrics(new(mockMetricsProvider))
	})

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req, err := http.NewRequest("GET", tc.path, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			code := http.StatusOK
			if tc.should404 {
				code = http.StatusNotFound
			}
			f.reset()
			f.server.DiagnosticHandler.ServeHTTP(f.recorder, req)
			if f.recorder.Code != code {
				t.Errorf("Expected code %v from %v %v but got: %+v", code, req.Method, req.URL, f.recorder)
			}
		})
	}

}

func TestDistributedTracingEnabled(t *testing.T) {
	t.Parallel()

	c := []byte(`{"distributed_tracing": {
		"type": "grpc"
		}}`)

	ctx := context.Background()
	_, _, _, err := distributedtracing.Init(ctx, c, "foo")
	if err != nil {
		t.Fatalf("Unexpected error initializing gRPC trace exporter %v", err)
	}

	c = []byte(`{"distributed_tracing": {
		"type": "http"
		}}`)

	_, _, _, err = distributedtracing.Init(ctx, c, "foo")
	if err != nil {
		t.Fatalf("Unexpected error initializing HTTP trace exporter %v", err)
	}
}

func TestDistributedTracingResourceAttributes(t *testing.T) {
	t.Parallel()

	attributes := map[attribute.Key]string{
		semconv.DeploymentEnvironmentKey: "prod",
		semconv.ServiceNameKey:           "my-service",
		semconv.ServiceVersionKey:        "1.0",
		semconv.ServiceNamespaceKey:      "my-namespace",
		semconv.ServiceInstanceIDKey:     "1",
	}

	c := fmt.Appendf(nil, `{"distributed_tracing": {
		"type": "grpc",
		"service_name": "%s",
		"resource": {
			"service_namespace": "%s",
			"service_version": "%s",
			"service_instance_id": "%s",
			"deployment_environment": "%s"
		}
		}}`, attributes[semconv.ServiceNameKey],
		attributes[semconv.ServiceNamespaceKey],
		attributes[semconv.ServiceVersionKey],
		attributes[semconv.ServiceInstanceIDKey],
		attributes[semconv.DeploymentEnvironmentKey])

	ctx := context.Background()
	_, traceProvider, resource, err := distributedtracing.Init(ctx, c, "foo")
	if err != nil {
		t.Fatalf("Unexpected error initializing trace exporter %v", err)
	}
	if traceProvider == nil {
		t.Fatalf("Tracer provider was not initialized")
	}
	if resource == nil {
		t.Fatalf("Resource was not initialized")
	}
	if len(resource.Attributes()) != 5 {
		t.Fatalf("Unexpected resource attributes count. Expected: %v, Got: %v", 5, len(resource.Attributes()))
	}

	for _, value := range resource.Attributes() {
		if attribute.StringValue(attributes[value.Key]) != value.Value {
			t.Fatalf("Unexpected resource attribute. Expected: %v, Got: %v", attributes[value.Key], value)
		}
	}

}

func TestCertPoolReloading(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tempDir := t.TempDir()

	serverCertPath := filepath.Join(tempDir, "serverCert.pem")
	serverCertKeyPath := filepath.Join(tempDir, "serverCertKey.pem")
	clientCertPath := filepath.Join(tempDir, "clientCert.pem")
	clientCertKeyPath := filepath.Join(tempDir, "clientCertKey.pem")
	caCertPath := filepath.Join(tempDir, "ca.pem")

	san := net.ParseIP("127.0.0.1")

	// create the CA cert used in the cert pool and for signing server certs
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	caSubj := pkix.Name{
		CommonName:   "CA",
		SerialNumber: caSerial.String(),
	}
	caTemplate := &x509.Certificate{
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		PublicKeyAlgorithm:    x509.ECDSA,
		PublicKey:             caKey.Public(),
		SerialNumber:          caSerial,
		Issuer:                caSubj,
		Subject:               caSubj,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(100 * time.Hour * 24 * 365),
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		DNSNames:              nil,
		EmailAddresses:        nil,
		IPAddresses:           nil,
	}

	caCertData, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}

	caCertPEMEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertData,
	})

	// we write an empty file for now
	err = os.WriteFile(caCertPath, []byte{}, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// create a cert and key for the server to load at startup
	var serverCert tls.Certificate

	serverCertKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverCert.PrivateKey = serverCertKey

	serverCertSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	serverCertTemplate := &x509.Certificate{
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		PublicKeyAlgorithm:    x509.ECDSA,
		PublicKey:             serverCertKey.Public(),
		SerialNumber:          serverCertSerial,
		Issuer:                caSubj,
		Subject: pkix.Name{
			CommonName:   "Server 1",
			SerialNumber: serverCertSerial.String(),
		},
		NotBefore:      time.Now(),
		NotAfter:       time.Now().Add(99 * time.Hour * 24 * 365),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:           false,
		DNSNames:       nil,
		EmailAddresses: nil,
		IPAddresses:    []net.IP{san},
	}

	serverCertData, err := x509.CreateCertificate(rand.Reader, serverCertTemplate, caTemplate, serverCertKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}

	serverCertPEMEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertData,
	})

	serverCertKeyMarshalled, _ := x509.MarshalPKCS8PrivateKey(serverCert.PrivateKey)
	serverCertKeyPEMEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: serverCertKeyMarshalled,
	})

	err = os.WriteFile(serverCertPath, serverCertPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(serverCertKeyPath, serverCertKeyPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// create a cert and key for the client to test client auth
	var clientCert tls.Certificate

	clientCertKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientCert.PrivateKey = clientCertKey

	clientCertSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	clientCertTemplate := &x509.Certificate{
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		PublicKeyAlgorithm:    x509.ECDSA,
		PublicKey:             clientCertKey.Public(),
		SerialNumber:          clientCertSerial,
		Issuer:                caSubj,
		Subject: pkix.Name{
			CommonName:   "Client",
			SerialNumber: clientCertSerial.String(),
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(99 * time.Hour * 24 * 365),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertData, err := x509.CreateCertificate(rand.Reader, clientCertTemplate, caTemplate, clientCertKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}

	clientCertPEMEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertData,
	})

	clientCertKeyMarshalled, _ := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	clientCertKeyPEMEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: clientCertKeyMarshalled,
	})

	err = os.WriteFile(clientCertPath, clientCertPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(clientCertKeyPath, clientCertKeyPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// configure the server to use the certs
	initialCertPool := x509.NewCertPool()
	ok := initialCertPool.AppendCertsFromPEM(caCertPEMEncoded)
	if !ok {
		t.Fatal("failed to add CA cert to cert pool")
	}

	initialCert, err := tls.LoadX509KeyPair(serverCertPath, serverCertKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Unexpected error creating listener while finding free port: %s", err)
	}

	serverAddress := listener.Addr().String()
	err = listener.Close()
	if err != nil {
		t.Fatalf("Unexpected error closing listener to free port: %s", err)
	}

	t.Log("server address:", serverAddress)

	server := New().
		WithAddresses([]string{serverAddress}).
		WithStore(inmem.New()).
		WithCertificate(&initialCert).
		WithCertPool(x509.NewCertPool()). // empty cert pool
		WithAuthentication(AuthenticationTLS).
		WithTLSConfig(
			&TLSConfig{
				CertFile:     serverCertPath,
				KeyFile:      serverCertKeyPath,
				CertPoolFile: caCertPath, // currently empty
			},
		)

	// start the server referencing the certs
	m, err := plugins.New([]byte{}, "test", server.store)
	if err != nil {
		t.Fatal(err)
	}
	server = server.WithManager(m)
	if err = m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	server, err = server.Init(ctx)
	if err != nil {
		t.Fatal(err)
	}

	loops, err := server.Listeners()
	if err != nil {
		t.Fatal(err)
	}

	for _, loop := range loops {
		go func(serverLoop func() error) {
			errc := make(chan error)
			errc <- serverLoop()
			err := <-errc
			t.Errorf("Unexpected error from server loop: %s", err)
		}(loop)
	}

	// wait for the server to start
	retries := 10
	for {
		if retries == 0 {
			t.Fatal("failed to start server before deadline")
		}
		_, err = tls.Dial("tcp", serverAddress, &tls.Config{RootCAs: initialCertPool})
		if err != nil {
			retries--
			time.Sleep(300 * time.Millisecond)
			continue
		}
		t.Log("server started")
		break
	}

	// make the first request and check that the server is not trusting the client cert
	clientKeyPair, err := tls.LoadX509KeyPair(clientCertPath, clientCertKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      initialCertPool,
				Certificates: []tls.Certificate{clientKeyPair},
			},
		},
	}

	// make a request and check that the server doesn't trust the client cert yet since it has no CA cert
	retries = 10
	expectedError := "remote error: tls"
	for {
		if retries == 0 {
			t.Fatal("server didn't return expected error before deadline")
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/data", serverAddress), nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.Do(req)
		if !strings.Contains(err.Error(), expectedError) {
			t.Log("retrying, expected error:", expectedError, "but got:", err)
			retries--
			time.Sleep(300 * time.Millisecond)
			continue
		}

		break
	}

	// update the cert pool file to include the CA cert
	err = os.WriteFile(caCertPath, caCertPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// make a second request and check that the server now trusts the client cert
	retries = 10
	for {
		if retries == 0 {
			t.Fatal("server didn't accept client cert before deadline")
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/data", serverAddress), nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.Do(req)
		if err != nil {
			t.Log("server still doesn't trust client cert")
			retries--
			time.Sleep(300 * time.Millisecond)
			continue
		}

		break
	}

	// update the cert pool file to a new & different CA that hasn't signed the client cert
	caKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caSerial, err = rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	caSubj = pkix.Name{
		CommonName:   "CA 2",
		SerialNumber: caSerial.String(),
	}

	caTemplate = &x509.Certificate{
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		PublicKeyAlgorithm:    x509.ECDSA,
		PublicKey:             caKey.Public(),
		SerialNumber:          caSerial,
		Issuer:                caSubj,
		Subject:               caSubj,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(100 * time.Hour * 24 * 365),
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		DNSNames:              nil,
		EmailAddresses:        nil,
		IPAddresses:           nil,
	}

	caCertData, err = x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}

	caCertPEMEncoded = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertData,
	})

	err = os.WriteFile(caCertPath, caCertPEMEncoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// make a final request and check that the server doesn't trust the client again
	// since the loaded CA cert is different from the one that signed the client cert
	retries = 10
	for {
		if retries == 0 {
			t.Fatal("server didn't accept client cert before deadline")
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/data", serverAddress), nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.Do(req)
		if err == nil {
			t.Log("server still trusts client cert")
			retries--
			time.Sleep(300 * time.Millisecond)
			continue
		}

		if !strings.Contains(err.Error(), "remote error: tls") {
			t.Fatalf("expected unknown certificate authority error (server has different CA) but got: %s", err)
		}

		break
	}

	err = server.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Unexpected error shutting down server: %s", err)
	}

}

func TestCertReloading(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	ctx := context.Background()

	testCases := map[string]struct {
		Server func(
			addr string,
			initialCert *tls.Certificate,
			initialCertPool *x509.CertPool,
			certFilePath, keyFilePath, caCertPath string,
		) *Server
	}{
		"fs notified server": {
			Server: func(
				addr string,
				initialCert *tls.Certificate,
				initialCertPool *x509.CertPool,
				certFilePath, keyFilePath, caCertPath string,
			) *Server {
				return New().
					WithAddresses([]string{addr}).
					WithStore(inmem.New()).
					WithCertificate(initialCert).
					WithCertPool(initialCertPool).
					WithTLSConfig(
						&TLSConfig{
							CertFile:     certFilePath,
							KeyFile:      keyFilePath,
							CertPoolFile: caCertPath,
						},
					)
			},
		},
		"interval reloaded server": {
			Server: func(
				addr string,
				initialCert *tls.Certificate,
				initialCertPool *x509.CertPool,
				certFilePath, keyFilePath, _ string,
			) *Server {
				return New().
					WithAddresses([]string{addr}).
					WithStore(inmem.New()).
					WithCertificate(initialCert).
					WithCertPool(initialCertPool).
					WithCertificatePaths(
						certFilePath,
						keyFilePath,
						1*time.Second,
					)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			tempDir := t.TempDir()

			serverCert1Path := filepath.Join(tempDir, "serverCert1.pem")
			serverCert1KeyPath := filepath.Join(tempDir, "serverCert1Key.pem")
			serverCert2Path := filepath.Join(tempDir, "serverCert2.pem")
			serverCert2KeyPath := filepath.Join(tempDir, "serverCert2Key.pem")
			caCertPath := filepath.Join(tempDir, "ca.pem")

			t.Helper()

			san := net.ParseIP("127.0.0.1")

			// create the CA cert used in the cert pool and for signing server certs
			caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}

			caSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			if err != nil {
				t.Fatal(err)
			}
			caSubj := pkix.Name{
				CommonName:   "CA",
				SerialNumber: caSerial.String(),
			}
			caTemplate := &x509.Certificate{
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA256,
				PublicKeyAlgorithm:    x509.ECDSA,
				PublicKey:             caKey.Public(),
				SerialNumber:          caSerial,
				Issuer:                caSubj,
				Subject:               caSubj,
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(100 * time.Hour * 24 * 365),
				KeyUsage:              x509.KeyUsageCertSign,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				IsCA:                  true,
			}

			caCertData, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caKey.Public(), caKey)
			if err != nil {
				t.Fatal(err)
			}

			caCertPEMEncoded := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: caCertData,
			})

			err = os.WriteFile(caCertPath, caCertPEMEncoded, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			// create a cert and key for the server to load at startup
			var serverCert1 tls.Certificate

			serverCert1Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			serverCert1.PrivateKey = serverCert1Key

			serverCert1Serial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			if err != nil {
				t.Fatal(err)
			}
			serverCert1Template := &x509.Certificate{
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA256,
				PublicKeyAlgorithm:    x509.ECDSA,
				PublicKey:             serverCert1Key.Public(),
				SerialNumber:          serverCert1Serial,
				Issuer:                caSubj,
				Subject: pkix.Name{
					CommonName:   "Server 1",
					SerialNumber: serverCert1Serial.String(),
				},
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(99 * time.Hour * 24 * 365),
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				IPAddresses: []net.IP{san},
			}

			serverCert1Data2, err := x509.CreateCertificate(rand.Reader, serverCert1Template, caTemplate, serverCert1Key.Public(), caKey)
			if err != nil {
				t.Fatal(err)
			}

			serverCert1PEMEncoded := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: serverCert1Data2,
			})

			serverCert1KeyMarshalled, _ := x509.MarshalPKCS8PrivateKey(serverCert1.PrivateKey)
			serverCert1KeyPEMEncoded := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: serverCert1KeyMarshalled,
			})

			err = os.WriteFile(serverCert1Path, serverCert1PEMEncoded, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(serverCert1KeyPath, serverCert1KeyPEMEncoded, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			// create a cert to load after startup
			var serverCert2 tls.Certificate

			serverCert2Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			serverCert2.PrivateKey = serverCert2Key

			serverCert2Serial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
			if err != nil {
				t.Fatal(err)
			}
			serverCert1Template = &x509.Certificate{
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA256,
				PublicKeyAlgorithm:    x509.ECDSA,
				PublicKey:             serverCert2Key.Public(),
				SerialNumber:          serverCert2Serial,
				Issuer:                caSubj,
				Subject: pkix.Name{
					CommonName:   "Server 2",
					SerialNumber: serverCert1Serial.String(),
				},
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(99 * time.Hour * 24 * 365),
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				IPAddresses: []net.IP{san},
			}

			serverCert2Data2, err := x509.CreateCertificate(rand.Reader, serverCert1Template, caTemplate, serverCert2Key.Public(), caKey)
			if err != nil {
				t.Fatal(err)
			}
			serverCert2.Certificate = [][]byte{serverCert2Data2, caCertData}

			serverCert2PEMEncoded := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: serverCert2Data2,
			})

			serverCert2KeyMarshalled, _ := x509.MarshalPKCS8PrivateKey(serverCert2.PrivateKey)
			serverCert2KeyPEMEncoded := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: serverCert2KeyMarshalled,
			})

			err = os.WriteFile(serverCert2Path, serverCert2PEMEncoded, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(serverCert2KeyPath, serverCert2KeyPEMEncoded, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			certPool2 := x509.NewCertPool()
			ok := certPool2.AppendCertsFromPEM(caCertPEMEncoded)
			if !ok {
				t.Fatal("failed to add CA cert to cert pool")
			}
			certPool, _, _, serverCert1Data, serverCert2Data := certPool2, &serverCert1, &serverCert2, serverCert1Data2, serverCert2Data2

			initialCert, err := tls.LoadX509KeyPair(serverCert1Path, serverCert1KeyPath)
			if err != nil {
				t.Fatal(err)
			}

			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("Unexpected error creating listener while finding free port: %s", err)
			}

			serverAddress := listener.Addr().String()
			err = listener.Close()
			if err != nil {
				t.Fatalf("Unexpected error closing listener to free port: %s", err)
			}

			t.Log("server address:", serverAddress)

			server := tc.Server(serverAddress, &initialCert, certPool, serverCert1Path, serverCert1KeyPath, caCertPath)

			// start the server referencing the certs
			m, err := plugins.New([]byte{}, "test", server.store)
			if err != nil {
				t.Fatal(err)
			}
			server = server.WithManager(m)
			if err = m.Start(ctx); err != nil {
				t.Fatal(err)
			}
			server, err = server.Init(ctx)
			if err != nil {
				t.Fatal(err)
			}

			loops, err := server.Listeners()
			if err != nil {
				t.Fatal(err)
			}

			for _, loop := range loops {
				go func(serverLoop func() error) {
					errc := make(chan error)
					errc <- serverLoop()
					err := <-errc
					t.Errorf("Unexpected error from server loop: %s", err)
				}(loop)
			}

			// wait for the server to start
			retries := 10
			for {
				if retries == 0 {
					t.Fatal("failed to start server before deadline")
				}
				_, err = tls.Dial("tcp", serverAddress, &tls.Config{RootCAs: certPool})
				if err != nil {
					retries--
					time.Sleep(300 * time.Millisecond)
					continue
				}
				t.Log("server started")
				break
			}

			// make the first connection, check that the server 1 cert is returned
			retries = 10
			for {
				if retries == 0 {
					t.Fatal("failed to get serverCert1 before deadline")
				}
				conn, err := tls.Dial("tcp", serverAddress, &tls.Config{RootCAs: certPool})
				if err != nil {
					t.Fatal(err)
				}
				err = conn.Close()
				if err != nil {
					t.Fatal(err)
				}

				certs := conn.ConnectionState().PeerCertificates
				if len(certs) != 1 {
					t.Fatalf("expected 1 cert, got %d", len(certs))
				}

				servedCert := certs[0]
				if !bytes.Equal(servedCert.Raw, serverCert1Data) {
					retries--
					time.Sleep(300 * time.Millisecond)
					t.Logf("expected serverCert1, got %s", servedCert.Subject)
					continue
				}

				break
			}

			// update the cert and key files by moving the second cert into place instead
			err = os.Rename(serverCert2Path, serverCert1Path)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Rename(serverCert2KeyPath, serverCert1KeyPath)
			if err != nil {
				t.Fatal(err)
			}

			// make another connection, check that the server 2 cert is returned
			retries = 10
			for {
				if retries == 0 {
					t.Fatal("failed to get serverCert2 before deadline")
				}

				conn, err := tls.Dial("tcp", serverAddress, &tls.Config{RootCAs: certPool})
				if err != nil {
					t.Fatal(err)
				}
				err = conn.Close()
				if err != nil {
					t.Fatal(err)
				}
				certs := conn.ConnectionState().PeerCertificates
				if len(certs) != 1 {
					t.Fatalf("expected 1 cert, got %d", len(certs))
				}

				servedCert := certs[0]
				if !bytes.Equal(servedCert.Raw, serverCert2Data) {
					retries--
					time.Sleep(300 * time.Millisecond)
					t.Logf("expected serverCert2, got %s", servedCert.Subject)
					continue
				}

				break
			}

			// remove the certs on disk, and check that the server still serves the previous certs
			err = os.Remove(serverCert1Path)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Remove(serverCert1KeyPath)
			if err != nil {
				t.Fatal(err)
			}

			// make a third connection, and check that the server 2 cert is still returned despite the certs being removed
			retries = 10
			for {
				if retries == 0 {
					t.Fatal("failed to get serverCert2 before deadline")
				}

				conn, err := tls.Dial("tcp", serverAddress, &tls.Config{RootCAs: certPool})
				if err != nil {
					t.Fatal(err)
				}
				err = conn.Close()
				if err != nil {
					t.Fatal(err)
				}

				certs := conn.ConnectionState().PeerCertificates
				if len(certs) != 1 {
					t.Fatalf("expected 1 cert, got %d", len(certs))
				}

				servedCert := certs[0]
				if !bytes.Equal(servedCert.Raw, serverCert2Data) {
					retries--
					time.Sleep(300 * time.Millisecond)
					t.Logf("expected serverCert2, got %s", servedCert.Subject)
					continue
				}

				break
			}

			err = server.Shutdown(ctx)
			if err != nil {
				t.Fatalf("Unexpected error shutting down server: %s", err)
			}
		})
	}

}

type mockHTTPHandler struct{}

func (*mockHTTPHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type mockMetricsProvider struct{}

func (*mockMetricsProvider) RegisterEndpoints(registrar func(string, string, http.Handler)) {
	registrar("/metrics", "GET", new(mockHTTPHandler))
}

func (*mockMetricsProvider) InstrumentHandler(handler http.Handler, _ string) http.Handler {
	return handler
}

type listenerHook func() error

type mockHTTPListener struct {
	shutdownHook listenerHook
	addrs        string
	t            httpListenerType
}

func (m mockHTTPListener) Addr() string {
	return m.addrs
}

func (mockHTTPListener) ListenAndServe() error {
	return errors.New("not implemented")
}

func (mockHTTPListener) ListenAndServeTLS(string, string) error {
	return errors.New("not implemented")
}

func (m mockHTTPListener) Shutdown(context.Context) error {
	var err error
	if m.shutdownHook != nil {
		err = m.shutdownHook()
	}
	return err
}

func (m mockHTTPListener) Type() httpListenerType {
	return m.t
}

func zipString(input string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(input)); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

func TestStringPathToDataRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		path   string
		expRef string
		expErr string
	}{
		{path: "foo", expRef: `data.foo`},
		{path: "foo/", expRef: `data.foo`},
		{path: "foo/bar", expRef: `data.foo.bar`},
		{path: "foo/bar/", expRef: `data.foo.bar`},
		{path: "foo/../bar", expRef: `data.foo[".."].bar`},

		// Path injection attack
		// url path: `foo%22%5D%3Bmalicious_call%28%29%3Bx%3D%5B%22`
		// url decoded: `foo"];malicious_call();x=["`
		// data ref .String(): `data.foo["\"];malicious_call();x=[\""]`
		// Above attack is mitigated by rejecting any ref component containing string terminators (`"`).
		{
			note:   "string terminals inside ref term",
			path:   "foo%22%5D%3Bmalicious_call%28%29%3Bx%3D%5B%22", // foo"];malicious_call();x=["
			expErr: `invalid ref term 'foo"];malicious_call();x=["'`,
		},
	}

	for _, tc := range cases {
		note := tc.note
		if note == "" {
			note = strings.ReplaceAll(tc.path, "/", "_")
		}

		t.Run(note, func(t *testing.T) {
			ref, err := stringPathToDataRef(tc.path)

			if tc.expRef != "" {
				if err != nil {
					t.Fatalf("Expected ref:\n\n%s\n\nbut got error:\n\n%s", tc.expRef, err)
				}
				if refStr := ref.String(); refStr != tc.expRef {
					t.Fatalf("Expected ref:\n\n%s\n\nbut got:\n\n%s", tc.expRef, refStr)
				}
			}

			if tc.expErr != "" {
				if ref != nil {
					t.Fatalf("Expected error:\n\n%s\n\nbut got ref:\n\n%s", tc.expErr, ref.String())
				}
				if errStr := err.Error(); errStr != tc.expErr {
					t.Fatalf("Expected error:\n\n%s\n\nbut got ref:\n\n%s", tc.expErr, errStr)
				}
			}
		})
	}
}

func TestParseRefQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		raw     string
		expBody ast.Body
		expErr  string
	}{
		{
			note:   "unparseable",
			raw:    `}abc{`,
			expErr: "failed to parse query",
		},
		{
			note:   "empty",
			raw:    ``,
			expErr: "no ref",
		},
		{
			note:    "single ref",
			raw:     `data.foo.bar`,
			expBody: ast.MustParseBody(`data.foo.bar`),
		},
		{
			note:   "multiple refs,';' separated",
			raw:    `data.foo.bar;data.baz.qux`,
			expErr: "complex query",
		},
		{
			note: "multiple refs,newline separated",
			raw: `data.foo.bar
data.baz.qux`,
			expErr: "complex query",
		},
		{
			note:   "single ref + call",
			raw:    `data.foo.bar;data.baz.qux()`,
			expErr: "complex query",
		},
		{
			note:   "single ref + assignment",
			raw:    `data.foo.bar;x := 42`,
			expErr: "complex query",
		},
		{
			note:   "single call",
			raw:    `data.foo.bar()`,
			expErr: "complex query",
		},
		{
			note:   "single assignment",
			raw:    `x := 42`,
			expErr: "complex query",
		},
		{
			note:   "single unification",
			raw:    `x = 42`,
			expErr: "complex query",
		},
		{
			note:   "single equality",
			raw:    `x == 42`,
			expErr: "complex query",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			body, err := parseRefQuery(tc.raw)

			if tc.expBody != nil {
				if err != nil {
					t.Fatalf("Expected body:\n\n%s\n\nbut got error:\n\n%s", tc.expBody, err)
				}
				if body.String() != tc.expBody.String() {
					t.Fatalf("Expected body:\n\n%s\n\nbut got:\n\n%s", tc.expBody, body.String())
				}
			}

			if tc.expErr != "" {
				if body != nil {
					t.Fatalf("Expected error:\n\n%s\n\nbut got body:\n\n%s", tc.expErr, body.String())
				}
				if errStr := err.Error(); errStr != tc.expErr {
					t.Fatalf("Expected error:\n\n%s\n\nbut got body:\n\n%s", tc.expErr, errStr)
				}
			}
		})
	}
}
