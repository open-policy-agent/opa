// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package proto

import (
	"context"
	"net/url"
	"reflect"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/e2e/proto/protoroundtrip"
	"github.com/open-policy-agent/opa/e2e/proto/protoschemacheck"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/bundle"
)

func TestManifestProtoConsistency(t *testing.T) {
	protoschemacheck.Run(t, protoschemacheck.Spec{
		ProtoPath:   "manifest.proto",
		ImportPaths: []string{"../../v1/bundle"},
		Messages: []protoschemacheck.MessageSpec{
			{
				Name:   "Manifest",
				GoType: reflect.TypeOf(bundle.Manifest{}),
				// roots_set is wire-form bookkeeping: it preserves the
				// nil-vs-explicit-empty distinction that Manifest.Roots
				// (*[]string) carries on the Go side. No Go counterpart by
				// design.
				SkipProtoFields: []string{"roots_set"},
			},
			{
				Name:   "WasmResolver",
				GoType: reflect.TypeOf(bundle.WasmResolver{}),
			},
			{
				Name:   "Annotations",
				GoType: reflect.TypeOf(ast.Annotations{}),
				// `comments` and `node` are unexported on the Go side
				// and skipped by protoschemacheck's json-tag walk.
			},
			{
				Name:   "SchemaAnnotation",
				GoType: reflect.TypeOf(ast.SchemaAnnotation{}),
				// ast.Ref is []*Term in Go; modeled as canonical-form
				// string in proto. *any Definition modeled as
				// google.protobuf.Value. Neither admits a reflect-level
				// type match against a proto string field.
				OpaqueProtoFields: []string{"path", "schema"},
			},
			{
				Name:   "CompileAnnotation",
				GoType: reflect.TypeOf(ast.CompileAnnotation{}),
				// Same Ref → string mapping as SchemaAnnotation.
				OpaqueProtoFields: []string{"unknowns", "mask_rule"},
			},
			{
				Name:   "AuthorAnnotation",
				GoType: reflect.TypeOf(ast.AuthorAnnotation{}),
			},
			{
				Name:   "RelatedResourceAnnotation",
				GoType: reflect.TypeOf(ast.RelatedResourceAnnotation{}),
				// url.URL → string via its String() method.
				OpaqueProtoFields: []string{"ref"},
			},
			{
				Name:   "Location",
				GoType: reflect.TypeOf(location.Location{}),
				// `Text`, `Offset`, `Tabs` are tagged `json:"-"` and
				// skipped by the json-tag walk.
			},
		},
	})
}

func TestManifestProtoRoundTrip(t *testing.T) {
	codec := loadManifestCodec(t)
	regoV1 := 1

	tests := []struct {
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
		{
			note: "wasm resolver with annotations",
			manifest: bundle.Manifest{
				Revision: "abc123",
				WasmResolvers: []bundle.WasmResolver{{
					Entrypoint: "http/example/authz/allow",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{{
						Scope:         "rule",
						Title:         "Allow rule",
						Description:   "Permits authorized requests",
						Entrypoint:    true,
						Organizations: []string{"acme"},
						Authors: []*ast.AuthorAnnotation{
							{Name: "Alice", Email: "alice@example.com"},
						},
						Custom: map[string]any{"sla": "p99-100ms"},
						Labels: map[string]any{"team": "platform"},
					}},
				}},
			},
		},
		{
			note: "annotations with refs, schemas, compile, location",
			manifest: bundle.Manifest{
				Revision: "abc123",
				WasmResolvers: []bundle.WasmResolver{{
					Entrypoint: "http/example/authz/allow",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{{
						Scope: "package",
						RelatedResources: []*ast.RelatedResourceAnnotation{
							{Ref: mustParseURL("https://example.com/docs"), Description: "design doc"},
						},
						Schemas: []*ast.SchemaAnnotation{
							{
								Path:       ast.MustParseRef("input.user"),
								Schema:     ast.MustParseRef("schema.user"),
								Definition: ptrAny(map[string]any{"type": "object"}),
							},
						},
						Compile: &ast.CompileAnnotation{
							Unknowns: []ast.Ref{ast.MustParseRef("input.x"), ast.MustParseRef("input.y")},
							MaskRule: ast.MustParseRef("data.policy.mask"),
						},
						Location: &location.Location{File: "policy.rego", Row: 3, Col: 1},
					}},
				}},
			},
		},
	}

	opts := []cmp.Option{
		cmpopts.EquateEmpty(),
		// ast.Annotations carries unexported `comments` and `node` that
		// aren't part of the wire shape; ignore them in the diff.
		cmpopts.IgnoreUnexported(ast.Annotations{}),
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			bs, err := codec.Encode(&tc.manifest)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			var got bundle.Manifest
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

func normalizeRoots(m *bundle.Manifest) {
	if m.Roots != nil && len(*m.Roots) == 0 {
		m.Roots = nil
	}
}

func mustParseURL(s string) url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return *u
}

func ptrAny(v any) *any {
	return &v
}

func loadManifestCodec(t *testing.T) *protoroundtrip.Codec {
	t.Helper()
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{"../../v1/bundle"},
		}),
	}
	files, err := c.Compile(context.Background(), "manifest.proto")
	if err != nil {
		t.Fatalf("compile manifest.proto: %v", err)
	}
	codec := protoroundtrip.NewCodec(files[0])
	codec.RegisterRoot(reflect.TypeOf(bundle.Manifest{}), "Manifest")
	codec.RegisterRoot(reflect.TypeOf(bundle.WasmResolver{}), "WasmResolver")
	codec.RegisterRoot(reflect.TypeOf(ast.Annotations{}), "Annotations")
	codec.RegisterRoot(reflect.TypeOf(ast.SchemaAnnotation{}), "SchemaAnnotation")
	codec.RegisterRoot(reflect.TypeOf(ast.CompileAnnotation{}), "CompileAnnotation")
	codec.RegisterRoot(reflect.TypeOf(ast.AuthorAnnotation{}), "AuthorAnnotation")
	codec.RegisterRoot(reflect.TypeOf(ast.RelatedResourceAnnotation{}), "RelatedResourceAnnotation")
	codec.RegisterRoot(reflect.TypeOf(location.Location{}), "Location")

	// ast.Ref ↔ canonical dotted form. Empty refs round-trip through "".
	codec.RegisterScalarConverter(reflect.TypeOf(ast.Ref{}), protoroundtrip.ScalarConverter{
		Encode: func(rv reflect.Value) (string, error) {
			ref := rv.Interface().(ast.Ref)
			if len(ref) == 0 {
				return "", nil
			}
			return ref.String(), nil
		},
		Decode: func(s string) (reflect.Value, error) {
			if s == "" {
				return reflect.ValueOf(ast.Ref(nil)), nil
			}
			ref, err := ast.ParseRef(s)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(ref), nil
		},
	})

	// url.URL ↔ its String() form. url.Parse("") returns the zero URL
	// without error, so empty strings round-trip naturally.
	codec.RegisterScalarConverter(reflect.TypeOf(url.URL{}), protoroundtrip.ScalarConverter{
		Encode: func(rv reflect.Value) (string, error) {
			u := rv.Interface().(url.URL)
			return u.String(), nil
		},
		Decode: func(s string) (reflect.Value, error) {
			u, err := url.Parse(s)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(*u), nil
		},
	})

	return codec
}
