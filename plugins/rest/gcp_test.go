package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/keys"
)

func TestGCPMetadataAuthPlugin(t *testing.T) {
	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.Et9HFtf9R3GEMA0IICOfFMVXY7kkTX1wr4qCyhIf58U"

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		return
	}))
	defer s.Close()

	ts := httptest.NewServer(http.Handler(&gcpMetadataHandler{idToken}))
	defer ts.Close()

	config := fmt.Sprintf(`{
        "name": "foo",
        "url": "%s",
        "allow_insecure_tls": true,
        "credentials": {
            "gcp_metadata": {
                "audience": "https://example.org",
                "endpoint": "%s"
            }
        }
    }`, s.URL, ts.URL)
	client, err := New([]byte(config), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = client.Do(ctx, "GET", "test")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestIdentityTokenFromMetadataService(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.Et9HFtf9R3GEMA0IICOfFMVXY7kkTX1wr4qCyhIf58U"

	ts := httptest.NewServer(http.Handler(&gcpMetadataHandler{token}))
	defer ts.Close()

	tests := []struct {
		audience          string
		identityToken     string
		identityTokenPath string
		err               error
	}{
		{"https://example.org", token, defaultIdentityTokenPath, nil},
		{"", "", defaultIdentityTokenPath, errGCPMetadataInvalidRequest},
		{"https://example.org", "", "/status/bad/request", errGCPMetadataInvalidRequest},
		{"https://example.org", "", "/status/not/found", errGCPMetadataNotFound},
		{"https://example.org", "", "/status/internal/server/error", errGCPMetadataUnexpected},
	}

	for _, tt := range tests {
		token, err := identityTokenFromMetadataService(ts.URL, tt.identityTokenPath, tt.audience)
		if !errors.Is(err, tt.err) {
			t.Fatalf("Unexpected error, got %v, want %v", err, tt.err)
		}

		if token != tt.identityToken {
			t.Fatalf("Unexpected id token, got %v, want %v", token, tt.identityToken)
		}
	}
}

type gcpMetadataHandler struct {
	identityToken string
}

func (h *gcpMetadataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	audience := r.URL.Query()["audience"][0]

	if audience == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	switch path {
	case defaultIdentityTokenPath:
		fmt.Fprint(w, h.identityToken)
	case "/status/bad/request":
		http.Error(w, "", http.StatusBadRequest)
	case "/status/not/found":
		http.Error(w, "", http.StatusNotFound)
	case "/status/internal/server/error":
		http.Error(w, "", http.StatusInternalServerError)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}
