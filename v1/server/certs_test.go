package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestCertNotifyWatcherIsReadyWhenListenersReturn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	poolPath := filepath.Join(dir, "ca.pem")

	initialPEM, serverCert := newSelfSignedCert(t, "initial")
	if err := os.WriteFile(poolPath, initialPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	initialPool := x509.NewCertPool()
	initialPool.AppendCertsFromPEM(initialPEM)

	m, err := plugins.New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	server := New().
		WithAddresses([]string{"127.0.0.1:0"}).
		WithStore(inmem.New()).
		WithManager(m).
		WithCertificate(&serverCert).
		WithCertPool(initialPool).
		WithTLSConfig(&TLSConfig{CertPoolFile: poolPath})

	if err := m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	server, err = server.Init(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	loops, err := server.Listeners()
	if err != nil {
		t.Fatal(err)
	}

	// Replace the CA file before any loop runs. A change made once Listeners has returned must not be
	// missed, regardless of how long the loops take to get scheduled.
	updatedPEM, _ := newSelfSignedCert(t, "updated")
	updatedPath := filepath.Join(dir, "ca-updated.pem")
	if err := os.WriteFile(updatedPath, updatedPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(updatedPath, poolPath); err != nil {
		t.Fatal(err)
	}

	for _, loop := range loops {
		go func() { _ = loop() }()
	}

	updatedPool := x509.NewCertPool()
	updatedPool.AppendCertsFromPEM(updatedPEM)

	test.EventuallyOrFatal(t, 3*time.Second, func() bool {
		server.tlsConfigMtx.RLock()
		defer server.tlsConfigMtx.RUnlock()

		return server.certPool.Equal(updatedPool)
	})
}

func newSelfSignedCert(t *testing.T, commonName string) ([]byte, tls.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		BasicConstraintsValid: true,
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
}
