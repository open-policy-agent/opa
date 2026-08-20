// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"encoding/json"
	"fmt"
	"net/url"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	pb "github.com/open-policy-agent/opa/v1/bundle/v1pb"
)

// ManifestToProto converts a bundle Manifest to its protobuf wire-form,
// defined in v1/bundle/manifest.proto. The compiled-only fileRegoVersions
// cache is intentionally not modeled. Roots presence (nil vs explicit-empty)
// is preserved via the `roots_set` wire field.
func ManifestToProto(m *Manifest) (*pb.Manifest, error) {
	if m == nil {
		return nil, nil
	}
	out := &pb.Manifest{
		Revision: new(m.Revision),
	}
	if m.Roots != nil {
		out.Roots = append([]string(nil), (*m.Roots)...)
		out.RootsSet = new(true)
	}
	if len(m.WasmResolvers) > 0 {
		out.Wasm = make([]*pb.WasmResolver, len(m.WasmResolvers))
		for i := range m.WasmResolvers {
			wr, err := wasmResolverToProto(&m.WasmResolvers[i])
			if err != nil {
				return nil, fmt.Errorf("manifest wasm[%d]: %w", i, err)
			}
			out.Wasm[i] = wr
		}
	}
	if m.RegoVersion != nil {
		out.RegoVersion = new(int32(*m.RegoVersion))
	}
	if len(m.FileRegoVersions) > 0 {
		out.FileRegoVersions = make(map[string]int32, len(m.FileRegoVersions))
		for k, v := range m.FileRegoVersions {
			out.FileRegoVersions[k] = int32(v)
		}
	}
	if len(m.Metadata) > 0 {
		s, err := jsonNormalizeStruct(m.Metadata)
		if err != nil {
			return nil, fmt.Errorf("manifest metadata: %w", err)
		}
		out.Metadata = s
	}
	return out, nil
}

func wasmResolverToProto(w *WasmResolver) (*pb.WasmResolver, error) {
	if w == nil {
		return nil, nil
	}
	out := &pb.WasmResolver{
		Entrypoint: new(w.Entrypoint),
		Module:     new(w.Module),
	}
	if len(w.Annotations) > 0 {
		out.Annotations = make([]*pb.Annotations, len(w.Annotations))
		for i, a := range w.Annotations {
			ap, err := annotationsToProto(a)
			if err != nil {
				return nil, fmt.Errorf("annotations[%d]: %w", i, err)
			}
			out.Annotations[i] = ap
		}
	}
	return out, nil
}

func annotationsToProto(a *ast.Annotations) (*pb.Annotations, error) {
	if a == nil {
		return nil, nil
	}
	out := &pb.Annotations{
		Scope:         new(a.Scope),
		Title:         new(a.Title),
		Entrypoint:    new(a.Entrypoint),
		Description:   new(a.Description),
		Organizations: append([]string(nil), a.Organizations...),
	}
	if len(a.RelatedResources) > 0 {
		out.RelatedResources = make([]*pb.RelatedResourceAnnotation, len(a.RelatedResources))
		for i, r := range a.RelatedResources {
			out.RelatedResources[i] = relatedResourceToProto(r)
		}
	}
	if len(a.Authors) > 0 {
		out.Authors = make([]*pb.AuthorAnnotation, len(a.Authors))
		for i, au := range a.Authors {
			out.Authors[i] = authorToProto(au)
		}
	}
	if len(a.Schemas) > 0 {
		out.Schemas = make([]*pb.SchemaAnnotation, len(a.Schemas))
		for i, s := range a.Schemas {
			sa, err := schemaToProto(s)
			if err != nil {
				return nil, fmt.Errorf("schemas[%d]: %w", i, err)
			}
			out.Schemas[i] = sa
		}
	}
	if a.Compile != nil {
		out.Compile = compileToProto(a.Compile)
	}
	if len(a.Custom) > 0 {
		s, err := jsonNormalizeStruct(a.Custom)
		if err != nil {
			return nil, fmt.Errorf("custom: %w", err)
		}
		out.Custom = s
	}
	if len(a.Labels) > 0 {
		s, err := jsonNormalizeStruct(a.Labels)
		if err != nil {
			return nil, fmt.Errorf("labels: %w", err)
		}
		out.Labels = s
	}
	if a.Location != nil {
		out.Location = locationToProto(a.Location)
	}
	return out, nil
}

