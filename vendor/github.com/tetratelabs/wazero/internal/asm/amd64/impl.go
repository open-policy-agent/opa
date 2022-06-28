package amd64

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
)

// NodeImpl implements asm.Node for amd64.
type NodeImpl struct {
	// NOTE: fields here are exported for testing with the amd64_debug package.

	Instruction asm.Instruction

	OffsetInBinaryField asm.NodeOffsetInBinary // Field suffix to dodge conflict with OffsetInBinary

	// JumpTarget holds the target node in the linked for the jump-kind instruction.
	JumpTarget *NodeImpl
	Flag       NodeFlag
	// next holds the next node from this node in the assembled linked list.
	Next *NodeImpl

	Types                    OperandTypes
	SrcReg, DstReg           asm.Register
	SrcConst, DstConst       asm.ConstantValue
	SrcMemIndex, DstMemIndex asm.Register
	SrcMemScale, DstMemScale byte

	Arg byte

	// readInstructionAddressBeforeTargetInstruction holds the instruction right before the target of
	// read instruction address instruction. See asm.assemblerBase.CompileReadInstructionAddress.
	readInstructionAddressBeforeTargetInstruction asm.Instruction

	// JumpOrigins hold all the nodes trying to jump into this node. In other words, all the nodes with .JumpTarget == this.
	JumpOrigins map[*NodeImpl]struct{}

	staticConst *asm.StaticConst
}

type NodeFlag byte

const (
	// NodeFlagInitializedForEncoding is always set to indicate that node is already initialized. Notably, this is used to judge
	// whether a jump is backward or forward before encoding.
	NodeFlagInitializedForEncoding NodeFlag = 1 << iota
	NodeFlagBackwardJump
	// NodeFlagShortForwardJump is set to false by default and only used by forward branch jumps, which means .JumpTarget != nil and
	// the target node is encoded after this node. False by default means that we Encode all the jumps with JumpTarget
	// as short jump (i.e. relative signed 8-bit integer offset jump) and try to Encode as small as possible.
	NodeFlagShortForwardJump
)

func (n *NodeImpl) isInitializedForEncoding() bool {
	return n.Flag&NodeFlagInitializedForEncoding != 0
}

func (n *NodeImpl) isJumpNode() bool {
	return n.JumpTarget != nil
}

func (n *NodeImpl) isBackwardJump() bool {
	return n.isJumpNode() && (n.Flag&NodeFlagBackwardJump != 0)
}

func (n *NodeImpl) isForwardJump() bool {
	return n.isJumpNode() && (n.Flag&NodeFlagBackwardJump == 0)
}

func (n *NodeImpl) isForwardShortJump() bool {
	return n.isForwardJump() && n.Flag&NodeFlagShortForwardJump != 0
}

// AssignJumpTarget implements asm.Node.AssignJumpTarget.
func (n *NodeImpl) AssignJumpTarget(target asm.Node) {
	n.JumpTarget = target.(*NodeImpl)
}

// AssignDestinationConstant implements asm.Node.AssignDestinationConstant.
func (n *NodeImpl) AssignDestinationConstant(value asm.ConstantValue) {
	n.DstConst = value
}

// AssignSourceConstant implements asm.Node.AssignSourceConstant.
func (n *NodeImpl) AssignSourceConstant(value asm.ConstantValue) {
	n.SrcConst = value
}

// OffsetInBinary implements asm.Node.OffsetInBinary.
func (n *NodeImpl) OffsetInBinary() asm.NodeOffsetInBinary {
	return n.OffsetInBinaryField
}

// String implements fmt.Stringer.
//
// This is for debugging purpose, and the format is almost same as the AT&T assembly syntax,
// meaning that this should look like "INSTRUCTION ${from}, ${to}" where each operand
// might be embraced by '[]' to represent the memory location.
func (n *NodeImpl) String() (ret string) {
	instName := InstructionName(n.Instruction)
	switch n.Types {
	case OperandTypesNoneToNone:
		ret = instName
	case OperandTypesNoneToRegister:
		ret = fmt.Sprintf("%s %s", instName, RegisterName(n.DstReg))
	case OperandTypesNoneToMemory:
		if n.DstMemIndex != asm.NilRegister {
			ret = fmt.Sprintf("%s [%s + 0x%x + %s*0x%x]", instName,
				RegisterName(n.DstReg), n.DstConst, RegisterName(n.DstMemIndex), n.DstMemScale)
		} else {
			ret = fmt.Sprintf("%s [%s + 0x%x]", instName, RegisterName(n.DstReg), n.DstConst)
		}
	case OperandTypesNoneToBranch:
		ret = fmt.Sprintf("%s {%v}", instName, n.JumpTarget)
	case OperandTypesRegisterToNone:
		ret = fmt.Sprintf("%s %s", instName, RegisterName(n.SrcReg))
	case OperandTypesRegisterToRegister:
		ret = fmt.Sprintf("%s %s, %s", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg))
	case OperandTypesRegisterToMemory:
		if n.DstMemIndex != asm.NilRegister {
			ret = fmt.Sprintf("%s %s, [%s + 0x%x + %s*0x%x]", instName, RegisterName(n.SrcReg),
				RegisterName(n.DstReg), n.DstConst, RegisterName(n.DstMemIndex), n.DstMemScale)
		} else {
			ret = fmt.Sprintf("%s %s, [%s + 0x%x]", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), n.DstConst)
		}
	case OperandTypesRegisterToConst:
		ret = fmt.Sprintf("%s %s, 0x%x", instName, RegisterName(n.SrcReg), n.DstConst)
	case OperandTypesMemoryToRegister:
		if n.SrcMemIndex != asm.NilRegister {
			ret = fmt.Sprintf("%s [%s + %d + %s*0x%x], %s", instName,
				RegisterName(n.SrcReg), n.SrcConst, RegisterName(n.SrcMemIndex), n.SrcMemScale, RegisterName(n.DstReg))
		} else {
			ret = fmt.Sprintf("%s [%s + 0x%x], %s", instName, RegisterName(n.SrcReg), n.SrcConst, RegisterName(n.DstReg))
		}
	case OperandTypesMemoryToConst:
		if n.SrcMemIndex != asm.NilRegister {
			ret = fmt.Sprintf("%s [%s + %d + %s*0x%x], 0x%x", instName,
				RegisterName(n.SrcReg), n.SrcConst, RegisterName(n.SrcMemIndex), n.SrcMemScale, n.DstConst)
		} else {
			ret = fmt.Sprintf("%s [%s + 0x%x], 0x%x", instName, RegisterName(n.SrcReg), n.SrcConst, n.DstConst)
		}
	case OperandTypesConstToMemory:
		if n.DstMemIndex != asm.NilRegister {
			ret = fmt.Sprintf("%s 0x%x, [%s + 0x%x + %s*0x%x]", instName, n.SrcConst,
				RegisterName(n.DstReg), n.DstConst, RegisterName(n.DstMemIndex), n.DstMemScale)
		} else {
			ret = fmt.Sprintf("%s 0x%x, [%s + 0x%x]", instName, n.SrcConst, RegisterName(n.DstReg), n.DstConst)
		}
	case OperandTypesConstToRegister:
		ret = fmt.Sprintf("%s 0x%x, %s", instName, n.SrcConst, RegisterName(n.DstReg))
	}
	return
}

// OperandType represents where an operand is placed for an instruction.
// Note: this is almost the same as obj.AddrType in GO assembler.
type OperandType byte

const (
	OperandTypeNone OperandType = iota
	OperandTypeRegister
	OperandTypeMemory
	OperandTypeConst
	OperandTypeStaticConst
	OperandTypeBranch
)

func (o OperandType) String() (ret string) {
	switch o {
	case OperandTypeNone:
		ret = "none"
	case OperandTypeRegister:
		ret = "register"
	case OperandTypeMemory:
		ret = "memory"
	case OperandTypeConst:
		ret = "const"
	case OperandTypeBranch:
		ret = "branch"
	case OperandTypeStaticConst:
		ret = "static-const"
	}
	return
}

// OperandTypes represents the only combinations of two OperandTypes used by wazero
type OperandTypes struct{ src, dst OperandType }

var (
	OperandTypesNoneToNone            = OperandTypes{OperandTypeNone, OperandTypeNone}
	OperandTypesNoneToRegister        = OperandTypes{OperandTypeNone, OperandTypeRegister}
	OperandTypesNoneToMemory          = OperandTypes{OperandTypeNone, OperandTypeMemory}
	OperandTypesNoneToBranch          = OperandTypes{OperandTypeNone, OperandTypeBranch}
	OperandTypesRegisterToNone        = OperandTypes{OperandTypeRegister, OperandTypeNone}
	OperandTypesRegisterToRegister    = OperandTypes{OperandTypeRegister, OperandTypeRegister}
	OperandTypesRegisterToMemory      = OperandTypes{OperandTypeRegister, OperandTypeMemory}
	OperandTypesRegisterToConst       = OperandTypes{OperandTypeRegister, OperandTypeConst}
	OperandTypesMemoryToRegister      = OperandTypes{OperandTypeMemory, OperandTypeRegister}
	OperandTypesMemoryToConst         = OperandTypes{OperandTypeMemory, OperandTypeConst}
	OperandTypesConstToRegister       = OperandTypes{OperandTypeConst, OperandTypeRegister}
	OperandTypesConstToMemory         = OperandTypes{OperandTypeConst, OperandTypeMemory}
	OperandTypesStaticConstToRegister = OperandTypes{OperandTypeStaticConst, OperandTypeRegister}
	OperandTypesRegisterToStaticConst = OperandTypes{OperandTypeRegister, OperandTypeStaticConst}
)

// String implements fmt.Stringer
func (o OperandTypes) String() string {
	return fmt.Sprintf("from:%s,to:%s", o.src, o.dst)
}

// AssemblerImpl implements Assembler.
type AssemblerImpl struct {
	asm.BaseAssemblerImpl
	EnablePadding   bool
	Root, Current   *NodeImpl
	nodeCount       int
	Buf             *bytes.Buffer
	ForceReAssemble bool
	// MaxDisplacementForConstantPool is fixed to defaultMaxDisplacementForConstantPool
	// but have it as a field here for testability.
	MaxDisplacementForConstantPool int

	pool *asm.StaticConstPool
}

// compile-time check to ensure AssemblerImpl implements Assembler.
var _ Assembler = &AssemblerImpl{}

func NewAssemblerImpl() *AssemblerImpl {
	return &AssemblerImpl{Buf: bytes.NewBuffer(nil), EnablePadding: true, pool: asm.NewStaticConstPool(),
		MaxDisplacementForConstantPool: defaultMaxDisplacementForConstantPool}
}

// newNode creates a new Node and appends it into the linked list.
func (a *AssemblerImpl) newNode(instruction asm.Instruction, types OperandTypes) *NodeImpl {
	n := &NodeImpl{
		Instruction: instruction,
		Next:        nil,
		Types:       types,
		JumpOrigins: map[*NodeImpl]struct{}{},
	}
	a.addNode(n)
	a.nodeCount++
	return n
}

// addNode appends the new node into the linked list.
func (a *AssemblerImpl) addNode(node *NodeImpl) {
	if a.Root == nil {
		a.Root = node
		a.Current = node
	} else {
		parent := a.Current
		parent.Next = node
		a.Current = node
	}

	for _, o := range a.SetBranchTargetOnNextNodes {
		origin := o.(*NodeImpl)
		origin.JumpTarget = node
	}
	a.SetBranchTargetOnNextNodes = nil
}

