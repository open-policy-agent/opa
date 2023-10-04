//go:build slow
// +build slow

package download

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/plugins/rest"
)

var errUnauthorized = errors.New("401 Unauthorized")

type testFixture struct {
	d                         *Downloader
	client                    rest.Client
	server                    *testServer
	updates                   []Update
	mockBundleActivationError bool
	etags                     map[string]string
}

func newTestFixture(t *testing.T, opts ...fixtureOpt) testFixture {
	t.Helper()

	ts := newTestServer(t)
	ts.start()

	restConfig := []byte(fmt.Sprintf(`{"url": %q}`, ts.server.URL))

	client, err := rest.New(restConfig, map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	fixture := testFixture{
		server: ts,
		client: client,
		etags:  make(map[string]string),
	}

	for i, opt := range opts {
		if err := opt(&fixture); err != nil {
			t.Fatalf("Failed applying option #%d: %s", i, err)
		}
	}

	return fixture
}

type fixtureOpt func(*testFixture) error

// withPublicRegistryAuth sets up a token auth flow according to
// the spec https://docs.docker.com/registry/spec/auth/token/.
//
// This authentication method is implemented by public
// repositories of Github Container Registry, Docker Hub and
// AWS ECR (and likely others) and corresponds with the auth
// method `token` of the github.com/distribution/distribution
// registry project.
// See https://docs.docker.com/registry/configuration/#token.
//
// The token issuing and validation differs between providers
// and we only use a minimal version for testing.
func withPublicRegistryAuth() fixtureOpt {
	const token = "some-test-token"
	tokenServer := httptest.NewServer(tokenHandler(token))

	const wwwAuthenticateFmt = "Bearer realm=%q service=%q scope=%q"
	tokenServiceURL := tokenServer.URL + "/token"
	wwwAuthenticate := fmt.Sprintf(wwwAuthenticateFmt,
		tokenServiceURL,
		"testRegistry.io",
		"[pull]")

	return func(tf *testFixture) error {
		tf.server.customAuth = func(w http.ResponseWriter, r *http.Request) error {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("no authorization header: %w", errUnauthorized)
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("expects bearer scheme: %w", errUnauthorized)
			}

			bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
			if bearerToken != token {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("token %q doesn't match %q: %w", bearerToken, token, errUnauthorized)
			}

			return nil
		}

		return nil
	}
}

// withAuthenticatedTokenAuth sets up a token auth flow according to
// the spec https://docs.docker.com/registry/spec/auth/token/.
//
// The flow is the same as for public registries but additionally
// the request for fetching the token also has to be authenticated.
// Used for example with gitlab registries.
//
// The token issuing and validation differs between providers,
// and we only use a minimal version for testing.
func withAuthenticatedTokenAuth() fixtureOpt {
	const token = "some-test-token"
	tokenServer := httptest.NewServer(tokenHandlerAuth("c2VjcmV0", token))

	const wwwAuthenticateFmt = "Bearer realm=%q service=%q scope=%q"
	tokenServiceURL := tokenServer.URL + "/token"
	wwwAuthenticate := fmt.Sprintf(wwwAuthenticateFmt,
		tokenServiceURL,
		"testRegistry.io",
		"[pull]")

	return func(tf *testFixture) error {
		tf.server.customAuth = func(w http.ResponseWriter, r *http.Request) error {

			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("no authorization header: %w", errUnauthorized)
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("expects bearer scheme: %w", errUnauthorized)
			}

			bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
			if bearerToken != token {
				w.Header().Set("WWW-Authenticate", wwwAuthenticate)
				return fmt.Errorf("token %q doesn't match %q: %w", bearerToken, token, errUnauthorized)
			}

			return nil
		}

		return nil
	}
}

// tokenHandler returns an http.Handler that responds with the
// specified token to GET /token requests.
func tokenHandler(issuedToken string) http.HandlerFunc {
	return tokenHandlerAuth("", issuedToken)
}

