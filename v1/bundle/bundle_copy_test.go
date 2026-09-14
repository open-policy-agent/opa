// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBundleCopyData(t *testing.T) {
	b := Bundle{
		Data: map[string]any{
			"str":  "hello",
			"bool": true,
			"num":  json.Number("123"),
			"nil":  nil,
			"arr":  []any{"a", json.Number("1"), map[string]any{"nested": "b"}},
			"obj": map[string]any{
				"deep": map[string]any{
					"deeper": []any{json.Number("1"), json.Number("2")},
				},
			},
			"raw_int": 7,
		},
	}
	b.Manifest.Init()

	cpy := b.Copy()

	want := map[string]any{
		"str":  "hello",
		"bool": true,
		"num":  json.Number("123"),
		"nil":  nil,
		"arr":  []any{"a", json.Number("1"), map[string]any{"nested": "b"}},
		"obj": map[string]any{
			"deep": map[string]any{
				"deeper": []any{json.Number("1"), json.Number("2")},
			},
		},
		"raw_int": json.Number("7"),
	}
	if !reflect.DeepEqual(cpy.Data, want) {
		t.Fatalf("unexpected copy:\ngot:  %#v\nwant: %#v", cpy.Data, want)
	}

	cpy.Data["obj"].(map[string]any)["deep"].(map[string]any)["deeper"] = "mutated"
	if reflect.DeepEqual(b.Data["obj"], cpy.Data["obj"]) {
		t.Fatal("expected original bundle data to be unaffected by mutating the copy")
	}

	b.Data["arr"].([]any)[0] = "mutated"
	if reflect.DeepEqual(b.Data["arr"], cpy.Data["arr"]) {
		t.Fatal("expected copy to be unaffected by mutating the original")
	}
}

func TestBundleCopyNilData(t *testing.T) {
	b := Bundle{}
	b.Manifest.Init()

	cpy := b.Copy()
	if cpy.Data != nil {
		t.Fatalf("expected nil data to stay nil, got: %#v", cpy.Data)
	}
}