// EncodeNode encodes the given node into writer.
func (a *AssemblerImpl) EncodeNode(n *NodeImpl) (err error) {
	switch n.Types {
	case OperandTypesNoneToNone:
		err = a.encodeNoneToNone(n)
	case OperandTypesNoneToRegister:
		err = a.EncodeNoneToRegister(n)
	case OperandTypesNoneToMemory:
		err = a.EncodeNoneToMemory(n)
	case OperandTypesNoneToBranch:
		// Branching operand can be encoded as relative jumps.
		err = a.EncodeRelativeJump(n)
	case OperandTypesRegisterToNone:
		err = a.EncodeRegisterToNone(n)
	case OperandTypesRegisterToRegister:
		err = a.EncodeRegisterToRegister(n)
	case OperandTypesRegisterToMemory:
		err = a.EncodeRegisterToMemory(n)
	case OperandTypesRegisterToConst:
		err = a.EncodeRegisterToConst(n)
	case OperandTypesMemoryToRegister:
		err = a.EncodeMemoryToRegister(n)
	case OperandTypesConstToRegister:
		err = a.EncodeConstToRegister(n)
	case OperandTypesConstToMemory:
		err = a.EncodeConstToMemory(n)
	case OperandTypesMemoryToConst:
		err = a.EncodeMemoryToConst(n)
	case OperandTypesStaticConstToRegister:
		err = a.encodeStaticConstToRegister(n)
	case OperandTypesRegisterToStaticConst:
		err = a.encodeRegisterToStaticConst(n)
	default:
		err = fmt.Errorf("encoder undefined for [%s] operand type", n.Types)
	}
	return
}

// Assemble implements asm.AssemblerBase
func (a *AssemblerImpl) Assemble() ([]byte, error) {
	a.InitializeNodesForEncoding()

	// Continue encoding until we are not forced to re-assemble which happens when
	// a short relative jump ends up the offset larger than 8-bit length.
	for {
		err := a.Encode()
		if err != nil {
			return nil, err
		}

		if !a.ForceReAssemble {
			break
		} else {
			// We reset the length of buffer but don't delete the underlying slice since
			// the binary size will roughly the same after reassemble.
			a.Buf.Reset()
			// Reset the re-assemble Flag in order to avoid the infinite loop!
			a.ForceReAssemble = false
		}
	}

	code := a.Buf.Bytes()
	for _, cb := range a.OnGenerateCallbacks {
		if err := cb(code); err != nil {
			return nil, err
		}
	}
	return code, nil
}

// InitializeNodesForEncoding initializes NodeImpl.Flag and determine all the jumps
// are forward or backward jump.
func (a *AssemblerImpl) InitializeNodesForEncoding() {
	for n := a.Root; n != nil; n = n.Next {
		n.Flag |= NodeFlagInitializedForEncoding
		if target := n.JumpTarget; target != nil {
			if target.isInitializedForEncoding() {
				// This means the target exists behind.
				n.Flag |= NodeFlagBackwardJump
			} else {
				// Otherwise, this is forward jump.
				// We start with assuming that the jump can be short (8-bit displacement).
				// If it doens't fit, we change this Flag in resolveRelativeForwardJump.
				n.Flag |= NodeFlagShortForwardJump
			}
		}
	}

	// Roughly allocate the buffer by assuming an instruction has 5-bytes length on average.
	a.Buf.Grow(a.nodeCount * 5)
}

func (a *AssemblerImpl) Encode() (err error) {
	for n := a.Root; n != nil; n = n.Next {
		// If an instruction needs NOP padding, we do so before encoding it.
		// https://www.intel.com/content/dam/support/us/en/documents/processors/mitigations-jump-conditional-code-erratum.pdf
		if a.EnablePadding {
			if err = a.maybeNOPPadding(n); err != nil {
				return
			}
		}

		// After the padding, we can finalize the offset of this instruction in the binary.
		n.OffsetInBinaryField = uint64(a.Buf.Len())

		if err = a.EncodeNode(n); err != nil {
			return
		}

		err = a.ResolveForwardRelativeJumps(n)
		if err != nil {
			err = fmt.Errorf("invalid relative forward jumps: %w", err)
			break
		}

		a.maybeFlushConstants(n.Next == nil)
	}
	return
}

// maybeNOPPadding maybe appends NOP instructions before the node `n`.
// This is necessary to avoid Intel's jump erratum:
// https://www.intel.com/content/dam/support/us/en/documents/processors/mitigations-jump-conditional-code-erratum.pdf
func (a *AssemblerImpl) maybeNOPPadding(n *NodeImpl) (err error) {
	var instructionLen int32

	// See in Section 2.1 in for when we have to pad NOP.
	// https://www.intel.com/content/dam/support/us/en/documents/processors/mitigations-jump-conditional-code-erratum.pdf
	switch n.Instruction {
	case RET, JMP, JCC, JCS, JEQ, JGE, JGT, JHI, JLE, JLS, JLT, JMI, JNE, JPC, JPS:
		// In order to know the instruction length before writing into the binary,
		// we try encoding it with the temporary buffer.
		saved := a.Buf
		a.Buf = bytes.NewBuffer(nil)

		// Assign the temporary offset which may or may not be correct depending on the padding decision.
		n.OffsetInBinaryField = uint64(saved.Len())

		// Encode the node and get the instruction length.
		if err = a.EncodeNode(n); err != nil {
			return
		}
		instructionLen = int32(a.Buf.Len())

		// Revert the temporary buffer.
		a.Buf = saved
	case // The possible fused jump instructions if the next node is a conditional jump instruction.
		CMPL, CMPQ, TESTL, TESTQ, ADDL, ADDQ, SUBL, SUBQ, ANDL, ANDQ, INCQ, DECQ:
		instructionLen, err = a.fusedInstructionLength(n)
		if err != nil {
			return err
		}
	}

	if instructionLen == 0 {
		return
	}

	const boundaryInBytes int32 = 32
	const mask int32 = boundaryInBytes - 1

	var padNum int
	currentPos := int32(a.Buf.Len())
	if used := currentPos & mask; used+instructionLen >= boundaryInBytes {
		padNum = int(boundaryInBytes - used)
	}

	a.padNOP(padNum)
	return
}

// fusedInstructionLength returns the length of "macro fused instruction" if the
// instruction sequence starting from `n` can be fused by processor. Otherwise,
// returns zero.
func (a *AssemblerImpl) fusedInstructionLength(n *NodeImpl) (ret int32, err error) {
	// Find the next non-NOP instruction.
	next := n.Next
	for ; next != nil && next.Instruction == NOP; next = next.Next {
	}

	if next == nil {
		return
	}

	inst, jmpInst := n.Instruction, next.Instruction

	if !(jmpInst == JCC || jmpInst == JCS || jmpInst == JEQ || jmpInst == JGE || jmpInst == JGT ||
		jmpInst == JHI || jmpInst == JLE || jmpInst == JLS || jmpInst == JLT || jmpInst == JMI ||
		jmpInst == JNE || jmpInst == JPC || jmpInst == JPS) {
		// If the next instruction is not jump kind, the instruction will not be fused.
		return
	}

	// How to determine whether the instruction can be fused is described in
	// Section 3.4.2.2 of "Intel Optimization Manual":
	// https://www.intel.com/content/dam/doc/manual/64-ia-32-architectures-optimization-manual.pdf
	isTest := inst == TESTL || inst == TESTQ
	isCmp := inst == CMPQ || inst == CMPL
	isTestCmp := isTest || isCmp
	if isTestCmp && ((n.Types.src == OperandTypeMemory && n.Types.dst == OperandTypeConst) ||
		(n.Types.src == OperandTypeConst && n.Types.dst == OperandTypeMemory)) {
		// The manual says: "CMP and TEST can not be fused when comparing MEM-IMM".
		return
	}

	// Implement the decision according to the table 3-1 in the manual.
	isAnd := inst == ANDL || inst == ANDQ
	if !isTest && !isAnd {
		if jmpInst == JMI || jmpInst == JPL || jmpInst == JPS || jmpInst == JPC {
			// These jumps are only fused for TEST or AND.
			return
		}
		isAdd := inst == ADDL || inst == ADDQ
		isSub := inst == SUBL || inst == SUBQ
		if !isCmp && !isAdd && !isSub {
			if jmpInst == JCS || jmpInst == JCC || jmpInst == JHI || jmpInst == JLS {
				// Thses jumpst are only fused for TEST, AND, CMP, ADD, or SUB.
				return
			}
		}
	}

	// Now the instruction is ensured to be fused by the processor.
	// In order to know the fused instruction length before writing into the binary,
	// we try encoding it with the temporary buffer.
	saved := a.Buf
	savedLen := uint64(saved.Len())
	a.Buf = bytes.NewBuffer(nil)

	for _, fused := range []*NodeImpl{n, next} {
		// Assign the temporary offset which may or may not be correct depending on the padding decision.
		fused.OffsetInBinaryField = savedLen + uint64(a.Buf.Len())

		// Encode the node into the temporary buffer.
		if err = a.EncodeNode(fused); err != nil {
			return
		}
	}

	ret = int32(a.Buf.Len())

	// Revert the temporary buffer.
	a.Buf = saved
	return
}

// nopOpcodes is the multi byte NOP instructions table derived from section 5.8 "Code Padding with Operand-Size Override and Multibyte NOP"
// in "AMD Software Optimization Guide for AMD Family 15h Processors" https://www.amd.com/system/files/TechDocs/47414_15h_sw_opt_guide.pdf
//
// Note: We use up to 9 bytes NOP variant to line our implementation with Go's assembler.
// TODO: After golang-asm removal, add 9, 10 and 11 bytes variants.
var nopOpcodes = [][9]byte{
	{0x90},
	{0x66, 0x90},
	{0x0f, 0x1f, 0x00},
	{0x0f, 0x1f, 0x40, 0x00},
	{0x0f, 0x1f, 0x44, 0x00, 0x00},
	{0x66, 0x0f, 0x1f, 0x44, 0x00, 0x00},
	{0x0f, 0x1f, 0x80, 0x00, 0x00, 0x00, 0x00},
	{0x0f, 0x1f, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x66, 0x0f, 0x1f, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
}

func (a *AssemblerImpl) padNOP(num int) {
	for num > 0 {
		singleNopNum := num
		if singleNopNum > len(nopOpcodes) {
			singleNopNum = len(nopOpcodes)
		}
		a.Buf.Write(nopOpcodes[singleNopNum-1][:singleNopNum])
		num -= singleNopNum
	}
}

// CompileStandAlone implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileStandAlone(instruction asm.Instruction) asm.Node {
	return a.newNode(instruction, OperandTypesNoneToNone)
}

// CompileConstToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileConstToRegister(
	instruction asm.Instruction,
	value asm.ConstantValue,
	destinationReg asm.Register,
) (inst asm.Node) {
	n := a.newNode(instruction, OperandTypesConstToRegister)
	n.SrcConst = value
	n.DstReg = destinationReg
	return n
}

// CompileRegisterToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileRegisterToRegister(instruction asm.Instruction, from, to asm.Register) {
	n := a.newNode(instruction, OperandTypesRegisterToRegister)
	n.SrcReg = from
	n.DstReg = to
}

// CompileMemoryToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileMemoryToRegister(
	instruction asm.Instruction,
	sourceBaseReg asm.Register,
	sourceOffsetConst asm.ConstantValue,
	destinationReg asm.Register,
) {
	n := a.newNode(instruction, OperandTypesMemoryToRegister)
	n.SrcReg = sourceBaseReg
	n.SrcConst = sourceOffsetConst
	n.DstReg = destinationReg
}

// CompileRegisterToMemory implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileRegisterToMemory(
	instruction asm.Instruction,
	sourceRegister, destinationBaseRegister asm.Register,
	destinationOffsetConst asm.ConstantValue,
) {
	n := a.newNode(instruction, OperandTypesRegisterToMemory)
	n.SrcReg = sourceRegister
	n.DstReg = destinationBaseRegister
	n.DstConst = destinationOffsetConst
}

// CompileJump implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileJump(jmpInstruction asm.Instruction) asm.Node {
	return a.newNode(jmpInstruction, OperandTypesNoneToBranch)
}

// CompileJumpToMemory implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileJumpToMemory(
	jmpInstruction asm.Instruction,
	baseReg asm.Register,
	offset asm.ConstantValue,
) {
	n := a.newNode(jmpInstruction, OperandTypesNoneToMemory)
	n.DstReg = baseReg
	n.DstConst = offset
}

