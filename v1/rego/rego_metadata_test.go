// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
)

func init() {
	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.transform_metadata",
		Decl: types.NewFunction(nil, types.B),
	})

	topdown.RegisterBuiltinFunc(
		"test.transform_metadata",
		func(bctx topdown.BuiltinContext, _ []*ast.Term, iter func(*ast.Term) error) error {
			if bctx.OutgoingMetadata != nil {
				if v, ok := bctx.IncomingMetadata["correlation_id"]; ok {
					bctx.OutgoingMetadata["correlation_id"] = v
					bctx.OutgoingMetadata["processed"] = true
				}
			}
			return iter(ast.BooleanTerm(true))
		},
	)
}

func TestEvalMetadataTransformViaBuiltin(t *testing.T) {
	module := `package test
		p if { test.transform_metadata() }
	`

	r := New(
		Query("data.test.p"),
		Module("test.rego", module),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	incoming := map[string]any{
		"correlation_id": "req-42",
	}
	outgoing := map[string]any{}

	rs, err := pq.Eval(t.Context(), EvalIncomingMetadata(incoming), EvalOutgoingMetadata(outgoing))
	if err != nil {
		t.Fatal(err)
	}

	if len(rs) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rs))
	}

	if outgoing["correlation_id"] != "req-42" {
		t.Fatalf("Expected outgoing correlation_id='req-42', got %v", outgoing["correlation_id"])
	}
	if outgoing["processed"] != true {
		t.Fatalf("Expected outgoing processed=true, got %v", outgoing["processed"])
	}
}
