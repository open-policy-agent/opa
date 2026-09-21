// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import "testing"

func TestTranslateField(t *testing.T) {
	tests := []struct {
		note         string
		field        string
		translations map[string]any
		exp          string
	}{
		{
			note:  "no translations",
			field: "fruit.name",
			exp:   "fruit.name",
		},
		{
			note:  "table and column",
			field: "fruit.name",
			translations: map[string]any{"fruit": map[string]any{
				"$self": "F",
				"name":  "N",
			}},
			exp: "F.N",
		},
		{
			note:  "short unknown via $table",
			field: "name",
			translations: map[string]any{
				"name":  map[string]any{"$table": "fruit"},
				"fruit": map[string]any{"$self": "F", "name": "N"},
			},
			exp: "F.N",
		},
		{
			note:         "non-string $self is ignored",
			field:        "fruit.name",
			translations: map[string]any{"fruit": map[string]any{"$self": 123, "name": "N"}},
			exp:          "fruit.N",
		},
		{
			note:         "non-string $table is ignored",
			field:        "name",
			translations: map[string]any{"name": map[string]any{"$table": []any{"fruit"}}},
			exp:          "name",
		},
		{
			note:         "non-string column is ignored",
			field:        "fruit.name",
			translations: map[string]any{"fruit": map[string]any{"$self": "F", "name": nil}},
			exp:          "F.name",
		},
		{
			note:         "non-map table mapping is ignored",
			field:        "fruit.name",
			translations: map[string]any{"fruit": "F"},
			exp:          "fruit.name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if act := translateField(tc.field, tc.translations); act != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, act)
			}
		})
	}
}
