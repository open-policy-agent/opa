// Package genjsonschema builds a JSON Schema (Draft 2020-12) by reflecting
// over Go type definitions. It powers OPA's `genplanschema` and
// `genmanifestschema` commands: each generator wraps a Builder, plugs in a
// TypeResolver for its domain-specific shapes, walks a root struct, and
// renders the accumulated $defs.
//
// The Builder treats struct types as named definitions referenced via
// `#/$defs/<TypeName>`, and reflects fields based on their `json:"..."` tags
// (handling `omitempty`, embedded structs, and skipping unexported fields).
package genjsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// TypeResolver returns a JSON Schema for t and reports handled=true to short
// circuit the Builder's default handling. A resolver runs first on every
// type the Builder visits via ReflectType (after pointer unwrapping), so it
// can intercept polymorphic interfaces, opaque types, and named structs
// whose JSON shape isn't derivable from reflection alone.
//
// A resolver that reports handled=true must return a non-nil schema; the
// Builder treats a nil schema with handled=true as an error so silent JSON
// `null` output is impossible.
//
// The Builder is passed in so resolvers can recurse — e.g., to translate the
// underlying type of a polymorphic union and accumulate further $defs.
type TypeResolver func(b *Builder, t reflect.Type) (schema any, handled bool, err error)

// Builder accumulates struct definitions in $defs and offers ReflectType /
// AddStruct entry points used by both the caller and its TypeResolver.
type Builder struct {
	defs                map[string]OrderedMap
	resolver            TypeResolver
	openAdditionalProps map[reflect.Type]bool
}

// NewBuilder returns a Builder. resolver may be nil, in which case only
// the built-in cases are used.
func NewBuilder(resolver TypeResolver) *Builder {
	return &Builder{
		defs:                map[string]OrderedMap{},
		resolver:            resolver,
		openAdditionalProps: map[reflect.Type]bool{},
	}
}

// AllowAdditionalProperties opts t out of the default `additionalProperties:
// false` constraint, so the generated schema accepts unknown keys on that
// struct. Use this for top-level types whose runtime decoder is lenient and
// where embedders are known to attach custom fields (e.g. the bundle
// Manifest).
func (b *Builder) AllowAdditionalProperties(t reflect.Type) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	b.openAdditionalProps[t] = true
}

// DefsOrdered returns the accumulated $defs in name-sorted order so the
// rendered schema is byte-stable across runs.
func (b *Builder) DefsOrdered() OrderedMap {
	names := make([]string, 0, len(b.defs))
	for n := range b.defs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make(OrderedMap, 0, len(names))
	for _, n := range names {
		out = append(out, Entry{n, b.defs[n]})
	}
	return out
}

// DefRef returns the JSON pointer ref ("#/$defs/Name") for name. It does not
// check that the def exists — useful for forward references that will be
// filled in later.
func (*Builder) DefRef(name string) string {
	return "#/$defs/" + name
}

// HasDef reports whether name is currently registered (including reserved
// but not yet filled in).
func (b *Builder) HasDef(name string) bool {
	_, ok := b.defs[name]
	return ok
}

// Reserve marks name as in-flight so recursive references emitted while
// building its body can resolve to a $ref instead of looping. Returns true
// if the name was newly reserved, false if it already existed (in which
// case the caller should not call SetDef).
func (b *Builder) Reserve(name string) bool {
	if _, ok := b.defs[name]; ok {
		return false
	}
	b.defs[name] = nil
	return true
}

// SetDef stores schema under name. Typically paired with Reserve.
func (b *Builder) SetDef(name string, schema OrderedMap) {
	b.defs[name] = schema
}

// AddNamedDef stores schema under name and returns DefRef(name). Use when
// the body has no recursive references back to itself. Returns an error if
// name is already registered (whether via Reserve, SetDef, AddNamedDef, or
// AddStruct) — collisions are always a programming error in this API.
func (b *Builder) AddNamedDef(name string, schema OrderedMap) (string, error) {
	if _, ok := b.defs[name]; ok {
		return "", fmt.Errorf("AddNamedDef: %q is already registered", name)
	}
	b.defs[name] = schema
	return b.DefRef(name), nil
}