// CompileJumpToRegister implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileJumpToRegister(jmpInstruction asm.Instruction, reg asm.Register) {
	n := a.newNode(jmpInstruction, OperandTypesNoneToRegister)
	n.DstReg = reg
}

// CompileReadInstructionAddress implements the same method as documented on asm.AssemblerBase.
func (a *AssemblerImpl) CompileReadInstructionAddress(
	destinationRegister asm.Register,
	beforeAcquisitionTargetInstruction asm.Instruction,
) {
	n := a.newNode(LEAQ, OperandTypesMemoryToRegister)
	n.DstReg = destinationRegister
	n.readInstructionAddressBeforeTargetInstruction = beforeAcquisitionTargetInstruction
}

// CompileRegisterToRegisterWithArg implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileRegisterToRegisterWithArg(
	instruction asm.Instruction,
	from, to asm.Register,
	arg byte,
) {
	n := a.newNode(instruction, OperandTypesRegisterToRegister)
	n.SrcReg = from
	n.DstReg = to
	n.Arg = arg
}

// CompileMemoryWithIndexToRegister implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileMemoryWithIndexToRegister(
	instruction asm.Instruction,
	srcBaseReg asm.Register,
	srcOffsetConst asm.ConstantValue,
	srcIndex asm.Register,
	srcScale int16,
	dstReg asm.Register,
) {
	n := a.newNode(instruction, OperandTypesMemoryToRegister)
	n.SrcReg = srcBaseReg
	n.SrcConst = srcOffsetConst
	n.SrcMemIndex = srcIndex
	n.SrcMemScale = byte(srcScale)
	n.DstReg = dstReg
}

// CompileMemoryWithIndexAndArgToRegister implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileMemoryWithIndexAndArgToRegister(
	instruction asm.Instruction,
	srcBaseReg asm.Register,
	srcOffsetConst asm.ConstantValue,
	srcIndex asm.Register,
	srcScale int16,
	dstReg asm.Register,
	arg byte,
) {
	n := a.newNode(instruction, OperandTypesMemoryToRegister)
	n.SrcReg = srcBaseReg
	n.SrcConst = srcOffsetConst
	n.SrcMemIndex = srcIndex
	n.SrcMemScale = byte(srcScale)
	n.DstReg = dstReg
	n.Arg = arg
}

// CompileRegisterToMemoryWithIndex implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileRegisterToMemoryWithIndex(
	instruction asm.Instruction,
	srcReg, dstBaseReg asm.Register,
	dstOffsetConst asm.ConstantValue,
	dstIndex asm.Register,
	dstScale int16,
) {
	n := a.newNode(instruction, OperandTypesRegisterToMemory)
	n.SrcReg = srcReg
	n.DstReg = dstBaseReg
	n.DstConst = dstOffsetConst
	n.DstMemIndex = dstIndex
	n.DstMemScale = byte(dstScale)
}

// CompileRegisterToMemoryWithIndexAndArg implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileRegisterToMemoryWithIndexAndArg(
	instruction asm.Instruction,
	srcReg, dstBaseReg asm.Register,
	dstOffsetConst asm.ConstantValue,
	dstIndex asm.Register,
	dstScale int16,
	arg byte,
) {
	n := a.newNode(instruction, OperandTypesRegisterToMemory)
	n.SrcReg = srcReg
	n.DstReg = dstBaseReg
	n.DstConst = dstOffsetConst
	n.DstMemIndex = dstIndex
	n.DstMemScale = byte(dstScale)
	n.Arg = arg
}

// CompileRegisterToConst implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileRegisterToConst(
	instruction asm.Instruction,
	srcRegister asm.Register,
	value asm.ConstantValue,
) asm.Node {
	n := a.newNode(instruction, OperandTypesRegisterToConst)
	n.SrcReg = srcRegister
	n.DstConst = value
	return n
}

// CompileRegisterToNone implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileRegisterToNone(instruction asm.Instruction, register asm.Register) {
	n := a.newNode(instruction, OperandTypesRegisterToNone)
	n.SrcReg = register
}

// CompileNoneToRegister implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileNoneToRegister(instruction asm.Instruction, register asm.Register) {
	n := a.newNode(instruction, OperandTypesNoneToRegister)
	n.DstReg = register
}

// CompileNoneToMemory implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileNoneToMemory(
	instruction asm.Instruction,
	baseReg asm.Register,
	offset asm.ConstantValue,
) {
	n := a.newNode(instruction, OperandTypesNoneToMemory)
	n.DstReg = baseReg
	n.DstConst = offset
}

// CompileConstToMemory implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileConstToMemory(
	instruction asm.Instruction,
	value asm.ConstantValue,
	dstbaseReg asm.Register,
	dstOffset asm.ConstantValue,
) asm.Node {
	n := a.newNode(instruction, OperandTypesConstToMemory)
	n.SrcConst = value
	n.DstReg = dstbaseReg
	n.DstConst = dstOffset
	return n
}

// CompileMemoryToConst implements the same method as documented on amd64.Assembler.
func (a *AssemblerImpl) CompileMemoryToConst(
	instruction asm.Instruction,
	srcBaseReg asm.Register,
	srcOffset, value asm.ConstantValue,
) asm.Node {
	n := a.newNode(instruction, OperandTypesMemoryToConst)
	n.SrcReg = srcBaseReg
	n.SrcConst = srcOffset
	n.DstConst = value
	return n
}

func errorEncodingUnsupported(n *NodeImpl) error {
	return fmt.Errorf("%s is unsupported for %s type", InstructionName(n.Instruction), n.Types)
}

func (a *AssemblerImpl) encodeNoneToNone(n *NodeImpl) (err error) {
	switch n.Instruction {
	case CDQ:
		// https://www.felixcloutier.com/x86/cwd:cdq:cqo
		err = a.Buf.WriteByte(0x99)
	case CQO:
		// https://www.felixcloutier.com/x86/cwd:cdq:cqo
		_, err = a.Buf.Write([]byte{RexPrefixW, 0x99})
	case NOP:
		// Simply optimize out the NOP instructions.
	case RET:
		// https://www.felixcloutier.com/x86/ret
		err = a.Buf.WriteByte(0xc3)
	case UD2:
		// https://mudongliang.github.io/x86/html/file_module_x86_id_318.html
		_, err = a.Buf.Write([]byte{0x0f, 0x0b})
	default:
		err = errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) EncodeNoneToRegister(n *NodeImpl) (err error) {
	regBits, prefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldRM)
	if err != nil {
		return err
	}

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
	modRM := 0b11_000_000 | // Specifying that opeand is register.
		regBits
	if n.Instruction == JMP {
		// JMP's opcode is defined as "FF /4" meaning that we have to have "4"
		// in 4-6th bits in the ModRM byte. https://www.felixcloutier.com/x86/jmp
		modRM |= 0b00_100_000
	} else if n.Instruction == NEGQ {
		prefix |= RexPrefixW
		modRM |= 0b00_011_000
	} else if n.Instruction == INCQ {
		prefix |= RexPrefixW
	} else if n.Instruction == DECQ {
		prefix |= RexPrefixW
		modRM |= 0b00_001_000
	} else {
		if RegSP <= n.DstReg && n.DstReg <= RegDI {
			// If the destination is one byte length register, we need to have the default prefix.
			// https: //wiki.osdev.org/X86-64_Instruction_Encoding#Registers
			prefix |= RexPrefixDefault
		}
	}

	if prefix != RexPrefixNone {
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#Encoding
		if err = a.Buf.WriteByte(prefix); err != nil {
			return
		}
	}

	switch n.Instruction {
	case JMP:
		// https://www.felixcloutier.com/x86/jmp
		_, err = a.Buf.Write([]byte{0xff, modRM})
	case SETCC:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x93, modRM})
	case SETCS:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x92, modRM})
	case SETEQ:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x94, modRM})
	case SETGE:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9d, modRM})
	case SETGT:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9f, modRM})
	case SETHI:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x97, modRM})
	case SETLE:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9e, modRM})
	case SETLS:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x96, modRM})
	case SETLT:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9c, modRM})
	case SETNE:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x95, modRM})
	case SETPC:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9b, modRM})
	case SETPS:
		// https://www.felixcloutier.com/x86/setcc
		_, err = a.Buf.Write([]byte{0x0f, 0x9a, modRM})
	case NEGQ:
		// https://www.felixcloutier.com/x86/neg
		_, err = a.Buf.Write([]byte{0xf7, modRM})
	case INCQ:
		// https://www.felixcloutier.com/x86/inc
		_, err = a.Buf.Write([]byte{0xff, modRM})
	case DECQ:
		// https://www.felixcloutier.com/x86/dec
		_, err = a.Buf.Write([]byte{0xff, modRM})
	default:
		err = errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) EncodeNoneToMemory(n *NodeImpl) (err error) {
	RexPrefix, modRM, sbi, displacementWidth, err := n.GetMemoryLocation()
	if err != nil {
		return err
	}

	var opcode byte
	switch n.Instruction {
	case INCQ:
		// https://www.felixcloutier.com/x86/inc
		RexPrefix |= RexPrefixW
		opcode = 0xff
	case DECQ:
		// https://www.felixcloutier.com/x86/dec
		RexPrefix |= RexPrefixW
		modRM |= 0b00_001_000 // DEC needs "/1" extension in ModRM.
		opcode = 0xff
	case JMP:
		// https://www.felixcloutier.com/x86/jmp
		modRM |= 0b00_100_000 // JMP needs "/4" extension in ModRM.
		opcode = 0xff
	default:
		return errorEncodingUnsupported(n)
	}

	if RexPrefix != RexPrefixNone {
		a.Buf.WriteByte(RexPrefix)
	}

	a.Buf.Write([]byte{opcode, modRM})

	if sbi != nil {
		a.Buf.WriteByte(*sbi)
	}

	if displacementWidth != 0 {
		a.WriteConst(n.DstConst, displacementWidth)
	}
	return
}

type relativeJumpOpcode struct{ short, long []byte }

func (o relativeJumpOpcode) instructionLen(short bool) int64 {
	if short {
		return int64(len(o.short)) + 1 // 1 byte = 8 bit offset
	} else {
		return int64(len(o.long)) + 4 // 4 byte = 32 bit offset
	}
}

var relativeJumpOpcodes = map[asm.Instruction]relativeJumpOpcode{
	// https://www.felixcloutier.com/x86/jcc
	JCC: {short: []byte{0x73}, long: []byte{0x0f, 0x83}},
	JCS: {short: []byte{0x72}, long: []byte{0x0f, 0x82}},
	JEQ: {short: []byte{0x74}, long: []byte{0x0f, 0x84}},
	JGE: {short: []byte{0x7d}, long: []byte{0x0f, 0x8d}},
	JGT: {short: []byte{0x7f}, long: []byte{0x0f, 0x8f}},
	JHI: {short: []byte{0x77}, long: []byte{0x0f, 0x87}},
	JLE: {short: []byte{0x7e}, long: []byte{0x0f, 0x8e}},
	JLS: {short: []byte{0x76}, long: []byte{0x0f, 0x86}},
	JLT: {short: []byte{0x7c}, long: []byte{0x0f, 0x8c}},
	JMI: {short: []byte{0x78}, long: []byte{0x0f, 0x88}},
	JPL: {short: []byte{0x79}, long: []byte{0x0f, 0x89}},
	JNE: {short: []byte{0x75}, long: []byte{0x0f, 0x85}},
	JPC: {short: []byte{0x7b}, long: []byte{0x0f, 0x8b}},
	JPS: {short: []byte{0x7a}, long: []byte{0x0f, 0x8a}},
	// https://www.felixcloutier.com/x86/jmp
	JMP: {short: []byte{0xeb}, long: []byte{0xe9}},
}

