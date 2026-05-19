// Command genplanschema writes the IR plan JSON Schema, generated from the
// Go type definitions in v1/ir, to the path given as its single argument.
//
// Invoked via go:generate from main.go.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strings"

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
	b := newSchemaBuilder()
	rootRef, err := b.addStruct(reflect.TypeOf(ir.Policy{}))
	if err != nil {
		return nil, err
	}

	root := orderedMap{
		{"$schema", "https://json-schema.org/draft/2020-12/schema"},
		{"$id", "https://openpolicyagent.org/schemas/ir/v1/plan.schema.json"},
		{"title", "OPA IR Plan"},
		{"description", "JSON Schema for the IR plan produced by `opa build -t plan`. Generated from v1/ir/ir.go."},
		{"$ref", rootRef},
		{"$defs", b.defsOrdered()},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type schemaBuilder struct {
	defs map[string]orderedMap
}

func newSchemaBuilder() *schemaBuilder {
	return &schemaBuilder{defs: map[string]orderedMap{}}
}

func (b *schemaBuilder) defsOrdered() orderedMap {
	names := make([]string, 0, len(b.defs))
	for n := range b.defs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make(orderedMap, 0, len(names))
	for _, n := range names {
		out = append(out, orderedEntry{n, b.defs[n]})
	}
	return out
}

func (b *schemaBuilder) addStruct(t reflect.Type) (string, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("addStruct: expected struct, got %s", t.Kind())
	}
	name := t.Name()
	if name == "" {
		return "", errors.New("addStruct: anonymous struct not supported")
	}
	if _, ok := b.defs[name]; ok {
		return "#/$defs/" + name, nil
	}
	// Some structs ship a hand-written MarshalJSON whose JSON shape diverges
	// from what reflection alone can infer. Override those.
	if override, ok := structOverrides[name]; ok {
		b.defs[name] = override()
		return "#/$defs/" + name, nil
	}
	b.defs[name] = nil

	schema, err := b.reflectStructBody(t)
	if err != nil {
		return "", err
	}
	b.defs[name] = schema
	return "#/$defs/" + name, nil
}

func (b *schemaBuilder) reflectStructBody(t reflect.Type) (orderedMap, error) {
	properties := orderedMap{}
	var required []string

	if err := b.collectFields(t, &properties, &required); err != nil {
		return nil, err
	}

	sort.Strings(required)

	out := orderedMap{
		{"type", "object"},
		{"properties", properties},
	}
	if len(required) > 0 {
		out = append(out, orderedEntry{"required", required})
	}
	out = append(out, orderedEntry{"additionalProperties", false})
	return out, nil
}

func (b *schemaBuilder) collectFields(t reflect.Type, properties *orderedMap, required *[]string) error {
	type pendingField struct {
		name     string
		schema   any
		required bool
	}
	var fields []pendingField

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if err := b.collectFields(ft, properties, required); err != nil {
					return err
				}
				continue
			}
		}
		name, opts := parseJSONTag(f.Tag.Get("json"), f.Name)
		if name == "-" {
			continue
		}
		schema, err := b.reflectType(f.Type)
		if err != nil {
			return fmt.Errorf("field %s.%s: %w", t.Name(), f.Name, err)
		}
		// encoding/json emits "null" for nil slices, maps, pointers, and
		// interfaces. When such a field is not tagged omitempty, the encoder
		// includes it (as null) rather than skipping it, so the schema must
		// admit null in addition to the field's nominal type.
		if !opts.omitempty && fieldCanBeNull(f.Type) {
			schema = makeNullable(schema)
		}
		fields = append(fields, pendingField{
			name:     name,
			schema:   schema,
			required: !opts.omitempty,
		})
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].name < fields[j].name })
	for _, f := range fields {
		*properties = append(*properties, orderedEntry{f.name, f.schema})
		if f.required {
			*required = append(*required, f.name)
		}
	}
	return nil
}

