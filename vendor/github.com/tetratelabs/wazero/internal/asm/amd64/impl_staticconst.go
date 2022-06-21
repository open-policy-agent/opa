package amd64

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
)

type constPool struct {
	firstUseOffsetInBinary *asm.NodeOffsetInBinary
	consts                 []asm.StaticConst
	poolSizeInBytes        int

	// offsetFinalizedCallbacks are functions called when the offsets of the
	// constants in the binary have been determined.
	offsetFinalizedCallbacks map[string][]func(offsetOfConstInBinary int)
}

func newConstPool() constPool {
	return constPool{offsetFinalizedCallbacks: map[string][]func(offsetOfConstInBinary int){}}
}

func (p *constPool) addConst(c asm.StaticConst) {
	key := asm.StaticConstKey(c)
	if _, ok := p.offsetFinalizedCallbacks[key]; !ok {
		p.consts = append(p.consts, c)
		p.poolSizeInBytes += len(c)
		p.offsetFinalizedCallbacks[key] = []func(int){}
	}
}

// defaultMaxDisplacementForConstantPool is the maximum displacement allowed for literal move instructions which access
// the constant pool. This is set as 2 ^30 conservatively while the actual limit is 2^31 since we actually allow this
// limit plus max(length(c) for c in the pool) so we must ensure that limit is less than 2^31.
const defaultMaxDisplacementForConstantPool = 1 << 30

func (a *AssemblerImpl) maybeFlushConstants(isEndOfFunction bool) {
	if a.pool.firstUseOffsetInBinary == nil {
		return
	}

	if isEndOfFunction ||
		// If the distance between (the first use in binary) and (end of constant pool) can be larger
		// than MaxDisplacementForConstantPool, we have to emit the constant pool now, otherwise
		// a const might be unreachable by a literal move whose maximum offset is +- 2^31.
		((a.pool.poolSizeInBytes+a.Buf.Len())-int(*a.pool.firstUseOffsetInBinary)) >= a.MaxDisplacementForConstantPool {
		if !isEndOfFunction {
			// Adds the jump instruction to skip the constants if this is not the end of function.
			//
			// TODO: consider NOP padding for this jump, though this rarely happens as most functions should be
			// small enough to fit all consts after the end of function.
			if a.pool.poolSizeInBytes >= math.MaxInt8-2 {
				// long (near-relative) jump: https://www.felixcloutier.com/x86/jmp
				a.Buf.WriteByte(0xe9)
				a.WriteConst(int64(a.pool.poolSizeInBytes), 32)
			} else {
				// short jump: https://www.felixcloutier.com/x86/jmp
				a.Buf.WriteByte(0xeb)
				a.WriteConst(int64(a.pool.poolSizeInBytes), 8)
			}
		}

		for _, c := range a.pool.consts {
			offset := a.Buf.Len()
			a.Buf.Write(c)
			for _, callback := range a.pool.offsetFinalizedCallbacks[asm.StaticConstKey(c)] {
				callback(offset)
			}
		}

		a.pool = newConstPool() // reset
	}
}

func (a *AssemblerImpl) encodeStaticConstToRegister(n *NodeImpl) (err error) {
	a.pool.addConst(n.staticConst)

	dstReg3Bits, rexPrefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldReg)
	if err != nil {
		return err
	}

	var inst []byte // mandatory prefix
	key := asm.StaticConstKey(n.staticConst)
	a.pool.offsetFinalizedCallbacks[key] = append(a.pool.offsetFinalizedCallbacks[key],
		func(offsetOfConstInBinary int) {
			bin := a.Buf.Bytes()
			displacement := offsetOfConstInBinary - int(n.OffsetInBinary()) - len(inst)
			displacementOffsetInInstruction := n.OffsetInBinary() + uint64(len(inst)-4)
			binary.LittleEndian.PutUint32(bin[displacementOffsetInInstruction:], uint32(int32(displacement)))
		})

	nodeOffset := uint64(a.Buf.Len())
	a.pool.firstUseOffsetInBinary = &nodeOffset

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
	modRM := 0b00_000_101 | // Indicate "MOVDQU [RIP + 32bit displacement], DstReg" encoding.
		(dstReg3Bits << 3) // Place the DstReg on ModRM:reg.

	var mandatoryPrefix byte
	var opcodes []byte
	switch n.Instruction {
	case MOVDQU:
		// https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
		mandatoryPrefix = 0xf3
		opcodes = []byte{0x0f, 0x6f}
	case LEAQ:
		// https://www.felixcloutier.com/x86/lea
		rexPrefix |= RexPrefixW
		opcodes = []byte{0x8d}
	case MOVUPD:
		// https://www.felixcloutier.com/x86/movupd
		mandatoryPrefix = 0x66
		opcodes = []byte{0x0f, 0x10}
	default:
		err = errorEncodingUnsupported(n)
		return
	}

	if mandatoryPrefix != 0 {
		inst = append(inst, mandatoryPrefix)
	}
	if rexPrefix != RexPrefixNone {
		inst = append(inst, rexPrefix)
	}

	inst = append(inst, opcodes...)
	inst = append(inst, modRM,
		0x0, 0x0, 0x0, 0x0, // Preserve 4 bytes for displacement.
	)

	a.Buf.Write(inst)
	return
}

// CompileLoadStaticConstToRegister implements Assembler.CompileLoadStaticConstToRegister.
func (a *AssemblerImpl) CompileLoadStaticConstToRegister(instruction asm.Instruction, c asm.StaticConst, dstReg asm.Register) (err error) {
	if len(c)%2 != 0 {
		err = fmt.Errorf("the length of a static constant must be even but was %d", len(c))
		return
	}

	n := a.newNode(instruction, OperandTypesStaticConstToRegister)
	n.DstReg = dstReg
	n.staticConst = c
	return
}
