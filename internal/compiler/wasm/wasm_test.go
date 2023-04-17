// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/internal/wasm/instruction"
	"github.com/open-policy-agent/opa/internal/wasm/module"
)

func TestCompilerHelloWorld(t *testing.T) {

	policy, err := planner.New().
		WithQueries([]planner.QuerySet{
			{
				Name:    "test",
				Queries: []ast.Body{ast.MustParseBody(`input.foo = 1`)},
			},
		}).Plan()

	if err != nil {
		t.Fatal(err)
	}

	c := New().WithPolicy(policy)
	_, err = c.Compile()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompilerBadDataSegment(t *testing.T) {

	result, err := getLowestFreeDataSegmentOffset(&module.Module{})
	if err != nil || result != 0 {
		t.Fatal("expected zero but got:", result, "err:", err)
	}

	_, err = getLowestFreeDataSegmentOffset(&module.Module{Data: module.DataSection{
		Segments: []module.DataSegment{
			{
				Offset: module.Expr{
					Instrs: []instruction.Instruction{},
				},
			},
		},
	}})
	if err == nil || err.Error() != "bad data segment offset instructions" {
		t.Fatal("unexpected err:", err)
	}

	_, err = getLowestFreeDataSegmentOffset(&module.Module{Data: module.DataSection{
		Segments: []module.DataSegment{
			{
				Offset: module.Expr{
					Instrs: []instruction.Instruction{
						instruction.I64Const{Value: 100},
					},
				},
			},
		},
	}})
	if err == nil || err.Error() != "bad data segment offset expr" {
		t.Fatal("unexpected err:", err)
	}

	result, err = getLowestFreeDataSegmentOffset(&module.Module{Data: module.DataSection{
		Segments: []module.DataSegment{
			{
				Init: []byte("foo"),
				Offset: module.Expr{
					Instrs: []instruction.Instruction{
						instruction.I32Const{Value: 106},
					},
				},
			},
			{
				Init: []byte("bar"),
				Offset: module.Expr{
					Instrs: []instruction.Instruction{
						instruction.I32Const{Value: 100},
					},
				},
			},
		},
	}})
	if err != nil || result != 109 {
		t.Fatal("expected 106 but got:", result, "err:", err)
	}
}
