// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/internal/file/archive"
)

func TestDoInspectWasmRelatedResources(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision":"", "roots":["example"], "wasm":[{
			"entrypoint":"example/allow", "module":"/policy.wasm", "annotations":[{
				"scope":"rule", "entrypoint":true, "related_resources":[{
					"ref":"https://example.com/privacy?purpose=analytics#retention",
					"description":"Retention policy"
				}]
			}]
		}]}`},
		{"/policy.wasm", "modules-compiled-as-wasm-binary"},
	}
	bundleFile := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundleFile, archive.MustWriteTarGz(files).Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{formats.JSON, formats.Pretty} {
		t.Run(format, func(t *testing.T) {
			params := newInspectCommandParams()
			params.listAnnotations = true
			if err := params.outputFormat.Set(format); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := doInspect(params, bundleFile, &out); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"https://example.com/privacy?purpose=analytics#retention", "Retention policy"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("expected %q in inspect output:\n%s", want, &out)
				}
			}
		})
	}
}
