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
			expected: &Error{Code: BuiltinErr, Message: "asn1: structure error"},
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

func TestCryptoMd5(t *testing.T) {

	tests := []struct {
		note     string
		rule     []string
		expected interface{}
	}{
		{
			note:     "crypto.md5 with string",
			rule:     []string{`p[hash] { hash := crypto.md5("lorem ipsum") }`},
			expected: `["80a751fde577028640c419000e33eba6"]`,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rule, tc.expected)
	}

}

func TestCryptoSha1(t *testing.T) {

	tests := []struct {
		note     string
		rule     []string
		expected interface{}
	}{
		{
			note:     "crypto.sha1 with string",
			rule:     []string{`p[hash] { hash := crypto.sha1("lorem ipsum") }`},
			expected: `["bfb7759a67daeb65410490b4d98bb9da7d1ea2ce"]`,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rule, tc.expected)
	}

}

func TestCryptoSha256(t *testing.T) {

	tests := []struct {
		note     string
		rule     []string
		expected interface{}
	}{
		{
			note:     "crypto.sha256 with string",
			rule:     []string{`p[hash] { hash := crypto.sha256("lorem ipsum") }`},
			expected: `["5e2bf57d3f40c4b6df69daf1936cb766f832374b4fc0259a7cbff06e2f70f269"]`,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rule, tc.expected)
	}

}

