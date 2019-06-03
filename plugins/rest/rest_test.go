// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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

	if results[2].config.Credentials.Bearer.Scheme != "Bearer" {
		t.Fatalf("Expected token scheme to be set to Bearer but got: %v", results[2].config.Credentials.Bearer.Scheme)
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

func TestBearerToken(t *testing.T) {
	ts := testServer{
		t:               t,
		expBearerScheme: "Acmecorp-Token",
		expBearerToken:  "secret",
	}
	ts.start()
	defer ts.stop()
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"credentials": {
			"bearer": {
				"scheme": "Acmecorp-Token",
				"token": "secret"
			}
		}
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

func TestClientCert(t *testing.T) {
	ts := testServer{
		t:                t,
		tls:              true,
		expectClientCert: true,
	}
	ts.start()
	defer ts.stop()
	tmpPem, err := ioutil.TempFile("", "client.pem")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.Remove(tmpPem.Name())
	tmpKey, err := ioutil.TempFile("", "client.key")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.Remove(tmpKey.Name())
	if _, err := tmpPem.Write([]byte(ts.clientCertPem)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := tmpPem.Close(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, err := tmpKey.Write([]byte(ts.clientCertKey)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := tmpKey.Close(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"allow_insecure_tls": true,
		"credentials": {
			"client_tls": {
				"cert": %q,
				"private_key": %q,
				"private_key_passphrase": "secret",
			}
		}
	}`, ts.server.URL, tmpPem.Name(), tmpKey.Name())
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()
	if _, err := client.Do(ctx, "GET", "test"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	w.WriteHeader(200)
}

var (
	clientCertPem = []byte(`-----BEGIN CERTIFICATE-----
MIIC/jCCAeYCAQEwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCc2UxEzARBgNV
BAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0
ZDAeFw0xODEwMTQyMTMwNTVaFw0xOTEwMTQyMTMwNTVaMEUxCzAJBgNVBAYTAkFV
MRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRz
IFB0eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDEPiWZCKrb
FIusaNlOJ4R41ARd63PVJglwYxxoOfBUVvmgh7Sq5ccQDWQvs5QpQSt6HcQHsoS+
behOxl13sW7UY2nQiBSmFqnd8PkgZg89q9tmk0cRdrl90crCs72Lt3t/AgRC1YEz
WQ7Fa2ig/k60ftwOq98Ogsjc6+/ToIiZD2BKy/3DHTl5TXNuPCSvZCKkFGM3zlse
H5UtY1ZaO5gFC+SotJ0RrGfEiJY6nuqXcMRHTj5NluGZhkQR/1TdHa9nAWG2TlUD
IEabN5yggtvH0Wz0lQD7okTyOOC+X9gUbOoILUR98SUytAYaiiPbAAlOpvdSjPS+
LzT22wBSPjRxAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAKMW44mCKXIjx7p+c8pn
qmmaioYtXrWeDE3/gAKhkB3Z4pY4ajEGogGNP19t2DoGDx7y2KJpMA777HoagclW
HFATMYN1J6YSkTrXFJnItvaQnv8mMqK4xR2kN4yO1CEITANakhu4pYZn0oU1sxEY
R3Cl3YMMR/OHoPpR2FKaX0G67xZZ2SXHf2jN2KsRV38PHfmb4ASX3Cbg6hzl1+du
ORxvL+DSwh2/n8Vdby0SdRQ7BxfqtSaIRogtScN2QzquaHeW1ErENfRqmeV/XHJr
1bmaSvfZe+CZnlLCeTlHcxu0i1fkdoYgi/oRWFPI1DBH6F1cGY+wWMuS4Job1zOK
OtQ=
-----END CERTIFICATE-----
`)
	clientCertKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: DES-EDE3-CBC,64355BD1C400418C

6gIaC8pHDkQFFDBEInaH5V28EhU/d5lo+XUzGxPLdUKW2dgy+WgLq7apD0YwMh1N
8FEazXR1h++WihI1zNaoOdLyGjW1uyzi4RiuWpZPzbn4Ms4nPy8ZOEqMJZD2XTIG
19AibbBfUO9nmfs3G0afcgEM9QhWcij7DYO9QJBc3WEz5A4zC5jO+r2V4b5a2a1Q
UL0vWtuBEJ0XVaog9AGWOr4dV5rfpDIOKEMkGYURyUVe67AeL8Km5iuMdPyDuyjw
mEedculc15i3QwmVeJGNkhIbr8VRBQbzvdZlI4VeicFwLfhGS0pRTupKEVZuIpXI
rRWnCvCNJVIXIzqCWDknsJz4UrR8fXSpjuX3+R5XOJVvkgwlR5At5CbsVSu8T2NS
NAJzXLaqpl4QJCOIG4oTIZ9oZ8x/dcOE/ey8/TLXHknlKJvKfvvpEhIFcyKgF4p4
jJEaAE2o67V82hENZB8WhiXb5IaUFGPlii1B1L56mKnP8gv6HHBuxjmQ/DdzBOvZ
DVPyZ/Yy7VpcYr/5iGNEjEW9Td4apPAQTAkcEOrj9v5nslHUuyRnzC07hKa/gvG2
hrd9CgQ6ZmgT/AczASA0sbvliDwpSvublEAHiBtwWe5rmxxcFE937ljU/QdU6jiR
abxymx3gHZMIIG5YoqbXhuntYXeiZdiCTn62n9yO/7Lvps4kIBqL11fZenkuDxR9
QIDgoxzIZX3Ts16fUJdEVoPd0kLizuntuiNVUFukhKyz8eBzTI8xYgfnPjisxdEb
eU7Wzw/jntu5DHjUnREyiWLZK+MDCYk2wdlqMR4+4p0hWGBPgK2o9QNW2j1MePsO
pAhV0YBYKt1VMNrQv5M89DWkFFffuj7IZUsUPeO7A4Gs7NJ9eArjCKijKptX2Osw
Lh1Zb6assP6Mqd63UUINC31FQwwTjXjApA/sRYsVTQplbMJ1RVF4IRdH8K/HKVRW
1ACB+DZqDox5eS7xjxQ+tJozU1LdDOi8i7M673IFF7vAFzSHROsXgOkxZbVEEwrQ
F8rtogulYjgZpHEOWAcla961nE/j+wDzC8Uc9XNjvBnDyTeVqaA8aYzbDWOVTZ9n
i5HJgvaGdEQVt6tWGKDtGTUYLHHhXSslRKh77gprA2wuofR1qXzgEij2h7KIcoqA
kw/e2lwc4XFhU1/6mZSD8X3B8oOQQegv4h55xJzO7lZUNb2yXjAlm3HwD1tl3499
YfwbxGI9OAMomlq61W2rPDMWeDN0v9vSJ9iebE7rPe4A3RJwdfm2lYui0B+o/rLB
ppmX9Mv5LVFaXsDI4q41tziQOM26WhOzx+vF8h1l+aeTo5G3mTlT64+mJ26HcPDP
c+jtZ0vWdvf67HTncZxhoITFd5wKp2yru8wRTCT+VCSABZfMZQ6SYGyzVP+Wgf6t
U65k7iKsT2gUhk5QJIg0ZGvERDiGLXupcoyGhuoZhLm4HmmOZzvDx6f9VjM7Npt0
IJdvDV2sh3QXk4LTwn/0gCw+LxBBuubw3XKYyRKbzw6jYgqsazRNVn2zdkuchcc8
EnVu9NNEzAkTEEYIG99ECBmCIR9QknQXfqHRa5zNBndjBPJuOyVUwA==
-----END RSA PRIVATE KEY-----
`)
)

func (t *testServer) start() {
	t.server = httptest.NewUnstartedServer(http.HandlerFunc(t.handle))
	t.clientCertPem = clientCertPem
	t.clientCertKey = clientCertKey
	if t.tls {
		t.server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
		t.server.StartTLS()
	} else {
		t.server.Start()
	}
}

func (t *testServer) stop() {
	t.server.Close()
}
