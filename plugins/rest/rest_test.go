// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {

	tests := []struct {
		input   string
		wantErr bool
	}{
		{
			input: `{
				"name": "foo",
				"url": "bad scheme://authority",
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url", "http://localhost/some/path",
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"token": "secret",
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"scheme": "Acmecorp-Token",
						"token": "secret"
					}
				}
			}`,
		},
	}

	var results []Client

	for _, tc := range tests {
		client, err := New([]byte(tc.input))
		if err != nil && !tc.wantErr {
			t.Fatalf("Unexpected error: %v", err)
		}
		results = append(results, client)
	}

	if results[2].config.Credentials.Bearer.Scheme != "Bearer" {
		t.Fatalf("Expected token scheme to be set to Bearer but got: %v", results[2].config.Credentials.Bearer.Scheme)
	}

	if results[3].config.Credentials.Bearer.Scheme != "Acmecorp-Token" {
		t.Fatalf("Expected custom token but got: %v", results[3].config.Credentials.Bearer.Scheme)
	}

}

func TestValidUrl(t *testing.T) {
	ts := testServer{
		t:         t,
		expMethod: "GET",
		expPath:   "/test",
	}
	ts.start()
	defer ts.stop()
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
	}`, ts.server.URL)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()
	if _, err := client.Do(ctx, "GET", "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestBearerToken(t *testing.T) {
	ts := testServer{
		t:               t,
		expBearerScheme: "Acmecorp-Token",
		expBearerToken:  "secret",
	}
	ts.start()
	defer ts.stop()
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"credentials": {
			"bearer": {
				"scheme": "Acmecorp-Token",
				"token": "secret"
			}
		}
	}`, ts.server.URL)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()
	if _, err := client.Do(ctx, "GET", "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

type testServer struct {
	t               *testing.T
	server          *httptest.Server
	expPath         string
	expMethod       string
	expBearerToken  string
	expBearerScheme string
	tls             bool
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {
	if t.expMethod != "" && t.expMethod != r.Method {
		t.t.Fatalf("Expected method %v, got %v", t.expMethod, r.Method)
	}
	if t.expPath != "" && t.expPath != r.URL.Path {
		t.t.Fatalf("Expected path %q, got %q", t.expPath, r.URL.Path)
	}
	if (t.expBearerToken != "" || t.expBearerScheme != "") && len(r.Header["Authorization"]) == 0 {
		t.t.Fatal("Expected bearer token, but didn't get any")
	}
	auth := r.Header["Authorization"][0]
	if t.expBearerScheme != "" && !strings.HasPrefix(auth, t.expBearerScheme) {
		t.t.Fatalf("Expected bearer scheme %q, got authorization header %q", t.expBearerScheme, auth)
	}
	if t.expBearerToken != "" && !strings.HasSuffix(auth, t.expBearerToken) {
		t.t.Fatalf("Expected bearer token %q, got authorization header %q", t.expBearerToken, auth)
	}
	w.WriteHeader(200)
}

func (t *testServer) start() {
	t.server = httptest.NewUnstartedServer(http.HandlerFunc(t.handle))
	if t.tls {
		t.server.StartTLS()
	} else {
		t.server.Start()
	}
}

func (t *testServer) stop() {
	t.server.Close()
}
