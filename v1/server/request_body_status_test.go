// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestRequestBodyLimitStatus(t *testing.T) {
	oversized := strings.Repeat(" ", 65) + "{}"
	type requestCase struct {
		name, method, path, body string
		chunked, gzip, tooLarge  bool
		status                   int
	}
	cases := []requestCase{
		{name: "content length", body: oversized, tooLarge: true},
		{name: "chunked", body: oversized, chunked: true, tooLarge: true},
		{name: "gzip", body: oversized, gzip: true, tooLarge: true},
		{name: "under limit", body: `{"input":{}}`, status: http.StatusOK},
		{name: "boundary", body: `{"input":"` + strings.Repeat("x", 52) + `"}`, status: http.StatusOK},
		{name: "malformed json", body: `{"input":`, status: http.StatusBadRequest},
	}
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodPost, "/"}, {http.MethodPost, "/v0/data/test"},
		{http.MethodPost, "/v1/query"}, {http.MethodPost, "/v1/compile"},
		{http.MethodPost, "/v1/compile/test"}, {http.MethodPut, "/v1/data/test"},
		{http.MethodPatch, "/v1/data/test"}, {http.MethodPut, "/v1/policies/test"},
	} {
		cases = append(cases, requestCase{name: endpoint.method + " " + endpoint.path, method: endpoint.method, path: endpoint.path, body: oversized, chunked: true, tooLarge: true})
	}
	for _, authz := range []bool{false, true} {
		bodies := map[string]string{}
		for _, version := range []ast.RegoVersion{ast.RegoUndefined, ast.RegoV0, ast.RegoV1, ast.RegoV2} {
			t.Run(fmt.Sprintf("%s/authorization=%t", version, authz), func(t *testing.T) {
				ctx := t.Context()
				store := inmem.New()
				if err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
					return store.UpsertPolicy(ctx, txn, "authz", []byte("package system.authz\nimport rego.v1\nallow := true"))
				}); err != nil {
					t.Fatal(err)
				}
				manager, err := plugins.New([]byte(`{"server":{"decoding":{"max_length":64,"gzip":{"max_length":64}}}}`), "test", store,
					plugins.WithParserOptions(ast.ParserOptions{RegoVersion: version}))
				if err != nil {
					t.Fatal(err)
				}
				if err := manager.Start(ctx); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { manager.Stop(ctx) })
				srv := New().WithStore(store).WithManager(manager)
				if authz {
					srv.WithAuthorization(AuthorizationBasic)
				}
				if _, err := srv.Init(ctx); err != nil {
					t.Fatal(err)
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						method, path := tc.method, tc.path
						if method == "" {
							method = http.MethodPost
						}
						if path == "" {
							path = "/v1/data/test"
						}
						body := []byte(tc.body)
						if tc.gzip {
							body = mustGZIPPayload(body)
						}
						req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
						if tc.chunked {
							req.ContentLength = -1
						}
						if tc.gzip {
							req.Header.Set("Content-Encoding", "gzip")
						}
						recorder := httptest.NewRecorder()
						srv.Handler.ServeHTTP(recorder, req)
						status := tc.status
						if tc.tooLarge {
							status = http.StatusBadRequest
							if version == ast.RegoV2 {
								status = http.StatusRequestEntityTooLarge
							}
							var payload types.ErrorV1
							if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
								t.Fatal(err)
							}
							if payload.Code != types.CodeInvalidParameter || !strings.Contains(payload.Message, "request body too large") {
								t.Fatalf("unexpected error payload: %s", recorder.Body.String())
							}
						}
						if recorder.Code != status {
							t.Errorf("status: got %d, want %d; body: %s", recorder.Code, status, recorder.Body.String())
						}
						if version == ast.RegoUndefined {
							bodies[tc.name] = recorder.Body.String()
						} else if recorder.Body.String() != bodies[tc.name] {
							t.Errorf("error body changed: got %s, want %s", recorder.Body.String(), bodies[tc.name])
						}
					})
				}
			})
		}
	}
}
