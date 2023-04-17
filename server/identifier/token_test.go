// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package identifier_test

import (
	"net/http"
	"testing"

	"github.com/open-policy-agent/opa/server/identifier"
)

func TestTokenBased(t *testing.T) {

	mock := &mockHandler{}
	handler := identifier.NewTokenBased(mock)

	req, err := http.NewRequest(http.MethodGet, "/foo/bar/baz", nil)
	if err != nil {
		t.Fatalf("Unexpected error creating request: %v", err)
	}

	tests := []struct {
		value           string
		expected        string
		identityDefined bool
	}{
		{
			"",
			"",
			false,
		},
		{
			"Bearer this-is-the-token",
			"this-is-the-token",
			true,
		},
		{
			"Bearer    this-is-the-token-with-spaces",
			"this-is-the-token-with-spaces",
			true,
		},
	}

	for _, tc := range tests {

		if tc.value != "" {
			req.Header.Set("Authorization", tc.value)
		}

		handler.ServeHTTP(nil, req)

		if mock.identityDefined != tc.identityDefined {
			t.Fatalf("Expected identityDefined to be %v but got: %v", tc.identityDefined, mock.identityDefined)
		}

		if mock.identity != tc.expected {
			t.Fatalf("Expected identity to be %s but got: %s", tc.expected, mock.identity)
		}
	}

}
