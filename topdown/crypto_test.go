package topdown

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestX509ParseAndVerify(t *testing.T) {
	rootCA := `-----BEGIN CERTIFICATE-----
MIIBoDCCAUagAwIBAgIRAJXcMYZALXooNq/VV/grXhMwCgYIKoZIzj0EAwIwLjER
MA8GA1UEChMIT1BBIFRlc3QxGTAXBgNVBAMTEE9QQSBUZXN0IFJvb3QgQ0EwHhcN
MjEwNzAxMTc0MTUzWhcNMzEwNjI5MTc0MTUzWjAuMREwDwYDVQQKEwhPUEEgVGVz
dDEZMBcGA1UEAxMQT1BBIFRlc3QgUm9vdCBDQTBZMBMGByqGSM49AgEGCCqGSM49
AwEHA0IABFqhdZA5LjsJgzsBvhgzfayZFOk+C7PmGCi7xz6zOC3xWORJZSNOyZeJ
YzSKFmoMZkcFMfslTW1jp9fwe1xl3HWjRTBDMA4GA1UdDwEB/wQEAwIBBjASBgNV
HRMBAf8ECDAGAQH/AgEBMB0GA1UdDgQWBBTch60qxQvLl+AfDfcaXmjvT8GvpzAK
BggqhkjOPQQDAgNIADBFAiBqraIP0l2U0oNuH0+rf36hDks94wSB5EGlGH3lYNMR
ugIhANkbukX5hOP8pJDRWP/pYuv6MBnRY4BS8gpp9Vu31qOb
-----END CERTIFICATE-----`
	intermediateCA := `-----BEGIN CERTIFICATE-----
MIIByDCCAW6gAwIBAgIQC0k4DPGrh9me73EJX5zntTAKBggqhkjOPQQDAjAuMREw
DwYDVQQKEwhPUEEgVGVzdDEZMBcGA1UEAxMQT1BBIFRlc3QgUm9vdCBDQTAeFw0y
MTA3MDExNzQxNTNaFw0zMTA2MjkxNzQxNTNaMDYxETAPBgNVBAoTCE9QQSBUZXN0
MSEwHwYDVQQDExhPUEEgVGVzdCBJbnRlcm1lZGlhdGUgQ0EwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAARvXQa7fy476gDI81nqLYb2SnD459WxBmU0hk2bA3ZuNtI+
H20KXz6ISmxH3MZ2WBm6rOy7y4Gn+WMCJuxzcl5jo2YwZDAOBgNVHQ8BAf8EBAMC
AQYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUuslZNjJl0V8I1Gj17IID
ALy/9WEwHwYDVR0jBBgwFoAU3IetKsULy5fgHw33Gl5o70/Br6cwCgYIKoZIzj0E
AwIDSAAwRQIgUwsYApW9Tsm6AstWswaKGie0srB4FUkUbfKwWmUI2JgCIQCBTySN
MF+EiQAMKyz/N9KUuXEckC356WvKcyJaYYcV0w==
-----END CERTIFICATE-----`
	leaf := `-----BEGIN CERTIFICATE-----
MIIB8zCCAZqgAwIBAgIRAID4gPKg7DDiuOfzUYFSXLAwCgYIKoZIzj0EAwIwNjER
MA8GA1UEChMIT1BBIFRlc3QxITAfBgNVBAMTGE9QQSBUZXN0IEludGVybWVkaWF0
ZSBDQTAeFw0yMTA3MDUxNzQ5NTBaFw0zNjA3MDExNzQ5NDdaMCUxIzAhBgNVBAMT
Gm5vdGFyZWFsc2l0ZS5vcGEubG9jYWxob3N0MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAE1YSXZXeaGGL+XeYyoPi/QdA39Ds4fgxSHJTMh+js393kByPm2PNtFkem
tUii3KCRJw3SEh3z0JWr/9y4+ua2L6OBmTCBljAOBgNVHQ8BAf8EBAMCB4AwHQYD
VR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBRL0P0g17viZHo9
CnXe3ZQJm48LXTAfBgNVHSMEGDAWgBS6yVk2MmXRXwjUaPXsggMAvL/1YTAlBgNV
HREEHjAcghpub3RhcmVhbHNpdGUub3BhLmxvY2FsaG9zdDAKBggqhkjOPQQDAgNH
ADBEAiAtmZewL94ijN0YwUGaJM9BXCaoTQPwkzugqjCj+K912QIgKKFvbPu4asrE
nwy7dzejHmQUcZ/aUNbc4VTbiv15ESk=
-----END CERTIFICATE-----`

	t.Run("TestFullChainPEM", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, intermediateCA, leaf}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err != nil {
			t.Error("x509 verification failed when it was expected to succeed")
		}
	})

	t.Run("TestFullChainBase64", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, intermediateCA, leaf}, "\n")
		b64 := base64.StdEncoding.EncodeToString([]byte(chain))

		parsed, err := getX509CertsFromString(b64)
		if err != nil {
			t.Fatalf("failed to parse base64 cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err != nil {
			t.Error("x509 verification failed when it was expected to succeed")
		}
	})

	t.Run("TestWrongOrder", func(t *testing.T) {
		chain := strings.Join([]string{leaf, intermediateCA, rootCA}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})

	t.Run("TestMissingIntermediate", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, leaf}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})

	t.Run("TestTooFewCerts", func(t *testing.T) {
		parsed, err := getX509CertsFromString(leaf)
		if err != nil {
			t.Fatalf("failed to parse leaf cert: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})
}
