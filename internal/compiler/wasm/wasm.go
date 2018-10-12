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
	opaFuncPrefix       = "opa_"
	opaJSONParse        = "opa_json_parse"
	opaBoolean          = "opa_boolean"
	opaStringTerminated = "opa_string_terminated"
	opaNumberInt        = "opa_number_int"
	opaValueBooleanSet  = "opa_value_boolean_set"
	opaValueNotEqual    = "opa_value_not_equal"
	opaValueCompare     = "opa_value_compare"
	opaValueGet         = "opa_value_get"
	opaValueIter        = "opa_value_iter"
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
	stringData   []byte            // null-terminated strings to write into data section
	funcs        map[string]uint32 // maps exported function names to function indices
	evalIndex    uint32            // function section index of eval func

	localMax uint32
}

// New returns a new compiler object.
func New() *Compiler {
	c := &Compiler{
		code:         &module.CodeEntry{},
		stringOffset: 2048,
		localMax:     2, // assume that locals start at 0..2 then increment monotonically
	}
	c.stages = []func() error{
		c.initModule,
		c.compileStrings,
		c.emitEntry,
		c.compilePlan,
		c.emitLocals,
		c.emitFunctionSection,
		c.emitExportSection,
		c.emitCodeSection,
		c.emitDataSection,
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
		}
	}

	return c.module, nil
}

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

	return nil
}

func (c *Compiler) compileStrings() error {

	c.stringAddrs = make([]uint32, len(c.policy.Static.Strings))
	var buf bytes.Buffer

	for i, s := range c.policy.Static.Strings {
		addr := uint32(buf.Len()) + uint32(c.stringOffset)
		buf.WriteString(s.Value)
		buf.WriteByte(0)
		c.stringAddrs[i] = addr
	}

	c.stringData = buf.Bytes()
	return nil
}

func (c *Compiler) emitEntry() error {
	c.appendInstr(instruction.GetLocal{Index: c.local(ir.InputRaw)})
	c.appendInstr(instruction.GetLocal{Index: c.local(ir.InputLen)})
	c.appendInstr(instruction.Call{Index: c.function(opaJSONParse)})
	c.appendInstr(instruction.SetLocal{Index: c.local(ir.Input)})
	return nil
}

func (c *Compiler) compilePlan() error {

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

	return nil
}

