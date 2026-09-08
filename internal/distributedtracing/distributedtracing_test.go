// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitNoTypeReturnsAllNil(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {}}`)
	exp, tp, res, opts, err := Init(t.Context(), raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	if exp != nil || tp != nil || res != nil || opts != nil {
		t.Fatal("expected all nil when type is not set")
	}
}

func TestInitExcludePathsServerOptions(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {"type": "grpc", "exclude_paths": ["/health**"]}}`)
	_, _, _, opts, err := Init(t.Context(), raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := len(opts), 1; got != exp {
		t.Fatalf("got %d server tracing option(s), expected %d", got, exp)
	}

	raw = []byte(`{"distributed_tracing": {"type": "grpc"}}`)
	if _, _, _, opts, err = Init(t.Context(), raw, "test"); err != nil {
		t.Fatal(err)
	}
	if len(opts) != 0 {
		t.Fatalf("got %d server tracing option(s), expected none", len(opts))
	}
}

func TestInitInvalidExcludePath(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {"type": "grpc", "exclude_paths": ["/health["]}}`)
	_, _, _, _, err := Init(t.Context(), raw, "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if exp := "invalid distributed_tracing.exclude_paths pattern '/health['"; !strings.Contains(err.Error(), exp) {
		t.Fatalf("expected error to contain %q, got %q", exp, err.Error())
	}
}

func TestExcludePathsFilter(t *testing.T) {
	tests := []struct {
		note     string
		patterns []string
		path     string
		traced   bool
	}{
		{note: "no patterns", path: "/health", traced: true},
		{note: "exact match", patterns: []string{"/health"}, path: "/health", traced: false},
		{note: "exact match, other path", patterns: []string{"/health"}, path: "/v1/data", traced: true},
		{note: "single star doesn't cross separators", patterns: []string{"/health*"}, path: "/health/live", traced: true},
		{note: "double star crosses separators", patterns: []string{"/health**"}, path: "/health/live", traced: false},
		{note: "double star matches bare prefix", patterns: []string{"/health**"}, path: "/health", traced: false},
		{note: "prefix isn't matched implicitly", patterns: []string{"/health"}, path: "/health/live", traced: true},
		{note: "second pattern matches", patterns: []string{"/metrics", "/health"}, path: "/health", traced: false},
		{note: "query string is not part of the path", patterns: []string{"/health"}, path: "/health?bundles=true", traced: false},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := distributedTracingConfig{Type: "grpc", ExcludePaths: tc.patterns}
			if err := c.validateAndInjectDefaults(); err != nil {
				t.Fatal(err)
			}

			filter := excludePathsFilter(c.excludePaths)
			if got := filter(httptest.NewRequest(http.MethodGet, tc.path, nil)); got != tc.traced {
				t.Errorf("filter(%q) = %v, expected %v", tc.path, got, tc.traced)
			}
		})
	}
}
