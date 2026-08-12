// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir_test

import (
	"math"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/open-policy-agent/opa/v1/ir"
	pb "github.com/open-policy-agent/opa/v1/ir/v1pb"
)

func TestPolicyToProtoRoundTrip(t *testing.T) {
	loc := ir.Location{File: 1, Row: 2, Col: 3}
	withLoc := func(s ir.Stmt) ir.Stmt {
		s.SetLocation(loc.File, loc.Row, loc.Col, "", nil)
		return s
	}

	p := &ir.Policy{
		Static: &ir.Static{
			Strings:      []*ir.StringConst{{Value: "hello"}, {Value: "world"}},
			BuiltinFuncs: []*ir.BuiltinFunc{{Name: "eq"}},
			Files:        []*ir.StringConst{{Value: "a.rego"}},
		},
		Plans: &ir.Plans{Plans: []*ir.Plan{{
			Name: "p",
			Blocks: []*ir.Block{{Stmts: []ir.Stmt{
				withLoc(&ir.ReturnLocalStmt{Source: ir.Local(1)}),
				withLoc(&ir.CallStmt{Func: "f", Args: []ir.Operand{{Value: ir.Local(2)}}, Result: ir.Local(3)}),
				withLoc(&ir.CallDynamicStmt{Args: []ir.Local{1, 2}, Result: 3, Path: []ir.Operand{{Value: ir.StringIndex(0)}}}),
				withLoc(&ir.BlockStmt{Blocks: []*ir.Block{{Stmts: []ir.Stmt{withLoc(&ir.NopStmt{})}}}}),
				withLoc(&ir.BreakStmt{Index: 1}),
				withLoc(&ir.DotStmt{Source: ir.Operand{Value: ir.Local(1)}, Key: ir.Operand{Value: ir.StringIndex(0)}, Target: ir.Local(2)}),
				withLoc(&ir.LenStmt{Source: ir.Operand{Value: ir.Local(1)}, Target: ir.Local(2)}),
				withLoc(&ir.ScanStmt{Source: 1, Key: 2, Value: 3, Block: &ir.Block{Stmts: []ir.Stmt{withLoc(&ir.NopStmt{})}}}),
				withLoc(&ir.NotStmt{Block: &ir.Block{Stmts: []ir.Stmt{withLoc(&ir.NopStmt{})}}}),
				withLoc(&ir.AssignIntStmt{Value: 42, Target: ir.Local(1)}),
				withLoc(&ir.AssignVarStmt{Source: ir.Operand{Value: ir.Local(1)}, Target: ir.Local(2)}),
				withLoc(&ir.AssignVarOnceStmt{Source: ir.Operand{Value: ir.Local(1)}, Target: ir.Local(2)}),
				withLoc(&ir.ResetLocalStmt{Target: ir.Local(1)}),
				withLoc(&ir.MakeNullStmt{Target: ir.Local(1)}),
				withLoc(&ir.MakeNumberIntStmt{Value: 7, Target: ir.Local(1)}),
				withLoc(&ir.MakeNumberRefStmt{Index: 0, Target: ir.Local(1)}),
				withLoc(&ir.MakeArrayStmt{Capacity: 4, Target: ir.Local(1)}),
				withLoc(&ir.MakeObjectStmt{Target: ir.Local(1)}),
				withLoc(&ir.MakeSetStmt{Target: ir.Local(1)}),
				withLoc(&ir.EqualStmt{A: ir.Operand{Value: ir.Local(1)}, B: ir.Operand{Value: ir.Bool(true)}}),
				withLoc(&ir.NotEqualStmt{A: ir.Operand{Value: ir.Local(1)}, B: ir.Operand{Value: ir.Local(2)}}),
				withLoc(&ir.IsArrayStmt{Source: ir.Operand{Value: ir.Local(1)}}),
				withLoc(&ir.IsObjectStmt{Source: ir.Operand{Value: ir.Local(1)}}),
				withLoc(&ir.IsSetStmt{Source: ir.Operand{Value: ir.Local(1)}}),
				withLoc(&ir.IsDefinedStmt{Source: ir.Local(1)}),
				withLoc(&ir.IsUndefinedStmt{Source: ir.Local(1)}),
				withLoc(&ir.ArrayAppendStmt{Value: ir.Operand{Value: ir.Local(1)}, Array: ir.Local(2)}),
				withLoc(&ir.ObjectInsertStmt{Key: ir.Operand{Value: ir.StringIndex(0)}, Value: ir.Operand{Value: ir.Local(1)}, Object: ir.Local(2)}),
				withLoc(&ir.ObjectInsertOnceStmt{Key: ir.Operand{Value: ir.StringIndex(0)}, Value: ir.Operand{Value: ir.Local(1)}, Object: ir.Local(2)}),
				withLoc(&ir.ObjectMergeStmt{A: 1, B: 2, Target: 3}),
				withLoc(&ir.SetAddStmt{Value: ir.Operand{Value: ir.Local(1)}, Set: ir.Local(2)}),
				withLoc(&ir.WithStmt{Local: 1, Path: []int{0, 1}, Value: ir.Operand{Value: ir.Local(2)}, Block: &ir.Block{Stmts: []ir.Stmt{withLoc(&ir.NopStmt{})}}}),
				withLoc(&ir.ResultSetAddStmt{Value: ir.Local(1)}),
			}}},
		}}},
		Funcs: &ir.Funcs{Funcs: []*ir.Func{{
			Name:   "g",
			Params: []ir.Local{1, 2},
			Return: ir.Local(3),
			Path:   []string{"a", "b"},
			Blocks: []*ir.Block{{Stmts: []ir.Stmt{withLoc(&ir.NopStmt{})}}},
		}}},
	}

	pbPolicy, err := ir.PolicyToProto(p)
	if err != nil {
		t.Fatalf("PolicyToProto: %v", err)
	}
	bs, err := proto.Marshal(pbPolicy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &pb.Policy{}
	if err := proto.Unmarshal(bs, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs2, err := proto.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !proto.Equal(pbPolicy, decoded) {
		t.Fatal("decoded proto differs from original")
	}
	if len(bs) != len(bs2) {
		t.Fatalf("re-encoded byte length differs: %d vs %d", len(bs), len(bs2))
	}
}

func TestPolicyToProtoNil(t *testing.T) {
	got, err := ir.PolicyToProto(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

func TestPolicyToProtoEmitsLocationTripleAlways(t *testing.T) {
	p := &ir.Policy{
		Plans: &ir.Plans{Plans: []*ir.Plan{{
			Name: "p",
			Blocks: []*ir.Block{{Stmts: []ir.Stmt{
				&ir.NopStmt{}, // no SetLocation call
			}}},
		}}},
	}
	pbPolicy, err := ir.PolicyToProto(p)
	if err != nil {
		t.Fatal(err)
	}
	stmt := pbPolicy.Plans.Plans[0].Blocks[0].Stmts[0]
	if stmt.File == nil || *stmt.File != 0 {
		t.Errorf("File: want 0 (always present), got %v", stmt.File)
	}
	if stmt.Row == nil || *stmt.Row != 0 {
		t.Errorf("Row: want 0 (always present), got %v", stmt.Row)
	}
	if stmt.Col == nil || *stmt.Col != 0 {
		t.Errorf("Col: want 0 (always present), got %v", stmt.Col)
	}
}

// unsupportedStmt is a Stmt kind the encoder switch doesn't handle.
type unsupportedStmt struct{ ir.Location }

func TestPolicyToProtoUnknownStmtReturnsError(t *testing.T) {
	p := &ir.Policy{
		Plans: &ir.Plans{Plans: []*ir.Plan{{
			Name: "p",
			Blocks: []*ir.Block{{Stmts: []ir.Stmt{
				&unsupportedStmt{},
			}}},
		}}},
	}

	got, err := ir.PolicyToProto(p)
	if err == nil {
		t.Fatal("expected error for unknown Stmt type, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil result on error, got %v", got)
	}
	if !strings.Contains(err.Error(), "unsupportedStmt") {
		t.Fatalf("error should name the offending type: %v", err)
	}
}

func TestPolicyToProtoInt32OverflowReturnsError(t *testing.T) {
	p := &ir.Policy{
		Plans: &ir.Plans{Plans: []*ir.Plan{{
			Name: "p",
			Blocks: []*ir.Block{{Stmts: []ir.Stmt{
				&ir.ReturnLocalStmt{Source: ir.Local(math.MaxInt32 + 1)},
			}}},
		}}},
	}

	got, err := ir.PolicyToProto(p)
	if err == nil {
		t.Fatal("expected error for int32 overflow, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil result on error, got %v", got)
	}
	if !strings.Contains(err.Error(), "overflows int32") {
		t.Fatalf("error should mention the overflow: %v", err)
	}
}