// tokenHandlerAuth returns an http.Handler that responds with the
// specified token to GET /token requests.
//
// If expectedToken is not empty, the handler will check that the
// Authorization header matches the expected token.
func tokenHandlerAuth(expectedToken, issuedToken string) http.HandlerFunc {
	tokenResponse := struct {
		Token string `json:"token"`
	}{
		Token: issuedToken,
	}

	responseBody, err := json.Marshal(tokenResponse)
	if err != nil {
		panic("failed to marshal token response: " + err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// If no expected token is set, we don't check the Authorization header.
		if expectedToken == "" {
			_, _ = w.Write(responseBody)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
		if bearerToken != expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_, _ = w.Write(responseBody)
	}
}

func (t *testFixture) setClient(client rest.Client) {
	t.client = client
}

func (t *testFixture) oneShot(ctx context.Context, u Update) {

	t.updates = append(t.updates, u)

	if u.Error != nil {
		etag := t.etags["test/bundle1"]
		t.d.SetCache(etag)
		return
	}

	if u.Bundle != nil {
		if t.mockBundleActivationError {
			etag := t.etags["test/bundle1"]
			t.d.SetCache(etag)
			return
		}
	}

	t.etags["test/bundle1"] = u.ETag
}

type fileInfo struct {
	name   string
	length int64
}

type testServer struct {
	t              *testing.T
	customAuth     func(http.ResponseWriter, *http.Request) error
	expCode        int
	expEtag        string
	expAuth        string
	bundles        map[string]bundle.Bundle
	server         *httptest.Server
	etagInResponse bool
	longPoll       bool
	testdataHashes map[string]fileInfo
}

func newTestServer(t *testing.T) *testServer {
	return &testServer{
		t: t,
		bundles: map[string]bundle.Bundle{
			"test/bundle1": {
				Manifest: bundle.Manifest{
					Revision: "quickbrownfaux",
				},
				Data: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": json.Number("1"),
						"baz": "qux",
					},
				},
				Modules: []bundle.ModuleFile{
					{
						Path: `/example.rego`,
						Raw:  []byte("package foo\n\ncorge=1"),
					},
				},
			},
			"test/bundle2": {
				Manifest: bundle.Manifest{
					Revision: deltaBundleMode,
				},
				Patch: bundle.Patch{Data: []bundle.PatchOperation{
					{
						Op:    "upsert",
						Path:  "/a/c/d",
						Value: []string{"foo", "bar"},
					},
				}},
			},
		},
	}
}

