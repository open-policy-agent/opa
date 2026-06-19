// Command genplanschema writes the IR plan JSON Schema, generated from the
// Go type definitions in v1/ir, to the path given as its single argument.
//
// Invoked via go:generate from main.go.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"

	"github.com/open-policy-agent/opa/internal/genjsonschema"
	"github.com/open-policy-agent/opa/v1/ir"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s path/to/plan.schema.json", os.Args[0])
	}
	bs, err := reflectSchema()
	if err != nil {
		log.Fatalf("reflect schema: %v", err)
	}
	if err := os.WriteFile(os.Args[1], bs, 0o644); err != nil {
		log.Fatalf("write %s: %v", os.Args[1], err)
	}
}

// reflectSchema generates a JSON Schema describing the IR plan produced by `opa build -t plan`.
func reflectSchema() ([]byte, error) {
	b := genjsonschema.NewBuilder(planResolver)

	// MakeNumberRefStmt's MarshalJSON emits both the canonical "index" key
	// and the deprecated "Index" key for backwards compatibility. Pre-register
	// the hand-written schema so any AddStruct that would otherwise reflect
	// the type short-circuits to it.
	if _, err := b.AddNamedDef("MakeNumberRefStmt", makeNumberRefStmtSchema()); err != nil {
		return nil, err
	}

	rootRef, err := b.AddStruct(reflect.TypeOf(ir.Policy{}))
	if err != nil {
		return nil, err
	}

	root := genjsonschema.Map(
		"$schema", "https://json-schema.org/draft/2020-12/schema",
		"$id", "https://openpolicyagent.org/schemas/ir/v1/plan.schema.json",
		"title", "OPA IR Plan",
		"description", "JSON Schema for the IR plan produced by `opa build -t plan`. Generated from v1/ir/ir.go.",
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

// planResolver intercepts IR-specific types whose JSON shape isn't derivable
// from straight reflection: the polymorphic Stmt/Val unions, the discriminated
// Operand/Block envelopes, and the opaque types.Function declaration on
// BuiltinFunc.
func planResolver(b *genjsonschema.Builder, t reflect.Type) (any, bool, error) {
	switch t.Kind() {
	case reflect.Struct:
		switch {
		case t == reflect.TypeOf(ir.Operand{}):
			ref, err := addOperand(b)
			if err != nil {
				return nil, false, err
			}
			return genjsonschema.Map("$ref", ref), true, nil
		case t == reflect.TypeOf(ir.Block{}):
			ref, err := addBlock(b)
			if err != nil {
				return nil, false, err
			}
			return genjsonschema.Map("$ref", ref), true, nil
		case t.PkgPath() == "github.com/open-policy-agent/opa/v1/types" && t.Name() == "Function":
			// BuiltinFunc.Decl: opaque slot. The full types.Function shape is
			// out of scope per the issue's "good enough" criteria.
			return genjsonschema.Map(
				"type", "object",
				"description", "BuiltinFunc declaration; opaque in this schema.",
			), true, nil
		}
	case reflect.Interface:
		switch {
		case t == reflect.TypeOf((*ir.Stmt)(nil)).Elem():
			ref, err := addStmtUnion(b)
			if err != nil {
				return nil, false, err
			}
			return genjsonschema.Map("$ref", ref), true, nil
		case t == reflect.TypeOf((*ir.Val)(nil)).Elem():
			ref, err := addValUnion(b)
			if err != nil {
				return nil, false, err
			}
			return genjsonschema.Map("$ref", ref), true, nil
		}
	}
	return nil, false, nil
}

func addOperand(b *genjsonschema.Builder) (string, error) {
	const name = "Operand"
	if !b.Reserve(name) {
		return b.DefRef(name), nil
	}
	ref, err := addValUnion(b)
	if err != nil {
		return "", err
	}
	b.SetDef(name, genjsonschema.Map("$ref", ref))
	return b.DefRef(name), nil
}

func addValUnion(b *genjsonschema.Builder) (string, error) {
	const name = "Val"
	if !b.Reserve(name) {
		return b.DefRef(name), nil
	}
	vals := ir.ValKinds()
	kinds := sortedKeys(vals)
	branches := make([]any, 0, len(kinds))
	for _, kind := range kinds {
		valueSchema, err := b.ReflectType(reflect.TypeOf(vals[kind]))
		if err != nil {
			return "", fmt.Errorf("val %q: %w", kind, err)
		}
		branches = append(branches, genjsonschema.Map(
			"type", "object",
			"properties", genjsonschema.Map(
				"type", genjsonschema.Map("const", kind),
				"value", valueSchema,
			),
			"required", []string{"type", "value"},
			"additionalProperties", false,
		))
	}
	b.SetDef(name, genjsonschema.Map("oneOf", branches))
	return b.DefRef(name), nil
}

func addBlock(b *genjsonschema.Builder) (string, error) {
	const name = "Block"
	if !b.Reserve(name) {
		return b.DefRef(name), nil
	}
	stmtRef, err := addStmtUnion(b)
	if err != nil {
		return "", err
	}
	b.SetDef(name, genjsonschema.Map(
		"type", "object",
		"properties", genjsonschema.Map(
			"stmts", genjsonschema.Map(
				"type", "array",
				"items", genjsonschema.Map("$ref", stmtRef),
			),
		),
		"required", []string{"stmts"},
		"additionalProperties", false,
	))
	return b.DefRef(name), nil
}

func addStmtUnion(b *genjsonschema.Builder) (string, error) {
	const name = "Stmt"
	if !b.Reserve(name) {
		return b.DefRef(name), nil
	}

	stmts := ir.StmtKinds()
	kinds := sortedKeys(stmts)
	branches := make([]any, 0, len(kinds))
	for _, kind := range kinds {
		bodyRef, err := b.AddStruct(reflect.TypeOf(stmts[kind]))
		if err != nil {
			return "", fmt.Errorf("stmt %q: %w", kind, err)
		}
		branches = append(branches, genjsonschema.Map(
			"type", "object",
			"properties", genjsonschema.Map(
				"type", genjsonschema.Map("const", kind),
				"stmt", genjsonschema.Map("$ref", bodyRef),
			),
			"required", []string{"type", "stmt"},
			"additionalProperties", false,
		))
	}
	b.SetDef(name, genjsonschema.Map("oneOf", branches))
	return b.DefRef(name), nil
}

// makeNumberRefStmtSchema mirrors MakeNumberRefStmt's MarshalJSON, which
// emits both the canonical "index" key and the deprecated "Index" key for
// backwards compatibility. "index" is required; "Index" is permitted but
// flagged deprecated so consumers know not to depend on it.
func makeNumberRefStmtSchema() genjsonschema.OrderedMap {
	return genjsonschema.Map(
		"type", "object",
		"properties", genjsonschema.Map(
			"col", genjsonschema.Map("type", "integer"),
			"file", genjsonschema.Map("type", "integer"),
			"row", genjsonschema.Map("type", "integer"),
			"index", genjsonschema.Map("type", "integer"),
			"Index", genjsonschema.Map(
				"type", "integer",
				"deprecated", true,
				"description", "Deprecated alias for `index`. Both keys are emitted by current OPA versions for backwards compatibility; will be removed in a future major release. Read `index` instead.",
			),
			"target", genjsonschema.Map("type", "integer"),
		),
		"required", []string{"col", "file", "index", "row", "target"},
		"additionalProperties", false,
	)
}

// sortedKeys returns the keys of m in lexicographic order so the polymorphic
// Stmt/Val unions render their branches in a byte-stable order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
