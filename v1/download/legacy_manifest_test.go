// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

// A nested .manifest in a downloaded tarball loads like before, with a warning.
func TestDownloadLegacyManifestPlacement(t *testing.T) {
	t.Parallel()

	body := archive.MustWriteTarGz([][2]string{
		{"bundle/.manifest", `{"revision":"r1"}`},
		{"bundle/data.json", `{"a":1}`},
	}).Bytes()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(body)
	}))
	defer ts.Close()

	client, err := rest.New([]byte(fmt.Sprintf(`{"url": %q}`, ts.URL)), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	logger := test.New()
	var got *Update
	d := New(Config{}, client, "/bundles/legacy").WithCallback(func(_ context.Context, u Update) error {
		got = &u
		return nil
	})
	d.logger = logger

	if err := d.oneShot(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Bundle == nil {
		t.Fatal("expected a bundle")
	}

	exp := map[string]any{"bundle": map[string]any{"a": json.Number("1")}}
	if !reflect.DeepEqual(exp, got.Bundle.Data) {
		t.Fatalf("expected data %v, got %v", exp, got.Bundle.Data)
	}

	for _, e := range logger.Entries() {
		if e.Level == logging.Warn && strings.Contains(e.Message, "is expected at the bundle root") {
			return
		}
	}
	t.Fatal("expected deprecation warning to be logged")
}
