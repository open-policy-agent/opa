// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/util/test"
)

func TestNew(t *testing.T) {

	tests := []struct {
		input   string
		wantErr bool
	}{
		{
			input: `{
				"name": "foo",
				"url": "bad scheme://authority",
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url", "http://localhost/some/path",
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"token": "secret",
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"scheme": "Acmecorp-Token",
						"token": "secret"
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"client_tls": {}
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"client_tls": {
						"cert": "cert.pem"
					}
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
					}
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"environment_credentials": {}
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"metadata_credentials": {
							"aws_region": "us-east-1",
							"iam_role": "my_iam_role"
						}
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"metadata_credentials": {
							"aws_region": "us-east-1",
						}
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"metadata_credentials": {}
					}
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"metadata_credentials": {
							"aws_region": "us-east-1",
							"iam_role": "my_iam_role"
						},
						"environment_credentials": {}
					}
				}
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"s3_signing": {
						"environment_credentials": {}
					},
					"bearer": {
						"scheme": "Acmecorp-Token",
						"token": "secret"
					}					
				}
			}`,
			wantErr: true,
		},
	}

	var results []Client

	for _, tc := range tests {
		client, err := New([]byte(tc.input))
		if err != nil && !tc.wantErr {
			t.Fatalf("Unexpected error: %v", err)
		}
		results = append(results, client)
	}

	if results[3].config.Credentials.Bearer.Scheme != "Acmecorp-Token" {
		t.Fatalf("Expected custom token but got: %v", results[3].config.Credentials.Bearer.Scheme)
	}

}

func TestValidUrl(t *testing.T) {
	ts := testServer{
		t:         t,
		expMethod: "GET",
		expPath:   "/test",
	}
	ts.start()
	defer ts.stop()
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
	}`, ts.server.URL)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()
	if _, err := client.Do(ctx, "GET", "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func testBearerToken(t *testing.T, scheme, token string) {
	ts := testServer{
		t:               t,
		expBearerScheme: scheme,
		expBearerToken:  token,
	}
	ts.start()
	defer ts.stop()
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"credentials": {
			"bearer": {
				"scheme": "%s",
				"token": "%s"
			}
		}
	}`, ts.server.URL, scheme, token)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()
	if _, err := client.Do(ctx, "GET", "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestBearerTokenDefaultScheme(t *testing.T) {
	testBearerToken(t, "", "secret")
}

func TestBearerTokenCustomScheme(t *testing.T) {
	testBearerToken(t, "Acmecorp-Token", "secret")
}

func TestClientCert(t *testing.T) {
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

		ctx := context.Background()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Scramble the keys in the server
		ts.stop()
		ts.start()

		// Ensure the keys don't work anymore, make a new client as the url will have changed
		client = newTestClient(t, &ts, certPath, keyPath)
		_, err := client.Do(ctx, "GET", "test")
		expectedErrMsg := "tls: bad certificate"
		if err == nil || !strings.Contains(err.Error(), expectedErrMsg) {
			t.Fatalf("Expected '%s' error but request succeeded", expectedErrMsg)
		}

		// Update the key files and try again..
		if err := ioutil.WriteFile(filepath.Join(path, "client.pem"), ts.clientCertPem, 0644); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if err := ioutil.WriteFile(filepath.Join(path, "client.key"), ts.clientCertKey, 0644); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
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
			"credentials": {
				"client_tls": {
					"cert": %q,
					"private_key": %q
				}
			}
		}`, ts.server.URL, certPath, keypath)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return &client
}

type testServer struct {
	t                *testing.T
	server           *httptest.Server
	expPath          string
	expMethod        string
	expBearerToken   string
	expBearerScheme  string
	tls              bool
	clientCertPem    []byte
	clientCertKey    []byte
	expectClientCert bool
	serverCertPool   *x509.CertPool
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {
	if t.expMethod != "" && t.expMethod != r.Method {
		t.t.Fatalf("Expected method %v, got %v", t.expMethod, r.Method)
	}
	if t.expPath != "" && t.expPath != r.URL.Path {
		t.t.Fatalf("Expected path %q, got %q", t.expPath, r.URL.Path)
	}
	if (t.expBearerToken != "" || t.expBearerScheme != "") && len(r.Header["Authorization"]) == 0 {
		t.t.Fatal("Expected bearer token, but didn't get any")
	}
	if len(r.Header["Authorization"]) > 0 {
		auth := r.Header["Authorization"][0]
		if t.expBearerScheme != "" && !strings.HasPrefix(auth, t.expBearerScheme) {
			t.t.Fatalf("Expected bearer scheme %q, got authorization header %q", t.expBearerScheme, auth)
		}
		if t.expBearerToken != "" && !strings.HasSuffix(auth, t.expBearerToken) {
			t.t.Fatalf("Expected bearer token %q, got authorization header %q", t.expBearerToken, auth)
		}
	}
	if t.expectClientCert {
		if len(r.TLS.PeerCertificates) == 0 {
			t.t.Fatal("Expected client certificate but didn't get any")
		}
	}
	ua := r.Header.Get("user-Agent")
	if ua != version.UserAgent {
		t.t.Errorf("Unexpected User-Agent string: %s", ua)
	}

	w.WriteHeader(200)
}

func (t *testServer) generateClientKeys() {
	// generate a new set of root key+cert objects
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.t.Fatalf("generating random key: %v", err)
	}

	rootCertTmpl, err := certTemplate()
	if err != nil {
		t.t.Fatalf("creating cert template: %v", err)
	}
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	rootCert, rootCertPEM, err := createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.t.Fatalf("error creating cert: %v", err)
	}

	// save a copy of the root certificate for clients to use
	t.serverCertPool = x509.NewCertPool()
	t.serverCertPool.AppendCertsFromPEM(rootCertPEM)

	// create a key-pair for the client
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.t.Fatalf("generating random key: %v", err)
	}

	// create a template for the client
	clientCertTmpl, err := certTemplate()
	if err != nil {
		t.t.Fatalf("creating cert template: %v", err)
	}
	clientCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	clientCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	// the root cert signs the client cert
	_, t.clientCertPem, err = createCert(clientCertTmpl, rootCert, &clientKey.PublicKey, rootKey)
	if err != nil {
		t.t.Fatalf("error creating cert: %v", err)
	}

	// encode and load the cert and private key for the client
	t.clientCertKey = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})

}

func (t *testServer) start() {
	t.server = httptest.NewUnstartedServer(http.HandlerFunc(t.handle))

	if t.tls {
		t.generateClientKeys()
		t.server.TLS = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  t.serverCertPool,
		}
		t.server.StartTLS()
	} else {
		t.server.Start()
	}
}

func (t *testServer) stop() {
	t.server.Close()
}

// helper function to create a cert template with a serial number and other required fields
func certTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"OPA"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour), // valid for an hour
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

func createCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}
