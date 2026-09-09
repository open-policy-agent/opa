// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package yaml provides YAML <-> JSON conversion for OPA, on top of
// go.yaml.in/yaml/v3.
//
// It replaces sigs.k8s.io/yaml, which is pinned to go.yaml.in/yaml/v2 and
// therefore resolves YAML 1.1 boolean spellings (on/off/yes/no) in positions
// where the YAML 1.2 core schema calls for strings.
package yaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// Marshal serializes obj as YAML. obj is first round-tripped through
// encoding/json so that `json` struct tags and json.Marshaler
// implementations are honoured, matching the behaviour callers relied on
// from sigs.k8s.io/yaml.
func Marshal(obj any) ([]byte, error) {
	bs, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling into JSON: %w", err)
	}
	var jsonObj any
	if err := yaml.Unmarshal(bs, &jsonObj); err != nil {
		return nil, err
	}
	return marshalYAML(jsonObj)
}

// marshalYAML emits YAML at 2-space indentation. go-yaml v3 defaults to 4,
// where sigs.k8s.io/yaml (on go-yaml v2) emitted 2.
func marshalYAML(obj any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(obj); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// JSONOpt configures the encoding/json decoder used by Unmarshal.
type JSONOpt func(*json.Decoder) *json.Decoder

// Unmarshal decodes a YAML document into obj, using encoding/json semantics
// (`json` struct tags, json.Unmarshaler) rather than go-yaml's.
func Unmarshal(bs []byte, obj any, opts ...JSONOpt) error {
	js, err := YAMLToJSON(bs)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(js))
	for _, opt := range opts {
		d = opt(d)
	}
	if err := d.Decode(obj); err != nil {
		return fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	return nil
}

// YAMLToJSON converts a single YAML document to JSON.
func YAMLToJSON(bs []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(bs, &node); err != nil {
		return nil, err
	}

	var obj any
	if node.Kind != 0 { // an empty document decodes to the zero Node
		normalize(&node, map[*yaml.Node]struct{}{})
		if err := node.Decode(&obj); err != nil {
			return nil, err
		}
	}

	obj, err := jsonable(obj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}

// normalize rewrites the node tree before it is decoded, so that documents
// go-yaml v2 accepted keep working under v3.
//
// Implicitly resolved !!timestamp scalars are re-tagged !!str. !!timestamp is
// a YAML 1.1 type that go-yaml v3 still resolves in the core schema; leaving
// it in place would silently rewrite `2023-01-01` to `2023-01-01T00:00:00Z`
// on the way to JSON.
//
// Repeated merge keys are folded into the sequence form, and duplicate
// mapping keys are collapsed to the last occurrence. go-yaml v3 rejects both
// outright; go-yaml v2 accepted them, and turning documents that load today
// into hard errors is a bigger change than this package is trying to make.
//
// Anchors make the node graph a DAG, so visited guards against re-walking a
// shared subtree.
func normalize(n *yaml.Node, visited map[*yaml.Node]struct{}) {
	if n == nil {
		return
	}
	if _, ok := visited[n]; ok {
		return
	}
	visited[n] = struct{}{}

	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!timestamp" && n.Style == 0 {
			n.Tag = "!!str"
		}
	case yaml.MappingNode:
		n.Content = collapseMergeKeys(n.Content)
		n.Content = dedupeKeys(n.Content)
	}

	normalize(n.Alias, visited)
	for _, c := range n.Content {
		normalize(c, visited)
	}
}

