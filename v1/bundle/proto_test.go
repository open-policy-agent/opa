// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	pb "github.com/open-policy-agent/opa/v1/bundle/v1pb"
)

func TestManifestProtoRoundTrip(t *testing.T) {
	regoV1 := 1
	roots := []string{"", "a/b"}
	relatedURL, err := url.Parse("https://example.com/policy")
	if err != nil {
		t.Fatal(err)
	}
	def := any(map[string]any{"type": "string"})

	m := &Manifest{
		Revision: "rev-1",
		Roots:    &roots,
		WasmResolvers: []WasmResolver{{
			Entrypoint: "data.example.allow",
			Module:     "/policy.wasm",
			Annotations: []*ast.Annotations{{
				Scope:         "rule",
				Title:         "t",
				Entrypoint:    true,
				Description:   "d",
				Organizations: []string{"o1", "o2"},
				RelatedResources: []*ast.RelatedResourceAnnotation{{
					Ref:         *relatedURL,
					Description: "rr",
				}},
				Authors: []*ast.AuthorAnnotation{{Name: "Ada", Email: "ada@example.com"}},
				Schemas: []*ast.SchemaAnnotation{{
					Path:       ast.MustParseRef("input.x"),
					Schema:     ast.MustParseRef("schema.foo"),
					Definition: &def,
				}},
				Compile: &ast.CompileAnnotation{
					MaskRule: ast.MustParseRef("data.mask.rule"),
					Unknowns: []ast.Ref{ast.MustParseRef("input.unknown")},
				},
				Custom:   map[string]any{"k": "v"},
				Labels:   map[string]any{"team": "core"},
				Location: &location.Location{File: "f.rego", Row: 1, Col: 2},
			}},
		}},
		RegoVersion:      &regoV1,
		FileRegoVersions: map[string]int{"a.rego": 1},
		Metadata:         map[string]any{"k": "v", "n": float64(7)},
	}

	pbManifest, err := ManifestToProto(m)
	if err != nil {
		t.Fatalf("to proto: %v", err)
	}
	bs, err := proto.Marshal(pbManifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &pb.Manifest{}
	if err := proto.Unmarshal(bs, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := ManifestFromProto(decoded)
	if err != nil {
		t.Fatalf("from proto: %v", err)
	}

	if !m.Equal(*got) {
		t.Fatal("manifest semantic equality failed after round trip")
	}
	if !maps.Equal(m.FileRegoVersions, got.FileRegoVersions) {
		t.Fatalf("file rego versions: want %v, got %v", m.FileRegoVersions, got.FileRegoVersions)
	}
	if got.WasmResolvers[0].Annotations[0].Compile.MaskRule.String() != "data.mask.rule" {
		t.Fatalf("compile mask rule round-trip lost: %v", got.WasmResolvers[0].Annotations[0].Compile.MaskRule)
	}
	if got.WasmResolvers[0].Annotations[0].RelatedResources[0].Ref.String() != "https://example.com/policy" {
		t.Fatalf("related resource ref round-trip lost: %v", got.WasmResolvers[0].Annotations[0].RelatedResources[0].Ref)
	}
}

func TestManifestToProtoNil(t *testing.T) {
	got, err := ManifestToProto(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestManifestFromProtoNil(t *testing.T) {
	got, err := ManifestFromProto(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// TestManifestMetadataAcceptsJSONTypes pins fidelity parity with the
// JSON path. structpb.NewStruct rejects ints, json.RawMessage,
// time-like values, etc.; the encoder routes Metadata through JSON so
// the proto path accepts whatever the JSON path does.
func TestManifestMetadataAcceptsJSONTypes(t *testing.T) {
	m := &Manifest{
		Revision: "r",
		Metadata: map[string]any{
			"count": int(7),
			"raw":   json.RawMessage(`{"k":"v"}`),
			"nested": map[string]any{
				"items": []any{int64(1), int64(2)},
			},
		},
	}
	pm, err := ManifestToProto(m)
	if err != nil {
		t.Fatalf("ManifestToProto rejected JSON-friendly types: %v", err)
	}
	out, err := ManifestFromProto(pm)
	if err != nil {
		t.Fatal(err)
	}
	got := out.Metadata
	if got["count"].(float64) != 7 {
		t.Fatalf("count not preserved: %v", got["count"])
	}
	raw, ok := got["raw"].(map[string]any)
	if !ok || raw["k"].(string) != "v" {
		t.Fatalf("json.RawMessage not preserved: %v", got["raw"])
	}
}

func TestManifestFromProtoMalformedRef(t *testing.T) {
	pm := &pb.Manifest{
		Wasm: []*pb.WasmResolver{{
			Annotations: []*pb.Annotations{{
				Compile: &pb.CompileAnnotation{MaskRule: new("not a ref!!!")},
			}},
		}},
	}
	if _, err := ManifestFromProto(pm); err == nil {
		t.Fatal("expected error for malformed mask_rule, got nil")
	}
}

// TestManifestRootsPresenceRoundTrip pins the nil-vs-explicit-empty
// distinction across the proto round-trip. `repeated string` cannot
// carry that bit on its own; the `roots_set` wire field does.
func TestManifestRootsPresenceRoundTrip(t *testing.T) {
	cases := []struct {
		note string
		in   *[]string
	}{
		{note: "nil (default to [\"\"])", in: nil},
		{note: "explicit empty (owns nothing)", in: &[]string{}},
		{note: "populated", in: &[]string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			pm, err := ManifestToProto(&Manifest{Revision: "r", Roots: tc.in})
			if err != nil {
				t.Fatal(err)
			}
			out, err := ManifestFromProto(pm)
			if err != nil {
				t.Fatal(err)
			}
			if (tc.in == nil) != (out.Roots == nil) {
				t.Fatalf("roots nil-ness lost: in=%v out=%v", tc.in, out.Roots)
			}
			if tc.in != nil && !slices.Equal(*tc.in, *out.Roots) {
				t.Fatalf("roots values lost: want %v, got %v", *tc.in, *out.Roots)
			}
		})
	}
}

// TestSchemaAnnotationBareVarRoundTrip pins the schema-ref decoder.
// Annotations using `schemas: [{input: schema}]` produce a bare Var Ref
// from the canonical parser (parseSchemaRef in v1/ast). The encoder
// serializes that as the literal "schema". ast.ParseRef cannot decode a
// bare Var, so the proto round-trip used to fail with `expected ref but
// got schema` — load-blocking for any annotation using this idiom.
func TestSchemaAnnotationBareVarRoundTrip(t *testing.T) {
	cases := []struct {
		note string
		ref  ast.Ref
	}{
		{note: "bare schema var", ref: ast.SchemaRootRef.Copy()},
		{note: "schema with subpath", ref: ast.MustParseRef("schema.foo")},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			m := &Manifest{
				Revision: "r1",
				WasmResolvers: []WasmResolver{{
					Annotations: []*ast.Annotations{{
						Schemas: []*ast.SchemaAnnotation{{
							Path:   ast.MustParseRef("input.x"),
							Schema: tc.ref,
						}},
					}},
				}},
			}

			pm, err := ManifestToProto(m)
			if err != nil {
				t.Fatal(err)
			}
			out, err := ManifestFromProto(pm)
			if err != nil {
				t.Fatalf("decode failed for %v: %v", tc.ref, err)
			}
			gotRef := out.WasmResolvers[0].Annotations[0].Schemas[0].Schema
			if !gotRef.Equal(tc.ref) {
				t.Fatalf("schema ref not preserved: want %v, got %v", tc.ref, gotRef)
			}
		})
	}
}
