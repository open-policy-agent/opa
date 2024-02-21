//go:build slow
// +build slow

package download

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/plugins/rest"
)

// when changed the layer hash & size should be updated in signed.manifest
//go:generate go run github.com/open-policy-agent/opa build -b --signing-alg HS256 --signing-key secret testdata/signed_bundle_data --output testdata/signed.tar.gz
//go:generate go run github.com/open-policy-agent/opa build --v1-compatible -b --signing-alg HS256 --signing-key secret testdata/rego_v1_bundle_data --output testdata/rego_v1.tar.gz

func TestOCIDownloaderWithBundleVerificationConfig(t *testing.T) {
	vc := bundle.NewVerificationConfig(map[string]*bundle.KeyConfig{"default": {Key: "secret", Algorithm: "HS256"}}, "", "", nil)
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

	updates := make(chan *Update)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(config, fixture.client, "ghcr.io/org/repo:signed", "/tmp/opa/").WithCallback(func(_ context.Context, u Update) {
		if u.Error != nil {
			t.Fatalf("expected no error but got: %v", u.Error)
		}
		updates <- &u
	}).WithBundleVerificationConfig(vc)

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

func TestOCIDownloaderWithRegoV1Bundle(t *testing.T) {
	tests := []struct {
		note        string
		regoVersion ast.RegoVersion
		expErr      string
	}{
		{
			note:   "non-1.0 compatible OCI downloader",
			expErr: "rego_parse_error",
		},
		{
			note:        "1.0 compatible OCI downloader",
			regoVersion: ast.RegoV1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			vc := bundle.NewVerificationConfig(map[string]*bundle.KeyConfig{"default": {Key: "secret", Algorithm: "HS256"}}, "", "", nil)
			ctx := context.Background()
			fixture := newTestFixture(t)
			fixture.server.expEtag = "sha256:c5834dbce332cabe6ae68a364de171a50bf5b08024c27d7c08cc72878b4df7ff"

			// We might get multiple updates, a buffered channel will make sure we save the first one.
			updates := make(chan *Update, 1)

			config := Config{}
			if err := config.ValidateAndInjectDefaults(); err != nil {
				t.Fatal(err)
			}

			d := NewOCI(config, fixture.client, "ghcr.io/org/repo:rego_v1", "/tmp/opa/").
				WithBundleParserOpts(ast.ParserOptions{RegoVersion: tc.regoVersion}).
				WithCallback(func(_ context.Context, u Update) {
					// We might get multiple updates before the test ends, and we don't want to block indefinitely.
					select {
					case updates <- &u:
					}
				}).WithBundleVerificationConfig(vc)

			d.Start(ctx)

			// Give time for some download events to occur
			time.Sleep(1 * time.Second)

			// We only care about the first update
			u1 := <-updates

			if tc.expErr != "" {
				if u1.Error == nil {
					t.Fatalf("expected error but got: %v", u1)
				} else {
					if !strings.Contains(u1.Error.Error(), tc.expErr) {
						t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", tc.expErr, u1.Error)
					}
				}
			} else {
				if u1.Error != nil {
					t.Fatalf("expected no error but got: %v", u1.Error)
				}

				if u1.Bundle == nil || len(u1.Bundle.Modules) == 0 {
					t.Fatal("expected bundle with at least one module but got:", u1)
				}

				if !strings.HasSuffix(u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path) {
					t.Fatalf("expected URL to have path as suffix but got %v and %v", u1.Bundle.Modules[0].URL, u1.Bundle.Modules[0].Path)
				}
			}

			d.Stop(ctx)
		})
	}
}

func TestOCIStartStop(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t)
	fixture.server.expEtag = "sha256:cc09b0f5ac97b11637c96ff1b0fbbc287c5ba0169813edaa71fe58424e95f0b7"

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

// TestOCIPublicRegistryAuth tests the registry `token` auth
// that is implemented by public registries (more details are
// in the doc comment of withPublicRegistryAuth).
//
// Other tests that don't explicitly set an authentication method
// implicitly test no authentication - this is different from
// the mechanism used by public registries.
func TestOCIPublicRegistryAuth(t *testing.T) {
	fixture := newTestFixture(t, withPublicRegistryAuth())

	restConfig := []byte(fmt.Sprintf(`{
		"url": %q,
		"type": "oci"
	}`, fixture.server.server.URL))

	client, err := rest.New(restConfig, map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("failed to create rest client: %s", err)
	}
	fixture.client = client

	d := NewOCI(Config{}, fixture.client, "ghcr.io/org/repo:latest", t.TempDir())

	if err := d.oneShot(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

// TestOCITokenAuth tests the registry `token` auth that is used for some registries (f.e. gitlab).
// After the initial fetch the token has to be added to the request that fetches the temporary token.
// This test verifies that the token is added to the second token request.
func TestOCITokenAuth(t *testing.T) {
	ctx := context.Background()
	fixture := newTestFixture(t, withAuthenticatedTokenAuth())
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
		t.Fatalf("failed to create rest client: %s", err)
	}
	fixture.setClient(client)

	config := Config{}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}

	d := NewOCI(Config{}, fixture.client, "ghcr.io/org/repo:latest", t.TempDir())

	if err := d.oneShot(ctx); err != nil {
		t.Fatalf("Unexpected error: %s", err)
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
