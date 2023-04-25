//go:build slow
// +build slow

package download

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/plugins/rest"
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

func TestOCIBearerAuthPlugin(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	plainToken := "secret"
	token := base64.StdEncoding.EncodeToString([]byte(plainToken)) // token should be base64 encoded
	fixture.server.expAuth = fmt.Sprintf("Bearer %s", token)       // test on private repository
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

	restConf := fmt.Sprintf(`{
		"url": %q,
		"type": "oci",
		"credentials": {
			"bearer": {
				"token": %q
			}
		}
	}`, fixture.server.server.URL, plainToken)

	client, err := rest.New([]byte(restConf), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	fixture.setClient(client)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", "/tmp/oci")

	if err := d.oneShot(ctx); err != nil {
		t.Fatal(err)
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
	fixture := newTestFixture(t)
	token := base64.StdEncoding.EncodeToString([]byte("secret")) // token should be base64 encoded
	fixture.server.expAuth = fmt.Sprintf("Bearer %s", token)     // test on private repository
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
		"type": "oci",
		"credentials": {
			"bearer": {
				"token": "secret"
			}
		}
	}`, fixture.server.server.URL))

	client, err := rest.New(restConfig, map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	fixture.setClient(client)

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
	if err := d.oneShot(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Give time for some download events to occur
	time.Sleep(1 * time.Second)

	// second call to verify if nil bundle is returned and same etag
	err = d.oneShot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestOCIPublicRegistry(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expAuth = "" // Do not expect authorization header to be set

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
	}`, fixture.server.server.URL))

	tc, err := rest.New(restConfig, map[string]*keys.Config{})
	if err != nil {
		t.Fatal("failed to create rest client without credentials")
	}
	fixture.setClient(tc) // set a client without configured credentials

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", "/tmp/oci")

	err = d.oneShot(ctx)
	if err != nil {
		t.Fatal("unexpected error")
	}
}

func TestOCICustomAuthPlugin(t *testing.T) {
	fixture := newTestFixture(t)
	defer fixture.server.stop()

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
		"credentials": {
			"plugin": "my_plugin"
		}
	}`, fixture.server.server.URL))

	client, err := rest.New(
		restConfig,
		map[string]*keys.Config{},
		rest.AuthPluginLookup(mockAuthPluginLookup),
	)
	if err != nil {
		t.Fatal(err)
	}

	fixture.setClient(client)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:latest", tmpDir)

	if err := d.oneShot(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func mockAuthPluginLookup(string) rest.HTTPAuthPlugin {
	return &mockAuthPlugin{}
}

type mockAuthPlugin struct{}

func (p *mockAuthPlugin) NewClient(c rest.Config) (*http.Client, error) {
	tlsConfig, err := rest.DefaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	timeoutSec := 10

	client := rest.DefaultRoundTripperClient(
		tlsConfig,
		int64(timeoutSec),
	)

	return client, nil
}

func (*mockAuthPlugin) Prepare(r *http.Request) error {
	r.Header.Set("Authorization", "Bearer secret")
	return nil
}