func (a *AssemblerImpl) ResolveForwardRelativeJumps(target *NodeImpl) (err error) {
	offsetInBinary := int64(target.OffsetInBinary())
	for origin := range target.JumpOrigins {
		shortJump := origin.isForwardShortJump()
		op := relativeJumpOpcodes[origin.Instruction]
		instructionLen := op.instructionLen(shortJump)

		// Calculate the offset from the EIP (at the time of executing this jump instruction)
		// to the target instruction. This value is always >= 0 as here we only handle forward jumps.
		offset := offsetInBinary - (int64(origin.OffsetInBinary()) + instructionLen)
		if shortJump {
			if offset > math.MaxInt8 {
				// This forces reassemble in the outer loop inside AssemblerImpl.Assemble().
				a.ForceReAssemble = true
				// From the next reAssemble phases, this forward jump will be encoded long jump and
				// allocate 32-bit offset bytes by default. This means that this `origin` node
				// will always enter the "long jump offset encoding" block below
				origin.Flag ^= NodeFlagShortForwardJump
			} else {
				a.Buf.Bytes()[origin.OffsetInBinary()+uint64(instructionLen)-1] = byte(offset)
			}
		} else { // long jump offset encoding.
			if offset > math.MaxInt32 {
				return fmt.Errorf("too large jump offset %d for encoding %s", offset, InstructionName(origin.Instruction))
			}
			binary.LittleEndian.PutUint32(a.Buf.Bytes()[origin.OffsetInBinary()+uint64(instructionLen)-4:], uint32(offset))
		}
	}
	return nil
}

func (a *AssemblerImpl) EncodeRelativeJump(n *NodeImpl) (err error) {
	if n.JumpTarget == nil {
		err = fmt.Errorf("jump target must not be nil for relative %s", InstructionName(n.Instruction))
		return
	}

	op, ok := relativeJumpOpcodes[n.Instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}

	var isShortJump bool
	// offsetOfEIP means the offset of EIP register at the time of executing this jump instruction.
	// Relative jump instructions can be encoded with the signed 8-bit or 32-bit integer offsets from the EIP.
	var offsetOfEIP int64 = 0 // We set zero and resolve later once the target instruction is encoded for forward jumps
	if n.isBackwardJump() {
		// If this is the backward jump, we can calculate the exact offset now.
		offsetOfJumpInstruction := int64(n.JumpTarget.OffsetInBinary()) - int64(n.OffsetInBinary())
		isShortJump = offsetOfJumpInstruction-2 >= math.MinInt8
		offsetOfEIP = offsetOfJumpInstruction - op.instructionLen(isShortJump)
	} else {
		// For forward jumps, we resolve the offset when we Encode the target node. See AssemblerImpl.ResolveForwardRelativeJumps.
		n.JumpTarget.JumpOrigins[n] = struct{}{}
		isShortJump = n.isForwardShortJump()
	}

	if offsetOfEIP < math.MinInt32 { // offsetOfEIP is always <= 0 as we don't calculate it for forward jump here.
		return fmt.Errorf("too large jump offset %d for encoding %s", offsetOfEIP, InstructionName(n.Instruction))
	}

	if isShortJump {
		a.Buf.Write(op.short)
		a.WriteConst(offsetOfEIP, 8)
	} else {
		a.Buf.Write(op.long)
		a.WriteConst(offsetOfEIP, 32)
	}
	return
}

func (a *AssemblerImpl) EncodeRegisterToNone(n *NodeImpl) (err error) {
	regBits, prefix, err := register3bits(n.SrcReg, registerSpecifierPositionModRMFieldRM)
	if err != nil {
		return err
	}

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
	modRM := 0b11_000_000 | // Specifying that opeand is register.
		regBits

	var opcode byte
	switch n.Instruction {
	case DIVL:
		// https://www.felixcloutier.com/x86/div
		modRM |= 0b00_110_000
		opcode = 0xf7
	case DIVQ:
		// https://www.felixcloutier.com/x86/div
		prefix |= RexPrefixW
		modRM |= 0b00_110_000
		opcode = 0xf7
	case IDIVL:
		// https://www.felixcloutier.com/x86/idiv
		modRM |= 0b00_111_000
		opcode = 0xf7
	case IDIVQ:
		// https://www.felixcloutier.com/x86/idiv
		prefix |= RexPrefixW
		modRM |= 0b00_111_000
		opcode = 0xf7
	case MULL:
		// https://www.felixcloutier.com/x86/mul
		modRM |= 0b00_100_000
		opcode = 0xf7
	case MULQ:
		// https://www.felixcloutier.com/x86/mul
		prefix |= RexPrefixW
		modRM |= 0b00_100_000
		opcode = 0xf7
	default:
		err = errorEncodingUnsupported(n)
	}

	if prefix != RexPrefixNone {
		a.Buf.WriteByte(prefix)
	}

	a.Buf.Write([]byte{opcode, modRM})
	return
}

var registerToRegisterOpcode = map[asm.Instruction]struct {
	opcode                           []byte
	rPrefix                          RexPrefix
	mandatoryPrefix                  byte
	srcOnModRMReg                    bool
	isSrc8bit                        bool
	needArg                          bool
	requireSrcFloat, requireDstFloat bool
}{
	// https://www.felixcloutier.com/x86/add
	ADDL: {opcode: []byte{0x1}, srcOnModRMReg: true},
	ADDQ: {opcode: []byte{0x1}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/and
	ANDL: {opcode: []byte{0x21}, srcOnModRMReg: true},
	ANDQ: {opcode: []byte{0x21}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/cmp
	CMPL: {opcode: []byte{0x39}},
	CMPQ: {opcode: []byte{0x39}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/cmovcc
	CMOVQCS: {opcode: []byte{0x0f, 0x42}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/addsd
	ADDSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x58}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/addss
	ADDSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x58}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/addpd
	ANDPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x54}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/addps
	ANDPS: {opcode: []byte{0x0f, 0x54}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/bsr
	BSRL: {opcode: []byte{0xf, 0xbd}},
	BSRQ: {opcode: []byte{0xf, 0xbd}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/comisd
	COMISD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x2f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/comiss
	COMISS: {opcode: []byte{0x0f, 0x2f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtsd2ss
	CVTSD2SS: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x5a}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtsi2sd
	CVTSL2SD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x2a}, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtsi2sd
	CVTSQ2SD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x2a}, rPrefix: RexPrefixW, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtsi2ss
	CVTSL2SS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x2a}, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtsi2ss
	CVTSQ2SS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x2a}, rPrefix: RexPrefixW, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtss2sd
	CVTSS2SD: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5a}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvttsd2si
	CVTTSD2SL: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x2c}, requireSrcFloat: true},
	CVTTSD2SQ: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x2c}, rPrefix: RexPrefixW, requireSrcFloat: true},
	// https://www.felixcloutier.com/x86/cvttss2si
	CVTTSS2SL: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x2c}, requireSrcFloat: true},
	CVTTSS2SQ: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x2c}, rPrefix: RexPrefixW, requireSrcFloat: true},
	// https://www.felixcloutier.com/x86/divsd
	DIVSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x5e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/divss
	DIVSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/lzcnt
	LZCNTL: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xbd}},
	LZCNTQ: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xbd}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/maxsd
	MAXSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x5f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/maxss
	MAXSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/minsd
	MINSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x5d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/minss
	MINSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/movsx:movsxd
	MOVBLSX: {opcode: []byte{0x0f, 0xbe}, isSrc8bit: true},
	// https://www.felixcloutier.com/x86/movzx
	MOVBLZX: {opcode: []byte{0x0f, 0xb6}, isSrc8bit: true},
	// https://www.felixcloutier.com/x86/movzx
	MOVWLZX: {opcode: []byte{0x0f, 0xb7}, isSrc8bit: true},
	// https://www.felixcloutier.com/x86/movsx:movsxd
	MOVBQSX: {opcode: []byte{0x0f, 0xbe}, rPrefix: RexPrefixW, isSrc8bit: true},
	// https://www.felixcloutier.com/x86/movsx:movsxd
	MOVLQSX: {opcode: []byte{0x63}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/movsx:movsxd
	MOVWQSX: {opcode: []byte{0x0f, 0xbf}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/movsx:movsxd
	MOVWLSX: {opcode: []byte{0x0f, 0xbf}},
	// https://www.felixcloutier.com/x86/mulss
	MULSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x59}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/mulsd
	MULSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x59}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/or
	ORL: {opcode: []byte{0x09}, srcOnModRMReg: true},
	ORQ: {opcode: []byte{0x09}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/orpd
	ORPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x56}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/orps
	ORPS: {opcode: []byte{0x0f, 0x56}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/popcnt
	POPCNTL: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xb8}},
	POPCNTQ: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xb8}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/roundss
	ROUNDSS: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x0a}, needArg: true, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/roundsd
	ROUNDSD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x0b}, needArg: true, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/sqrtss
	SQRTSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x51}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/sqrtsd
	SQRTSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x51}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/sub
	SUBL: {opcode: []byte{0x29}, srcOnModRMReg: true},
	SUBQ: {opcode: []byte{0x29}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/subss
	SUBSS: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/subsd
	SUBSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x5c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/test
	TESTL: {opcode: []byte{0x85}, srcOnModRMReg: true},
	TESTQ: {opcode: []byte{0x85}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/tzcnt
	TZCNTL: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xbc}},
	TZCNTQ: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xbc}, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/ucomisd
	UCOMISD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x2e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/ucomiss
	UCOMISS: {opcode: []byte{0x0f, 0x2e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/xor
	XORL: {opcode: []byte{0x31}, srcOnModRMReg: true},
	XORQ: {opcode: []byte{0x31}, rPrefix: RexPrefixW, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/xorpd
	XORPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x57}, requireSrcFloat: true, requireDstFloat: true},
	XORPS: {opcode: []byte{0x0f, 0x57}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x20}, requireSrcFloat: false, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pinsrw
	PINSRW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xc4}, requireSrcFloat: false, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x22}, requireSrcFloat: false, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
	PINSRQ: {mandatoryPrefix: 0x66, rPrefix: RexPrefixW, opcode: []byte{0x0f, 0x3a, 0x22}, requireSrcFloat: false, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
	MOVDQU: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x6f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/movdqa:vmovdqa32:vmovdqa64
	MOVDQA: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x6f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/paddb:paddw:paddd:paddq
	PADDB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xfc}, requireSrcFloat: true, requireDstFloat: true},
	PADDW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xfd}, requireSrcFloat: true, requireDstFloat: true},
	PADDD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xfe}, requireSrcFloat: true, requireDstFloat: true},
	PADDQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd4}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubb:psubw:psubd
	PSUBB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf8}, requireSrcFloat: true, requireDstFloat: true},
	PSUBW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf9}, requireSrcFloat: true, requireDstFloat: true},
	PSUBD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xfa}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubq
	PSUBQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xfb}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/addps
	ADDPS: {opcode: []byte{0x0f, 0x58}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/addpd
	ADDPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x58}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/subps
	SUBPS: {opcode: []byte{0x0f, 0x5c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/subpd
	SUBPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x5c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pxor
	PXOR: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xef}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pand
	PAND: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xdb}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/por
	POR: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xeb}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pandn
	PANDN: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xdf}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pshufb
	PSHUFB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x0}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pshufd
	PSHUFD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x70}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x14}, requireSrcFloat: true, requireDstFloat: false, needArg: true, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/pextrw
	PEXTRW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xc5}, requireSrcFloat: true, requireDstFloat: false, needArg: true},
	// https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x16}, requireSrcFloat: true, requireDstFloat: false, needArg: true, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
	PEXTRQ: {rPrefix: RexPrefixW, mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x16}, requireSrcFloat: true, requireDstFloat: false, needArg: true, srcOnModRMReg: true},
	// https://www.felixcloutier.com/x86/insertps
	INSERTPS: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x21}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/movlhps
	MOVLHPS: {opcode: []byte{0x0f, 0x16}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/ptest
	PTEST: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x17}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pcmpeqb:pcmpeqw:pcmpeqd
	PCMPEQB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x74}, requireSrcFloat: true, requireDstFloat: true},
	PCMPEQW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x75}, requireSrcFloat: true, requireDstFloat: true},
	PCMPEQD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x76}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pcmpeqq
	PCMPEQQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x29}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/paddusb:paddusw
	PADDUSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xdc}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/movsd
	MOVSD: {mandatoryPrefix: 0xf2, opcode: []byte{0x0f, 0x10}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/packsswb:packssdw
	PACKSSWB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x63}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovmskb
	PMOVMSKB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd7}, requireSrcFloat: true, requireDstFloat: false},
	// https://www.felixcloutier.com/x86/movmskps
	MOVMSKPS: {opcode: []byte{0x0f, 0x50}, requireSrcFloat: true, requireDstFloat: false},
	// https://www.felixcloutier.com/x86/movmskpd
	MOVMSKPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x50}, requireSrcFloat: true, requireDstFloat: false},
	// https://www.felixcloutier.com/x86/psraw:psrad:psraq
	PSRAD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe2}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psraw:psrad:psraq
	PSRAW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe1}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd3}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd2}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
	PSRLW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd1}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf1}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf2}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psllw:pslld:psllq
	PSLLQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf3}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/punpcklbw:punpcklwd:punpckldq:punpcklqdq
	PUNPCKLBW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x60}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/punpckhbw:punpckhwd:punpckhdq:punpckhqdq
	PUNPCKHBW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x68}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cmpps
	CMPPS: {opcode: []byte{0x0f, 0xc2}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/cmppd
	CMPPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xc2}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pcmpgtq
	PCMPGTQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x37}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x66}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x65}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pcmpgtb:pcmpgtw:pcmpgtd
	PCMPGTB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x64}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminsd:pminsq
	PMINSD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x39}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xee}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxsb:pmaxsw:pmaxsd:pmaxsq
	PMAXSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminsb:pminsw
	PMINSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xea}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminsb:pminsw
	PMINSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x38}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminud:pminuq
	PMINUD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminub:pminuw
	PMINUW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3a}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pminub:pminuw
	PMINUB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xda}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxud:pmaxuq
	PMAXUD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxub:pmaxuw
	PMAXUW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x3e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaxub:pmaxuw
	PMAXUB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xde}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmullw
	PMULLW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd5}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmulld:pmullq
	PMULLD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x40}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmuludq
	PMULUDQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf4}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubsb:psubsw
	PSUBSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe8}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubsb:psubsw
	PSUBSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe9}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubusb:psubusw
	PSUBUSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd8}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/psubusb:psubusw
	PSUBUSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xd9}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/paddsb:paddsw
	PADDSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xed}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/paddsb:paddsw
	PADDSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xec}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/paddusb:paddusw
	PADDUSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xdd}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pavgb:pavgw
	PAVGB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe0}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pavgb:pavgw
	PAVGW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe3}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x1c}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x1d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pabsb:pabsw:pabsd:pabsq
	PABSD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x1e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/blendvpd
	BLENDVPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x15}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/maxpd
	MAXPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x5f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/maxps
	MAXPS: {opcode: []byte{0x0f, 0x5f}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/minpd
	MINPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x5d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/minps
	MINPS: {opcode: []byte{0x0f, 0x5d}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/andnpd
	ANDNPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x55}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/andnps
	ANDNPS: {opcode: []byte{0x0f, 0x55}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/mulps
	MULPS: {opcode: []byte{0x0f, 0x59}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/mulpd
	MULPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x59}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/divps
	DIVPS: {opcode: []byte{0x0f, 0x5e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/divpd
	DIVPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x5e}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/sqrtps
	SQRTPS: {opcode: []byte{0x0f, 0x51}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/sqrtpd
	SQRTPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x51}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/roundps
	ROUNDPS: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x08}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/roundpd
	ROUNDPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x09}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/palignr
	PALIGNR: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x3a, 0x0f}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/punpcklbw:punpcklwd:punpckldq:punpcklqdq
	PUNPCKLWD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x61}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/punpckhbw:punpckhwd:punpckhdq:punpckhqdq
	PUNPCKHWD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x69}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmulhuw
	PMULHUW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe4}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmuldq
	PMULDQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x28}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmulhrsw
	PMULHRSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x0b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovsx
	PMOVSXBW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x20}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovsx
	PMOVSXWD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x23}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovsx
	PMOVSXDQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x25}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovzx
	PMOVZXBW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x30}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovzx
	PMOVZXWD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x33}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmovzx
	PMOVZXDQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x35}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmulhw
	PMULHW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe5}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cmpps
	CMPEQPS: {opcode: []byte{0x0f, 0xc2}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/cmppd
	CMPEQPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xc2}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/cvttps2dq
	CVTTPS2DQ: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0x5b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtdq2ps
	CVTDQ2PS: {opcode: []byte{0x0f, 0x5b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtdq2pd
	CVTDQ2PD: {mandatoryPrefix: 0xf3, opcode: []byte{0x0f, 0xe6}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtpd2ps
	CVTPD2PS: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x5a}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvtps2pd
	CVTPS2PD: {opcode: []byte{0x0f, 0x5a}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/movupd
	MOVUPD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x10}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/shufps
	SHUFPS: {opcode: []byte{0x0f, 0xc6}, requireSrcFloat: true, requireDstFloat: true, needArg: true},
	// https://www.felixcloutier.com/x86/pmaddwd
	PMADDWD: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xf5}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/unpcklps
	UNPCKLPS: {opcode: []byte{0x0f, 0x14}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/packuswb
	PACKUSWB: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x67}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/packsswb:packssdw
	PACKSSDW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x6b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/packusdw
	PACKUSDW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x2b}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/pmaddubsw
	PMADDUBSW: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0x38, 0x04}, requireSrcFloat: true, requireDstFloat: true},
	// https://www.felixcloutier.com/x86/cvttpd2dq
	CVTTPD2DQ: {mandatoryPrefix: 0x66, opcode: []byte{0x0f, 0xe6}, requireDstFloat: true, requireSrcFloat: true},
}

