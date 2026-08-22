//go:build !opa_no_oci

package download

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

// ociTestRef is only ever resolved against a closed port, so the download
// always fails fast without any network access.
const ociTestRef = "ghcr.io/org/repo:latest"

// newOCIStoreTestClient returns a client pointing at a port nothing listens
// on. Downloads made through it fail with connection refused immediately,
// which is all these tests need: they exercise store setup and teardown, not
// the download itself.
func newOCIStoreTestClient(t *testing.T) rest.Client {
	t.Helper()

	client, err := rest.New([]byte(`{"url": "http://127.0.0.1:1", "type": "oci"}`), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func ociStoreTestConfig(t *testing.T, trigger plugins.TriggerMode) Config {
	t.Helper()

	config := Config{Trigger: &trigger}
	if err := config.ValidateAndInjectDefaults(); err != nil {
		t.Fatal(err)
	}
	return config
}

func assertRemoved(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to have been removed but got: %v", path, err)
	}
}

func TestOCIStorePath(t *testing.T) {
	tests := []struct {
		note        string
		explicit    bool
		wantTemp    bool
		wantRemoved bool
	}{
		{note: "default store path", explicit: false, wantTemp: true, wantRemoved: true},
		{note: "explicit store path", explicit: true, wantTemp: false, wantRemoved: false},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var storePath string
			if tc.explicit {
				storePath = filepath.Join(t.TempDir(), "oci")
			}

			d := NewOCI(ociStoreTestConfig(t, plugins.TriggerManual), newOCIStoreTestClient(t), ociTestRef, storePath)

			if d.localStoreIsTemp != tc.wantTemp {
				t.Fatalf("expected localStoreIsTemp to be %v but got %v", tc.wantTemp, d.localStoreIsTemp)
			}

			if tc.explicit {
				if d.localStorePath != storePath {
					t.Fatalf("expected store path %q but got %q", storePath, d.localStorePath)
				}
			} else {
				if shared := filepath.Join(t.TempDir(), "opa", "oci"); d.localStorePath == shared {
					t.Fatalf("expected a private store but got the shared path %q", shared)
				}

				if base := filepath.Base(d.localStorePath); !strings.HasPrefix(base, "opa-oci-") {
					t.Fatalf("expected a temporary store directory but got %q", d.localStorePath)
				}
			}

			info, err := os.Stat(d.localStorePath)
			if err != nil {
				t.Fatalf("expected the store directory to exist: %v", err)
			}

			// Windows doesn't model Unix permission bits, so there's nothing
			// meaningful to assert there.
			if !tc.explicit && runtime.GOOS != "windows" {
				if perm := info.Mode().Perm(); perm != 0o700 {
					t.Fatalf("expected store directory %q to be private (0700) but got %04o", d.localStorePath, perm)
				}
			}

			d.Stop(t.Context())

			if tc.wantRemoved {
				assertRemoved(t, d.localStorePath)
			} else if _, err := os.Stat(filepath.Join(d.localStorePath, "oci-layout")); err != nil {
				t.Fatalf("expected the store to survive Stop: %v", err)
			}
		})
	}
}

func TestOCIDefaultStorePathIsUnique(t *testing.T) {
	client := newOCIStoreTestClient(t)
	config := ociStoreTestConfig(t, plugins.TriggerManual)

	first := NewOCI(config, client, ociTestRef, "")
	t.Cleanup(func() { first.Stop(t.Context()) })

	second := NewOCI(config, client, ociTestRef, "")
	t.Cleanup(func() { second.Stop(t.Context()) })

	if first.localStorePath == second.localStorePath {
		t.Fatalf("expected each downloader to get its own store but both got %q", first.localStorePath)
	}
}

func TestOCIStopRemovesTemporaryStore(t *testing.T) {
	tests := []struct {
		note    string
		trigger plugins.TriggerMode
		start   bool
		// afterStop runs once the downloader has been stopped, to check that
		// post-stop operations are safe and still leave the store removed.
		afterStop func(*testing.T, *OCIDownloader)
	}{
		{
			note:    "manual trigger",
			trigger: plugins.TriggerManual,
		},
		{
			note:    "periodic trigger",
			trigger: plugins.TriggerPeriodic,
			start:   true,
		},
		{
			note:    "second stop is a no-op",
			trigger: plugins.TriggerPeriodic,
			start:   true,
			afterStop: func(t *testing.T, d *OCIDownloader) {
				t.Helper()

				done := make(chan struct{})
				go func() {
					defer close(done)
					d.Stop(t.Context())
				}()

				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Fatal("expected the second Stop to be a no-op but it blocked")
				}
			},
		},
		{
			note:    "trigger after stop is rejected",
			trigger: plugins.TriggerManual,
			afterStop: func(t *testing.T, d *OCIDownloader) {
				t.Helper()

				err := d.Trigger(t.Context())
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				if !strings.Contains(err.Error(), "downloader stopped") {
					t.Fatalf("expected a stopped downloader error but got: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			d := NewOCI(ociStoreTestConfig(t, tc.trigger), newOCIStoreTestClient(t), ociTestRef, "")
			storePath := d.localStorePath

			if _, err := os.Stat(storePath); err != nil {
				t.Fatalf("expected the store directory to exist: %v", err)
			}

			if tc.start {
				d.Start(t.Context())
			}

			d.Stop(t.Context())

			if tc.afterStop != nil {
				tc.afterStop(t, d)
			}

			assertRemoved(t, storePath)
		})
	}
}

func TestOCIStopWaitsForInFlightTrigger(t *testing.T) {
	inCallback := make(chan struct{})
	release := make(chan struct{})

	d := NewOCI(ociStoreTestConfig(t, plugins.TriggerManual), newOCIStoreTestClient(t), ociTestRef, "").
		WithCallback(func(context.Context, Update) error {
			close(inCallback)
			<-release
			return nil
		})
	storePath := d.localStorePath

	triggerDone := make(chan struct{})
	go func() {
		defer close(triggerDone)
		_ = d.Trigger(t.Context())
	}()

	select {
	case <-inCallback:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the download callback")
	}

	stopDone := make(chan struct{})
	go func() {
		defer close(stopDone)
		d.Stop(t.Context())
	}()

	select {
	case <-stopDone:
		t.Fatal("expected Stop to wait for the in-flight download")
	case <-time.After(250 * time.Millisecond):
	}

	close(release)

	select {
	case <-stopDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Stop")
	}

	<-triggerDone

	assertRemoved(t, storePath)
}
