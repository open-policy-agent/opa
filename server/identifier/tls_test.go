// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package identifier_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/server/identifier"
)

// Note: In these tests, we don't worry about the server actually verifying the
// client's certs; that's done in a different place. We only request it, and
// check what the identifier does with it.

func TestTLSBased(t *testing.T) {
	mock := &mockHandler{}
	handler := identifier.NewTLSBased(mock)

	tests := []struct {
		desc     string
		cert     string
		key      string
		expected string
		defined  bool
	}{
		{
			desc: "no cert",
		},
		{
			desc:     "cert with CN=<name>",
			cert:     "testdata/cn-cert.pem",
			key:      "testdata/key.pem",
			expected: "CN=my-client",
			defined:  true,
		},
		{
			desc:     "cert with long DN",
			cert:     "testdata/ou-cert.pem",
			key:      "testdata/key.pem",
			expected: "OU=opa-client-01,O=Torchwood",
			defined:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Note: some re-use happens if this server is outside of the tests loop,
			// causing weird overlaps. Let's keep setting up a fresh one in each
			// iteration to be safe.
			s := httptest.NewUnstartedServer(handler)
			s.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
			s.StartTLS()
			defer s.Close()
			c := s.Client() // trusts the httptest server's TLS cert

			if tc.cert != "" && tc.key != "" {
				cert, err := tls.LoadX509KeyPair(tc.cert, tc.key)
				if err != nil {
					t.Fatalf("read test cert/key (%s/%s): %s", tc.cert, tc.key, err)
				}
				c.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{cert}
			}

			_, err := c.Get(s.URL)
			if err != nil {
				t.Fatalf("unexpected error in GET %s: %s", s.URL, err)
			}
			if mock.defined != tc.defined {
				t.Fatalf("Expected defined to be %v but got: %v", tc.defined, mock.defined)
			}

			if mock.identity != tc.expected {
				t.Fatalf("Expected identity to be %s but got: %s", tc.expected, mock.identity)
			}
		})
	}

}
