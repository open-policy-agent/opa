// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package wasm contains an IR->WASM compiler backend.
package wasm

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/internal/compiler/wasm/opa"
	"github.com/open-policy-agent/opa/internal/ir"
	"github.com/open-policy-agent/opa/internal/wasm/encoding"
	"github.com/open-policy-agent/opa/internal/wasm/instruction"
	"github.com/open-policy-agent/opa/internal/wasm/module"
	"github.com/open-policy-agent/opa/internal/wasm/types"
	"github.com/pkg/errors"
)

const (
	opaTypeNull int32 = iota + 1
	opaTypeBoolean
	opaTypeNumber
	opaTypeString
	opaTypeArray
	opaTypeObject
)

const (
	opaFuncPrefix        = "opa_"
	opaJSONParse         = "opa_json_parse"
	opaNull              = "opa_null"
	opaBoolean           = "opa_boolean"
	opaNumberInt         = "opa_number_int"
	opaNumberSize        = "opa_number_size"
	opaArrayWithCap      = "opa_array_with_cap"
	opaArrayAppend       = "opa_array_append"
	opaObject            = "opa_object"
	opaObjectInsert      = "opa_object_insert"
	opaSet               = "opa_set"
	opaSetAdd            = "opa_set_add"
	opaStringTerminated  = "opa_string_terminated"
	opaValueBooleanSet   = "opa_value_boolean_set"
	opaValueNumberSetInt = "opa_value_number_set_int"
	opaValueCompare      = "opa_value_compare"
	opaValueGet          = "opa_value_get"
	opaValueIter         = "opa_value_iter"
	opaValueLength       = "opa_value_length"
	opaValueType         = "opa_value_type"
)

// Compiler implements an IR->WASM compiler backend.
type Compiler struct {
	stages []func() error // compiler stages to execute
	errors []error        // compilation errors encountered

	policy *ir.Policy        // input policy to compile
	module *module.Module    // output WASM module
	code   *module.CodeEntry // output WASM code

	stringOffset int32             // null-terminated string data base offset
	stringAddrs  []uint32          // null-terminated string constant addresses
	funcs        map[string]uint32 // maps exported function names to function indices

	nextLocal uint32
	locals    map[ir.Local]uint32
}

// New returns a new compiler object.
func New() *Compiler {
	c := &Compiler{}
	c.stages = []func() error{
		c.initModule,
		c.compileStrings,
		c.compileFuncs,
		c.compilePlan,
	}
	return c
}

// WithPolicy sets the policy to compile.
func (c *Compiler) WithPolicy(p *ir.Policy) *Compiler {
	c.policy = p
	return c
}

// Compile returns a compiled WASM module.
func (c *Compiler) Compile() (*module.Module, error) {

	for _, stage := range c.stages {
		if err := stage(); err != nil {
			return nil, err
		} else if len(c.errors) > 0 {
			return nil, c.errors[0] // TODO(tsandall) return all errors.
		}
	}

	return c.module, nil
}

// initModule instantiates the module from the pre-compiled OPA binary. The
// module is then updated to include declarations for all of the functions that
// are about to be compiled.
func (c *Compiler) initModule() error {

	bs, err := opa.Bytes()
	if err != nil {
		return err
	}

	c.module, err = encoding.ReadModule(bytes.NewReader(bs))
	if err != nil {
		return err
	}

	c.funcs = make(map[string]uint32)

	for _, exp := range c.module.Export.Exports {
		if exp.Descriptor.Type == module.FunctionExportType && strings.HasPrefix(exp.Name, opaFuncPrefix) {
			c.funcs[exp.Name] = exp.Descriptor.Index
		}
	}

	for _, fn := range c.policy.Funcs.Funcs {

		params := make([]types.ValueType, len(fn.Params))
		for i := 0; i < len(params); i++ {
			params[i] = types.I32
		}

		tpe := module.FunctionType{
			Params:  params,
			Results: []types.ValueType{types.I32},
		}

		c.emitFunctionDecl(fn.Name, tpe, false)
	}

	c.emitFunctionDecl("eval", module.FunctionType{
		Params:  []types.ValueType{types.I32, types.I32},
		Results: []types.ValueType{types.I32},
	}, true)

	return nil
}

// compileStrings compiles string constants into the data section of the module.
// The strings are indexed for lookups in later stages.
func (c *Compiler) compileStrings() error {

	c.stringOffset = 2048
	c.stringAddrs = make([]uint32, len(c.policy.Static.Strings))
	var buf bytes.Buffer

	for i, s := range c.policy.Static.Strings {
		addr := uint32(buf.Len()) + uint32(c.stringOffset)
		buf.WriteString(s.Value)
		buf.WriteByte(0)
		c.stringAddrs[i] = addr
	}

	c.module.Data.Segments = append(c.module.Data.Segments, module.DataSegment{
		Index: 0,
		Offset: module.Expr{
			Instrs: []instruction.Instruction{
				instruction.I32Const{
					Value: c.stringOffset,
				},
			},
		},
		Init: buf.Bytes(),
	})

	return nil
}

