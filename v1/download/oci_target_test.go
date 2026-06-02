//go:build !opa_no_oci

package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// TestOCITargetAcceptHeaders verifies that ociTarget sends Accept headers on
// manifest requests. Registries such as ghcr.io return 404 for HEAD/GET requests
// that omit the Accept header; this test enforces the same behaviour.
func TestOCITargetAcceptHeaders(t *testing.T) {
	const mediaType = "application/vnd.oci.image.manifest.v1+json"
	digestHex := "sha256:" + strings.Repeat("a", 64)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// reject any request missing Accept entirely
		if r.Header.Get("Accept") == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// for GET (Fetch), Accept must be the exact descriptor media type
		if r.Method == http.MethodGet && r.Header.Get("Accept") != mediaType {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		w.Header().Set("Content-Type", mediaType)
		w.Header().Set("Docker-Content-Digest", digestHex)
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	target := &ociTarget{
		client:    &auth.Client{Client: http.DefaultClient, Cache: auth.NewCache()},
		registry:  serverURL.Host,
		repo:      "org/repo",
		plainHTTP: true,
	}

	// Resolve uses HEAD — any non-empty Accept is sufficient
	if _, err := target.Resolve(context.Background(), "latest"); err != nil {
		t.Fatalf("Resolve failed (Accept header likely missing): %v", err)
	}

	// Fetch uses GET — Accept must match the descriptor's media type exactly
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.Digest(digestHex),
		Size:      100,
	}
	rc, err := target.Fetch(context.Background(), desc)
	if err != nil {
		t.Fatalf("Fetch failed (Accept header likely missing or wrong): %v", err)
	}
	rc.Close()
}
