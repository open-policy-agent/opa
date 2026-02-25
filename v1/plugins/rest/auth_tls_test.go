package rest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-policy-agent/opa/v1/logging"
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

		refreshDuration := 5 * time.Minute
		plugin := &clientTLSAuthPlugin{
			Cert:                certPath,
			PrivateKey:          keyPath,
			CertRefreshDuration: refreshDuration,
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