func (t *testServer) handle(w http.ResponseWriter, r *http.Request) {

	if t.longPoll {

		var timeout time.Duration

		wait := getPreferHeaderField(r, "wait")
		if wait != "" {
			waitTime, err := strconv.Atoi(wait)
			if err != nil {
				panic(err)
			}
			timeout = time.Duration(waitTime) * time.Second
		}

		// simulate long operation
		time.Sleep(timeout)
	}

	if t.expCode != 0 {
		w.WriteHeader(t.expCode)
		return
	}

	if t.customAuth != nil {
		if err := t.customAuth(w, r); err != nil {
			t.t.Logf("Failed authorization: %s", err)
			if errors.Is(err, errUnauthorized) {
				w.WriteHeader(401)
				return
			}

			w.WriteHeader(500)
			return
		}
	} else if t.expAuth != "" {
		if r.Header.Get("Authorization") != t.expAuth {
			w.WriteHeader(401)
			return
		}
	}

	var buf bytes.Buffer

	if strings.HasPrefix(r.URL.Path, "/v2/org/repo/") {
		// build test data to hash map to serve testdata files by hash
		if t.testdataHashes == nil {
			t.testdataHashes = make(map[string]fileInfo)
			files, err := os.ReadDir("testdata")
			if err != nil {
				t.t.Fatalf("failed to read testdata directory: %s", err)
			}
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				hash, length, err := getFileSHAandSize("testdata/" + file.Name())
				if err != nil {
					t.t.Fatalf("failed to read testdata file: %s", err)
				}
				t.testdataHashes[fmt.Sprintf("%x", hash)] = fileInfo{name: file.Name(), length: length}
			}
		}
	}

	if strings.HasPrefix(r.URL.Path, "/v2/org/repo/blobs/sha256:") || strings.HasPrefix(r.URL.Path, "/v2/org/repo/manifests/sha256:") {
		sha := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v2/org/repo/blobs/sha256:"), "/v2/org/repo/manifests/sha256:")
		if fileInfo, ok := t.testdataHashes[sha]; ok {
			w.Header().Add("Content-Length", strconv.Itoa(int(fileInfo.length)))
			w.Header().Add("Content-Type", "application/gzip")
			w.Header().Add("Docker-Content-Digest", "sha256:"+sha)
			w.WriteHeader(200)
			bs, err := os.ReadFile("testdata/" + fileInfo.name)
			if err != nil {
				w.WriteHeader(404)
				return
			}
			buf.WriteString(string(bs))
			w.Write(buf.Bytes())
			return
		}
		w.WriteHeader(404)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/v2/org/repo/manifests/") {
		sha, size, err := getFileSHAandSize("testdata/" + strings.TrimPrefix(r.URL.Path, "/v2/org/repo/manifests/") + ".manifest")
		if err != nil {
			w.WriteHeader(404)
			return
		}

		w.Header().Add("Content-Length", strconv.Itoa(int(size)))
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", "sha256:"+fmt.Sprintf("%x", sha))
		w.WriteHeader(200)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/bundles/")
	b, ok := t.bundles[name]
	if !ok {
		w.WriteHeader(404)
		return
	}

	// check to verify if server can send a delta bundle to OPA
	if b.Manifest.Revision == deltaBundleMode {
		modes := strings.Split(getPreferHeaderField(r, "modes"), ",")

		found := false
		for _, m := range modes {
			if m == deltaBundleMode {
				found = true
				break
			}
		}

		if !found {
			panic("delta bundle requested but OPA does not support it")
		}
	}

	contentTypeShouldBeSend := true
	if t.expEtag != "" {
		etag := r.Header.Get("If-None-Match")
		if etag == t.expEtag {
			contentTypeShouldBeSend = false
			if t.etagInResponse {
				w.Header().Add("Etag", t.expEtag)
			}
			w.WriteHeader(304)
			return
		}
	}

	if t.longPoll && contentTypeShouldBeSend {
		// in 304 Content-Type is not send according https://datatracker.ietf.org/doc/html/rfc7232#section-4.1
		w.Header().Add("Content-Type", "application/vnd.openpolicyagent.bundles")
	} else {
		if r.URL.Path == "/bundles/not-a-bundle" {
			w.Header().Add("Content-Type", "text/html")
		} else {
			w.Header().Add("Content-Type", "application/gzip")
		}
	}

	if t.expEtag != "" {
		w.Header().Add("Etag", t.expEtag)
	}

	w.WriteHeader(200)

	if err := bundle.Write(&buf, b); err != nil {
		w.WriteHeader(500)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		panic(err)
	}
}

func (t *testServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *testServer) stop() {
	t.server.Close()
}

func getPreferHeaderField(r *http.Request, field string) string {
	for _, line := range r.Header.Values("prefer") {
		for _, part := range strings.Split(line, ";") {
			preference := strings.Split(strings.TrimSpace(part), "=")
			if len(preference) == 2 {
				if strings.ToLower(preference[0]) == field {
					return preference[1]
				}
			}
		}
	}
	return ""
}

func getFileSHAandSize(filePath string) ([]byte, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	hash := sha256.New()
	w, err := io.Copy(hash, f)
	if err != nil {
		return nil, w, err
	}
	return hash.Sum(nil), w, nil
}
