// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"context"
	"reflect"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/util/test/protoroundtrip"
	"github.com/open-policy-agent/opa/v1/util/test/protoschemacheck"
)

func TestManifestProtoConsistency(t *testing.T) {
	protoschemacheck.Run(t, protoschemacheck.Spec{
		ProtoPath: "manifest.proto",
		Messages: []protoschemacheck.MessageSpec{
			{
				Name:   "Manifest",
				GoType: reflect.TypeOf(Manifest{}),
			},
			{
				Name:   "WasmResolver",
				GoType: reflect.TypeOf(WasmResolver{}),
				// ast.Annotations has its own custom JSON marshaling; opaque.
				OpaqueProtoFields: []string{"annotations"},
			},
		},
	})
}

func TestManifestProtoRoundTrip(t *testing.T) {
	codec := loadManifestCodec(t)
	regoV1 := 1

	tests := []struct {
		note     string
		manifest Manifest
	}{
		{
			note:     "empty manifest",
			manifest: Manifest{},
		},
		{
			note: "revision and roots",
			manifest: func() Manifest {
				m := Manifest{Revision: "abc123"}
				m.Init()
				m.AddRoot("roles")
				m.AddRoot("http/example/authz")
				return m
			}(),
		},
		{
			note: "rego version and per-file overrides",
			manifest: Manifest{
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
			manifest: Manifest{
				Revision: "abc123",
				WasmResolvers: []WasmResolver{
					{Entrypoint: "http/example/authz/allow", Module: "/policy.wasm"},
				},
			},
		},
		{
			note: "metadata",
			manifest: Manifest{
				Revision: "abc123",
				Metadata: map[string]any{
					"build_id": "ci-1234",
					"tags":     []any{"prod", "us-east-1"},
					"nested":   map[string]any{"k": 1.5, "ok": true},
				},
			},
		},
	}

	opts := []cmp.Option{cmpopts.EquateEmpty()}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := codec.Encode(&tc.manifest)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			var got Manifest
			if err := codec.Decode(bs, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			// EquateEmpty handles nil-vs-empty slice/map, but not the
			// *[]string pointer case on Manifest.Roots.
			normalizeRoots(&tc.manifest)
			normalizeRoots(&got)
			if diff := cmp.Diff(tc.manifest, got, opts...); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func normalizeRoots(m *Manifest) {
	if m.Roots != nil && len(*m.Roots) == 0 {
		m.Roots = nil
	}
}

func loadManifestCodec(t *testing.T) *protoroundtrip.Codec {
	t.Helper()
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{"."},
		}),
	}
	files, err := c.Compile(context.Background(), "manifest.proto")
	if err != nil {
		t.Fatalf("compile manifest.proto: %v", err)
	}
	codec := protoroundtrip.NewCodec(files[0])
	codec.RegisterRoot(reflect.TypeOf(Manifest{}), "Manifest")
	codec.RegisterRoot(reflect.TypeOf(WasmResolver{}), "WasmResolver")
	codec.SkipProtoFields(reflect.TypeOf(WasmResolver{}), "annotations")
	return codec
}
