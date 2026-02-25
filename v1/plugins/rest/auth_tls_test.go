package rest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func generateTestCertificate(t *testing.T, certPath, keyPath string, serialNumber int64) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(serialNumber),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "Test Cert",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to open cert file for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatalf("failed to write certificate: %v", err)
	}
	if err := certOut.Close(); err != nil {
		t.Fatalf("error closing cert file: %v", err)
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to open key file for writing: %v", err)
	}
	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		t.Fatalf("error closing key file: %v", err)
	}
}

func TestClientTLSAuthPlugin_CertificateRotation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tmpDir := t.TempDir()
		certPath := filepath.Join(tmpDir, "cert.pem")
		keyPath := filepath.Join(tmpDir, "key.pem")

		generateTestCertificate(t, certPath, keyPath, 1)

		refreshDurationSeconds := int64(5 * 60)
		plugin := &clientTLSAuthPlugin{
			Cert:                       certPath,
			PrivateKey:                 keyPath,
			CertRefreshDurationSeconds: &refreshDurationSeconds,
		}

		config := Config{
			URL:                          "https://example.com",
			ResponseHeaderTimeoutSeconds: &[]int64{10}[0], // NB(sr): new(0) when go.mod says 1.26
			logger:                       logging.New(),
		}

		client, err := plugin.NewClient(config)
		if err != nil {
			t.Fatalf("NewClient() failed: %v", err)
		}

		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatal("client transport is not *http.Transport")
		}

		if transport.TLSClientConfig.GetClientCertificate == nil {
			t.Fatal("client transport is has no GetClientCertificate")
		}
		cert1, err := transport.TLSClientConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
		if err != nil {
			t.Fatalf("first GetClientCertificate failed: %v", err)
		}

		if len(cert1.Certificate) == 0 {
			t.Fatal("first certificate is empty")
		}

		parsedCert1, err := x509.ParseCertificate(cert1.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse first certificate: %v", err)
		}

		if parsedCert1.SerialNumber.Int64() != 1 {
			t.Errorf("first certificate serial number = %d, want 1", parsedCert1.SerialNumber.Int64())
		}

		time.Sleep(3 * time.Minute)

		cert2, err := transport.TLSClientConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
		if err != nil {
			t.Fatalf("second GetClientCertificate failed: %v", err)
		}

		parsedCert2, err := x509.ParseCertificate(cert2.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse second certificate: %v", err)
		}

		if parsedCert2.SerialNumber.Int64() != 1 {
			t.Errorf("second certificate serial number = %d, want 1 (should be cached)", parsedCert2.SerialNumber.Int64())
		}

		generateTestCertificate(t, certPath, keyPath, 2)

		time.Sleep(3 * time.Minute)

		cert3, err := transport.TLSClientConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
		if err != nil {
			t.Fatalf("third GetClientCertificate failed: %v", err)
		}

		parsedCert3, err := x509.ParseCertificate(cert3.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse third certificate: %v", err)
		}

		if parsedCert3.SerialNumber.Int64() != 2 {
			t.Errorf("third certificate serial number = %d, want 2 (should be reloaded)", parsedCert3.SerialNumber.Int64())
		}

		if parsedCert1.SerialNumber.Cmp(parsedCert3.SerialNumber) == 0 {
			t.Error("certificate was not rotated after refresh duration")
		}
	})
}

func TestClientTLSAuthPlugin_ConfigParsing(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	generateTestCertificate(t, certPath, keyPath, 1)

	tests := []struct {
		name                      string
		buildConfig               func(cert, key, ca string) string
		expectSystemCARequired    bool
		expectCertRefreshDuration time.Duration
		expectError               bool
	}{
		{
			name: "system_ca_required true",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"system_ca_required": true
						}
					}
				}`, cert, key)
			},
			expectSystemCARequired: true,
		},
		{
			name: "system_ca_required false",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"system_ca_required": false
						}
					}
				}`, cert, key)
			},
			expectSystemCARequired: false,
		},
		{
			name: "cert_refresh_duration_seconds set (5 minutes)",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"cert_refresh_duration_seconds": %d
						}
					}
				}`, cert, key, int64(5*60))
			},
			expectSystemCARequired:    false,
			expectCertRefreshDuration: time.Minute * 5,
		},
		{
			name: "both system_ca_required and cert_refresh_duration_seconds",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"system_ca_required": true,
							"cert_refresh_duration_seconds": %d
						}
					}
				}`, cert, key, int64(10*60+30))
			},
			expectSystemCARequired:    true,
			expectCertRefreshDuration: 10*time.Minute + 30*time.Second,
		},
		{
			name: "cert_refresh_duration_seconds omitted",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q
						}
					}
				}`, cert, key)
			},
			expectSystemCARequired:    false,
			expectCertRefreshDuration: time.Duration(0),
		},
		{
			name: "cert_refresh_duration_seconds with hours (2h)",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"cert_refresh_duration_seconds": %d
						}
					}
				}`, cert, key, int64(2*60*60))
			},
			expectSystemCARequired:    false,
			expectCertRefreshDuration: 2 * time.Hour,
		},
		{
			name: "deprecated ca_cert field with system_ca_required",
			buildConfig: func(cert, key, ca string) string {
				return fmt.Sprintf(`{
					"name": "test",
					"url": "https://example.com",
					"credentials": {
						"client_tls": {
							"cert": %q,
							"private_key": %q,
							"ca_cert": %q,
							"system_ca_required": true
						}
					}
				}`, cert, key, ca)
			},
			expectSystemCARequired: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.buildConfig(certPath, keyPath, certPath)

			client, err := New([]byte(config), map[string]*keys.Config{})
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if client.config.Credentials.ClientTLS == nil {
				t.Fatal("ClientTLS credentials not parsed")
			}

			if client.config.Credentials.ClientTLS.SystemCARequired != tc.expectSystemCARequired {
				t.Errorf("SystemCARequired = %v, want %v",
					client.config.Credentials.ClientTLS.SystemCARequired,
					tc.expectSystemCARequired)
			}

			if tc.expectCertRefreshDuration == 0 {
				if client.config.Credentials.ClientTLS.CertRefreshDurationSeconds != nil {
					t.Errorf("CertRefreshDurationSeconds = %v, want nil",
						client.config.Credentials.ClientTLS.CertRefreshDurationSeconds)
				}
			} else {
				if client.config.Credentials.ClientTLS.CertRefreshDurationSeconds == nil {
					t.Errorf("CertRefreshDurationSeconds = nil, want %v", tc.expectCertRefreshDuration)
				} else if time.Duration(*client.config.Credentials.ClientTLS.CertRefreshDurationSeconds)*time.Second != tc.expectCertRefreshDuration {
					t.Errorf("CertRefreshDurationSeconds = %v, want %v",
						*client.config.Credentials.ClientTLS.CertRefreshDurationSeconds,
						tc.expectCertRefreshDuration)
				}
			}
		})
	}
}

