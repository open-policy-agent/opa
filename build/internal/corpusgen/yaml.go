// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package corpusgen holds what the conformance corpus generators have in common:
// editing a case's YAML in place, through the node tree rather than by
// re-marshalling the case struct, so that comments, key order, and the hand
// authoring around a generated field all survive; and the parts of a case's
// schema both generators have to interpret the same way.
package corpusgen

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/v1/test/conformance"
)

// Literal returns a block scalar node, the form a multi-line fixture is written
// as.
func Literal(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.LiteralStyle, Value: s}
}

// Scalar returns a plain scalar node with the given tag.
func Scalar(tag, value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
}

// MapValue returns the value node of key on the mapping n, or nil.
func MapValue(n *yaml.Node, key string) *yaml.Node {
	if i := keyIndex(n, key); i >= 0 {
		return n.Content[i+1]
	}
	return nil
}

// SetMapValue sets key on the mapping n, inserting it ahead of the first of
// before that is present when it is not already there.
func SetMapValue(n *yaml.Node, key string, value *yaml.Node, before ...string) {
	if i := keyIndex(n, key); i >= 0 {
		n.Content[i+1] = value
		return
	}

	at := len(n.Content)
	for _, b := range before {
		if i := keyIndex(n, b); i >= 0 && i < at {
			at = i
		}
	}

	n.Content = slices.Insert(n.Content, at, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

// DeleteMapValue removes key from the mapping n, reporting whether it was there.
func DeleteMapValue(n *yaml.Node, key string) bool {
	i := keyIndex(n, key)
	if i < 0 {
		return false
	}
	n.Content = slices.Delete(n.Content, i, i+2)
	return true
}

func keyIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// ErrorsNode renders diagnostics as a want_errors sequence. Module is emitted
// only where the error is not against the default module, so a single-module
// case carries no attribution it does not need.
func ErrorsNode(errs []conformance.Error) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}

	for _, e := range errs {
		m := &yaml.Node{Kind: yaml.MappingNode}
		if e.Module != "" && e.Module != conformance.DefaultModuleName {
			SetMapValue(m, "module", Scalar("!!str", e.Module))
		}
		SetMapValue(m, "code", Scalar("!!str", e.Code))
		SetMapValue(m, "row", Scalar("!!int", strconv.Itoa(e.Row)))
		if e.Col != 0 {
			SetMapValue(m, "col", Scalar("!!int", strconv.Itoa(e.Col)))
		}
		SetMapValue(m, "message", Scalar("!!str", e.Message))
		seq.Content = append(seq.Content, m)
	}

	return seq
}

// Encode renders a corpus document, with the leading marker the corpus files
// carry.
func Encode(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ModulesNode renders Rego sources as a sequence of block scalars, the form a
// module is written as in a corpus file.
func ModulesNode(modules []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, m := range modules {
		seq.Content = append(seq.Content, Literal(strings.TrimRight(m, "\n")+"\n"))
	}
	return seq
}