func relatedResourceToProto(r *ast.RelatedResourceAnnotation) *pb.RelatedResourceAnnotation {
	if r == nil {
		return nil
	}
	return &pb.RelatedResourceAnnotation{
		Ref:         new(r.Ref.String()),
		Description: new(r.Description),
	}
}

func authorToProto(a *ast.AuthorAnnotation) *pb.AuthorAnnotation {
	if a == nil {
		return nil
	}
	return &pb.AuthorAnnotation{
		Name:  new(a.Name),
		Email: new(a.Email),
	}
}

func schemaToProto(s *ast.SchemaAnnotation) (*pb.SchemaAnnotation, error) {
	if s == nil {
		return nil, nil
	}
	out := &pb.SchemaAnnotation{
		Path:   new(s.Path.String()),
		Schema: new(s.Schema.String()),
	}
	if s.Definition != nil {
		v, err := jsonNormalizeValue(*s.Definition)
		if err != nil {
			return nil, fmt.Errorf("definition: %w", err)
		}
		out.Definition = v
	}
	return out, nil
}

func compileToProto(c *ast.CompileAnnotation) *pb.CompileAnnotation {
	if c == nil {
		return nil
	}
	out := &pb.CompileAnnotation{
		MaskRule: new(c.MaskRule.String()),
	}
	if len(c.Unknowns) > 0 {
		out.Unknowns = make([]string, len(c.Unknowns))
		for i, u := range c.Unknowns {
			out.Unknowns[i] = u.String()
		}
	}
	return out
}

func locationToProto(l *location.Location) *pb.Location {
	if l == nil {
		return nil
	}
	return &pb.Location{
		File: new(l.File),
		Row:  new(int32(l.Row)),
		Col:  new(int32(l.Col)),
	}
}

// ManifestFromProto is the inverse of ManifestToProto.
func ManifestFromProto(m *pb.Manifest) (*Manifest, error) {
	if m == nil {
		return nil, nil
	}
	out := &Manifest{
		Revision: m.GetRevision(),
	}
	if m.GetRootsSet() {
		roots := make([]string, len(m.Roots))
		copy(roots, m.Roots)
		out.Roots = &roots
	}
	if len(m.Wasm) > 0 {
		out.WasmResolvers = make([]WasmResolver, len(m.Wasm))
		for i, wr := range m.Wasm {
			converted, err := wasmResolverFromProto(wr)
			if err != nil {
				return nil, fmt.Errorf("manifest wasm[%d]: %w", i, err)
			}
			out.WasmResolvers[i] = converted
		}
	}
	if m.RegoVersion != nil {
		v := int(*m.RegoVersion)
		out.RegoVersion = &v
	}
	if len(m.FileRegoVersions) > 0 {
		out.FileRegoVersions = make(map[string]int, len(m.FileRegoVersions))
		for k, v := range m.FileRegoVersions {
			out.FileRegoVersions[k] = int(v)
		}
	}
	if m.Metadata != nil {
		out.Metadata = m.Metadata.AsMap()
	}
	return out, nil
}

func wasmResolverFromProto(w *pb.WasmResolver) (WasmResolver, error) {
	out := WasmResolver{
		Entrypoint: w.GetEntrypoint(),
		Module:     w.GetModule(),
	}
	if len(w.Annotations) > 0 {
		out.Annotations = make([]*ast.Annotations, len(w.Annotations))
		for i, a := range w.Annotations {
			converted, err := annotationsFromProto(a)
			if err != nil {
				return WasmResolver{}, fmt.Errorf("annotations[%d]: %w", i, err)
			}
			out.Annotations[i] = converted
		}
	}
	return out, nil
}

