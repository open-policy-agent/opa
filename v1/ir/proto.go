// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import (
	"fmt"
	"math"

	pb "github.com/open-policy-agent/opa/v1/ir/v1pb"
)

// PolicyToProto converts an IR Policy to its protobuf wire-form,
// defined in v1/ir/plan.proto. Returns an error if the policy contains
// a Stmt or Val kind not yet covered by the encoder switch.
func PolicyToProto(p *Policy) (out *pb.Policy, err error) {
	if p == nil {
		return nil, nil
	}
	defer func() {
		if r := recover(); r != nil {
			out = nil
			err = fmt.Errorf("ir: PolicyToProto: %v", r)
		}
	}()
	return &pb.Policy{
		Static: staticToProto(p.Static),
		Plans:  plansToProto(p.Plans),
		Funcs:  funcsToProto(p.Funcs),
	}, nil
}

func staticToProto(s *Static) *pb.Static {
	if s == nil {
		return nil
	}
	out := &pb.Static{
		Strings:      make([]*pb.StringConst, len(s.Strings)),
		BuiltinFuncs: make([]*pb.BuiltinFunc, len(s.BuiltinFuncs)),
		Files:        make([]*pb.StringConst, len(s.Files)),
	}
	for i, sc := range s.Strings {
		out.Strings[i] = stringConstToProto(sc)
	}
	for i, bf := range s.BuiltinFuncs {
		out.BuiltinFuncs[i] = builtinFuncToProto(bf)
	}
	for i, f := range s.Files {
		out.Files[i] = stringConstToProto(f)
	}
	return out
}

func stringConstToProto(s *StringConst) *pb.StringConst {
	if s == nil {
		return nil
	}
	return &pb.StringConst{Value: new(s.Value)}
}

func builtinFuncToProto(b *BuiltinFunc) *pb.BuiltinFunc {
	if b == nil {
		return nil
	}
	return &pb.BuiltinFunc{Name: new(b.Name)}
}

func plansToProto(p *Plans) *pb.Plans {
	if p == nil {
		return nil
	}
	out := &pb.Plans{Plans: make([]*pb.Plan, len(p.Plans))}
	for i, pl := range p.Plans {
		out.Plans[i] = planToProto(pl)
	}
	return out
}

func planToProto(p *Plan) *pb.Plan {
	if p == nil {
		return nil
	}
	out := &pb.Plan{Name: new(p.Name), Blocks: make([]*pb.Block, len(p.Blocks))}
	for i, b := range p.Blocks {
		out.Blocks[i] = blockToProto(b)
	}
	return out
}

func funcsToProto(f *Funcs) *pb.Funcs {
	if f == nil {
		return nil
	}
	out := &pb.Funcs{Funcs: make([]*pb.Func, len(f.Funcs))}
	for i, fn := range f.Funcs {
		out.Funcs[i] = funcToProto(fn)
	}
	return out
}

func funcToProto(f *Func) *pb.Func {
	if f == nil {
		return nil
	}
	out := &pb.Func{
		Name:   new(f.Name),
		Params: localsToInt32s(f.Params),
		Result: new(toInt32(f.Return)),
		Blocks: make([]*pb.Block, len(f.Blocks)),
		Path:   f.Path,
	}
	for i, b := range f.Blocks {
		out.Blocks[i] = blockToProto(b)
	}
	return out
}

func blockToProto(b *Block) *pb.Block {
	if b == nil {
		return nil
	}
	out := &pb.Block{Stmts: make([]*pb.Stmt, len(b.Stmts))}
	for i, s := range b.Stmts {
		out.Stmts[i] = stmtToProto(s)
	}
	return out
}

func operandToProto(o Operand) *pb.Operand {
	return &pb.Operand{Value: valToProto(o.Value)}
}

func operandsToProto(os []Operand) []*pb.Operand {
	out := make([]*pb.Operand, len(os))
	for i, o := range os {
		out[i] = operandToProto(o)
	}
	return out
}

func valToProto(v Val) *pb.Val {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case Local:
		return &pb.Val{Kind: &pb.Val_Local{Local: toInt32(x)}}
	case StringIndex:
		return &pb.Val{Kind: &pb.Val_StringIndex{StringIndex: toInt32(x)}}
	case Bool:
		return &pb.Val{Kind: &pb.Val_Bool{Bool: bool(x)}}
	default:
		panic(fmt.Sprintf("unsupported Val type %T", v))
	}
}

// toInt32 narrows an int-based value to int32, panicking if it would
// overflow. PolicyToProto recovers from the panic and returns it as an
// error, so callers don't need to check the bound themselves.
func toInt32[T ~int](v T) int32 {
	if int64(v) > math.MaxInt32 || int64(v) < math.MinInt32 {
		panic(fmt.Sprintf("value %d overflows int32", int64(v)))
	}
	return int32(v)
}