func (b *schemaBuilder) reflectType(t reflect.Type) (any, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return orderedMap{{"type", "string"}}, nil
	case reflect.Bool:
		return orderedMap{{"type", "boolean"}}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return orderedMap{{"type", "integer"}}, nil
	case reflect.Float32, reflect.Float64:
		return orderedMap{{"type", "number"}}, nil
	case reflect.Slice, reflect.Array:
		items, err := b.reflectType(t.Elem())
		if err != nil {
			return nil, err
		}
		return orderedMap{
			{"type", "array"},
			{"items", items},
		}, nil
	case reflect.Struct:
		switch {
		case t == reflect.TypeOf(ir.Operand{}):
			ref, err := b.addOperand()
			if err != nil {
				return nil, err
			}
			return orderedMap{{"$ref", ref}}, nil
		case t == reflect.TypeOf(ir.Block{}):
			ref, err := b.addBlock()
			if err != nil {
				return nil, err
			}
			return orderedMap{{"$ref", ref}}, nil
		case t.PkgPath() == "github.com/open-policy-agent/opa/v1/types" && t.Name() == "Function":
			// BuiltinFunc.Decl: opaque slot. The full types.Function shape is
			// out of scope per the issue's "good enough" criteria.
			return orderedMap{
				{"type", "object"},
				{"description", "BuiltinFunc declaration; opaque in this schema."},
			}, nil
		}
		ref, err := b.addStruct(t)
		if err != nil {
			return nil, err
		}
		return orderedMap{{"$ref", ref}}, nil
	case reflect.Interface:
		switch {
		case t == reflect.TypeOf((*ir.Stmt)(nil)).Elem():
			ref, err := b.addStmtUnion()
			if err != nil {
				return nil, err
			}
			return orderedMap{{"$ref", ref}}, nil
		case t == reflect.TypeOf((*ir.Val)(nil)).Elem():
			ref, err := b.addValUnion()
			if err != nil {
				return nil, err
			}
			return orderedMap{{"$ref", ref}}, nil
		}
	}
	return nil, fmt.Errorf("unsupported type %s (kind %s)", t.String(), t.Kind())
}

func (b *schemaBuilder) addOperand() (string, error) {
	if _, ok := b.defs["Operand"]; ok {
		return "#/$defs/Operand", nil
	}
	b.defs["Operand"] = nil
	ref, err := b.addValUnion()
	if err != nil {
		return "", err
	}
	b.defs["Operand"] = orderedMap{{"$ref", ref}}
	return "#/$defs/Operand", nil
}

func (b *schemaBuilder) addValUnion() (string, error) {
	const name = "Val"
	if _, ok := b.defs[name]; ok {
		return "#/$defs/" + name, nil
	}
	vals := ir.ValKinds()
	kinds := sortedKeys(vals)
	branches := make([]any, 0, len(kinds))
	for _, kind := range kinds {
		valueSchema, err := b.reflectType(reflect.TypeOf(vals[kind]))
		if err != nil {
			return "", fmt.Errorf("val %q: %w", kind, err)
		}
		branches = append(branches, orderedMap{
			{"type", "object"},
			{"properties", orderedMap{
				{"type", orderedMap{{"const", kind}}},
				{"value", valueSchema},
			}},
			{"required", []string{"type", "value"}},
			{"additionalProperties", false},
		})
	}
	b.defs[name] = orderedMap{{"oneOf", branches}}
	return "#/$defs/" + name, nil
}

func (b *schemaBuilder) addBlock() (string, error) {
	if _, ok := b.defs["Block"]; ok {
		return "#/$defs/Block", nil
	}
	b.defs["Block"] = nil
	stmtRef, err := b.addStmtUnion()
	if err != nil {
		return "", err
	}
	b.defs["Block"] = orderedMap{
		{"type", "object"},
		{"properties", orderedMap{
			{"stmts", orderedMap{
				{"type", "array"},
				{"items", orderedMap{{"$ref", stmtRef}}},
			}},
		}},
		{"required", []string{"stmts"}},
		{"additionalProperties", false},
	}
	return "#/$defs/Block", nil
}

