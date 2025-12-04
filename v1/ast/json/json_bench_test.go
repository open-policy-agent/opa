// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
)

// BenchmarkNodeToggleMarshalOptimized benchmarks the optimized direct writing implementation
func BenchmarkNodeToggleMarshalOptimized(b *testing.B) {
	nt := NewNodeToggle().WithTerm().WithPackage().WithRule().WithExpr()

	
	b.ReportAllocs()

	for b.Loop() {
		_, err := json.Marshal(nt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleMarshalAllFields benchmarks marshaling with all fields enabled
func BenchmarkNodeToggleMarshalAllFields(b *testing.B) {
	nt := NewNodeToggle().WithAll()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := json.Marshal(nt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleMarshalEmpty benchmarks marshaling with no fields enabled
func BenchmarkNodeToggleMarshalEmpty(b *testing.B) {
	nt := NodeToggle{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := json.Marshal(nt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleUnmarshalOptimized benchmarks the optimized unmarshaling implementation
func BenchmarkNodeToggleUnmarshalOptimized(b *testing.B) {
	nt := NewNodeToggle().WithTerm().WithPackage().WithRule().WithExpr()
	data, err := json.Marshal(nt)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var result NodeToggle
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleUnmarshalAllFields benchmarks unmarshaling with all fields
func BenchmarkNodeToggleUnmarshalAllFields(b *testing.B) {
	nt := NewNodeToggle().WithAll()
	data, err := json.Marshal(nt)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var result NodeToggle
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleRoundTrip benchmarks the full marshal + unmarshal cycle
func BenchmarkNodeToggleRoundTrip(b *testing.B) {
	nt := NewNodeToggle().WithTerm().WithPackage().WithRule().WithExpr()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		data, err := json.Marshal(nt)
		if err != nil {
			b.Fatal(err)
		}

		var result NodeToggle
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleParallel benchmarks concurrent marshaling
func BenchmarkNodeToggleParallel(b *testing.B) {
	nt := NewNodeToggle().WithTerm().WithPackage().WithRule().WithExpr()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := json.Marshal(nt)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark against the old implementation for comparison
// This simulates the old approach using an intermediate struct with pooling

type nodeToggleOld struct {
	Term           bool `json:"Term"`
	Package        bool `json:"Package"`
	Comment        bool `json:"Comment"`
	Import         bool `json:"Import"`
	Rule           bool `json:"Rule"`
	Head           bool `json:"Head"`
	Expr           bool `json:"Expr"`
	SomeDecl       bool `json:"SomeDecl"`
	Every          bool `json:"Every"`
	With           bool `json:"With"`
	Annotations    bool `json:"Annotations"`
	AnnotationsRef bool `json:"AnnotationsRef"`
}

var nodeToggleOldPool = sync.Pool{
	New: func() any {
		return &nodeToggleOld{}
	},
}

// NodeToggleWithOldMethod simulates the original pooled struct approach
type NodeToggleWithOldMethod struct {
	flags uint16
}

func (n NodeToggleWithOldMethod) MarshalJSON() ([]byte, error) {
	temp := nodeToggleOldPool.Get().(*nodeToggleOld)
	defer func() {
		// Reset all fields
		temp.Term = false
		temp.Package = false
		temp.Comment = false
		temp.Import = false
		temp.Rule = false
		temp.Head = false
		temp.Expr = false
		temp.SomeDecl = false
		temp.Every = false
		temp.With = false
		temp.Annotations = false
		temp.AnnotationsRef = false
		nodeToggleOldPool.Put(temp)
	}()

	temp.Term = n.flags&(1<<0) != 0
	temp.Package = n.flags&(1<<1) != 0
	temp.Comment = n.flags&(1<<2) != 0
	temp.Import = n.flags&(1<<3) != 0
	temp.Rule = n.flags&(1<<4) != 0
	temp.Head = n.flags&(1<<5) != 0
	temp.Expr = n.flags&(1<<6) != 0
	temp.SomeDecl = n.flags&(1<<7) != 0
	temp.Every = n.flags&(1<<8) != 0
	temp.With = n.flags&(1<<9) != 0
	temp.Annotations = n.flags&(1<<10) != 0
	temp.AnnotationsRef = n.flags&(1<<11) != 0

	return json.Marshal(temp)
}

// BenchmarkNodeToggleMarshalWithOldMethod benchmarks the original implementation with pooling
func BenchmarkNodeToggleMarshalWithOldMethod(b *testing.B) {
	old := NodeToggleWithOldMethod{
		flags: (1 << 0) | (1 << 1) | (1 << 4) | (1 << 6), // Term, Package, Rule, Expr
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := json.Marshal(old)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleMarshalOldApproach benchmarks without pooling (just the struct)
func BenchmarkNodeToggleMarshalOldApproach(b *testing.B) {
	old := nodeToggleOld{
		Term:    true,
		Package: true,
		Rule:    true,
		Expr:    true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := json.Marshal(old)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNodeToggleUnmarshalOldApproach benchmarks the old unmarshal approach
func BenchmarkNodeToggleUnmarshalOldApproach(b *testing.B) {
	old := nodeToggleOld{
		Term:    true,
		Package: true,
		Rule:    true,
		Expr:    true,
	}
	data, err := json.Marshal(old)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var result nodeToggleOld
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStringBuilderPool benchmarks the bytes buffer pool performance
func BenchmarkBytesBufferPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		buf := bytesBufferPool.Get().(*bytes.Buffer)
		buf.Grow(256)
		buf.WriteString(`{"Term":true,"Package":false}`)
		_ = buf.Bytes()
		buf.Reset()
		bytesBufferPool.Put(buf)
	}
}