func (c *Compiler) compileBlock(block ir.Block) ([]instruction.Instruction, error) {

	var instrs []instruction.Instruction

	for _, stmt := range block.Stmts {
		switch stmt := stmt.(type) {
		case ir.ReturnStmt:
			instrs = append(instrs, instruction.I32Const{Value: int32(stmt.Code)})
			instrs = append(instrs, instruction.Return{})
		case ir.AssignStmt:
			switch value := stmt.Value.(type) {
			case ir.BooleanConst:
				instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Target)})
				if value.Value {
					instrs = append(instrs, instruction.I32Const{Value: 1})
				} else {
					instrs = append(instrs, instruction.I32Const{Value: 0})
				}
				instrs = append(instrs, instruction.Call{Index: c.function(opaValueBooleanSet)})
			default:
				var buf bytes.Buffer
				ir.Pretty(&buf, stmt)
				return nil, fmt.Errorf("illegal assignment: %v", buf.String())
			}
		case ir.LoopStmt:
			if err := c.compileLoop(stmt, &instrs); err != nil {
				return nil, err
			}
		case ir.DotStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Source)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.Key)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueGet)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case ir.EqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueNotEqual)})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.LessThanStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32GeS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.LessThanEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32GtS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.GreaterThanStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32LeS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.GreaterThanEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Const{Value: 0})
			instrs = append(instrs, instruction.I32LtS{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.NotEqualStmt:
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.A)})
			instrs = append(instrs, instruction.GetLocal{Index: c.local(stmt.B)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaValueCompare)})
			instrs = append(instrs, instruction.I32Eqz{})
			instrs = append(instrs, instruction.BrIf{Index: 0})
		case ir.MakeBooleanStmt:
			instr := instruction.I32Const{}
			if stmt.Value {
				instr.Value = 1
			} else {
				instr.Value = 0
			}
			instrs = append(instrs, instr)
			instrs = append(instrs, instruction.Call{Index: c.function(opaBoolean)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case ir.MakeNumberIntStmt:
			instrs = append(instrs, instruction.I64Const{Value: stmt.Value})
			instrs = append(instrs, instruction.Call{Index: c.function(opaNumberInt)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		case ir.MakeStringStmt:
			instrs = append(instrs, instruction.I32Const{Value: c.stringAddr(stmt.Index)})
			instrs = append(instrs, instruction.Call{Index: c.function(opaStringTerminated)})
			instrs = append(instrs, instruction.SetLocal{Index: c.local(stmt.Target)})
		default:
			var buf bytes.Buffer
			ir.Pretty(&buf, stmt)
			return instrs, fmt.Errorf("illegal statement: %v", buf.String())
		}

	}

	return instrs, nil
}

func (c *Compiler) compileLoop(loop ir.LoopStmt, result *[]instruction.Instruction) error {
	var instrs = *result
	instrs = append(instrs, instruction.I32Const{Value: 0})
	instrs = append(instrs, instruction.SetLocal{Index: c.local(loop.Key)})
	body, err := c.compileLoopBody(loop)
	if err != nil {
		return err
	}
	instrs = append(instrs, instruction.Loop{Instrs: body})
	*result = instrs
	return nil
}

func (c *Compiler) compileLoopBody(loop ir.LoopStmt) ([]instruction.Instruction, error) {
	var instrs []instruction.Instruction

	// Execute iterator.
	instrs = append(instrs, instruction.GetLocal{Index: c.local(loop.Source)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(loop.Key)})
	instrs = append(instrs, instruction.Call{Index: c.function(opaValueIter)})

	// Check for emptiness.
	instrs = append(instrs, instruction.SetLocal{Index: c.local(loop.Key)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(loop.Key)})
	instrs = append(instrs, instruction.I32Eqz{})
	instrs = append(instrs, instruction.BrIf{Index: 1})

	// Load value.
	instrs = append(instrs, instruction.GetLocal{Index: c.local(loop.Source)})
	instrs = append(instrs, instruction.GetLocal{Index: c.local(loop.Key)})
	instrs = append(instrs, instruction.Call{Index: c.function(opaValueGet)})
	instrs = append(instrs, instruction.SetLocal{Index: c.local(loop.Value)})

	// Loop body.
	nested, err := c.compileBlock(loop.Block)
	if err != nil {
		return nil, err
	}

	instrs = append(instrs, nested...)

	return instrs, nil
}

func (c *Compiler) emitLocals() error {
	c.code.Func.Locals = []module.LocalDeclaration{
		{
			Count: c.localMax + 1,
			Type:  types.I32,
		},
	}
	return nil
}

func (c *Compiler) emitFunctionSection() error {

	var found bool
	var index uint32

	for i, tpe := range c.module.Type.Functions {
		if len(tpe.Params) == 2 && tpe.Params[0] == types.I32 && tpe.Params[1] == types.I32 {
			if len(tpe.Results) == 1 && tpe.Results[0] == types.I32 {
				index = uint32(i)
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("expected to find (int32, int32) -> int32 in type section")
	}

	c.module.Function.TypeIndices = append(c.module.Function.TypeIndices, index)

	var numFuncs int

	for _, imp := range c.module.Import.Imports {
		if imp.Descriptor.Kind() == module.FunctionImportType {
			numFuncs++
		}
	}

	c.evalIndex = uint32(numFuncs + len(c.module.Function.TypeIndices) - 1)

	return nil
}

func (c *Compiler) emitExportSection() error {
	c.module.Export.Exports = append(c.module.Export.Exports, module.Export{
		Name: "eval",
		Descriptor: module.ExportDescriptor{
			Type:  module.FunctionExportType,
			Index: c.evalIndex,
		},
	})
	return nil
}

func (c *Compiler) emitCodeSection() error {
	var buf bytes.Buffer
	if err := encoding.WriteCodeEntry(&buf, c.code); err != nil {
		return err
	}
	c.module.Code.Segments = append(c.module.Code.Segments, module.RawCodeSegment{
		Code: buf.Bytes(),
	})
	return nil
}

func (c *Compiler) emitDataSection() error {
	c.module.Data.Segments = append(c.module.Data.Segments, module.DataSegment{
		Index: 0,
		Offset: module.Expr{
			Instrs: []instruction.Instruction{
				instruction.I32Const{
					Value: c.stringOffset,
				},
			},
		},
		Init: c.stringData,
	})
	return nil
}

func (c *Compiler) stringAddr(index int) int32 {
	return int32(c.stringAddrs[index])
}

func (c *Compiler) local(l ir.Local) uint32 {
	u32 := uint32(l)
	if u32 > c.localMax {
		c.localMax = u32
	}
	return u32
}

func (c *Compiler) function(name string) uint32 {
	index, ok := c.funcs[name]
	if !ok {
		panic(fmt.Sprintf("illegal function name %q", name))
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
