// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build !go1.27

package ast

import (
	"encoding/json"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

// termJSON is used to serialize Term to JSON without map allocation.
type termJSON struct {
	Location *Location `json:"location,omitempty"`
	Type     string    `json:"type"`
	Value    Value     `json:"value"`
}

// MarshalJSON returns the JSON encoding of the term.
//
// Specialized marshalling logic is required to include a type hint for Value.
func (term *Term) MarshalJSON() ([]byte, error) {
	d := termJSON{
		Type:  ValueName(term.Value),
		Value: term.Value,
	}
	jsonOptions := astJSON.GetOptions().MarshalOptions
	if jsonOptions.IncludeLocation.Term {
		d.Location = term.Location
	}
	return json.Marshal(d)
}

// MarshalJSON returns JSON encoded bytes representing arr.
func (arr *Array) MarshalJSON() ([]byte, error) {
	if len(arr.elems) == 0 {
		return []byte(`[]`), nil
	}
	return json.Marshal(arr.elems)
}

// MarshalJSON returns JSON encoded bytes representing num.
func (num Number) MarshalJSON() ([]byte, error) {
	return json.Marshal(json.Number(num))
}

// MarshalJSON returns JSON encoded bytes representing obj.
func (obj *object) MarshalJSON() ([]byte, error) {
	sl := make([][2]*Term, obj.Len())
	for i, node := range obj.sortedKeys() {
		sl[i] = Item(node.key, node.value)
	}
	return json.Marshal(sl)
}

// MarshalJSON returns JSON encoded bytes representing s.
func (s *set) MarshalJSON() ([]byte, error) {
	if s.keys == nil {
		return []byte(`[]`), nil
	}
	return json.Marshal(s.sortedKeys())
}

func (lob *lazyObj) MarshalJSON() ([]byte, error) {
	return lob.force().(*object).MarshalJSON()
}

func (n *Not) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"type":          "not",
		"body":          n.Body,
		"explicit_body": n.ExplicitBody,
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Not {
		if n.Location != nil {
			data["location"] = n.Location
		}
	}

	return json.Marshal(data)
}

func (n *Not) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}

	return unmarshalNot(n, v)
}
