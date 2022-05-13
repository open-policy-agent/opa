//go:build slow
// +build slow

package download

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOCIStartStop(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "" // test on public registry
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

	updates := make(chan *Update)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", "/tmp/opa/").WithCallback(func(_ context.Context, u Update) {
		updates <- &u
	})

	d.Start(ctx)

	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	u1 := <-updates

	if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
		t.Fatal("expected bundle with at least one module but got:", u1)
	}

	if !strings.HasSuffix(u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path) {
		t.Fatalf("expected URL to have path as suffix but got %v and %v", u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path)
	}

	d.Stop(ctx)
}

func TestOCIAuth(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	token := base64.StdEncoding.EncodeToString([]byte("secret")) // token should be base64 encoded
	fixture.server.expAuth = fmt.Sprintf("Bearer %s", token)     // test on private repository
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", "/tmp/oci")

	err := d.oneShot(ctx)
	if err != nil {
		t.Fatal("unexpected error")
	}
}

func TestOCIFailureAuthn(t *testing.T) {

	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "Bearer badsecret"
	defer fixture.server.stop()

	d := NewOCI(Config{}, fixture.client, "ghcr.io/org/repo:latest", "/tmp/oci")

	err := d.oneShot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatal("expected 401 Unauthorized message")
	}
}

func TestOCIEtag(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "" // test on public registry
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"
	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}
	firstResponse := Update{ETag: ""}
	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", "/tmp/oci").WithCallback(func(_ context.Context, u Update) {
		if firstResponse.ETag == "" {
			firstResponse = u
			return
		}

		if u.ETag != firstResponse.ETag || u.Bundle != nil {
			t.Fatal("expected nil bundle and same etag but got:", u)
		}
	})

	// fill firstResponse
	err := d.oneShot(ctx)
	if err != nil {
		t.Fatal("unexpected error")
	}
	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	// second call to verify if nil bundle is returned and same etag
	err = d.oneShot(ctx)
	if err != nil {
		t.Fatal("unexpected error")
	}
}
