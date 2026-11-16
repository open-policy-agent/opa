// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestBodyLimitCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name       string
		v0, v1, v2 bool
		status     int
	}{
		{"default", false, false, false, http.StatusBadRequest},
		{"v0", true, false, false, http.StatusBadRequest},
		{"v1", false, true, false, http.StatusBadRequest},
		{"v2", false, false, true, http.StatusRequestEntityTooLarge},
		{"v2 over v1", false, true, true, http.StatusRequestEntityTooLarge},
		{"v0 over v2", true, false, true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := NewParams()
			params.Addrs = &[]string{"localhost:0"}
			params.V0Compatible, params.V1Compatible, params.V2Compatible = tc.v0, tc.v1, tc.v2
			params.ConfigOverrides = []string{"server.decoding.max_length=16"}
			rt, err := NewRuntime(t.Context(), params)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := rt.buildServer(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			srv.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/data/test", strings.NewReader(strings.Repeat(" ", 17))))
			if recorder.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", recorder.Code, tc.status, recorder.Body.String())
			}
		})
	}
}