func (b *schemaBuilder) addStmtUnion() (string, error) {
	const name = "Stmt"
	if _, ok := b.defs[name]; ok {
		return "#/$defs/" + name, nil
	}
	// Reserve before recursing — concrete stmt bodies may contain *Block,
	// which recurses back through addStmtUnion.
	b.defs[name] = nil

	stmts := ir.StmtKinds()
	kinds := sortedKeys(stmts)
	branches := make([]any, 0, len(kinds))
	for _, kind := range kinds {
		bodyRef, err := b.addStruct(reflect.TypeOf(stmts[kind]))
		if err != nil {
			return "", fmt.Errorf("stmt %q: %w", kind, err)
		}
		branches = append(branches, orderedMap{
			{"type", "object"},
			{"properties", orderedMap{
				{"type", orderedMap{{"const", kind}}},
				{"stmt", orderedMap{{"$ref", bodyRef}}},
			}},
			{"required", []string{"type", "stmt"}},
			{"additionalProperties", false},
		})
	}
	b.defs[name] = orderedMap{{"oneOf", branches}}
	return "#/$defs/" + name, nil
}

// structOverrides maps struct type names to schema builders that bypass
// reflection. Use sparingly — only for types whose JSON form is governed by
// a hand-written MarshalJSON whose shape reflection alone cannot infer.
var structOverrides = map[string]func() orderedMap{
	"MakeNumberRefStmt": makeNumberRefStmtSchema,
}

// makeNumberRefStmtSchema mirrors MakeNumberRefStmt's MarshalJSON, which
// emits both the canonical "index" key and the deprecated "Index" key for
// backwards compatibility. "index" is required; "Index" is permitted but
// flagged deprecated so consumers know not to depend on it.
func makeNumberRefStmtSchema() orderedMap {
	return orderedMap{
		{"type", "object"},
		{"properties", orderedMap{
			{"col", orderedMap{{"type", "integer"}}},
			{"file", orderedMap{{"type", "integer"}}},
			{"row", orderedMap{{"type", "integer"}}},
			{"index", orderedMap{{"type", "integer"}}},
			{"Index", orderedMap{
				{"type", "integer"},
				{"deprecated", true},
				{"description", "Deprecated alias for `index`. Both keys are emitted by current OPA versions for backwards compatibility; will be removed in a future major release. Read `index` instead."},
			}},
			{"target", orderedMap{{"type", "integer"}}},
		}},
		{"required", []string{"col", "file", "index", "row", "target"}},
		{"additionalProperties", false},
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fieldCanBeNull(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface:
		return true
	}
	return false
}

// makeNullable returns a schema equivalent to the input that also accepts the
// JSON null value. For schemas built around "type": "X", the type field is
// widened to ["X", "null"]; for $ref schemas (which can't be widened in
// place), the result is a oneOf over the original and {"type": "null"}.
func makeNullable(schema any) any {
	m, ok := schema.(orderedMap)
	if !ok {
		return schema
	}
	for i, e := range m {
		if e.Key == "type" {
			if s, ok := e.Value.(string); ok {
				m[i] = orderedEntry{"type", []string{s, "null"}}
				return m
			}
		}
	}
	return orderedMap{
		{"oneOf", []any{m, orderedMap{{"type", "null"}}}},
	}
}

type jsonTagOpts struct {
	omitempty bool
}

func parseJSONTag(tag, fieldName string) (string, jsonTagOpts) {
	if tag == "" {
		return fieldName, jsonTagOpts{}
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = fieldName
	}
	var opts jsonTagOpts
	for _, p := range parts[1:] {
		if p == "omitempty" {
			opts.omitempty = true
		}
	}
	return name, opts
}

// orderedMap preserves insertion order for JSON object encoding so the
// generated schema is byte-stable across runs.
type orderedMap []orderedEntry

type orderedEntry struct {
	Key   string
	Value any
}

func (m orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range m {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(e.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
