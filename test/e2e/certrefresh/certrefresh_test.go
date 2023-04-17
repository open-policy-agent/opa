// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package certrefresh

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime
var pool *x509.CertPool

// print error to stderr, exit 1
func fatal(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

const (
	certFile0    = "testdata/server-cert.pem"
	certKeyFile0 = "testdata/server-key.pem"
	serial0      = "481849676048721749484276160748693385016044597443"
	certFile1    = "testdata/server-cert-new.pem"
	certKeyFile1 = "testdata/server-key-new.pem"
	serial1      = "481849676048721749484276160748693385016044597444"
)

var certFile, certKeyFile string

func TestMain(m *testing.M) {
	flag.Parse()
	caCertPEM, err := os.ReadFile("testdata/ca.pem")
	if err != nil {
		fatal(err)
	}
	pool = x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		fatal("failed to parse CA cert")
	}

	tmp, err := os.MkdirTemp("", "e2e_certrefresh")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(tmp)

	certFile = filepath.Join(tmp, "server-cert.pem")
	if err := copy(certFile0, certFile); err != nil {
		fatal(err)
	}

	certKeyFile = filepath.Join(tmp, "server-key.pem")
	if err := copy(certKeyFile0, certKeyFile); err != nil {
		fatal(err)
	}

	cert, err := tls.LoadX509KeyPair(certFile, certKeyFile)
	if err != nil {
		fatal(err)
	}

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"https://127.0.0.1:0"}
	testServerParams.CertPool = pool
	testServerParams.Certificate = &cert
	testServerParams.CertificateFile = certFile
	testServerParams.CertificateKeyFile = certKeyFile
	testServerParams.CertificateRefresh = time.Millisecond

	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		fatal(err)
	}

	// We need a client with proper TLS setup, otherwise the health check
	// that loops to determine if the server is ready will fail.
	testRuntime.Client = newClient()

	os.Exit(testRuntime.RunTests(m))
}

func TestCertificateRotation(t *testing.T) {
	wait := 20 * time.Millisecond // file reload happens every millisecond

	// before rotation
	cert := getCert(t)
	if exp, act := serial0, cert.SerialNumber.String(); exp != act {
		t.Fatalf("expected signature %s, got %s", exp, act)
	}

	// replace file on disk
	replaceCerts(t, certFile1, certKeyFile1)
	time.Sleep(wait)

	// after rotation
	cert = getCert(t)
	if exp, act := serial1, cert.SerialNumber.String(); exp != act {
		t.Fatalf("expected signature %s, got %s", exp, act)
	}

	// replace file with nothing
	replaceCerts(t, os.DevNull, os.DevNull)
	time.Sleep(wait)

	// second cert still used
	cert = getCert(t)
	if exp, act := serial1, cert.SerialNumber.String(); exp != act {
		t.Fatalf("expected signature %s, got %s", exp, act)
	}

	// go back to first cert
	replaceCerts(t, certFile0, certKeyFile0)
	time.Sleep(wait)
	cert = getCert(t)
	if exp, act := serial0, cert.SerialNumber.String(); exp != act {
		t.Fatalf("expected signature %s, got %s", exp, act)
	}
}

func newClient() *http.Client {
	c := *http.DefaultClient
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		RootCAs: pool,
	}
	c.Transport = tr
	return &c
}

func copy(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Ensure that our writes get committed to disk, even on slower systems.
	return dst.Sync()
}

func getCert(t *testing.T) *x509.Certificate {
	t.Helper()
	u, err := url.Parse(testRuntime.URL())
	if err != nil {
		t.Fatal(err)
	}
	c := newClient()
	cfg := c.Transport.(*http.Transport).TLSClientConfig
	conn, err := tls.Dial("tcp", u.Host, cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	return conn.ConnectionState().PeerCertificates[0]
}

func replaceCerts(t *testing.T, cert, key string) {
	t.Helper()

	if err := copy(cert, certFile); err != nil {
		t.Fatal(err)
	}
	if err := copy(key, certKeyFile); err != nil {
		t.Fatal(err)
	}
}
