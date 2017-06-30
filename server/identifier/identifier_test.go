// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package identifier

import (
	"net/http"
	"testing"
)

type mockHandler struct {
	identity string
}

func (h *mockHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.identity = Identity(r)
}

func TestTokenBased(t *testing.T) {

	mock := &mockHandler{}
	handler := NewTokenBased(mock)

	req, err := http.NewRequest(http.MethodGet, "/foo/bar/baz", nil)
	if err != nil {
		t.Fatalf("Unexpected error creating request: %v", err)
	}

	tests := []struct {
		value    string
		expected string
	}{
		{"", ""},
		{"Bearer this-is-the-token", "this-is-the-token"},
		{"Bearer    this-is-the-token-with-spaces", "this-is-the-token-with-spaces"},
	}

	for _, tc := range tests {

		if tc.value != "" {
			req.Header.Set("Authorization", tc.value)
		}

		handler.ServeHTTP(nil, req)

		if mock.identity != tc.expected {
			t.Fatalf("Expected identity to be %s but got: %s", tc.expected, mock.identity)
		}
	}

}