// compileFuncs compiles the policy functions and emits them into the module.
func (c *Compiler) compileFuncs() error {

	for _, fn := range c.policy.Funcs.Funcs {
		if err := c.compileFunc(fn); err != nil {
			return errors.Wrapf(err, "func %v", fn.Name)
		}
	}

	return nil
}

// compilePlan compiles the policy plan and emits the resulting function into
// the module.
func (c *Compiler) compilePlan() error {

	// reset local variables and declare raw ptr, len, and input ptr.
	c.nextLocal = 0
	c.locals = map[ir.Local]uint32{}
	_ = c.local(ir.InputRaw)
	_ = c.local(ir.InputLen)
	_ = c.local(ir.Input)

	c.code = &module.CodeEntry{}

	c.appendInstr(instruction.GetLocal{Index: c.local(ir.InputRaw)})
	c.appendInstr(instruction.GetLocal{Index: c.local(ir.InputLen)})
	c.appendInstr(instruction.Call{Index: c.function(opaJSONParse)})
	c.appendInstr(instruction.SetLocal{Index: c.local(ir.Input)})

	for i := range c.policy.Plan.Blocks {

		instrs, err := c.compileBlock(c.policy.Plan.Blocks[i])
		if err != nil {
			return errors.Wrapf(err, "block %d", i)
		}

		if i < len(c.policy.Plan.Blocks)-1 {
			c.appendInstr(instruction.Block{Instrs: instrs})
		} else {
			c.appendInstrs(instrs)
		}
	}

	c.code.Func.Locals = []module.LocalDeclaration{
		{
			Count: c.nextLocal,
			Type:  types.I32,
		},
	}

	return c.emitFunction("eval", c.code)
}

func (c *Compiler) compileFunc(fn *ir.Func) error {

	if len(fn.Params) == 0 {
		return fmt.Errorf("illegal function: zero args")
	}

	c.nextLocal = 0
	c.locals = map[ir.Local]uint32{}

	for _, a := range fn.Params {
		_ = c.local(a)
	}

	_ = c.local(fn.Return)

	c.code = &module.CodeEntry{}

	for i := range fn.Blocks {
		instrs, err := c.compileBlock(fn.Blocks[i])
		if err != nil {
			return errors.Wrapf(err, "block %d", i)
		}
		if i < len(fn.Blocks)-1 {
			c.appendInstr(instruction.Block{Instrs: instrs})
		} else {
			c.appendInstrs(instrs)
		}
	}

	c.code.Func.Locals = []module.LocalDeclaration{
		{
			Count: c.nextLocal,
			Type:  types.I32,
		},
	}

	var params []types.ValueType

	for i := 0; i < len(fn.Params); i++ {
		params = append(params, types.I32)
	}

	return c.emitFunction(fn.Name, c.code)
}

