// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package json

import (
	"encoding/json"
	"testing"
)

// TestNodeToggleMarshalUnmarshal tests that marshaling and unmarshaling
// preserves all field values correctly
func TestNodeToggleMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		nt   NodeToggle
	}{
		{
			name: "all false",
			nt:   NodeToggle{},
		},
		{
			name: "all true",
			nt:   NewNodeToggle().WithAll(),
		},
		{
			name: "single field - Term",
			nt:   NewNodeToggle().WithTerm(),
		},
		{
			name: "single field - Package",
			nt:   NewNodeToggle().WithPackage(),
		},
		{
			name: "single field - Comment",
			nt:   NewNodeToggle().WithComment(),
		},
		{
			name: "single field - Import",
			nt:   NewNodeToggle().WithImport(),
		},
		{
			name: "single field - Rule",
			nt:   NewNodeToggle().WithRule(),
		},
		{
			name: "single field - Head",
			nt:   NewNodeToggle().WithHead(),
		},
		{
			name: "single field - Expr",
			nt:   NewNodeToggle().WithExpr(),
		},
		{
			name: "single field - SomeDecl",
			nt:   NewNodeToggle().WithSomeDecl(),
		},
		{
			name: "single field - Every",
			nt:   NewNodeToggle().WithEvery(),
		},
		{
			name: "single field - With",
			nt:   NewNodeToggle().WithWith(),
		},
		{
			name: "single field - Annotations",
			nt:   NewNodeToggle().WithAnnotations(),
		},
		{
			name: "single field - AnnotationsRef",
			nt:   NewNodeToggle().WithAnnotationsRef(),
		},
		{
			name: "multiple fields",
			nt:   NewNodeToggle().WithTerm().WithPackage().WithRule().WithExpr(),
		},
		{
			name: "complex combination",
			nt: NewNodeToggle().
				WithTerm().
				WithComment().
				WithRule().
				WithHead().
				WithSomeDecl().
				WithWith().
				WithAnnotationsRef(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.nt)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			// Unmarshal
			var result NodeToggle
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			// Verify all fields match
			if result.Term() != tt.nt.Term() {
				t.Errorf("Term mismatch: got %v, want %v", result.Term(), tt.nt.Term())
			}
			if result.Package() != tt.nt.Package() {
				t.Errorf("Package mismatch: got %v, want %v", result.Package(), tt.nt.Package())
			}
			if result.Comment() != tt.nt.Comment() {
				t.Errorf("Comment mismatch: got %v, want %v", result.Comment(), tt.nt.Comment())
			}
			if result.Import() != tt.nt.Import() {
				t.Errorf("Import mismatch: got %v, want %v", result.Import(), tt.nt.Import())
			}
			if result.Rule() != tt.nt.Rule() {
				t.Errorf("Rule mismatch: got %v, want %v", result.Rule(), tt.nt.Rule())
			}
			if result.Head() != tt.nt.Head() {
				t.Errorf("Head mismatch: got %v, want %v", result.Head(), tt.nt.Head())
			}
			if result.Expr() != tt.nt.Expr() {
				t.Errorf("Expr mismatch: got %v, want %v", result.Expr(), tt.nt.Expr())
			}
			if result.SomeDecl() != tt.nt.SomeDecl() {
				t.Errorf("SomeDecl mismatch: got %v, want %v", result.SomeDecl(), tt.nt.SomeDecl())
			}
			if result.Every() != tt.nt.Every() {
				t.Errorf("Every mismatch: got %v, want %v", result.Every(), tt.nt.Every())
			}
			if result.With() != tt.nt.With() {
				t.Errorf("With mismatch: got %v, want %v", result.With(), tt.nt.With())
			}
			if result.Annotations() != tt.nt.Annotations() {
				t.Errorf("Annotations mismatch: got %v, want %v", result.Annotations(), tt.nt.Annotations())
			}
			if result.AnnotationsRef() != tt.nt.AnnotationsRef() {
				t.Errorf("AnnotationsRef mismatch: got %v, want %v", result.AnnotationsRef(), tt.nt.AnnotationsRef())
			}

			// Verify flags are identical
			if result.flags != tt.nt.flags {
				t.Errorf("flags mismatch: got %016b, want %016b", result.flags, tt.nt.flags)
			}
		})
	}
}

// TestNodeToggleJSONFormat tests that the JSON format is correct
func TestNodeToggleJSONFormat(t *testing.T) {
	nt := NewNodeToggle().WithTerm().WithRule()

	data, err := json.Marshal(nt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Unmarshal to a map to verify the structure
	var m map[string]bool
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map error: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{
		"Term", "Package", "Comment", "Import", "Rule", "Head",
		"Expr", "SomeDecl", "Every", "With", "Annotations", "AnnotationsRef",
	}

	for _, field := range expectedFields {
		if _, exists := m[field]; !exists {
			t.Errorf("Field %s not found in JSON", field)
		}
	}

	// Verify correct values
	if !m["Term"] {
		t.Error("Term should be true")
	}
	if !m["Rule"] {
		t.Error("Rule should be true")
	}
	if m["Package"] {
		t.Error("Package should be false")
	}
}

// TestNodeToggleUnmarshalPartial tests unmarshaling with missing fields
func TestNodeToggleUnmarshalPartial(t *testing.T) {
	// JSON with only some fields
	jsonData := `{"Term":true,"Rule":true}`

	var nt NodeToggle
	if err := json.Unmarshal([]byte(jsonData), &nt); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !nt.Term() {
		t.Error("Term should be true")
	}
	if !nt.Rule() {
		t.Error("Rule should be true")
	}
	if nt.Package() {
		t.Error("Package should be false")
	}
	if nt.Comment() {
		t.Error("Comment should be false")
	}
}

// TestNodeToggleUnmarshalInvalidJSON tests error handling
func TestNodeToggleUnmarshalInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "invalid JSON",
			data: `{Term:true}`,
		},
		{
			name: "wrong type",
			data: `{"Term":"yes"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nt NodeToggle
			if err := json.Unmarshal([]byte(tt.data), &nt); err == nil {
				t.Error("Expected error for invalid JSON, got nil")
			}
		})
	}
}

// TestNodeToggleMarshalConsistency tests that multiple marshals produce the same output
func TestNodeToggleMarshalConsistency(t *testing.T) {
	nt := NewNodeToggle().WithTerm().WithPackage().WithRule()

	data1, err := json.Marshal(nt)
	if err != nil {
		t.Fatalf("First marshal error: %v", err)
	}

	data2, err := json.Marshal(nt)
	if err != nil {
		t.Fatalf("Second marshal error: %v", err)
	}

	if string(data1) != string(data2) {
		t.Errorf("Marshal output inconsistent:\n%s\nvs\n%s", data1, data2)
	}
}

// TestNodeToggleRoundTrip tests multiple round trips
func TestNodeToggleRoundTrip(t *testing.T) {
	original := NewNodeToggle().WithAll()

	var current NodeToggle = original
	for i := 0; i < 100; i++ {
		// Marshal
		data, err := json.Marshal(current)
		if err != nil {
			t.Fatalf("Marshal iteration %d error: %v", i, err)
		}

		// Unmarshal
		var next NodeToggle
		if err := json.Unmarshal(data, &next); err != nil {
			t.Fatalf("Unmarshal iteration %d error: %v", i, err)
		}

		// Verify
		if next.flags != original.flags {
			t.Fatalf("Flags changed at iteration %d: got %016b, want %016b", i, next.flags, original.flags)
		}

		current = next
	}
}
