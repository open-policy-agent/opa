//go:build !opa_no_oci

package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

func TestNewOCICreatesPrivateTempStoreByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	predictableStorePath := filepath.Join(os.TempDir(), "opa", "oci")
	if err := os.MkdirAll(predictableStorePath, 0o755); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(Config{}, newOCIStorePathTestClient(t), "ghcr.io/org/repo:latest", "")

	if d.localStorePath == predictableStorePath {
		t.Fatalf("expected private OCI store path, got predictable path %q", d.localStorePath)
	}
	if filepath.Dir(d.localStorePath) != tmp {
		t.Fatalf("expected OCI store below temp dir %q, got %q", tmp, d.localStorePath)
	}
	if !strings.HasPrefix(filepath.Base(d.localStorePath), "opa-oci-") {
		t.Fatalf("expected OCI store basename to have opa-oci- prefix, got %q", filepath.Base(d.localStorePath))
	}
}

func TestNewOCIUsesExplicitStorePath(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "oci")

	d := NewOCI(Config{}, newOCIStorePathTestClient(t), "ghcr.io/org/repo:latest", storePath)

	if d.localStorePath != storePath {
		t.Fatalf("expected explicit OCI store path %q, got %q", storePath, d.localStorePath)
	}
}

func newOCIStorePathTestClient(t *testing.T) rest.Client {
	t.Helper()

	client, err := rest.New([]byte(`{"url":"http://example.com","type":"oci"}`), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
