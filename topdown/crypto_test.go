// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"
)

func TestCryptoX509ParseCertificates(t *testing.T) {

	rule := `
		p[x] {
			parsed := crypto.x509.parse_certificates(certs)
			x := oid_to_string(parsed[_].Issuer.Names[_].Type)
		}
	`

	tests := []struct {
		note     string
		certs    string
		rule     string
		expected interface{}
	}{
		{
			note:     "one",
			certs:    `MIIDujCCAqKgAwIBAgIIE31FZVaPXTUwDQYJKoZIhvcNAQEFBQAwSTELMAkGA1UEBhMCVVMxEzARBgNVBAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMTHEdvb2dsZSBJbnRlcm5ldCBBdXRob3JpdHkgRzIwHhcNMTQwMTI5MTMyNzQzWhcNMTQwNTI5MDAwMDAwWjBpMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNTW91bnRhaW4gVmlldzETMBEGA1UECgwKR29vZ2xlIEluYzEYMBYGA1UEAwwPbWFpbC5nb29nbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEfRrObuSW5T7q5CnSEqefEmtH4CCv6+5EckuriNr1CjfVvqzwfAhopXkLrq45EQm8vkmf7W96XJhC7ZM0dYi1/qOCAU8wggFLMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAaBgNVHREEEzARgg9tYWlsLmdvb2dsZS5jb20wCwYDVR0PBAQDAgeAMGgGCCsGAQUFBwEBBFwwWjArBggrBgEFBQcwAoYfaHR0cDovL3BraS5nb29nbGUuY29tL0dJQUcyLmNydDArBggrBgEFBQcwAYYfaHR0cDovL2NsaWVudHMxLmdvb2dsZS5jb20vb2NzcDAdBgNVHQ4EFgQUiJxtimAuTfwb+aUtBn5UYKreKvMwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBRK3QYWG7z2aLV29YG2u2IaulqBLzAXBgNVHSAEEDAOMAwGCisGAQQB1nkCBQEwMAYDVR0fBCkwJzAloCOgIYYfaHR0cDovL3BraS5nb29nbGUuY29tL0dJQUcyLmNybDANBgkqhkiG9w0BAQUFAAOCAQEAH6RYHxHdcGpMpFE3oxDoFnP+gtuBCHan2yE2GRbJ2Cw8Lw0MmuKqHlf9RSeYfd3BXeKkj1qO6TVKwCh+0HdZk283TZZyzmEOyclm3UGFYe82P/iDFt+CeQ3NpmBg+GoaVCuWAARJN/KfglbLyyYygcQq0SgeDh8dRKUiaW3HQSoYvTvdTuqzwK4CXsr3b5/dAOY8uMuG/IAR3FgwTbZ1dtoWRvOTa8hYiU6A475WuZKyEHcwnGYe57u2I2KbMgcKjPniocj4QzgYsVAVKW3IwaOhyE+vPxsiUkvQHdO2fojCkY8jg70jxM+gu59tPDNbw3Uh/2Ij310FgTHsnGQMyA==`,
			rule:     rule,
			expected: `["2.5.4.6", "2.5.4.10", "2.5.4.3"]`,
		},
		{
			note:     "multiple",
			certs:    `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`,
			rule:     rule,
			expected: `["2.5.4.10", "2.5.4.11", "2.5.4.3", "2.5.4.6"]`,
		},
		{
			note:     "bad",
			certs:    `YmFkc3RyaW5n`,
			rule:     rule,
			expected: fmt.Errorf("asn1: structure error"),
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		rules := []string{
			fmt.Sprintf("certs = %q { true }", tc.certs),
			fmt.Sprintf(`
				oid_to_string(oid) = concat(".", [s | s := format_int(oid[_], 10)]) { true }
			`),
			tc.rule,
		}
		runTopDownTestCase(t, data, tc.note, rules, tc.expected)
	}

}