func localsToInt32s(ls []Local) []int32 {
	out := make([]int32, len(ls))
	for i, l := range ls {
		out[i] = toInt32(l)
	}
	return out
}

func intsToInt32s(is []int) []int32 {
	out := make([]int32, len(is))
	for i, v := range is {
		out[i] = toInt32(v)
	}
	return out
}

func stmtToProto(s Stmt) *pb.Stmt {
	if s == nil {
		return nil
	}
	loc := s.GetLocation()
	out := &pb.Stmt{
		File:   new(toInt32(loc.File)),
		Col:    new(toInt32(loc.Col)),
		Row:    new(toInt32(loc.Row)),
		EndCol: new(toInt32(loc.EndCol)),
		EndRow: new(toInt32(loc.EndRow)),
	}
	switch x := s.(type) {
	case *ArrayAppendStmt:
		out.Kind = &pb.Stmt_ArrayAppendStmt{ArrayAppendStmt: &pb.ArrayAppendStmt{
			Value: operandToProto(x.Value),
			Array: new(toInt32(x.Array)),
		}}
	case *AssignIntStmt:
		out.Kind = &pb.Stmt_AssignIntStmt{AssignIntStmt: &pb.AssignIntStmt{
			Value:  new(x.Value),
			Target: new(toInt32(x.Target)),
		}}
	case *AssignVarOnceStmt:
		out.Kind = &pb.Stmt_AssignVarOnceStmt{AssignVarOnceStmt: &pb.AssignVarOnceStmt{
			Source: operandToProto(x.Source),
			Target: new(toInt32(x.Target)),
		}}
	case *AssignVarStmt:
		out.Kind = &pb.Stmt_AssignVarStmt{AssignVarStmt: &pb.AssignVarStmt{
			Source: operandToProto(x.Source),
			Target: new(toInt32(x.Target)),
		}}
	case *BlockStmt:
		body := &pb.BlockStmt{Blocks: make([]*pb.Block, len(x.Blocks))}
		for i, b := range x.Blocks {
			body.Blocks[i] = blockToProto(b)
		}
		out.Kind = &pb.Stmt_BlockStmt{BlockStmt: body}
	case *BreakStmt:
		out.Kind = &pb.Stmt_BreakStmt{BreakStmt: &pb.BreakStmt{Index: new(x.Index)}}
	case *CallDynamicStmt:
		out.Kind = &pb.Stmt_CallDynamicStmt{CallDynamicStmt: &pb.CallDynamicStmt{
			Args:   localsToInt32s(x.Args),
			Result: new(toInt32(x.Result)),
			Path:   operandsToProto(x.Path),
		}}
	case *CallStmt:
		out.Kind = &pb.Stmt_CallStmt{CallStmt: &pb.CallStmt{
			Function: new(x.Func),
			Args:     operandsToProto(x.Args),
			Result:   new(toInt32(x.Result)),
		}}
	case *DotStmt:
		out.Kind = &pb.Stmt_DotStmt{DotStmt: &pb.DotStmt{
			Source: operandToProto(x.Source),
			Key:    operandToProto(x.Key),
			Target: new(toInt32(x.Target)),
		}}
	case *EqualStmt:
		out.Kind = &pb.Stmt_EqualStmt{EqualStmt: &pb.EqualStmt{
			A: operandToProto(x.A),
			B: operandToProto(x.B),
		}}
	case *IsArrayStmt:
		out.Kind = &pb.Stmt_IsArrayStmt{IsArrayStmt: &pb.IsArrayStmt{Source: operandToProto(x.Source)}}
	case *IsDefinedStmt:
		out.Kind = &pb.Stmt_IsDefinedStmt{IsDefinedStmt: &pb.IsDefinedStmt{Source: new(toInt32(x.Source))}}
	case *IsObjectStmt:
		out.Kind = &pb.Stmt_IsObjectStmt{IsObjectStmt: &pb.IsObjectStmt{Source: operandToProto(x.Source)}}
	case *IsSetStmt:
		out.Kind = &pb.Stmt_IsSetStmt{IsSetStmt: &pb.IsSetStmt{Source: operandToProto(x.Source)}}
	case *IsUndefinedStmt:
		out.Kind = &pb.Stmt_IsUndefinedStmt{IsUndefinedStmt: &pb.IsUndefinedStmt{Source: new(toInt32(x.Source))}}
	case *LenStmt:
		out.Kind = &pb.Stmt_LenStmt{LenStmt: &pb.LenStmt{
			Source: operandToProto(x.Source),
			Target: new(toInt32(x.Target)),
		}}
	case *MakeArrayStmt:
		out.Kind = &pb.Stmt_MakeArrayStmt{MakeArrayStmt: &pb.MakeArrayStmt{
			Capacity: new(x.Capacity),
			Target:   new(toInt32(x.Target)),
		}}
	case *MakeNullStmt:
		out.Kind = &pb.Stmt_MakeNullStmt{MakeNullStmt: &pb.MakeNullStmt{Target: new(toInt32(x.Target))}}
	case *MakeNumberIntStmt:
		out.Kind = &pb.Stmt_MakeNumberIntStmt{MakeNumberIntStmt: &pb.MakeNumberIntStmt{
			Value:  new(x.Value),
			Target: new(toInt32(x.Target)),
		}}
	case *MakeNumberRefStmt:
		out.Kind = &pb.Stmt_MakeNumberRefStmt{MakeNumberRefStmt: &pb.MakeNumberRefStmt{
			Index:  new(toInt32(x.Index)),
			Target: new(toInt32(x.Target)),
		}}
	case *MakeObjectStmt:
		out.Kind = &pb.Stmt_MakeObjectStmt{MakeObjectStmt: &pb.MakeObjectStmt{Target: new(toInt32(x.Target))}}
	case *MakeSetStmt:
		out.Kind = &pb.Stmt_MakeSetStmt{MakeSetStmt: &pb.MakeSetStmt{Target: new(toInt32(x.Target))}}
	case *NopStmt:
		out.Kind = &pb.Stmt_NopStmt{NopStmt: &pb.NopStmt{}}
	case *NotEqualStmt:
		out.Kind = &pb.Stmt_NotEqualStmt{NotEqualStmt: &pb.NotEqualStmt{
			A: operandToProto(x.A),
			B: operandToProto(x.B),
		}}
	case *NotStmt:
		out.Kind = &pb.Stmt_NotStmt{NotStmt: &pb.NotStmt{Block: blockToProto(x.Block)}}
	case *ObjectInsertOnceStmt:
		out.Kind = &pb.Stmt_ObjectInsertOnceStmt{ObjectInsertOnceStmt: &pb.ObjectInsertOnceStmt{
			Key:    operandToProto(x.Key),
			Value:  operandToProto(x.Value),
			Object: new(toInt32(x.Object)),
		}}
	case *ObjectInsertStmt:
		out.Kind = &pb.Stmt_ObjectInsertStmt{ObjectInsertStmt: &pb.ObjectInsertStmt{
			Key:    operandToProto(x.Key),
			Value:  operandToProto(x.Value),
			Object: new(toInt32(x.Object)),
		}}
	case *ObjectMergeStmt:
		out.Kind = &pb.Stmt_ObjectMergeStmt{ObjectMergeStmt: &pb.ObjectMergeStmt{
			A:      new(toInt32(x.A)),
			B:      new(toInt32(x.B)),
			Target: new(toInt32(x.Target)),
		}}
	case *ResetLocalStmt:
		out.Kind = &pb.Stmt_ResetLocalStmt{ResetLocalStmt: &pb.ResetLocalStmt{Target: new(toInt32(x.Target))}}
	case *ResultSetAddStmt:
		out.Kind = &pb.Stmt_ResultSetAddStmt{ResultSetAddStmt: &pb.ResultSetAddStmt{Value: new(toInt32(x.Value))}}
	case *ReturnLocalStmt:
		out.Kind = &pb.Stmt_ReturnLocalStmt{ReturnLocalStmt: &pb.ReturnLocalStmt{Source: new(toInt32(x.Source))}}
	case *ScanStmt:
		out.Kind = &pb.Stmt_ScanStmt{ScanStmt: &pb.ScanStmt{
			Source: new(toInt32(x.Source)),
			Key:    new(toInt32(x.Key)),
			Value:  new(toInt32(x.Value)),
			Block:  blockToProto(x.Block),
		}}
	case *SetAddStmt:
		out.Kind = &pb.Stmt_SetAddStmt{SetAddStmt: &pb.SetAddStmt{
			Value: operandToProto(x.Value),
			Set:   new(toInt32(x.Set)),
		}}
	case *WithStmt:
		out.Kind = &pb.Stmt_WithStmt{WithStmt: &pb.WithStmt{
			Local: new(toInt32(x.Local)),
			Path:  intsToInt32s(x.Path),
			Value: operandToProto(x.Value),
			Block: blockToProto(x.Block),
		}}
	default:
		panic(fmt.Sprintf("unsupported Stmt type %T", s))
	}
	return out
}
