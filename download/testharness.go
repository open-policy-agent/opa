//go:build slow
// +build slow

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/plugins/rest"
)

type testFixture struct {
	d                         *Downloader
	client                    rest.Client
	server                    *testServer
	updates                   []Update
	mockBundleActivationError bool
	etags                     map[string]string
}

func newTestFixture(t *testing.T) testFixture {

	patch := bundle.PatchOperation{
		Op:    "upsert",
		Path:  "/a/c/d",
		Value: []string{"foo", "bar"},
	}

	ts := testServer{
		t:       t,
		expAuth: "Bearer secret",
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
				Patch: bundle.Patch{Data: []bundle.PatchOperation{patch}},
			},
		},
	}

	ts.start()

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
		"credentials": {
			"bearer": {
				"scheme": "Bearer",
				"token": "secret"
			}
		}
	}`, ts.server.URL))

	tc, err := rest.New(restConfig, map[string]*keys.Config{})

	if err != nil {
		t.Fatal(err)
	}

	return testFixture{
		client:  tc,
		server:  &ts,
		updates: []Update{},
		etags:   make(map[string]string),
	}
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

type testServer struct {
	t              *testing.T
	expCode        int
	expEtag        string
	expAuth        string
	bundles        map[string]bundle.Bundle
	server         *httptest.Server
	etagInResponse bool
	longPoll       bool
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

	if t.expAuth != "" {
		if r.Header.Get("Authorization") != t.expAuth {
			w.WriteHeader(401)
			return
		}
	}

	var buf bytes.Buffer

	if r.URL.Path == "/v2/org/repo/manifests/latest" {
		w.Header().Add("Content-Length", "596")
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", "sha256:fe9c2930b6d8cc1bf3fa0c560996a95c75f0d0668bee71138355d9784c8c99b8")
		w.WriteHeader(200)
		return
	}
	if r.URL.Path == "/v2/org/repo/manifests/sha256:fe9c2930b6d8cc1bf3fa0c560996a95c75f0d0668bee71138355d9784c8c99b8" {
		w.Header().Add("Content-Length", "596")
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", "sha256:fe9c2930b6d8cc1bf3fa0c560996a95c75f0d0668bee71138355d9784c8c99b8")
		w.WriteHeader(200)
		bs, err := ioutil.ReadFile("testdata/manifest.layer")
		if err != nil {
			w.WriteHeader(404)
			return
		}
		buf.WriteString(string(bs))
		w.Write(buf.Bytes())
		return
	}
	if r.URL.Path == "/v2/org/repo/blobs/sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff" {
		w.Header().Add("Content-Length", "464")
		w.Header().Add("Content-Type", "application/vnd.oci.image.layer.v1.tar+gzip,application/vnd.oci.image.config.v1+json")
		w.Header().Add("Docker-Content-Digest", "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff")
		w.WriteHeader(200)
		bs, err := ioutil.ReadFile("testdata/manifest.layer")
		if err != nil {
			w.WriteHeader(404)
			return
		}
		buf.WriteString(string(bs))
		buf.WriteTo(w)

		return
	}
	if r.URL.Path == "/v2/org/repo/blobs/sha256:b206ac766b0f3f880f6a62c4bb5ba5192d29deaefd989a1961603346a7555bdd" {
		w.Header().Add("Content-Length", "568")
		w.Header().Add("Content-Type", "application/vnd.oci.image.layer.v1.tar+gzip")
		w.Header().Add("Docker-Content-Digest", "sha256:b206ac766b0f3f880f6a62c4bb5ba5192d29deaefd989a1961603346a7555bdd")
		w.WriteHeader(200)
		bs, err := ioutil.ReadFile("testdata/tar.layer")
		if err != nil {
			w.WriteHeader(404)
			return
		}
		buf.WriteString(string(bs))
		w.Write(buf.Bytes())
		return
	}
	if r.URL.Path == "/v2/org/repo/blobs/sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a" {
		w.Header().Add("Content-Length", "2")
		w.Header().Add("Content-Type", "application/vnd.oci.image.config.v1+json")
		w.Header().Add("Docker-Content-Digest", "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a")
		w.WriteHeader(200)
		bs, err := ioutil.ReadFile("testdata/config.layer")
		if err != nil {
			w.WriteHeader(404)
			return
		}
		buf.WriteString(string(bs))
		w.Write(buf.Bytes())
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