func (c *Compiler) compileBlock(block *ir.Block) ([]instruction.Instruction, error) {

	var instrs []instruction.Instruction

	for _, stmt := range block.Stmts {
		switch stmt := stmt.(type) {
		case *ir.ReturnStmt:
			instrs = append(instrs, instruction.I32Const{Value: int32(stmt.Code)})
			instrs = append(instrs, instruction.Return{})
		case *ir.ReturnLocalStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.Return{})
		case *ir.BlockStmt:
			nested := make([]instruction.Instruction, len(stmt.Blocks))
			for i := range stmt.Blocks {
				block, err := c.compileBlock(stmt.Blocks[i])
				if err != nil {
					return nil, err
				}
				nested[i] = instruction.Block{Instrs: block}
			}
			instrs = append(instrs, instruction.Block{Instrs: nested})
		case *ir.BreakStmt:
			instrs = append(instrs, instruction.Br{Index: stmt.Index})
		case *ir.CallStmt:
			for _, arg := range stmt.Args {
				instrs = append(instrs, instruction.GetLocal{Index: c.local(arg)})
			}
			instrs = append(instrs, instruction.Call{Index: c.function(stmt.Func)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Result)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Result)})
			instrs = append(instrs, instruction.I32Eqz{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.AssignVarStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.AssignVarOnceStmt:
			instrs = append(instrs, instruction.Block{
				Instrs: []instruction.Instruction{
					instruction.Block{
						Instrs: []instruction.Instruction{
							instruction.GetLocal{Index: c.local(stmt.Target)},
							instruction.I32Eqz{},
							instruction.BrIf{Index: 0},
							instruction.GetLocal{Index: c.local(stmt.Target)},
							instruction.GetLocal{Index: c.local(stmt.Source)},
							instruction.Call{Index: c.function(opaValueCompare)},
							instruction.I32Eqz{},
							instruction.BrIf{Index: 1},
							instruction.Unreachable{}, // TODO(tsandall): replace with conflict error
						},
					},
					instruction.GetLocal{Index: c.local(stmt.Source)},
					instruction.SetLocal{Index: c.local(stmt.Target)},
				},
			})
		case *ir.AssignBooleanStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Target)})
			if stmt.Value {
				instrs = append(instrs, instruction.I32Const{Value: 1})
			} else {
				instrs = append(instrs, instruction.I32Const{Value: 0})
			}
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueBooleanSet)})
		case *ir.AssignIntStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Target)})
			instrs = append(instrs, instruction.I64Const{Value: stmt.Value})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueNumberSetInt)})
		case *ir.ScanStmt:
			if err := c.compileScan(stmt, &instrs); err != nil {
				return nil, err
			}
		case *ir.NotStmt:
			if err := c.compileNot(stmt, &instrs); err != nil {
				return nil, err
			}
		case *ir.DotStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Key)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueGet)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Target)})
			instrs = append(instrs, instruction.I32Eqz{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.LenStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueLength)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaNumberSize)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.EqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.LessThanStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32GeS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.LessThanEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32GtS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.GreaterThanStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32LeS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.GreaterThanEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32LtS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.NotEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Eqz{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.MakeNullStmt:
			instrs = append(instrs, instruction.Call{Index: c.function(opaNull)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeBooleanStmt:
			instr := instruction.I32Const{}
			if stmt.Value {
				instr.Value = 1
			} else {
				instr.Value = 0
			}
			instrs = append(instrs, instr)
			instrs = append(instrs, instruction.Call{Index: c.function(opaBoolean)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeNumberIntStmt:
			instrs = append(instrs, instruction.I64Const{Value: stmt.Value})
			instrs = append(instrs, instruction.Call{Index: c.function(opaNumberInt)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeStringStmt:
			instrs = append(instrs, instruction.I32Const{Value: c.stringAddr(stmt.Index)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaStringTerminated)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeArrayStmt:
			instrs = append(instrs, instruction.I32Const{Value: stmt.Capacity})
			instrs = append(instrs, instruction.Call{Index: c.function(opaArrayWithCap)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeObjectStmt:
			instrs = append(instrs, instruction.Call{Index: c.function(opaObject)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.MakeSetStmt:
			instrs = append(instrs, instruction.Call{Index: c.function(opaSet)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case *ir.IsArrayStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueType)})
			instrs = append(instrs, instruction.I32Const{Value: opaTypeArray})
			instrs = append(instrs, instruction.I32Ne{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.IsObjectStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueType)})
			instrs = append(instrs, instruction.I32Const{Value: opaTypeObject})
			instrs = append(instrs, instruction.I32Ne{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.IsUndefinedStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32Ne{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.IsDefinedStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.I32Eqz{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case *ir.ArrayAppendStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Array)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Value)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaArrayAppend)})
		case *ir.ObjectInsertStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Object)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Key)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Value)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaObjectInsert)})
		case *ir.ObjectInsertOnceStmt:
			tmp := c.genLocal()
			instrs = append(instrs, instruction.Block{
				Instrs: []instruction.Instruction{
					instruction.Block{
						Instrs: []instruction.Instruction{
							instruction.GetLocal{Index: c.local(stmt.Object)},
							instruction.GetLocal{Index: c.local(stmt.Key)},
							instruction.Call{Index: c.function(opaValueGet)},
							instruction.SetLocal{Index: tmp},
							instruction.GetLocal{Index: tmp},
							instruction.I32Eqz{},
							instruction.BrIf{Index: 0},
							instruction.GetLocal{Index: tmp},
							instruction.GetLocal{Index: c.local(stmt.Value)},
							instruction.Call{Index: c.function(opaValueCompare)},
							instruction.I32Eqz{},
							instruction.BrIf{Index: 1},
							instruction.Unreachable{}, // TODO(tsandall): replace with conflict error
						},
					},
					instruction.GetLocal{Index: c.local(stmt.Object)},
					instruction.GetLocal{Index: c.local(stmt.Key)},
					instruction.GetLocal{Index: c.local(stmt.Value)},
					instruction.Call{Index: c.function(opaObjectInsert)},
				},
			})
		case *ir.SetAddStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Set)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Value)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaSetAdd)})
		default:
			var buf bytes.Buffer
			ir.Pretty(&buf, stmt)
			return instrs, fmt.Errorf("illegal statement: %v", buf.String())
		}
	}

	return instrs, nil
}