// collapseMergeKeys rewrites a mapping that repeats `<<` into the spec's
// sequence form (`<<: [a, b]`), which go-yaml v3 accepts. go-yaml v2 applied
// repeated merge keys in document order, so preserve that order.
func collapseMergeKeys(content []*yaml.Node) []*yaml.Node {
	first := -1
	var merged []*yaml.Node

	for i := 0; i+1 < len(content); i += 2 {
		if content[i].Kind != yaml.ScalarNode || content[i].Tag != "!!merge" {
			continue
		}
		if first < 0 {
			first = i
		}
		if v := content[i+1]; v.Kind == yaml.SequenceNode {
			merged = append(merged, v.Content...)
		} else {
			merged = append(merged, v)
		}
	}

	if first < 0 || len(merged) < 2 {
		return content
	}

	out := make([]*yaml.Node, 0, len(content))
	for i := 0; i+1 < len(content); i += 2 {
		switch {
		case i == first:
			out = append(out, content[i], &yaml.Node{
				Kind:    yaml.SequenceNode,
				Tag:     "!!seq",
				Content: merged,
			})
		case content[i].Kind == yaml.ScalarNode && content[i].Tag == "!!merge":
			// dropped; folded into the sequence above
		default:
			out = append(out, content[i], content[i+1])
		}
	}
	return out
}

// dedupeKeys drops all but the last occurrence of each key in a mapping's
// flattened key/value Content slice, preserving the position of the first
// occurrence the way a last-wins map assignment would.
//
// go-yaml v3 rejects duplicate keys outright, but go-yaml v2 accepted them,
// and turning documents that load today into hard errors is a bigger change
// than this package is trying to make. Keys are compared by the string they
// will occupy in the resulting JSON object, so `1` and `"1"` collide here the
// same way they would there.
func dedupeKeys(content []*yaml.Node) []*yaml.Node {
	seen := make(map[string]int, len(content)/2)
	dropped := false

	for i := 0; i+1 < len(content); i += 2 {
		k := content[i]
		// Merge keys are not real keys, and non-scalar keys have no JSON
		// representation - both are handled elsewhere.
		if k.Kind != yaml.ScalarNode || k.Tag == "!!merge" {
			continue
		}
		var kv any
		if err := k.Decode(&kv); err != nil {
			continue
		}
		id, ok := keyString(kv)
		if !ok {
			continue
		}
		if prevVal, ok := seen[id]; ok {
			// Keep the earlier key node's position, take the later value.
			content[prevVal] = content[i+1]
			content[i], content[i+1] = nil, nil
			dropped = true
			continue
		}
		seen[id] = i + 1
	}

	if !dropped {
		return content
	}

	out := content[:0]
	for _, n := range content {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// JSONToYAML converts JSON to YAML, preserving nothing but the value.
func JSONToYAML(bs []byte) ([]byte, error) {
	var obj any
	// json.Number would be re-encoded as a quoted string by go-yaml, so decode
	// numbers as float64 the way encoding/json does by default.
	if err := json.Unmarshal(bs, &obj); err != nil {
		return nil, err
	}
	return marshalYAML(obj)
}

// jsonable rewrites the result of a go-yaml decode into something
// encoding/json can marshal: YAML permits mapping keys of any type, JSON
// only permits strings.
func jsonable(x any) (any, error) {
	switch x := x.(type) {
	case map[string]any:
		for k, v := range x {
			v, err := jsonable(v)
			if err != nil {
				return nil, err
			}
			x[k] = v
		}
		return x, nil
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			ks, ok := keyString(k)
			if !ok {
				return nil, fmt.Errorf("unsupported map key of type: %s, key: %+#v, value: %+#v", reflect.TypeOf(k), k, v)
			}
			v, err := jsonable(v)
			if err != nil {
				return nil, err
			}
			out[ks] = v
		}
		return out, nil
	case []any:
		for i, v := range x {
			v, err := jsonable(v)
			if err != nil {
				return nil, err
			}
			x[i] = v
		}
		return x, nil
	default:
		return x, nil
	}
}

func keyString(k any) (string, bool) {
	switch k := k.(type) {
	case string:
		return k, true
	case int:
		return strconv.Itoa(k), true
	case int64:
		return strconv.FormatInt(k, 10), true
	case uint64:
		return strconv.FormatUint(k, 10), true
	case float64:
		// Match how go-yaml renders floats when marshaling.
		switch s := strconv.FormatFloat(k, 'g', -1, 32); s {
		case "+Inf":
			return ".inf", true
		case "-Inf":
			return "-.inf", true
		case "NaN":
			return ".nan", true
		default:
			return s, true
		}
	case bool:
		return strconv.FormatBool(k), true
	default:
		return "", false
	}
}