func annotationsFromProto(a *pb.Annotations) (*ast.Annotations, error) {
	if a == nil {
		return nil, nil
	}
	out := &ast.Annotations{
		Scope:       a.GetScope(),
		Title:       a.GetTitle(),
		Entrypoint:  a.GetEntrypoint(),
		Description: a.GetDescription(),
	}
	if len(a.Organizations) > 0 {
		out.Organizations = append([]string(nil), a.Organizations...)
	}
	if len(a.RelatedResources) > 0 {
		out.RelatedResources = make([]*ast.RelatedResourceAnnotation, len(a.RelatedResources))
		for i, r := range a.RelatedResources {
			converted, err := relatedResourceFromProto(r)
			if err != nil {
				return nil, fmt.Errorf("related_resources[%d]: %w", i, err)
			}
			out.RelatedResources[i] = converted
		}
	}
	if len(a.Authors) > 0 {
		out.Authors = make([]*ast.AuthorAnnotation, len(a.Authors))
		for i, au := range a.Authors {
			out.Authors[i] = authorFromProto(au)
		}
	}
	if len(a.Schemas) > 0 {
		out.Schemas = make([]*ast.SchemaAnnotation, len(a.Schemas))
		for i, s := range a.Schemas {
			converted, err := schemaFromProto(s)
			if err != nil {
				return nil, fmt.Errorf("schemas[%d]: %w", i, err)
			}
			out.Schemas[i] = converted
		}
	}
	if a.Compile != nil {
		converted, err := compileFromProto(a.Compile)
		if err != nil {
			return nil, fmt.Errorf("compile: %w", err)
		}
		out.Compile = converted
	}
	if a.Custom != nil {
		out.Custom = a.Custom.AsMap()
	}
	if a.Labels != nil {
		out.Labels = a.Labels.AsMap()
	}
	if a.Location != nil {
		out.Location = locationFromProto(a.Location)
	}
	return out, nil
}

func relatedResourceFromProto(r *pb.RelatedResourceAnnotation) (*ast.RelatedResourceAnnotation, error) {
	if r == nil {
		return nil, nil
	}
	out := &ast.RelatedResourceAnnotation{
		Description: r.GetDescription(),
	}
	if ref := r.GetRef(); ref != "" {
		u, err := url.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("ref %q: %w", ref, err)
		}
		out.Ref = *u
	}
	return out, nil
}

func authorFromProto(a *pb.AuthorAnnotation) *ast.AuthorAnnotation {
	if a == nil {
		return nil
	}
	return &ast.AuthorAnnotation{
		Name:  a.GetName(),
		Email: a.GetEmail(),
	}
}

func schemaFromProto(s *pb.SchemaAnnotation) (*ast.SchemaAnnotation, error) {
	if s == nil {
		return nil, nil
	}
	out := &ast.SchemaAnnotation{}
	if p := s.GetPath(); p != "" {
		ref, err := ast.ParseRef(p)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", p, err)
		}
		out.Path = ref
	}
	if sc := s.GetSchema(); sc != "" {
		ref, err := ast.ParseSchemaRef(sc)
		if err != nil {
			return nil, fmt.Errorf("schema %q: %w", sc, err)
		}
		out.Schema = ref
	}
	if s.Definition != nil {
		def := s.Definition.AsInterface()
		out.Definition = &def
	}
	return out, nil
}

func compileFromProto(c *pb.CompileAnnotation) (*ast.CompileAnnotation, error) {
	if c == nil {
		return nil, nil
	}
	out := &ast.CompileAnnotation{}
	if mr := c.GetMaskRule(); mr != "" {
		ref, err := ast.ParseRef(mr)
		if err != nil {
			return nil, fmt.Errorf("mask_rule %q: %w", mr, err)
		}
		out.MaskRule = ref
	}
	if len(c.Unknowns) > 0 {
		out.Unknowns = make([]ast.Ref, len(c.Unknowns))
		for i, u := range c.Unknowns {
			ref, err := ast.ParseRef(u)
			if err != nil {
				return nil, fmt.Errorf("unknowns[%d] %q: %w", i, u, err)
			}
			out.Unknowns[i] = ref
		}
	}
	return out, nil
}

func locationFromProto(l *pb.Location) *location.Location {
	if l == nil {
		return nil
	}
	return &location.Location{
		File: l.GetFile(),
		Row:  int(l.GetRow()),
		Col:  int(l.GetCol()),
	}
}

// jsonNormalizeStruct routes a map through JSON before structpb.NewStruct
// so the proto path accepts the same value types the JSON path does.
func jsonNormalizeStruct(m map[string]any) (*structpb.Struct, error) {
	bs, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(bs, &normalized); err != nil {
		return nil, err
	}
	return structpb.NewStruct(normalized)
}

func jsonNormalizeValue(v any) (*structpb.Value, error) {
	bs, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var normalized any
	if err := json.Unmarshal(bs, &normalized); err != nil {
		return nil, err
	}
	return structpb.NewValue(normalized)
}
