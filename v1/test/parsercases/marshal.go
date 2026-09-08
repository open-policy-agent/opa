// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package parsercases

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

// MarshalOptions returns the JSON marshalling options a want_ast fixture is
// generated and compared with. Unlike cmd/parse.go, which toggles 12 of the 15
// node types, locations here are all-or-nothing: a fixture that pins positions
// pins them everywhere.
func MarshalOptions(locations, locationText bool) astJSON.Options {
	toggle := astJSON.NodeToggle{
		Term:           locations,
		Package:        locations,
		Comment:        locations,
		Import:         locations,
		Rule:           locations,
		Head:           locations,
		Expr:           locations,
		SomeDecl:       locations,
		Every:          locations,
		With:           locations,
		Annotations:    locations,
		AnnotationsRef: locations,
		Not:            locations,
		And:            locations,
		Or:             locations,
	}
	return astJSON.Options{
		MarshalOptions: astJSON.MarshalOptions{
			IncludeLocation:     toggle,
			IncludeLocationText: locations && locationText,
			ExcludeLocationFile: true,
		},
	}
}

// A fixture never records comments. The module is in the case already, so
// recording them again asserts nothing a reader cannot see, and it would require
// an implementation to retain them in a particular shape — OPA's own being one
// nobody should be held to, since Comment marshals its text as base64 and its
// position unconditionally. Both the generator and the runner clear them before
// marshalling; Validate rejects a fixture that carries them anyway.

// FormatAST renders marshalled AST JSON the way a want_ast fixture holds it:
// with the members of every object in lexical order, indented, so that a
// mismatch reads as a line diff, and plain ASCII, because a YAML emitter falls
// back to double-quoted style for a scalar holding an astral character, which
// would collapse the fixture onto a single line.
//
// The fixture is text rather than nested YAML because a YAML scalar cannot hold
// a Rego number literal faithfully: 1e6 is a string to a YAML parser and a
// float to a JSON one.
func FormatAST(bs []byte) (string, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, bs, ""); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.Grow(buf.Len())

	for _, r := range buf.String() {
		switch {
		case r < utf8.RuneSelf:
			sb.WriteByte(byte(r))
		case r > 0xFFFF:
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&sb, "\\u%04x\\u%04x", hi, lo)
		default:
			fmt.Fprintf(&sb, "\\u%04x", r)
		}
	}

	sb.WriteByte('\n')

	return sb.String(), nil
}

const astIndent = "  "

// writeCanonical indents raw, ordering the members of every object lexically.
//
// The order OPA emits them in is not something a conforming implementation
// should have to reproduce, and it is not even stable across OPA builds: the
// go1.27 marshallers in v1/ast write a rule as head-then-body and the pre-1.27
// ones write it as body-then-head, so a fixture recording either would depend on
// the toolchain the corpus was generated with.
//
// Only objects and arrays are rewritten. Every scalar is copied through as the
// bytes the marshaller produced, so a Rego number literal keeps its exact text —
// decoding one into a float64 would not round-trip it.
func writeCanonical(buf *bytes.Buffer, raw json.RawMessage, prefix string) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return errors.New("empty JSON value")
	}

	switch trimmed[0] {
	case '{':
		var members map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &members); err != nil {
			return err
		}
		if len(members) == 0 {
			buf.WriteString("{}")
			return nil
		}

		inner := prefix + astIndent
		buf.WriteString("{\n")
		for i, k := range slices.Sorted(maps.Keys(members)) {
			if i > 0 {
				buf.WriteString(",\n")
			}
			key, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.WriteString(inner)
			buf.Write(key)
			buf.WriteString(": ")
			if err := writeCanonical(buf, members[k], inner); err != nil {
				return err
			}
		}
		buf.WriteString("\n" + prefix + "}")

	case '[':
		var elements []json.RawMessage
		if err := json.Unmarshal(trimmed, &elements); err != nil {
			return err
		}
		if len(elements) == 0 {
			buf.WriteString("[]")
			return nil
		}

		inner := prefix + astIndent
		buf.WriteString("[\n")
		for i, e := range elements {
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString(inner)
			if err := writeCanonical(buf, e, inner); err != nil {
				return err
			}
		}
		buf.WriteString("\n" + prefix + "]")

	default:
		buf.Write(trimmed)
	}

	return nil
}