var RegisterToRegisterShiftOpcode = map[asm.Instruction]struct {
	opcode         []byte
	rPrefix        RexPrefix
	modRMExtension byte
}{
	// https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
	ROLL: {opcode: []byte{0xd3}},
	ROLQ: {opcode: []byte{0xd3}, rPrefix: RexPrefixW},
	RORL: {opcode: []byte{0xd3}, modRMExtension: 0b00_001_000},
	RORQ: {opcode: []byte{0xd3}, modRMExtension: 0b00_001_000, rPrefix: RexPrefixW},
	// https://www.felixcloutier.com/x86/sal:sar:shl:shr
	SARL: {opcode: []byte{0xd3}, modRMExtension: 0b00_111_000},
	SARQ: {opcode: []byte{0xd3}, modRMExtension: 0b00_111_000, rPrefix: RexPrefixW},
	SHLL: {opcode: []byte{0xd3}, modRMExtension: 0b00_100_000},
	SHLQ: {opcode: []byte{0xd3}, modRMExtension: 0b00_100_000, rPrefix: RexPrefixW},
	SHRL: {opcode: []byte{0xd3}, modRMExtension: 0b00_101_000},
	SHRQ: {opcode: []byte{0xd3}, modRMExtension: 0b00_101_000, rPrefix: RexPrefixW},
}

type registerToRegisterMOVOpcode struct {
	opcode          []byte
	mandatoryPrefix byte
	srcOnModRMReg   bool
	rPrefix         RexPrefix
}

var registerToRegisterMOVOpcodes = map[asm.Instruction]struct {
	i2i, i2f, f2i, f2f registerToRegisterMOVOpcode
}{
	MOVL: {
		// https://www.felixcloutier.com/x86/mov
		i2i: registerToRegisterMOVOpcode{opcode: []byte{0x89}, srcOnModRMReg: true},
		// https://www.felixcloutier.com/x86/movd:movq
		i2f: registerToRegisterMOVOpcode{opcode: []byte{0x0f, 0x6e}, mandatoryPrefix: 0x66, srcOnModRMReg: false},
		f2i: registerToRegisterMOVOpcode{opcode: []byte{0x0f, 0x7e}, mandatoryPrefix: 0x66, srcOnModRMReg: true},
	},
	MOVQ: {
		// https://www.felixcloutier.com/x86/mov
		i2i: registerToRegisterMOVOpcode{opcode: []byte{0x89}, srcOnModRMReg: true, rPrefix: RexPrefixW},
		// https://www.felixcloutier.com/x86/movd:movq
		i2f: registerToRegisterMOVOpcode{opcode: []byte{0x0f, 0x6e}, mandatoryPrefix: 0x66, srcOnModRMReg: false, rPrefix: RexPrefixW},
		f2i: registerToRegisterMOVOpcode{opcode: []byte{0x0f, 0x7e}, mandatoryPrefix: 0x66, srcOnModRMReg: true, rPrefix: RexPrefixW},
		// https://www.felixcloutier.com/x86/movq
		f2f: registerToRegisterMOVOpcode{opcode: []byte{0x0f, 0x7e}, mandatoryPrefix: 0xf3},
	},
}

func (a *AssemblerImpl) EncodeRegisterToRegister(n *NodeImpl) (err error) {
	// Alias for readability
	inst := n.Instruction

	if op, ok := registerToRegisterMOVOpcodes[inst]; ok {
		var opcode registerToRegisterMOVOpcode
		srcIsFloat, dstIsFloat := IsVectorRegister(n.SrcReg), IsVectorRegister(n.DstReg)
		if srcIsFloat && dstIsFloat {
			if inst == MOVL {
				return errors.New("MOVL for float to float is undefined")
			}
			opcode = op.f2f
		} else if srcIsFloat && !dstIsFloat {
			opcode = op.f2i
		} else if !srcIsFloat && dstIsFloat {
			opcode = op.i2f
		} else {
			opcode = op.i2i
		}

		rexPrefix, modRM, err := n.GetRegisterToRegisterModRM(opcode.srcOnModRMReg)
		if err != nil {
			return err
		}
		rexPrefix |= opcode.rPrefix

		if opcode.mandatoryPrefix != 0 {
			a.Buf.WriteByte(opcode.mandatoryPrefix)
		}

		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		a.Buf.Write(opcode.opcode)

		a.Buf.WriteByte(modRM)
		return nil
	} else if op, ok := registerToRegisterOpcode[inst]; ok {
		srcIsFloat, dstIsFloat := IsVectorRegister(n.SrcReg), IsVectorRegister(n.DstReg)
		if op.requireSrcFloat && !srcIsFloat {
			return fmt.Errorf("%s require float src register but got %s", InstructionName(inst), RegisterName(n.SrcReg))
		} else if op.requireDstFloat && !dstIsFloat {
			return fmt.Errorf("%s require float dst register but got %s", InstructionName(inst), RegisterName(n.DstReg))
		} else if !op.requireSrcFloat && srcIsFloat {
			return fmt.Errorf("%s require integer src register but got %s", InstructionName(inst), RegisterName(n.SrcReg))
		} else if !op.requireDstFloat && dstIsFloat {
			return fmt.Errorf("%s require integer dst register but got %s", InstructionName(inst), RegisterName(n.DstReg))
		}

		rexPrefix, modRM, err := n.GetRegisterToRegisterModRM(op.srcOnModRMReg)
		if err != nil {
			return err
		}
		rexPrefix |= op.rPrefix

		if op.isSrc8bit && RegSP <= n.SrcReg && n.SrcReg <= RegDI {
			// If an operand register is 8-bit length of SP, BP, DI, or SI register, we need to have the default prefix.
			// https: //wiki.osdev.org/X86-64_Instruction_Encoding#Registers
			rexPrefix |= RexPrefixDefault
		}

		if op.mandatoryPrefix != 0 {
			a.Buf.WriteByte(op.mandatoryPrefix)
		}

		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		a.Buf.Write(op.opcode)

		a.Buf.WriteByte(modRM)

		if op.needArg {
			a.WriteConst(int64(n.Arg), 8)
		}
		return nil
	} else if op, ok := RegisterToRegisterShiftOpcode[inst]; ok {
		if n.SrcReg != RegCX {
			return fmt.Errorf("shifting instruction %s require CX register as src but got %s", InstructionName(inst), RegisterName(n.SrcReg))
		} else if IsVectorRegister(n.DstReg) {
			return fmt.Errorf("shifting instruction %s require integer register as dst but got %s", InstructionName(inst), RegisterName(n.SrcReg))
		}

		reg3bits, rexPrefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldRM)
		if err != nil {
			return err
		}

		rexPrefix |= op.rPrefix
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}

		// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
		modRM := 0b11_000_000 |
			(op.modRMExtension) |
			reg3bits
		a.Buf.Write(append(op.opcode, modRM))
		return nil
	} else {
		return errorEncodingUnsupported(n)
	}
}

