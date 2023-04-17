// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package identifier_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
		desc                      string
		cert                      string
		key                       string
		identityExpected          string
		identityDefined           bool
		clientCertificatesDefined bool
	}{
		{
			desc: "no cert",
		},
		{
			desc:                      "cert with CN=<name>",
			cert:                      "testdata/cn-cert.pem",
			key:                       "testdata/key.pem",
			identityExpected:          "CN=my-client",
			identityDefined:           true,
			clientCertificatesDefined: true,
		},
		{
			desc:                      "cert with long DN",
			cert:                      "testdata/ou-cert.pem",
			key:                       "testdata/key.pem",
			identityExpected:          "OU=opa-client-01,O=Torchwood",
			identityDefined:           true,
			clientCertificatesDefined: true,
		},
		{
			desc:                      "SPIFFE cert",
			cert:                      "testdata/spiffe-svid-cert.pem",
			key:                       "testdata/spiffe-svid-key.pem",
			identityExpected:          "SERIALNUMBER=3064486355086639231,OU=Example Org Unit,O=Example Org,C=GB",
			identityDefined:           true,
			clientCertificatesDefined: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var err error

			// Note: some re-use happens if this server is outside of the tests loop,
			// causing weird overlaps. Let's keep setting up a fresh one in each
			// iteration to be safe.
			s := httptest.NewUnstartedServer(handler)
			s.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
			s.StartTLS()
			defer s.Close()
			c := s.Client() // trusts the httptest server's TLS cert

			var cert tls.Certificate
			if tc.cert != "" && tc.key != "" {
				cert, err = tls.LoadX509KeyPair(tc.cert, tc.key)
				if err != nil {
					t.Fatalf("read test cert/key (%s/%s): %s", tc.cert, tc.key, err)
				}
				c.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{cert}
			}

			_, err = c.Get(s.URL)
			if err != nil {
				t.Fatalf("unexpected error in GET %s: %s", s.URL, err)
			}
			if mock.identityDefined != tc.identityDefined {
				t.Fatalf("Expected identityDefined to be %v but got: %v", tc.identityDefined, mock.identityDefined)
			}

			if tc.identityDefined {
				if mock.identity != tc.identityExpected {
					t.Fatalf("Expected identity to be %s but got: %s", tc.identityExpected, mock.identity)
				}
			}

			if mock.clientCertificatesDefined != tc.clientCertificatesDefined {
				t.Fatalf("Expected clientCertificatesDefined to be %v but got: %v", tc.clientCertificatesDefined, mock.clientCertificatesDefined)
			}

			if tc.clientCertificatesDefined {
				if len(mock.clientCertificates) != 1 {
					t.Fatalf("Expected clientCertificates to have 1 cert but got: %d", len(mock.clientCertificates))
				}

				gotPemData := pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: mock.clientCertificates[0].Raw,
				})

				parsedWantedCert, err := x509.ParseCertificate(cert.Certificate[0])
				if err != nil {
					t.Fatalf("Error parsing expected cert: %s", err)
				}

				wantPemData := pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: parsedWantedCert.Raw,
				})

				if got, want := string(gotPemData), string(wantPemData); got != want {
					t.Fatalf("Expected clientCertificates to be \n%s\n but got: \n%s\n", want, got)
				}
			}
		})
	}
}