func TestClientCert(t *testing.T) {
	t.Parallel()

	ts := testServer{
		t:                t,
		tls:              true,
		expectClientCert: true,
	}
	ts.start()
	defer ts.stop()

	files := map[string]string{
		"client.pem": string(ts.clientCertPem),
		"client.key": string(ts.clientCertKey),
	}

	test.WithTempFS(files, func(path string) {
		certPath := filepath.Join(path, "client.pem")
		keyPath := filepath.Join(path, "client.key")

		client := newTestClient(t, &ts, certPath, keyPath)

		ctx := t.Context()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Scramble the keys in the server
		ts.stop()
		ts.start()

		// Ensure the keys don't work anymore, make a new client as the url will have changed
		client = newTestClient(t, &ts, certPath, keyPath)
		_, err := client.Do(ctx, "GET", "test")
		expectedErrMsg := func(s string) bool {
			switch {
			case strings.Contains(s, "tls: unknown certificate authority"):
			case strings.Contains(s, "tls: bad certificate"):
			default:
				return false
			}
			return true
		}
		if err == nil || !expectedErrMsg(err.Error()) {
			t.Fatalf("Unexpected error %v", err)
		}

		// Update the key files and try again..
		if err := os.WriteFile(filepath.Join(path, "client.pem"), ts.clientCertPem, 0600); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if err := os.WriteFile(filepath.Join(path, "client.key"), ts.clientCertKey, 0600); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestClientCertPassword(t *testing.T) {
	t.Parallel()

	ts := testServer{
		t:                  t,
		tls:                true,
		expectClientCert:   true,
		clientCertPassword: "password",
	}
	ts.start()
	defer ts.stop()

	files := map[string]string{
		"client.pem": string(ts.clientCertPem),
		"client.key": string(ts.clientCertKey),
	}

	test.WithTempFS(files, func(path string) {
		certPath := filepath.Join(path, "client.pem")
		keyPath := filepath.Join(path, "client.key")

		client := newTestClient(t, &ts, certPath, keyPath)

		ctx := t.Context()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestClientTLSWithCustomCACert(t *testing.T) {
	t.Parallel()

	ts := testServer{
		t:                t,
		tls:              true,
		expectClientCert: true,
	}
	ts.start()
	defer ts.stop()

	files := map[string]string{
		"client.pem": string(ts.clientCertPem),
		"client.key": string(ts.clientCertKey),
		"ca.pem":     string(ts.rootCertPEM),
	}

	test.WithTempFS(files, func(path string) {
		certPath := filepath.Join(path, "client.pem")
		keyPath := filepath.Join(path, "client.key")
		ts.caCert = filepath.Join(path, "ca.pem")

		client := newTestClient(t, &ts, certPath, keyPath)

		ctx := t.Context()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestClientTLSWithCustomCACertAndSystemCA(t *testing.T) {
	t.Parallel()

	ts := testServer{
		t:                t,
		tls:              true,
		expectClientCert: true,
		expectSystemCA:   true,
	}
	ts.start()
	defer ts.stop()

	files := map[string]string{
		"client.pem": string(ts.clientCertPem),
		"client.key": string(ts.clientCertKey),
		"ca.pem":     string(ts.rootCertPEM),
	}

	test.WithTempFS(files, func(path string) {
		certPath := filepath.Join(path, "client.pem")
		keyPath := filepath.Join(path, "client.key")
		ts.caCert = filepath.Join(path, "ca.pem")

		client := newTestClient(t, &ts, certPath, keyPath)

		ctx := t.Context()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func newTestClient(t *testing.T, ts *testServer, certPath string, keypath string) *Client {
	config := fmt.Sprintf(`{
			"name": "foo",
			"url": %q,
			"allow_insecure_tls": true,
			"tls": {"ca_cert": %q, system_ca_required: %v},
			"credentials": {
				"client_tls": {
					"cert": %q,
					"private_key": %q
				}
			}
		}`, ts.server.URL, ts.caCert, ts.expectSystemCA, certPath, keypath)
	client, err := New([]byte(config), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if ts.clientCertPassword != "" {
		client.Config().Credentials.ClientTLS.PrivateKeyPassphrase = ts.clientCertPassword
	}

	return &client
}