func (a *AssemblerImpl) EncodeRegisterToMemory(n *NodeImpl) (err error) {
	rexPrefix, modRM, sbi, displacementWidth, err := n.GetMemoryLocation()
	if err != nil {
		return err
	}

	var opcode []byte
	var mandatoryPrefix byte
	var isShiftInstruction bool
	var needArg bool
	switch n.Instruction {
	case CMPL:
		// https://www.felixcloutier.com/x86/cmp
		opcode = []byte{0x3b}
	case CMPQ:
		// https://www.felixcloutier.com/x86/cmp
		rexPrefix |= RexPrefixW
		opcode = []byte{0x3b}
	case MOVB:
		// https://www.felixcloutier.com/x86/mov
		opcode = []byte{0x88}
		// 1 byte register operands need default prefix for the following registers.
		if n.SrcReg >= RegSP && n.SrcReg <= RegDI {
			rexPrefix |= RexPrefixDefault
		}
	case MOVL:
		if IsVectorRegister(n.SrcReg) {
			// https://www.felixcloutier.com/x86/movd:movq
			opcode = []byte{0x0f, 0x7e}
			mandatoryPrefix = 0x66
		} else {
			// https://www.felixcloutier.com/x86/mov
			opcode = []byte{0x89}
		}
	case MOVQ:
		if IsVectorRegister(n.SrcReg) {
			// https://www.felixcloutier.com/x86/movq
			opcode = []byte{0x0f, 0xd6}
			mandatoryPrefix = 0x66
		} else {
			// https://www.felixcloutier.com/x86/mov
			rexPrefix |= RexPrefixW
			opcode = []byte{0x89}
		}
	case MOVW:
		// https://www.felixcloutier.com/x86/mov
		// Note: Need 0x66 to indicate that the operand size is 16-bit.
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#Operand-size_and_address-size_override_prefix
		mandatoryPrefix = 0x66
		opcode = []byte{0x89}
	case SARL:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		modRM |= 0b00_111_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case SARQ:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		rexPrefix |= RexPrefixW
		modRM |= 0b00_111_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case SHLL:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		modRM |= 0b00_100_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case SHLQ:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		rexPrefix |= RexPrefixW
		modRM |= 0b00_100_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case SHRL:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		modRM |= 0b00_101_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case SHRQ:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		rexPrefix |= RexPrefixW
		modRM |= 0b00_101_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case ROLL:
		// https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case ROLQ:
		// https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
		rexPrefix |= RexPrefixW
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case RORL:
		// https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
		modRM |= 0b00_001_000
		opcode = []byte{0xd3}
		isShiftInstruction = true
	case RORQ:
		// https://www.felixcloutier.com/x86/rcl:rcr:rol:ror
		rexPrefix |= RexPrefixW
		opcode = []byte{0xd3}
		modRM |= 0b00_001_000
		isShiftInstruction = true
	case MOVDQU:
		// https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
		mandatoryPrefix = 0xf3
		opcode = []byte{0x0f, 0x7f}
	case PEXTRB: // https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x14}
		needArg = true
	case PEXTRW: // https://www.felixcloutier.com/x86/pextrw
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x15}
		needArg = true
	case PEXTRD: // https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x16}
		needArg = true
	case PEXTRQ: // https://www.felixcloutier.com/x86/pextrb:pextrd:pextrq
		mandatoryPrefix = 0x66
		rexPrefix |= RexPrefixW // REX.W
		opcode = []byte{0x0f, 0x3a, 0x16}
		needArg = true
	default:
		return errorEncodingUnsupported(n)
	}

	if !isShiftInstruction {
		srcReg3Bits, prefix, err := register3bits(n.SrcReg, registerSpecifierPositionModRMFieldReg)
		if err != nil {
			return err
		}

		rexPrefix |= prefix
		modRM |= srcReg3Bits << 3 // Place the source register on ModRM:reg
	} else {
		if n.SrcReg != RegCX {
			return fmt.Errorf("shifting instruction %s require CX register as src but got %s", InstructionName(n.Instruction), RegisterName(n.SrcReg))
		}
	}

	if mandatoryPrefix != 0 {
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#Mandatory_prefix
		a.Buf.WriteByte(mandatoryPrefix)
	}

	if rexPrefix != RexPrefixNone {
		a.Buf.WriteByte(rexPrefix)
	}

	a.Buf.Write(opcode)

	a.Buf.WriteByte(modRM)

	if sbi != nil {
		a.Buf.WriteByte(*sbi)
	}

	if displacementWidth != 0 {
		a.WriteConst(n.DstConst, displacementWidth)
	}

	if needArg {
		a.WriteConst(int64(n.Arg), 8)
	}
	return
}

func (a *AssemblerImpl) EncodeRegisterToConst(n *NodeImpl) (err error) {
	regBits, prefix, err := register3bits(n.SrcReg, registerSpecifierPositionModRMFieldRM)
	if err != nil {
		return err
	}

	switch n.Instruction {
	case CMPL, CMPQ:
		if n.Instruction == CMPQ {
			prefix |= RexPrefixW
		}
		if prefix != RexPrefixNone {
			a.Buf.WriteByte(prefix)
		}
		is8bitConst := fitInSigned8bit(n.DstConst)
		// https://www.felixcloutier.com/x86/cmp
		if n.SrcReg == RegAX && !is8bitConst {
			a.Buf.Write([]byte{0x3d})
		} else {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM := 0b11_000_000 | // Specifying that opeand is register.
				0b00_111_000 | // CMP with immediate needs "/7" extension.
				regBits
			if is8bitConst {
				a.Buf.Write([]byte{0x83, modRM})
			} else {
				a.Buf.Write([]byte{0x81, modRM})
			}
		}
	default:
		err = errorEncodingUnsupported(n)
	}

	if fitInSigned8bit(n.DstConst) {
		a.WriteConst(n.DstConst, 8)
	} else {
		a.WriteConst(n.DstConst, 32)
	}
	return
}

func (a *AssemblerImpl) encodeReadInstructionAddress(n *NodeImpl) error {
	dstReg3Bits, rexPrefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldReg)
	if err != nil {
		return err
	}

	a.AddOnGenerateCallBack(func(code []byte) error {
		// Find the target instruction node.
		targetNode := n
		for ; targetNode != nil; targetNode = targetNode.Next {
			if targetNode.Instruction == n.readInstructionAddressBeforeTargetInstruction {
				targetNode = targetNode.Next
				break
			}
		}

		if targetNode == nil {
			return errors.New("BUG: target instruction not found for read instruction address")
		}

		offset := targetNode.OffsetInBinary() - (n.OffsetInBinary() + 7 /* 7 = the length of the LEAQ instruction */)
		if offset >= math.MaxInt32 {
			return errors.New("BUG: too large offset for LEAQ instruction")
		}

		binary.LittleEndian.PutUint32(code[n.OffsetInBinary()+3:], uint32(int32(offset)))
		return nil
	})

	// https://www.felixcloutier.com/x86/lea
	opcode := byte(0x8d)
	rexPrefix |= RexPrefixW

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
	modRM := 0b00_000_101 | // Indicate "LEAQ [RIP + 32bit displacement], DstReg" encoding.
		(dstReg3Bits << 3) // Place the DstReg on ModRM:reg.

	a.Buf.Write([]byte{rexPrefix, opcode, modRM})
	a.WriteConst(int64(0), 32) // Preserve
	return nil
}

func (a *AssemblerImpl) EncodeMemoryToRegister(n *NodeImpl) (err error) {
	if n.Instruction == LEAQ && n.readInstructionAddressBeforeTargetInstruction != NONE {
		return a.encodeReadInstructionAddress(n)
	}

	rexPrefix, modRM, sbi, displacementWidth, err := n.GetMemoryLocation()
	if err != nil {
		return err
	}

	dstReg3Bits, prefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldReg)
	if err != nil {
		return err
	}

	rexPrefix |= prefix
	modRM |= dstReg3Bits << 3 // Place the destination register on ModRM:reg

	var mandatoryPrefix byte
	var opcode []byte
	var needArg bool
	switch n.Instruction {
	case ADDL:
		// https://www.felixcloutier.com/x86/add
		opcode = []byte{0x03}
	case ADDQ:
		// https://www.felixcloutier.com/x86/add
		rexPrefix |= RexPrefixW
		opcode = []byte{0x03}
	case CMPL:
		// https://www.felixcloutier.com/x86/cmp
		opcode = []byte{0x39}
	case CMPQ:
		// https://www.felixcloutier.com/x86/cmp
		rexPrefix |= RexPrefixW
		opcode = []byte{0x39}
	case LEAQ:
		// https://www.felixcloutier.com/x86/lea
		rexPrefix |= RexPrefixW
		opcode = []byte{0x8d}
	case MOVBLSX:
		// https://www.felixcloutier.com/x86/movsx:movsxd
		opcode = []byte{0x0f, 0xbe}
	case MOVBLZX:
		// https://www.felixcloutier.com/x86/movzx
		opcode = []byte{0x0f, 0xb6}
	case MOVBQSX:
		// https://www.felixcloutier.com/x86/movsx:movsxd
		rexPrefix |= RexPrefixW
		opcode = []byte{0x0f, 0xbe}
	case MOVBQZX:
		// https://www.felixcloutier.com/x86/movzx
		rexPrefix |= RexPrefixW
		opcode = []byte{0x0f, 0xb6}
	case MOVLQSX:
		// https://www.felixcloutier.com/x86/movsx:movsxd
		rexPrefix |= RexPrefixW
		opcode = []byte{0x63}
	case MOVLQZX:
		// https://www.felixcloutier.com/x86/mov
		// Note: MOVLQZX means zero extending 32bit reg to 64-bit reg and
		// that is semantically equivalent to MOV 32bit to 32bit.
		opcode = []byte{0x8B}
	case MOVL:
		// https://www.felixcloutier.com/x86/mov
		// Note: MOVLQZX means zero extending 32bit reg to 64-bit reg and
		// that is semantically equivalent to MOV 32bit to 32bit.
		if IsVectorRegister(n.DstReg) {
			// https://www.felixcloutier.com/x86/movd:movq
			opcode = []byte{0x0f, 0x6e}
			mandatoryPrefix = 0x66
		} else {
			// https://www.felixcloutier.com/x86/mov
			opcode = []byte{0x8B}
		}
	case MOVQ:
		if IsVectorRegister(n.DstReg) {
			// https://www.felixcloutier.com/x86/movq
			opcode = []byte{0x0f, 0x7e}
			mandatoryPrefix = 0xf3
		} else {
			// https://www.felixcloutier.com/x86/mov
			rexPrefix |= RexPrefixW
			opcode = []byte{0x8B}
		}
	case MOVWLSX:
		// https://www.felixcloutier.com/x86/movsx:movsxd
		opcode = []byte{0x0f, 0xbf}
	case MOVWLZX:
		// https://www.felixcloutier.com/x86/movzx
		opcode = []byte{0x0f, 0xb7}
	case MOVWQSX:
		// https://www.felixcloutier.com/x86/movsx:movsxd
		rexPrefix |= RexPrefixW
		opcode = []byte{0x0f, 0xbf}
	case MOVWQZX:
		// https://www.felixcloutier.com/x86/movzx
		rexPrefix |= RexPrefixW
		opcode = []byte{0x0f, 0xb7}
	case SUBQ:
		// https://www.felixcloutier.com/x86/sub
		rexPrefix |= RexPrefixW
		opcode = []byte{0x2b}
	case SUBSD:
		// https://www.felixcloutier.com/x86/subsd
		opcode = []byte{0x0f, 0x5c}
		mandatoryPrefix = 0xf2
	case SUBSS:
		// https://www.felixcloutier.com/x86/subss
		opcode = []byte{0x0f, 0x5c}
		mandatoryPrefix = 0xf3
	case UCOMISD:
		// https://www.felixcloutier.com/x86/ucomisd
		opcode = []byte{0x0f, 0x2e}
		mandatoryPrefix = 0x66
	case UCOMISS:
		// https://www.felixcloutier.com/x86/ucomiss
		opcode = []byte{0x0f, 0x2e}
	case MOVDQU:
		// https://www.felixcloutier.com/x86/movdqu:vmovdqu8:vmovdqu16:vmovdqu32:vmovdqu64
		mandatoryPrefix = 0xf3
		opcode = []byte{0x0f, 0x6f}
	case PMOVSXBW: // https://www.felixcloutier.com/x86/pmovsx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x20}
	case PMOVSXWD: // https://www.felixcloutier.com/x86/pmovsx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x23}
	case PMOVSXDQ: // https://www.felixcloutier.com/x86/pmovsx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x25}
	case PMOVZXBW: // https://www.felixcloutier.com/x86/pmovzx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x30}
	case PMOVZXWD: // https://www.felixcloutier.com/x86/pmovzx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x33}
	case PMOVZXDQ: // https://www.felixcloutier.com/x86/pmovzx
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x38, 0x35}
	case PINSRB: // https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x20}
		needArg = true
	case PINSRW: // https://www.felixcloutier.com/x86/pinsrw
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0xc4}
		needArg = true
	case PINSRD: // https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x22}
		needArg = true
	case PINSRQ: // https://www.felixcloutier.com/x86/pinsrb:pinsrd:pinsrq
		rexPrefix |= RexPrefixW
		mandatoryPrefix = 0x66
		opcode = []byte{0x0f, 0x3a, 0x22}
		needArg = true
	default:
		return errorEncodingUnsupported(n)
	}

	if mandatoryPrefix != 0 {
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#Mandatory_prefix
		a.Buf.WriteByte(mandatoryPrefix)
	}

	if rexPrefix != RexPrefixNone {
		a.Buf.WriteByte(rexPrefix)
	}

	a.Buf.Write(opcode)

	a.Buf.WriteByte(modRM)

	if sbi != nil {
		a.Buf.WriteByte(*sbi)
	}

	if displacementWidth != 0 {
		a.WriteConst(n.SrcConst, displacementWidth)
	}

	if needArg {
		a.WriteConst(int64(n.Arg), 8)
	}
	return
}

