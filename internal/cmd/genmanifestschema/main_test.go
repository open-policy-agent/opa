package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/open-policy-agent/opa/v1/bundle"
)

const committedSchemaPath = "../../../v1/bundle/manifest.schema.json"

func TestSchemaDoesNotDrift(t *testing.T) {
	got, err := reflectSchema()
	if err != nil {
		t.Fatalf("reflectSchema: %v", err)
	}
	want, err := os.ReadFile(filepath.FromSlash(committedSchemaPath))
	if err != nil {
		t.Fatalf("read %s: %v", committedSchemaPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run `make generate` to update", committedSchemaPath)
	}
}

func TestSchemaValidatesRealManifests(t *testing.T) {
	schemaBytes, err := reflectSchema()
	if err != nil {
		t.Fatalf("reflectSchema: %v", err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("manifest.schema.json", schemaDoc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile("manifest.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	regoV1 := 1

	cases := []struct {
		note     string
		manifest bundle.Manifest
	}{
		{
			note:     "empty manifest",
			manifest: bundle.Manifest{},
		},
		{
			note: "revision and roots",
			manifest: func() bundle.Manifest {
				m := bundle.Manifest{Revision: "abc123"}
				m.Init()
				m.AddRoot("roles")
				m.AddRoot("http/example/authz")
				return m
			}(),
		},
		{
			note: "rego version and per-file overrides",
			manifest: bundle.Manifest{
				Revision:    "abc123",
				RegoVersion: &regoV1,
				FileRegoVersions: map[string]int{
					"/foo/*.rego":   0,
					"/policy1.rego": 0,
				},
			},
		},
		{
			note: "wasm resolvers",
			manifest: bundle.Manifest{
				Revision: "abc123",
				WasmResolvers: []bundle.WasmResolver{
					{Entrypoint: "http/example/authz/allow", Module: "/policy.wasm"},
				},
			},
		},
		{
			note: "metadata",
			manifest: bundle.Manifest{
				Revision: "abc123",
				Metadata: map[string]any{
					"build_id": "ci-1234",
					"tags":     []any{"prod", "us-east-1"},
					"nested":   map[string]any{"k": 1.5, "ok": true},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := json.Marshal(tc.manifest)
			if err != nil {
				t.Fatalf("marshal manifest: %v", err)
			}
			var doc any
			if err := json.Unmarshal(bs, &doc); err != nil {
				t.Fatalf("unmarshal manifest: %v", err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Fatalf("manifest does not validate:\n%s\n\nmanifest JSON:\n%s",
					strings.TrimSpace(err.Error()), string(bs))
			}
		})
	}

	// The bundle loader silently accepts unknown top-level keys, and
	// embedders rely on this to attach custom configuration alongside the
	// documented fields. The schema must not regress that.
	t.Run("manifest with extra top-level keys", func(t *testing.T) {
		raw := `{"revision": "x", "custom_app_setting": {"foo": {"enabled": true}}}`
		var doc any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Fatalf("expected extras to validate:\n%s", strings.TrimSpace(err.Error()))
		}
	})
}

func TestSchemaRejectsInvalidManifests(t *testing.T) {
	schemaBytes, err := reflectSchema()
	if err != nil {
		t.Fatalf("reflectSchema: %v", err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("manifest.schema.json", schemaDoc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile("manifest.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	cases := []struct {
		note string
		raw  string
	}{
		{
			note: "missing revision",
			raw:  `{"roots": ["a"]}`,
		},
		{
			note: "wrong type for revision",
			raw:  `{"revision": 123}`,
		},
		{
			note: "wrong type for roots entry",
			raw:  `{"revision": "x", "roots": [1]}`,
		},
		{
			note: "wrong type for rego_version",
			raw:  `{"revision": "x", "rego_version": "1"}`,
		},
		{
			note: "wrong value type in file_rego_versions",
			raw:  `{"revision": "x", "file_rego_versions": {"/a.rego": "1"}}`,
		},
		{
			note: "wasm entry with unknown field",
			raw:  `{"revision": "x", "wasm": [{"module": "/a.wasm", "extra": 1}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			var doc any
			if err := json.Unmarshal([]byte(tc.raw), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if err := schema.Validate(doc); err == nil {
				t.Fatalf("expected validation failure, got success for: %s", tc.raw)
			}
		})
	}
}