// AddStruct ensures t (a struct or pointer-to-struct) has a definition in
// $defs and returns its $ref. Recurses through fields, consulting the
// resolver for each.
func (b *Builder) AddStruct(t reflect.Type) (string, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("AddStruct: expected struct, got %s", t.Kind())
	}
	name := t.Name()
	if name == "" {
		return "", errors.New("AddStruct: anonymous struct not supported")
	}
	if _, ok := b.defs[name]; ok {
		return b.DefRef(name), nil
	}
	// Reserve before recursing so cyclic structs (or sibling structs that
	// reference back) emit a $ref instead of looping.
	b.defs[name] = nil

	schema, err := b.reflectStructBody(t)
	if err != nil {
		return "", err
	}
	b.defs[name] = schema
	return b.DefRef(name), nil
}

func (b *Builder) reflectStructBody(t reflect.Type) (OrderedMap, error) {
	properties := OrderedMap{}
	var required []string

	if err := b.collectFields(t, &properties, &required); err != nil {
		return nil, err
	}

	sort.Strings(required)

	out := OrderedMap{
		{"type", "object"},
		{"properties", properties},
	}
	if len(required) > 0 {
		out = append(out, Entry{"required", required})
	}
	if !b.openAdditionalProps[t] {
		out = append(out, Entry{"additionalProperties", false})
	}
	return out, nil
}

func (b *Builder) collectFields(t reflect.Type, properties *OrderedMap, required *[]string) error {
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
		schema, err := b.ReflectType(f.Type)
		if err != nil {
			return fmt.Errorf("field %s.%s: %w", t.Name(), f.Name, err)
		}
		// encoding/json emits "null" for nil slices, maps, pointers, and
		// interfaces. When such a field is not tagged omitempty, the encoder
		// includes it (as null) rather than skipping it, so the schema must
		// admit null in addition to the field's nominal type.
		if !opts.omitEmpty && fieldCanBeNull(f.Type) {
			schema = MakeNullable(schema)
		}
		fields = append(fields, pendingField{
			name:     name,
			schema:   schema,
			required: !opts.omitEmpty,
		})
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].name < fields[j].name })
	for _, f := range fields {
		*properties = append(*properties, Entry{f.name, f.schema})
		if f.required {
			*required = append(*required, f.name)
		}
	}
	return nil
}

// ReflectType returns a JSON Schema fragment describing t. Pointers are
// unwrapped. The resolver, if any, is consulted before built-in handling.
//
// Built-in handling covers: bool, all integer kinds, all float kinds, string,
// slice/array (item type recurses), map (string-keyed only; values recurse),
// the bare-`any` interface (matches anything), and named structs (which
// recurse through AddStruct).
func (b *Builder) ReflectType(t reflect.Type) (any, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if b.resolver != nil {
		schema, handled, err := b.resolver(b, t)
		if err != nil {
			return nil, err
		}
		if handled {
			if schema == nil {
				return nil, fmt.Errorf("resolver returned nil schema for %s; resolvers that report handled=true must return a non-nil schema", t.String())
			}
			return schema, nil
		}
	}

	switch t.Kind() {
	case reflect.String:
		return OrderedMap{{"type", "string"}}, nil
	case reflect.Bool:
		return OrderedMap{{"type", "boolean"}}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return OrderedMap{{"type", "integer"}}, nil
	case reflect.Float32, reflect.Float64:
		return OrderedMap{{"type", "number"}}, nil
	case reflect.Slice, reflect.Array:
		items, err := b.ReflectType(t.Elem())
		if err != nil {
			return nil, err
		}
		return OrderedMap{
			{"type", "array"},
			{"items", items},
		}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("unsupported map key type %s; only string keys are supported", t.Key())
		}
		// `map[string]any` carries no value-side constraint; emit a plain
		// object schema rather than `additionalProperties: {}` so the JSON
		// stays compact and readable.
		if isEmptyInterface(t.Elem()) {
			return OrderedMap{{"type", "object"}}, nil
		}
		elem, err := b.ReflectType(t.Elem())
		if err != nil {
			return nil, err
		}
		return OrderedMap{
			{"type", "object"},
			{"additionalProperties", elem},
		}, nil
	case reflect.Interface:
		if t.NumMethod() == 0 {
			// Bare `any` accepts any JSON value; an empty schema {} matches
			// everything per JSON Schema semantics.
			return OrderedMap{}, nil
		}
		return nil, fmt.Errorf("unsupported interface type %s (no resolver match)", t.String())
	case reflect.Struct:
		ref, err := b.AddStruct(t)
		if err != nil {
			return nil, err
		}
		return OrderedMap{{"$ref", ref}}, nil
	}
	return nil, fmt.Errorf("unsupported type %s (kind %s)", t.String(), t.Kind())
}

