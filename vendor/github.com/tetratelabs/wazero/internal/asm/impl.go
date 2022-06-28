package asm

import (
	"encoding/binary"
	"fmt"
)

// BaseAssemblerImpl includes code common to all architectures.
//
// Note: When possible, add code here instead of in architecture-specific files to reduce drift:
// As this is internal, exporting symbols only to reduce duplication is ok.
type BaseAssemblerImpl struct {
	// SetBranchTargetOnNextNodes holds branch kind instructions (BR, conditional BR, etc.)
	// where we want to set the next coming instruction as the destination of these BR instructions.
	SetBranchTargetOnNextNodes []Node

	// OnGenerateCallbacks holds the callbacks which are called after generating native code.
	OnGenerateCallbacks []func(code []byte) error
}

// SetJumpTargetOnNext implements AssemblerBase.SetJumpTargetOnNext
func (a *BaseAssemblerImpl) SetJumpTargetOnNext(nodes ...Node) {
	a.SetBranchTargetOnNextNodes = append(a.SetBranchTargetOnNextNodes, nodes...)
}

// AddOnGenerateCallBack implements AssemblerBase.AddOnGenerateCallBack
func (a *BaseAssemblerImpl) AddOnGenerateCallBack(cb func([]byte) error) {
	a.OnGenerateCallbacks = append(a.OnGenerateCallbacks, cb)
}

// BuildJumpTable implements AssemblerBase.BuildJumpTable
func (a *BaseAssemblerImpl) BuildJumpTable(table *StaticConst, labelInitialInstructions []Node) {
	a.AddOnGenerateCallBack(func(code []byte) error {
		// Compile the offset table for each target.
		base := labelInitialInstructions[0].OffsetInBinary()
		for i, nop := range labelInitialInstructions {
			if nop.OffsetInBinary()-base >= JumpTableMaximumOffset {
				return fmt.Errorf("too large br_table")
			}
			// We store the offset from the beginning of the L0's initial instruction.
			binary.LittleEndian.PutUint32(code[table.offsetInBinary+uint64(i*4):table.offsetInBinary+uint64((i+1)*4)],
				uint32(nop.OffsetInBinary())-uint32(base))
		}
		return nil
	})
}
