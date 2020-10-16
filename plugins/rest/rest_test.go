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
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
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
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"oauth2": {
						"token_url": ""
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
					"oauth2": {
						"token_url": "http://localhost",
						"client_id": "client_one",
						"client_secret": "super_secret"
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
					"oauth2": {
						"token_url": "https://localhost",
						"client_id": ""
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
					"oauth2": {
						"token_url": "https://localhost",
						"client_id": "client_one"
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
					"oauth2": {
						"token_url": "https://localhost",
						"client_id": "client_one",
						"client_secret": "super_secret"
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"oauth2": {
						"token_url": "https://localhost",
						"client_id": "client_one",
						"client_secret": "super_secret",
						"scopes": ["profile", "opa"]
					}
				}
			}`,
		},
	}

	var results []Client

	for _, tc := range tests {
		client, err := New([]byte(tc.input))
		if err != nil && !tc.wantErr {
			t.Fatalf("Unexpected parse error: %v", err)
		}
		plugin, err := client.config.authPlugin()
		if err != nil {
			if tc.wantErr {
				continue
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = plugin.NewClient(client.config)
		if err != nil && !tc.wantErr {
			t.Fatalf("Unexpected error: %v", err)
		} else if err == nil && tc.wantErr {
			t.Fatalf("Excpected error for input %v", tc.input)
		}

		if *client.config.ResponseHeaderTimeoutSeconds != defaultResponseHeaderTimeoutSeconds {
			t.Fatalf("Expected default response header timeout but got %v seconds", *client.config.ResponseHeaderTimeoutSeconds)
		}

		results = append(results, client)
	}

	if results[3].config.Credentials.Bearer.Scheme != "Acmecorp-Token" {
		t.Fatalf("Expected custom token but got: %v", results[3].config.Credentials.Bearer.Scheme)
	}
}

func TestNewWithResponseHeaderTimeout(t *testing.T) {
	input := `{
				"name": "foo",
				"url": "http://localhost",
				"response_header_timeout_seconds": 20
			}`

	client, err := New([]byte(input))
	if err != nil {
		t.Fatal("Unexpected error")
	}

	if *client.config.ResponseHeaderTimeoutSeconds != 20 {
		t.Fatalf("Expected response header timeout %v seconds but got %v seconds", 20, *client.config.ResponseHeaderTimeoutSeconds)
	}
}

func TestDoWithResponseHeaderTimeout(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		d                     time.Duration
		responseHeaderTimeout string
		wantErr               bool
		errMsg                string
	}{
		"response_headers_timeout_not_met": {1, "2", false, ""},
		"response_headers_timeout_met":     {2, "1", true, "net/http: timeout awaiting response headers"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			baseURL, teardown := getTestServerWithTimeout(tc.d)
			defer teardown()

			config := fmt.Sprintf(`{
				"name": "foo",
				"url": %q,
				"response_header_timeout_seconds": %v,
			}`, baseURL, tc.responseHeaderTimeout)
			client, err := New([]byte(config))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			_, err = client.Do(ctx, "GET", "/v1/test")
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("Expected error %v but got %v", tc.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
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

func TestBearerTokenPath(t *testing.T) {
	ts := testServer{
		t:                  t,
		expBearerScheme:    "",
		expBearerToken:     "secret",
		expBearerTokenPath: true,
	}
	ts.start()
	defer ts.stop()

	files := map[string]string{
		"token.txt": "secret",
	}

	test.WithTempFS(files, func(path string) {
		tokenPath := filepath.Join(path, "token.txt")

		client := newTestBearerClient(t, &ts, tokenPath)

		ctx := context.Background()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Stop server and update the token
		ts.stop()
		ts.expBearerToken = "newsecret"
		ts.start()

		// check client cannot access the server
		client = newTestBearerClient(t, &ts, tokenPath)

		if resp, err := client.Do(ctx, "GET", "test"); err == nil {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("Expected http status %v but got %v", http.StatusUnauthorized, resp.StatusCode)
			}

			expectedErrMsg := "Expected bearer token \"newsecret\", got authorization header \"Bearer secret\""

			if string(bodyBytes) != expectedErrMsg {
				t.Fatalf("Expected error message %v but got %v", expectedErrMsg, string(bodyBytes))
			}
		} else {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Update the token file and try again
		if err := ioutil.WriteFile(filepath.Join(path, "token.txt"), []byte("newsecret"), 0644); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestBearerTokenInvalidConfig(t *testing.T) {
	ts := testServer{
		t:               t,
		expBearerScheme: "",
		expBearerToken:  "secret",
	}
	ts.start()
	defer ts.stop()

	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"credentials": {
			"bearer": {
				"token_path": "%s",
				"token": "%s"
			}
		}
	}`, ts.server.URL, "token.txt", "secret")
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ctx := context.Background()

	_, err = client.Do(ctx, "GET", "test")

	if err == nil {
		t.Fatalf("Expected error but got nil")
	}

	if !strings.HasPrefix(err.Error(), "invalid config") {
		t.Fatalf("Unexpected error message %v\n", err)
	}
}

func newTestBearerClient(t *testing.T, ts *testServer, tokenPath string) *Client {
	config := fmt.Sprintf(`{
			"name": "foo",
			"url": %q,
			"credentials": {
				"bearer": {
					"token_path": %q
				}
			}
		}`, ts.server.URL, tokenPath)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return &client
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

func TestClientCertPassword(t *testing.T) {
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

		ctx := context.Background()
		if _, err := client.Do(ctx, "GET", "test"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestOauth2ClientCredentials(t *testing.T) {
	tests := []struct {
		ts      *testServer
		ots     *oauth2TestServer
		options testPluginCustomizer
		wantErr bool
	}{
		{
			ts:  &testServer{t: t, expBearerToken: "token_1"},
			ots: &oauth2TestServer{t: t},
		},
		{
			ts:      &testServer{t: t, expBearerToken: "token_1"},
			ots:     &oauth2TestServer{t: t, tokenType: "unknown"},
			wantErr: true,
		},
		{
			ts:  &testServer{t: t},
			ots: &oauth2TestServer{t: t},
			options: func(c *Config) {
				c.Credentials.OAuth2.ClientSecret = "not_super_secret"
			},
			wantErr: true,
		},
		{
			ts:  &testServer{t: t},
			ots: &oauth2TestServer{t: t, expScope: &[]string{"read", "opa"}},
			options: func(c *Config) {
				c.Credentials.OAuth2.Scopes = &[]string{"read", "opa"}
			},
		},
	}

	for _, tc := range tests {
		func() {
			tc.ts.start()
			defer tc.ts.stop()
			tc.ots.start()
			defer tc.ots.stop()

			if tc.options == nil {
				tc.options = func(c *Config) {}
			}

			client := newOauth2TestClient(t, tc.ts, tc.ots, tc.options)
			ctx := context.Background()
			_, err := client.Do(ctx, "GET", "test")
			if err != nil && !tc.wantErr {
				t.Fatalf("Unexpected error: %v", err)
			} else if err == nil && tc.wantErr {
				t.Fatalf("Expected error: %v", err)
			}
		}()
	}
}

func TestOauth2ClientCredentialsExpiringTokenIsRefreshed(t *testing.T) {
	ts := testServer{
		t:              t,
		expBearerToken: "token_1",
	}
	ts.start()
	ots := oauth2TestServer{
		t: t,
		// Issue tokens with a TTL below our considered minimum - this should force the client to fetch a new one the
		// second time the credentials are used rather than reusing the token it has
		tokenTTL: 9,
	}
	ots.start()
	defer ots.stop()

	client := newOauth2TestClient(t, &ts, &ots)
	ctx := context.Background()
	_, err := client.Do(ctx, "GET", "test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	ts.stop()
	ts = testServer{
		t:              t,
		expBearerToken: "token_2",
	}
	ts.start()
	defer ts.stop()

	client = newOauth2TestClient(t, &ts, &ots)
	ctx = context.Background()
	_, err = client.Do(ctx, "GET", "test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestOauth2ClientCredentialsNonExpiringTokenIsReused(t *testing.T) {
	ts := testServer{
		t:              t,
		expBearerToken: "token_1",
	}
	ts.start()
	defer ts.stop()

	ots := oauth2TestServer{
		t:        t,
		tokenTTL: 300,
	}
	ots.start()
	defer ots.stop()

	client := newOauth2TestClient(t, &ts, &ots)
	ctx := context.Background()
	_, err := client.Do(ctx, "GET", "test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	_, err = client.Do(ctx, "GET", "test")
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
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
	if ts.clientCertPassword != "" {
		client.Config().Credentials.ClientTLS.PrivateKeyPassphrase = ts.clientCertPassword
	}

	return &client
}

type testPluginCustomizer func(c *Config)

type testServer struct {
	t                  *testing.T
	server             *httptest.Server
	expPath            string
	expMethod          string
	expBearerToken     string
	expBearerScheme    string
	expBearerTokenPath bool
	tls                bool
	clientCertPem      []byte
	clientCertKey      []byte
	clientCertPassword string
	expectClientCert   bool
	serverCertPool     *x509.CertPool
}

type oauth2TestServer struct {
	t               *testing.T
	server          *httptest.Server
	expClientID     string
	expClientSecret string
	expScope        *[]string
	tokenType       string
	tokenTTL        int64
	invocations     int32
}

func newOauth2TestClient(t *testing.T, ts *testServer, ots *oauth2TestServer, options ...testPluginCustomizer) *Client {
	config := fmt.Sprintf(`{
			"name": "foo",
			"url": %q,
			"allow_insecure_tls": true,
			"credentials": {
				"oauth2": {
					"token_url": "%v/token",
					"client_id": "client_one",
					"client_secret": "super_secret"
				}
			}
		}`, ts.server.URL, ots.server.URL)
	client, err := New([]byte(config))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, option := range options {
		option(client.Config())
	}

	return &client
}

func (t *oauth2TestServer) start() {
	if t.tokenTTL == 0 {
		t.tokenTTL = 3600
	}
	if t.expScope == nil {
		t.expScope = &[]string{}
	}
	if t.tokenType == "" {
		t.tokenType = "bearer"
	}
	t.expClientID = "client_one"
	t.expClientSecret = "super_secret"

	t.server = httptest.NewUnstartedServer(http.HandlerFunc(t.handle))

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.t.Fatalf("generating random key: %v", err)
	}
	_, rootCertPem, err := createRootCert(rootKey)
	if err != nil {
		t.t.Fatalf("creating root cert: %v", err)
	}

	serverCertPool := x509.NewCertPool()
	serverCertPool.AppendCertsFromPEM(rootCertPem)
	t.server.TLS = &tls.Config{
		RootCAs: serverCertPool,
	}
	t.server.StartTLS()
}

func (t *oauth2TestServer) stop() {
	t.server.Close()
}

func (t *oauth2TestServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		t.t.Fatalf("Expected method POST, got %v", r.Method)
	}
	if r.URL.Path != "/token" {
		t.t.Fatalf("Expected path /token got %q", r.URL.Path)
	}

	if err := r.ParseForm(); err != nil {
		t.t.Fatal(err)
	}
	if r.Form["grant_type"][0] != "client_credentials" {
		t.t.Fatal("Expected grant_type=client_credentials")
	}

	if len(r.Form["scope"]) > 0 {
		scope := strings.Split(r.Form["scope"][0], " ")
		if !reflect.DeepEqual(*t.expScope, scope) {
			t.t.Fatalf("Expected scope %v, got %v", *t.expScope, scope)
		}
	} else if t.expScope != nil && len(*t.expScope) > 0 {
		t.t.Fatal("Expected scope to be provided")
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	split := strings.Split(authHeader, " ")
	credentials := split[len(split)-1]

	decoded, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		t.t.Fatal(err)
	}

	pair := strings.SplitN(string(decoded), ":", 2)
	if len(pair) != 2 || pair[0] != t.expClientID || pair[1] != t.expClientSecret {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error"": "invalid_client"}`))
		return
	}

	t.invocations++
	token := fmt.Sprintf("token_%v", t.invocations)

	w.WriteHeader(http.StatusOK)
	body := fmt.Sprintf(`{"token_type": "%v", "access_token": "%v", "expires_in": %v}`, t.tokenType, token, t.tokenTTL)
	_, _ = w.Write([]byte(body))
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
			errMsg := fmt.Sprintf("Expected bearer scheme %q, got authorization header %q", t.expBearerScheme, auth)
			if t.expBearerTokenPath {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(errMsg))
				return
			}
			t.t.Fatalf(errMsg)
		}
		if t.expBearerToken != "" && !strings.HasSuffix(auth, t.expBearerToken) {
			errMsg := fmt.Sprintf("Expected bearer token %q, got authorization header %q", t.expBearerToken, auth)
			if t.expBearerTokenPath {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(errMsg))
				return
			}
			t.t.Fatalf(errMsg)
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
	rootCert, rootCertPEM, err := createRootCert(rootKey)
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

	var pemBlock *pem.Block
	if t.clientCertPassword != "" {
		pemBlock, err = x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(clientKey),
			[]byte(t.clientCertPassword), x509.PEMCipherAES128)
		if err != nil {
			t.t.Fatalf("error encrypting pem block: %v", err)
		}
	} else {
		pemBlock = &pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
		}
	}

	// encode and load the cert and private key for the client
	t.clientCertKey = pem.EncodeToMemory(pemBlock)
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

func createRootCert(rootKey *rsa.PrivateKey) (cert *x509.Certificate, certPEM []byte, err error) {
	rootCertTmpl, err := certTemplate()
	if err != nil {
		return nil, nil, err
	}
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	return createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
}

func getTestServerWithTimeout(d time.Duration) (baseURL string, teardownFn func()) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc("/v1/test", func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(d * time.Second)
		w.WriteHeader(http.StatusOK)
	})
	return ts.URL, ts.Close
}