// MakeNullable returns a schema equivalent to the input that also accepts
// the JSON null value.
func MakeNullable(schema any) any {
	m, ok := schema.(OrderedMap)
	if !ok {
		return schema
	}
	for i, e := range m {
		if e.Key == "type" {
			switch v := e.Value.(type) {
			case string:
				if v == "null" {
					return m
				}
				out := cloneOrderedMap(m)
				out[i] = Entry{"type", []string{v, "null"}}
				return out
			case []string:
				for _, s := range v {
					if s == "null" {
						return m
					}
				}
				out := cloneOrderedMap(m)
				widened := make([]string, len(v)+1)
				copy(widened, v)
				widened[len(v)] = "null"
				out[i] = Entry{"type", widened}
				return out
			}
		}
	}
	if oneOfHasNullBranch(m) {
		return m
	}
	return OrderedMap{
		{"oneOf", []any{m, OrderedMap{{"type", "null"}}}},
	}
}

func cloneOrderedMap(m OrderedMap) OrderedMap {
	out := make(OrderedMap, len(m))
	copy(out, m)
	return out
}

// oneOfHasNullBranch reports whether m is an OrderedMap whose top-level
// `oneOf` already includes a `{"type": "null"}` branch — i.e., MakeNullable
// has already been applied.
func oneOfHasNullBranch(m OrderedMap) bool {
	for _, e := range m {
		if e.Key != "oneOf" {
			continue
		}
		branches, ok := e.Value.([]any)
		if !ok {
			return false
		}
		for _, br := range branches {
			bm, ok := br.(OrderedMap)
			if !ok {
				continue
			}
			for _, be := range bm {
				if be.Key == "type" {
					if s, ok := be.Value.(string); ok && s == "null" {
						return true
					}
				}
			}
		}
	}
	return false
}

// jsonTagOpts holds the parsed flags from a `json:"..."` struct tag. Only
// flags the schema generator cares about are tracked.
type jsonTagOpts struct {
	omitEmpty bool
}

// parseJSONTag splits the contents of a struct's `json:"..."` tag into the
// JSON field name and option flags. If the tag is empty or names an empty
// field, fieldName is used as the JSON name.
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
			opts.omitEmpty = true
		}
	}
	return name, opts
}

func isEmptyInterface(t reflect.Type) bool {
	return t.Kind() == reflect.Interface && t.NumMethod() == 0
}

func fieldCanBeNull(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface:
		return true
	}
	return false
}

// OrderedMap preserves insertion order for JSON object encoding so the
// generated schema is byte-stable across runs.
type OrderedMap []Entry

// Entry is a single key/value pair in an OrderedMap.
type Entry struct {
	Key   string
	Value any
}

// Map builds an OrderedMap from alternating key/value arguments. Keys must
// be strings; the function panics on an odd number of arguments or on a
// non-string key. Use this from outside the package to avoid the govet
// "composites" warning that fires on cross-package struct literals.
//
// Map panics rather than returning an error so it can be used as a literal
// constructor in deeply nested expressions without breaking the call-site
// readability that is its whole point. The conditions it panics on are
// programming errors in literal arguments, not runtime data conditions a
// caller could usefully handle.
func Map(pairs ...any) OrderedMap {
	if len(pairs)%2 != 0 {
		panic(fmt.Sprintf("genjsonschema.Map: odd number of arguments (%d)", len(pairs)))
	}
	m := make(OrderedMap, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			panic(fmt.Sprintf("genjsonschema.Map: key at position %d is %T, want string", i, pairs[i]))
		}
		m = append(m, Entry{Key: k, Value: pairs[i+1]})
	}
	return m
}

func (m OrderedMap) MarshalJSON() ([]byte, error) {
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