func TestCryptoX509ParseCertificateRequest(t *testing.T) {
	rule := `
		p = x {
    	x := crypto.x509.parse_certificate_request(cert)
		}
	`

	tests := []struct {
		note     string
		cert     string
		rule     string
		expected interface{}
	}{
		{
			note:     "valid base64 PEM encoded certificate",
			cert:     `LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQllUQ0NBUWdDQVFBd01ERXVNQ3dHQTFVRUF4TWxiWGt0Y0c5a0xtMTVMVzVoYldWemNHRmpaUzV3YjJRdQpZMngxYzNSbGNpNXNiMk5oYkRCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQk1DYzN6eE9TYkN2Ck9NYitoajh2MW9aU2tPdHhLMXphdmVXck9ZVVJ5WTJOUU8wZFBmN2JuRkNZSEhKRFBwMUJTSDFwK2xXMS83MjYKdU5qTUNFdnRPdGlnZGpCMEJna3Foa2lHOXcwQkNRNHhaekJsTUdNR0ExVWRFUVJjTUZxQ0pXMTVMWE4yWXk1dAplUzF1WVcxbGMzQmhZMlV1YzNaakxtTnNkWE4wWlhJdWJHOWpZV3lDSlcxNUxYQnZaQzV0ZVMxdVlXMWxjM0JoClkyVXVjRzlrTG1Oc2RYTjBaWEl1Ykc5allXeUhCTUFBQWhpSEJBb0FJZ0l3Q2dZSUtvWkl6ajBFQXdJRFJ3QXcKUkFJZ1o5RXNFNTlaZG9PWSs4Mm9Cc1Q1bUd3a2p6WDBqdFJqci9OazIzVVBqcUlDSUVXVk56T2wzSCtqZTA2MwpWRXhTQ080ZzBUSHNiTGhidXVFc1NCYS9VUVBXCi0tLS0tRU5EIENFUlRJRklDQVRFIFJFUVVFU1QtLS0tLQ==`,
			rule:     rule,
			expected: `{"Attributes":[{"Type":[1,2,840,113549,1,9,14],"Value":[[{"Type":[2,5,29,17],"Value":"MFqCJW15LXN2Yy5teS1uYW1lc3BhY2Uuc3ZjLmNsdXN0ZXIubG9jYWyCJW15LXBvZC5teS1uYW1lc3BhY2UucG9kLmNsdXN0ZXIubG9jYWyHBMAAAhiHBAoAIgI="}]]}],"DNSNames":["my-svc.my-namespace.svc.cluster.local","my-pod.my-namespace.pod.cluster.local"],"EmailAddresses":null,"Extensions":[{"Critical":false,"Id":[2,5,29,17],"Value":"MFqCJW15LXN2Yy5teS1uYW1lc3BhY2Uuc3ZjLmNsdXN0ZXIubG9jYWyCJW15LXBvZC5teS1uYW1lc3BhY2UucG9kLmNsdXN0ZXIubG9jYWyHBMAAAhiHBAoAIgI="}],"ExtraExtensions":null,"IPAddresses":["192.0.2.24","10.0.34.2"],"PublicKey":{"Curve":{"B":41058363725152142129326129780047268409114441015993725554835256314039467401291,"BitSize":256,"Gx":48439561293906451759052585252797914202762949526041747995844080717082404635286,"Gy":36134250956749795798585127919587881956611106672985015071877198253568414405109,"N":115792089210356248762697446949407573529996955224135760342422259061068512044369,"Name":"P-256","P":115792089210356248762697446949407573530086143415290314195533631308867097853951},"X":87121235785369381977155560510693052819781295827853218437619145832055783918989,"Y":29366966885721994211102509301276799147874820413529896705575441176811887475416},"PublicKeyAlgorithm":3,"Raw":"MIIBYTCCAQgCAQAwMDEuMCwGA1UEAxMlbXktcG9kLm15LW5hbWVzcGFjZS5wb2QuY2x1c3Rlci5sb2NhbDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABMCc3zxOSbCvOMb+hj8v1oZSkOtxK1zaveWrOYURyY2NQO0dPf7bnFCYHHJDPp1BSH1p+lW1/726uNjMCEvtOtigdjB0BgkqhkiG9w0BCQ4xZzBlMGMGA1UdEQRcMFqCJW15LXN2Yy5teS1uYW1lc3BhY2Uuc3ZjLmNsdXN0ZXIubG9jYWyCJW15LXBvZC5teS1uYW1lc3BhY2UucG9kLmNsdXN0ZXIubG9jYWyHBMAAAhiHBAoAIgIwCgYIKoZIzj0EAwIDRwAwRAIgZ9EsE59ZdoOY+82oBsT5mGwkjzX0jtRjr/Nk23UPjqICIEWVNzOl3H+je063VExSCO4g0THsbLhbuuEsSBa/UQPW","RawSubject":"MDAxLjAsBgNVBAMTJW15LXBvZC5teS1uYW1lc3BhY2UucG9kLmNsdXN0ZXIubG9jYWw=","RawSubjectPublicKeyInfo":"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEwJzfPE5JsK84xv6GPy/WhlKQ63ErXNq95as5hRHJjY1A7R09/tucUJgcckM+nUFIfWn6VbX/vbq42MwIS+062A==","RawTBSCertificateRequest":"MIIBCAIBADAwMS4wLAYDVQQDEyVteS1wb2QubXktbmFtZXNwYWNlLnBvZC5jbHVzdGVyLmxvY2FsMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEwJzfPE5JsK84xv6GPy/WhlKQ63ErXNq95as5hRHJjY1A7R09/tucUJgcckM+nUFIfWn6VbX/vbq42MwIS+062KB2MHQGCSqGSIb3DQEJDjFnMGUwYwYDVR0RBFwwWoIlbXktc3ZjLm15LW5hbWVzcGFjZS5zdmMuY2x1c3Rlci5sb2NhbIIlbXktcG9kLm15LW5hbWVzcGFjZS5wb2QuY2x1c3Rlci5sb2NhbIcEwAACGIcECgAiAg==","Signature":"MEQCIGfRLBOfWXaDmPvNqAbE+ZhsJI819I7UY6/zZNt1D46iAiBFlTczpdx/o3tOt1RMUgjuINEx7Gy4W7rhLEgWv1ED1g==","SignatureAlgorithm":10,"Subject":{"CommonName":"my-pod.my-namespace.pod.cluster.local","Country":null,"ExtraNames":null,"Locality":null,"Names":[{"Type":[2,5,4,3],"Value":"my-pod.my-namespace.pod.cluster.local"}],"Organization":null,"OrganizationalUnit":null,"PostalCode":null,"Province":null,"SerialNumber":"","StreetAddress":null},"URIs":null,"Version":0}`,
		},
		{
			note: "non bas64 encoded; but PEM encoded certificate",
			cert: `-----BEGIN CERTIFICATE REQUEST-----
MIIBYTCCAQgCAQAwMDEuMCwGA1UEAxMlbXktcG9kLm15LW5hbWVzcGFjZS5wb2Qu
Y2x1c3Rlci5sb2NhbDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABMCc3zxOSbCv
OMb+hj8v1oZSkOtxK1zaveWrOYURyY2NQO0dPf7bnFCYHHJDPp1BSH1p+lW1/726
uNjMCEvtOtigdjB0BgkqhkiG9w0BCQ4xZzBlMGMGA1UdEQRcMFqCJW15LXN2Yy5t
eS1uYW1lc3BhY2Uuc3ZjLmNsdXN0ZXIubG9jYWyCJW15LXBvZC5teS1uYW1lc3Bh
Y2UucG9kLmNsdXN0ZXIubG9jYWyHBMAAAhiHBAoAIgIwCgYIKoZIzj0EAwIDRwAw
RAIgZ9EsE59ZdoOY+82oBsT5mGwkjzX0jtRjr/Nk23UPjqICIEWVNzOl3H+je063
VExSCO4g0THsbLhbuuEsSBa/UQPW
-----END CERTIFICATE REQUEST-----`,
			rule:     rule,
			expected: &Error{Code: BuiltinErr, Message: "illegal base64 data at input byte 0"},
		},
		{
			note:     "error when object input is passed",
			cert:     `{"foo": 1}`,
			rule:     rule,
			expected: &Error{Code: BuiltinErr, Message: "illegal base64 data at input byte 0"},
		},
		{
			note:     "valid base64 encoded; invalid PEM certificate",
			cert:     `LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0=`,
			rule:     rule,
			expected: &Error{Code: BuiltinErr, Message: "invalid PEM-encoded certificate signing request"},
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		rules := []string{
			fmt.Sprintf("cert = %q { true }", tc.cert),
			tc.rule,
		}
		runTopDownTestCase(t, data, tc.note, rules, tc.expected)
	}

}