func (c *Compiler) compileScan(scan *ir.ScanStmt, result *[]instruction.Instruction) error {
	var instrs = *result
	instrs = append(instrs, instruction.I32Const{Value: 0})
	instrs = append(instrs, instruction.SetLocal{Index: c.local(scan.Key)})
	body, err := c.compileScanBlock(scan)
	if err != nil {
		return err
	}
	instrs = append(instrs, instruction.Loop{Instrs: body})
	*result = instrs
	return nil
}

func (c *Compiler) compileScanBlock(scan *ir.ScanStmt) ([]instruction.Instruction, error) {
	var instrs []instruction.Instruction

	// Execute iterator.
	instrs = append(instrs, instruction.GetLocal{Index: c.local(scan.Source)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(scan.Key)})
	instrs = append(instrs, instruction.Call{Index: c.function(opaValueIter)})

	// Check for emptiness.
	instrs = append(instrs, instruction.SetLocal{Index: c.local(scan.Key)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(scan.Key)})
	instrs = append(instrs, instruction.I32Eqz{})
	instrs = append(instrs, instruction.BrIf{Index: 1})

	// Load value.
	instrs = append(instrs, instruction.GetLocal{Index: c.local(scan.Source)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(scan.Key)})
	instrs = append(instrs, instruction.Call{Index: c.function(opaValueGet)})
	instrs = append(instrs, instruction.SetLocal{Index: c.local(scan.Value)})

	// Loop body.
	nested, err := c.compileBlock(scan.Block)
	if err != nil {
		return nil, err
	}

	instrs = append(instrs, nested...)
	instrs = append(instrs, instruction.Br{Index: 0})

	return instrs, nil
}

func (c *Compiler) compileNot(not *ir.NotStmt, result *[]instruction.Instruction) error {
	var instrs = *result

	nested, err := c.compileBlock(not.Block)
	if err != nil {
		return err
	}

	instrs = append(instrs, instruction.Block{Instrs: nested})
	*result = instrs
	return nil
}

func (c *Compiler) emitFunctionDecl(name string, tpe module.FunctionType, export bool) {

	typeIndex := c.emitFunctionType(tpe)
	c.module.Function.TypeIndices = append(c.module.Function.TypeIndices, typeIndex)
	c.module.Code.Segments = append(c.module.Code.Segments, module.RawCodeSegment{})
	c.funcs[name] = uint32(len(c.module.Function.TypeIndices))

	if export {
		c.module.Export.Exports = append(c.module.Export.Exports, module.Export{
			Name: name,
			Descriptor: module.ExportDescriptor{
				Type:  module.FunctionExportType,
				Index: c.funcs[name],
			},
		})
	}

}

func (c *Compiler) emitFunctionType(tpe module.FunctionType) uint32 {
	for i, other := range c.module.Type.Functions {
		if tpe.Equal(other) {
			return uint32(i)
		}
	}
	c.module.Type.Functions = append(c.module.Type.Functions, tpe)
	return uint32(len(c.module.Type.Functions) - 1)
}

func (c *Compiler) emitFunction(name string, entry *module.CodeEntry) error {
	var buf bytes.Buffer
	if err := encoding.WriteCodeEntry(&buf, entry); err != nil {
		return err
	}
	index := c.function(name)
	c.module.Code.Segments[index-1].Code = buf.Bytes()
	return nil
}

func (c *Compiler) stringAddr(index int) int32 {
	return int32(c.stringAddrs[index])
}

func (c *Compiler) local(l ir.Local) uint32 {
	var u32 uint32
	var exist bool
	if u32, exist = c.locals[l]; !exist {
		u32 = c.nextLocal
		c.locals[l] = u32
		c.nextLocal++
	}
	return u32
}

func (c *Compiler) genLocal() uint32 {
	l := c.nextLocal
	c.nextLocal++
	return l
}

func (c *Compiler) function(name string) uint32 {
	index, ok := c.funcs[name]
	if !ok {
		c.errors = append(c.errors, fmt.Errorf("illegal function reference %q", name))
	}
	return index
}

func (c *Compiler) appendInstr(instr instruction.Instruction) {
	c.code.Func.Expr.Instrs = append(c.code.Func.Expr.Instrs, instr)
}

func (c *Compiler) appendInstrs(instrs []instruction.Instruction) {
	for _, instr := range instrs {
		c.appendInstr(instr)
	}
}