func (a *AssemblerImpl) EncodeConstToRegister(n *NodeImpl) (err error) {
	regBits, rexPrefix, err := register3bits(n.DstReg, registerSpecifierPositionModRMFieldRM)
	if err != nil {
		return err
	}

	isFloatReg := IsVectorRegister(n.DstReg)
	switch n.Instruction {
	case PSLLD, PSLLQ, PSRLD, PSRLQ, PSRAW, PSRLW, PSLLW, PSRAD:
		if !isFloatReg {
			return fmt.Errorf("%s needs float register but got %s", InstructionName(n.Instruction), RegisterName(n.DstReg))
		}
	default:
		if isFloatReg {
			return fmt.Errorf("%s needs int register but got %s", InstructionName(n.Instruction), RegisterName(n.DstReg))
		}
	}

	if n.Instruction != MOVQ && !FitIn32bit(n.SrcConst) {
		return fmt.Errorf("constant must fit in 32-bit integer for %s, but got %d", InstructionName(n.Instruction), n.SrcConst)
	} else if (n.Instruction == SHLQ || n.Instruction == SHRQ) && (n.SrcConst < 0 || n.SrcConst > math.MaxUint8) {
		return fmt.Errorf("constant must fit in positive 8-bit integer for %s, but got %d", InstructionName(n.Instruction), n.SrcConst)
	} else if (n.Instruction == PSLLD ||
		n.Instruction == PSLLQ ||
		n.Instruction == PSRLD ||
		n.Instruction == PSRLQ) && (n.SrcConst < math.MinInt8 || n.SrcConst > math.MaxInt8) {
		return fmt.Errorf("constant must fit in signed 8-bit integer for %s, but got %d", InstructionName(n.Instruction), n.SrcConst)
	}

	isSigned8bitConst := fitInSigned8bit(n.SrcConst)
	switch inst := n.Instruction; inst {
	case ADDQ:
		// https://www.felixcloutier.com/x86/add
		rexPrefix |= RexPrefixW
		if n.DstReg == RegAX && !isSigned8bitConst {
			a.Buf.Write([]byte{rexPrefix, 0x05})
		} else {
			modRM := 0b11_000_000 | // Specifying that opeand is register.
				regBits
			if isSigned8bitConst {
				a.Buf.Write([]byte{rexPrefix, 0x83, modRM})
			} else {
				a.Buf.Write([]byte{rexPrefix, 0x81, modRM})
			}
		}
		if isSigned8bitConst {
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.WriteConst(n.SrcConst, 32)
		}
	case ANDQ:
		// https://www.felixcloutier.com/x86/and
		rexPrefix |= RexPrefixW
		if n.DstReg == RegAX && !isSigned8bitConst {
			a.Buf.Write([]byte{rexPrefix, 0x25})
		} else {
			modRM := 0b11_000_000 | // Specifying that opeand is register.
				0b00_100_000 | // AND with immediate needs "/4" extension.
				regBits
			if isSigned8bitConst {
				a.Buf.Write([]byte{rexPrefix, 0x83, modRM})
			} else {
				a.Buf.Write([]byte{rexPrefix, 0x81, modRM})
			}
		}
		if fitInSigned8bit(n.SrcConst) {
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.WriteConst(n.SrcConst, 32)
		}
	case MOVL:
		// https://www.felixcloutier.com/x86/mov
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		a.Buf.Write([]byte{0xb8 | regBits})
		a.WriteConst(n.SrcConst, 32)
	case MOVQ:
		// https://www.felixcloutier.com/x86/mov
		if FitIn32bit(n.SrcConst) {
			if n.SrcConst > math.MaxInt32 {
				if rexPrefix != RexPrefixNone {
					a.Buf.WriteByte(rexPrefix)
				}
				a.Buf.Write([]byte{0xb8 | regBits})
			} else {
				rexPrefix |= RexPrefixW
				modRM := 0b11_000_000 | // Specifying that opeand is register.
					regBits
				a.Buf.Write([]byte{rexPrefix, 0xc7, modRM})
			}
			a.WriteConst(n.SrcConst, 32)
		} else {
			rexPrefix |= RexPrefixW
			a.Buf.Write([]byte{rexPrefix, 0xb8 | regBits})
			a.WriteConst(n.SrcConst, 64)
		}
	case SHLQ:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		rexPrefix |= RexPrefixW
		modRM := 0b11_000_000 | // Specifying that opeand is register.
			0b00_100_000 | // SHL with immediate needs "/4" extension.
			regBits
		if n.SrcConst == 1 {
			a.Buf.Write([]byte{rexPrefix, 0xd1, modRM})
		} else {
			a.Buf.Write([]byte{rexPrefix, 0xc1, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case SHRQ:
		// https://www.felixcloutier.com/x86/sal:sar:shl:shr
		rexPrefix |= RexPrefixW
		modRM := 0b11_000_000 | // Specifying that opeand is register.
			0b00_101_000 | // SHR with immediate needs "/5" extension.
			regBits
		if n.SrcConst == 1 {
			a.Buf.Write([]byte{rexPrefix, 0xd1, modRM})
		} else {
			a.Buf.Write([]byte{rexPrefix, 0xc1, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case PSLLD:
		// https://www.felixcloutier.com/x86/psllw:pslld:psllq
		modRM := 0b11_000_000 | // Specifying that opeand is register.
			0b00_110_000 | // PSLL with immediate needs "/6" extension.
			regBits
		if rexPrefix != RexPrefixNone {
			a.Buf.Write([]byte{0x66, rexPrefix, 0x0f, 0x72, modRM})
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.Buf.Write([]byte{0x66, 0x0f, 0x72, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case PSLLQ:
		// https://www.felixcloutier.com/x86/psllw:pslld:psllq
		modRM := 0b11_000_000 | // Specifying that opeand is register.
			0b00_110_000 | // PSLL with immediate needs "/6" extension.
			regBits
		if rexPrefix != RexPrefixNone {
			a.Buf.Write([]byte{0x66, rexPrefix, 0x0f, 0x73, modRM})
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.Buf.Write([]byte{0x66, 0x0f, 0x73, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case PSRLD:
		// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
		// https://www.felixcloutier.com/x86/psllw:pslld:psllq
		modRM := 0b11_000_000 | // Specifying that operand is register.
			0b00_010_000 | // PSRL with immediate needs "/2" extension.
			regBits
		if rexPrefix != RexPrefixNone {
			a.Buf.Write([]byte{0x66, rexPrefix, 0x0f, 0x72, modRM})
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.Buf.Write([]byte{0x66, 0x0f, 0x72, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case PSRLQ:
		// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
		modRM := 0b11_000_000 | // Specifying that operand is register.
			0b00_010_000 | // PSRL with immediate needs "/2" extension.
			regBits
		if rexPrefix != RexPrefixNone {
			a.Buf.Write([]byte{0x66, rexPrefix, 0x0f, 0x73, modRM})
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.Buf.Write([]byte{0x66, 0x0f, 0x73, modRM})
			a.WriteConst(n.SrcConst, 8)
		}
	case PSRAW, PSRAD:
		// https://www.felixcloutier.com/x86/psraw:psrad:psraq
		modRM := 0b11_000_000 | // Specifying that operand is register.
			0b00_100_000 | // PSRAW with immediate needs "/4" extension.
			regBits
		a.Buf.WriteByte(0x66)
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}

		var op byte
		if inst == PSRAD {
			op = 0x72
		} else { // PSRAW
			op = 0x71
		}

		a.Buf.Write([]byte{0x0f, op, modRM})
		a.WriteConst(n.SrcConst, 8)
	case PSRLW:
		// https://www.felixcloutier.com/x86/psrlw:psrld:psrlq
		modRM := 0b11_000_000 | // Specifying that operand is register.
			0b00_010_000 | // PSRLW with immediate needs "/2" extension.
			regBits
		a.Buf.WriteByte(0x66)
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		a.Buf.Write([]byte{0x0f, 0x71, modRM})
		a.WriteConst(n.SrcConst, 8)
	case PSLLW:
		// https://www.felixcloutier.com/x86/psllw:pslld:psllq
		modRM := 0b11_000_000 | // Specifying that operand is register.
			0b00_110_000 | // PSLLW with immediate needs "/6" extension.
			regBits
		a.Buf.WriteByte(0x66)
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		a.Buf.Write([]byte{0x0f, 0x71, modRM})
		a.WriteConst(n.SrcConst, 8)
	case XORL, XORQ:
		// https://www.felixcloutier.com/x86/xor
		if inst == XORQ {
			rexPrefix |= RexPrefixW
		}
		if rexPrefix != RexPrefixNone {
			a.Buf.WriteByte(rexPrefix)
		}
		if n.DstReg == RegAX && !isSigned8bitConst {
			a.Buf.Write([]byte{0x35})
		} else {
			modRM := 0b11_000_000 | // Specifying that opeand is register.
				0b00_110_000 | // XOR with immediate needs "/6" extension.
				regBits
			if isSigned8bitConst {
				a.Buf.Write([]byte{0x83, modRM})
			} else {
				a.Buf.Write([]byte{0x81, modRM})
			}
		}
		if fitInSigned8bit(n.SrcConst) {
			a.WriteConst(n.SrcConst, 8)
		} else {
			a.WriteConst(n.SrcConst, 32)
		}
	default:
		err = errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) EncodeMemoryToConst(n *NodeImpl) (err error) {
	if !FitIn32bit(n.DstConst) {
		return fmt.Errorf("too large target const %d for %s", n.DstConst, InstructionName(n.Instruction))
	}

	rexPrefix, modRM, sbi, displacementWidth, err := n.GetMemoryLocation()
	if err != nil {
		return err
	}

	// Alias for readability.
	c := n.DstConst

	var opcode, constWidth byte
	switch n.Instruction {
	case CMPL:
		// https://www.felixcloutier.com/x86/cmp
		if fitInSigned8bit(c) {
			opcode = 0x83
			constWidth = 8
		} else {
			opcode = 0x81
			constWidth = 32
		}
		modRM |= 0b00_111_000
	default:
		return errorEncodingUnsupported(n)
	}

	if rexPrefix != RexPrefixNone {
		a.Buf.WriteByte(rexPrefix)
	}

	a.Buf.Write([]byte{opcode, modRM})

	if sbi != nil {
		a.Buf.WriteByte(*sbi)
	}

	if displacementWidth != 0 {
		a.WriteConst(n.SrcConst, displacementWidth)
	}

	a.WriteConst(c, constWidth)
	return
}

func (a *AssemblerImpl) EncodeConstToMemory(n *NodeImpl) (err error) {
	rexPrefix, modRM, sbi, displacementWidth, err := n.GetMemoryLocation()
	if err != nil {
		return err
	}

	// Alias for readability.
	inst := n.Instruction
	c := n.SrcConst

	if inst == MOVB && !fitInSigned8bit(c) {
		return fmt.Errorf("too large load target const %d for MOVB", c)
	} else if !FitIn32bit(c) {
		return fmt.Errorf("too large load target const %d for %s", c, InstructionName(n.Instruction))
	}

	var constWidth, opcode byte
	switch inst {
	case MOVB:
		opcode = 0xc6
		constWidth = 8
	case MOVL:
		opcode = 0xc7
		constWidth = 32
	case MOVQ:
		rexPrefix |= RexPrefixW
		opcode = 0xc7
		constWidth = 32
	default:
		return errorEncodingUnsupported(n)
	}

	if rexPrefix != RexPrefixNone {
		a.Buf.WriteByte(rexPrefix)
	}

	a.Buf.Write([]byte{opcode, modRM})

	if sbi != nil {
		a.Buf.WriteByte(*sbi)
	}

	if displacementWidth != 0 {
		a.WriteConst(n.DstConst, displacementWidth)
	}

	a.WriteConst(c, constWidth)
	return
}

func (a *AssemblerImpl) WriteConst(v int64, length byte) {
	switch length {
	case 8:
		a.Buf.WriteByte(byte(int8(v)))
	case 32:
		// TODO: any way to directly put little endian bytes into bytes.Buffer?
		offsetBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(offsetBytes, uint32(int32(v)))
		a.Buf.Write(offsetBytes)
	case 64:
		// TODO: any way to directly put little endian bytes into bytes.Buffer?
		offsetBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(offsetBytes, uint64(v))
		a.Buf.Write(offsetBytes)
	default:
		panic("BUG: length must be one of 8, 32 or 64")
	}
}

func (n *NodeImpl) GetMemoryLocation() (p RexPrefix, modRM byte, sbi *byte, displacementWidth byte, err error) {
	var baseReg, indexReg asm.Register
	var offset asm.ConstantValue
	var scale byte
	if n.Types.dst == OperandTypeMemory {
		baseReg, offset, indexReg, scale = n.DstReg, n.DstConst, n.DstMemIndex, n.DstMemScale
	} else if n.Types.src == OperandTypeMemory {
		baseReg, offset, indexReg, scale = n.SrcReg, n.SrcConst, n.SrcMemIndex, n.SrcMemScale
	} else {
		err = fmt.Errorf("memory location is not supported for %s", n.Types)
		return
	}

	if !FitIn32bit(offset) {
		err = errors.New("offset does not fit in 32-bit integer")
		return
	}

	if baseReg == asm.NilRegister && indexReg != asm.NilRegister {
		// [(index*scale) + displacement] addressing is possible, but we haven't used it for now.
		err = errors.New("addressing without base register but with index is not implemented")
	} else if baseReg == asm.NilRegister {
		modRM = 0b00_000_100 // Indicate that the memory location is specified by SIB.
		sbiValue := byte(0b00_100_101)
		sbi = &sbiValue
		displacementWidth = 32
	} else if indexReg == asm.NilRegister {
		modRM, p, err = register3bits(baseReg, registerSpecifierPositionModRMFieldRM)
		if err != nil {
			return
		}

		// Create ModR/M byte so that this instruction takes [R/M + displacement] operand if displacement !=0
		// and otherwise [R/M].
		withoutDisplacement := offset == 0 &&
			// If the target register is R13 or BP, we have to keep [R/M + displacement] even if the value
			// is zero since it's not [R/M] operand is not defined for these two registers.
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
			baseReg != RegR13 && baseReg != RegBP
		if withoutDisplacement {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b00_000_000 // Specifying that operand is memory without displacement
			displacementWidth = 0
		} else if fitInSigned8bit(offset) {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b01_000_000 // Specifying that operand is memory + 8bit displacement.
			displacementWidth = 8
		} else {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b10_000_000 // Specifying that operand is memory + 32bit displacement.
			displacementWidth = 32
		}

		// For SP and R12 register, we have [SIB + displacement] if the const is non-zero, otherwise [SIP].
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing
		//
		// Thefore we emit the SIB byte before the const so that [SIB + displacement] ends up [register + displacement].
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#32.2F64-bit_addressing_2
		if baseReg == RegSP || baseReg == RegR12 {
			sbiValue := byte(0b00_100_100)
			sbi = &sbiValue
		}
	} else {
		if indexReg == RegSP {
			err = errors.New("SP cannot be used for SIB index")
			return
		}

		modRM = 0b00_000_100 // Indicate that the memory location is specified by SIB.

		withoutDisplacement := offset == 0 &&
			// For R13 and BP, base registers cannot be encoded "without displacement" mod (i.e. 0b00 mod).
			baseReg != RegR13 && baseReg != RegBP
		if withoutDisplacement {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b00_000_000 // Specifying that operand is SIB without displacement
			displacementWidth = 0
		} else if fitInSigned8bit(offset) {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b01_000_000 // Specifying that operand is SIB + 8bit displacement.
			displacementWidth = 8
		} else {
			// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
			modRM |= 0b10_000_000 // Specifying that operand is SIB + 32bit displacement.
			displacementWidth = 32
		}

		var baseRegBits byte
		baseRegBits, p, err = register3bits(baseReg, registerSpecifierPositionModRMFieldRM)
		if err != nil {
			return
		}

		var indexRegBits byte
		var indexRegPrefix RexPrefix
		indexRegBits, indexRegPrefix, err = register3bits(indexReg, registerSpecifierPositionSIBIndex)
		if err != nil {
			return
		}
		p |= indexRegPrefix

		sbiValue := baseRegBits | (indexRegBits << 3)
		switch scale {
		case 1:
			sbiValue |= 0b00_000_000
		case 2:
			sbiValue |= 0b01_000_000
		case 4:
			sbiValue |= 0b10_000_000
		case 8:
			sbiValue |= 0b11_000_000
		default:
			err = fmt.Errorf("scale in SIB must be one of 1, 2, 4, 8 but got %d", scale)
			return
		}

		sbi = &sbiValue
	}
	return
}

// GetRegisterToRegisterModRM does XXXX
//
// TODO: srcOnModRMReg can be deleted after golang-asm removal. This is necessary to match our implementation
// with golang-asm, but in practice, there are equivalent opcodes to always have src on ModRM:reg without ambiguity.
func (n *NodeImpl) GetRegisterToRegisterModRM(srcOnModRMReg bool) (RexPrefix, modRM byte, err error) {
	var reg3bits, rm3bits byte
	if srcOnModRMReg {
		reg3bits, RexPrefix, err = register3bits(n.SrcReg,
			// Indicate that SrcReg will be specified by ModRM:reg.
			registerSpecifierPositionModRMFieldReg)
		if err != nil {
			return
		}

		var dstRexPrefix byte
		rm3bits, dstRexPrefix, err = register3bits(n.DstReg,
			// Indicate that DstReg will be specified by ModRM:r/m.
			registerSpecifierPositionModRMFieldRM)
		if err != nil {
			return
		}
		RexPrefix |= dstRexPrefix
	} else {
		rm3bits, RexPrefix, err = register3bits(n.SrcReg,
			// Indicate that SrcReg will be specified by ModRM:r/m.
			registerSpecifierPositionModRMFieldRM)
		if err != nil {
			return
		}

		var dstRexPrefix byte
		reg3bits, dstRexPrefix, err = register3bits(n.DstReg,
			// Indicate that DstReg will be specified by ModRM:reg.
			registerSpecifierPositionModRMFieldReg)
		if err != nil {
			return
		}
		RexPrefix |= dstRexPrefix
	}

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#ModR.2FM
	modRM = 0b11_000_000 | // Specifying that dst operand is register.
		(reg3bits << 3) |
		rm3bits

	return
}

// RexPrefix represents REX prefix https://wiki.osdev.org/X86-64_Instruction_Encoding#REX_prefix
type RexPrefix = byte

// REX prefixes are independent of each other and can be combined with OR.
const (
	RexPrefixNone    RexPrefix = 0x0000_0000 // Indicates that the instruction doesn't need RexPrefix.
	RexPrefixDefault RexPrefix = 0b0100_0000
	RexPrefixW                 = 0b0000_1000 | RexPrefixDefault // REX.W
	RexPrefixR                 = 0b0000_0100 | RexPrefixDefault // REX.R
	RexPrefixX                 = 0b0000_0010 | RexPrefixDefault // REX.X
	RexPrefixB                 = 0b0000_0001 | RexPrefixDefault // REX.B
)

// registerSpecifierPosition represents the position in the instruction bytes where an operand register is placed.
type registerSpecifierPosition byte

const (
	registerSpecifierPositionModRMFieldReg registerSpecifierPosition = iota
	registerSpecifierPositionModRMFieldRM
	registerSpecifierPositionSIBIndex
)

func register3bits(
	reg asm.Register,
	registerSpecifierPosition registerSpecifierPosition,
) (bits byte, prefix RexPrefix, err error) {
	prefix = RexPrefixNone
	if RegR8 <= reg && reg <= RegR15 || RegX8 <= reg && reg <= RegX15 {
		// https://wiki.osdev.org/X86-64_Instruction_Encoding#REX_prefix
		switch registerSpecifierPosition {
		case registerSpecifierPositionModRMFieldReg:
			prefix = RexPrefixR
		case registerSpecifierPositionModRMFieldRM:
			prefix = RexPrefixB
		case registerSpecifierPositionSIBIndex:
			prefix = RexPrefixX
		}
	}

	// https://wiki.osdev.org/X86-64_Instruction_Encoding#Registers
	switch reg {
	case RegAX, RegR8, RegX0, RegX8:
		bits = 0b000
	case RegCX, RegR9, RegX1, RegX9:
		bits = 0b001
	case RegDX, RegR10, RegX2, RegX10:
		bits = 0b010
	case RegBX, RegR11, RegX3, RegX11:
		bits = 0b011
	case RegSP, RegR12, RegX4, RegX12:
		bits = 0b100
	case RegBP, RegR13, RegX5, RegX13:
		bits = 0b101
	case RegSI, RegR14, RegX6, RegX14:
		bits = 0b110
	case RegDI, RegR15, RegX7, RegX15:
		bits = 0b111
	default:
		err = fmt.Errorf("invalid register [%s]", RegisterName(reg))
	}
	return
}

func FitIn32bit(v int64) bool {
	return math.MinInt32 <= v && v <= math.MaxUint32
}

func fitInSigned8bit(v int64) bool {
	return math.MinInt8 <= v && v <= math.MaxInt8
}

func IsVectorRegister(r asm.Register) bool {
	return RegX0 <= r && r <= RegX15
}
