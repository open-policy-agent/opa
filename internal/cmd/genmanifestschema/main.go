// Command genmanifestschema writes the bundle Manifest JSON Schema, generated
// from the Go type definitions in v1/bundle, to the path given as its single
// argument.
//
// Invoked via go:generate from main.go.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"reflect"

	"github.com/open-policy-agent/opa/internal/genjsonschema"
	"github.com/open-policy-agent/opa/v1/bundle"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s path/to/manifest.schema.json", os.Args[0])
	}
	bs, err := reflectSchema()
	if err != nil {
		log.Fatalf("reflect schema: %v", err)
	}
	if err := os.WriteFile(os.Args[1], bs, 0o644); err != nil {
		log.Fatalf("write %s: %v", os.Args[1], err)
	}
}

// reflectSchema generates a JSON Schema describing the bundle manifest emitted
// by `opa build`.
func reflectSchema() ([]byte, error) {
	b := genjsonschema.NewBuilder(manifestResolver)
	// The bundle loader accepts unknown top-level keys (no
	// DisallowUnknownFields), and embedders rely on this to attach custom
	// configuration alongside the documented fields. Keep the schema in
	// step with that contract; sub-records like WasmResolver stay strict.
	b.AllowAdditionalProperties(reflect.TypeFor[bundle.Manifest]())
	rootRef, err := b.AddStruct(reflect.TypeFor[bundle.Manifest]())
	if err != nil {
		return nil, err
	}

	root := genjsonschema.Map(
		"$schema", "https://json-schema.org/draft/2020-12/schema",
		"$id", "https://openpolicyagent.org/schemas/bundle/v1/manifest.schema.json",
		"title", "OPA Bundle Manifest",
		"description", "JSON Schema for the bundle `.manifest` file produced by `opa build`. Generated from v1/bundle/bundle.go.",
		"$ref", rootRef,
		"$defs", b.DefsOrdered(),
	)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// manifestResolver intercepts types whose JSON shape isn't usefully derivable
// from straight reflection. Today that's just ast.Annotations: it ships a
// hand-written MarshalJSON whose nested types (Ref, Location, etc.) carry
// their own custom encoders; modeling the full shape is out of scope per the
// issue's "good enough" criteria.
func manifestResolver(_ *genjsonschema.Builder, t reflect.Type) (any, bool, error) {
	if t.Kind() == reflect.Struct &&
		t.PkgPath() == "github.com/open-policy-agent/opa/v1/ast" &&
		t.Name() == "Annotations" {
		return genjsonschema.Map(
			"type", "object",
			"description", "Rego annotations; opaque in this schema. See the OPA documentation for the full annotation shape.",
		), true, nil
	}
	return nil, false, nil
}
