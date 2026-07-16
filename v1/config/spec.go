// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"reflect"
	"slices"
	"strings"
	"sync"
)

// ConfigSpec lists the recognized option keys of one config subtree, used to
// warn about unrecognized options. Each Pattern segment matches a path segment;
// "*" matches any segment (e.g. a map entry), and an empty Pattern matches the
// top-level object. Subtrees no spec matches are "open" (any key allowed).
type ConfigSpec struct {
	Pattern []string `json:"pattern"`
	Keys    []string `json:"keys"`
}

var (
	specMu          sync.RWMutex
	registeredSpecs []ConfigSpec
)

// RegisterConfigSpec registers specs so ParseConfig recognizes a plugin's or
// subsystem's options. Typically called from an init function.
func RegisterConfigSpec(specs ...ConfigSpec) {
	specMu.Lock()
	defer specMu.Unlock()
	registeredSpecs = append(registeredSpecs, specs...)
}

// SpecsFromStruct derives the ConfigSpecs for the config subtree decoded into T,
// rooted at pattern. Keys are T's JSON field names, so they stay in sync with
// the decode types. Nested structs become child specs; a map/slice contributes
// a "*" segment; anonymous embedded structs are flattened (as encoding/json
// does). Assumes the types are not self-referential.
func SpecsFromStruct[T any](pattern ...string) []ConfigSpec {
	var specs []ConfigSpec
	collectStructSpec(&specs, pattern, reflect.TypeFor[T]())
	return specs
}

func collectStructSpec(specs *[]ConfigSpec, pattern []string, t reflect.Type) {
	t = derefType(t)
	if t.Kind() != reflect.Struct {
		return
	}
	*specs = append(*specs, ConfigSpec{Pattern: slices.Clone(pattern), Keys: structKeys(specs, pattern, t)})
}

// structKeys returns t's JSON keys, recursing into nested object fields and
// flattening anonymous embedded structs.
func structKeys(specs *[]ConfigSpec, pattern []string, t reflect.Type) []string {
	var keys []string
	for i := range t.NumField() {
		field := t.Field(i)

		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}

		// encoding/json promotes an unnamed embedded struct's fields to this
		// level (even when the embedded type is unexported); flatten it.
		if field.Anonymous && name == "" && derefType(field.Type).Kind() == reflect.Struct {
			keys = append(keys, structKeys(specs, pattern, derefType(field.Type))...)
			continue
		}

		if !field.IsExported() {
			continue
		}

		if name == "" {
			name = field.Name // untagged: encoding/json uses the field name
		}
		keys = append(keys, name)

		collectFieldSpec(specs, slices.Concat(pattern, []string{name}), field.Type)
	}
	return keys
}

// collectFieldSpec recurses into a field: a struct becomes a child spec; a
// map/slice/array of structs becomes a "*" wildcard level.
func collectFieldSpec(specs *[]ConfigSpec, pattern []string, ft reflect.Type) {
	ft = derefType(ft)
	switch ft.Kind() {
	case reflect.Struct:
		collectStructSpec(specs, pattern, ft)
	case reflect.Map, reflect.Slice, reflect.Array:
		if elem := derefType(ft.Elem()); elem.Kind() == reflect.Struct {
			collectStructSpec(specs, slices.Concat(pattern, []string{"*"}), elem)
		}
	}
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// registeredConfigSpecs returns the registered specs in the policy's input shape.
func registeredConfigSpecs() []any {
	specMu.RLock()
	defer specMu.RUnlock()

	out := make([]any, 0, len(registeredSpecs))
	for _, s := range registeredSpecs {
		out = append(out, map[string]any{
			"pattern": toAnySlice(s.Pattern),
			"keys":    toAnySlice(s.Keys),
		})
	}
	return out
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
