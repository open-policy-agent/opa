package arm64

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
)

type NodeImpl struct {
	// NOTE: fields here are exported for testing with the amd64_debug package.

	Instruction asm.Instruction

	OffsetInBinaryField asm.NodeOffsetInBinary // Field suffix to dodge conflict with OffsetInBinary

	// JumpTarget holds the target node in the linked for the jump-kind instruction.
	JumpTarget *NodeImpl
	// next holds the next node from this node in the assembled linked list.
	Next *NodeImpl

	Types                            OperandTypes
	SrcReg, SrcReg2, DstReg, DstReg2 asm.Register
	SrcConst, DstConst               asm.ConstantValue

	VectorArrangement              VectorArrangement
	SrcVectorIndex, DstVectorIndex VectorIndex

	// readInstructionAddressBeforeTargetInstruction holds the instruction right before the target of
	// read instruction address instruction. See asm.assemblerBase.CompileReadInstructionAddress.
	readInstructionAddressBeforeTargetInstruction asm.Instruction

	// JumpOrigins hold all the nodes trying to jump into this node. In other words, all the nodes with .JumpTarget == this.
	JumpOrigins map[*NodeImpl]struct{}

	staticConst *asm.StaticConst
}

// AssignJumpTarget implements the same method as documented on asm.Node.
func (n *NodeImpl) AssignJumpTarget(target asm.Node) {
	n.JumpTarget = target.(*NodeImpl)
}

// AssignDestinationConstant implements the same method as documented on asm.Node.
func (n *NodeImpl) AssignDestinationConstant(value asm.ConstantValue) {
	n.DstConst = value
}

// AssignSourceConstant implements the same method as documented on asm.Node.
func (n *NodeImpl) AssignSourceConstant(value asm.ConstantValue) {
	n.SrcConst = value
}

// OffsetInBinary implements the same method as documented on asm.Node.
func (n *NodeImpl) OffsetInBinary() asm.NodeOffsetInBinary {
	return n.OffsetInBinaryField
}

// String implements fmt.Stringer.
//
// This is for debugging purpose, and the format is similar to the AT&T assembly syntax,
// meaning that this should look like "INSTRUCTION ${from}, ${to}" where each operand
// might be embraced by '[]' to represent the memory location, and multiple operands
// are embraced by `()`.
func (n *NodeImpl) String() (ret string) {
	instName := InstructionName(n.Instruction)
	switch n.Types {
	case OperandTypesNoneToNone:
		ret = instName
	case OperandTypesNoneToRegister:
		ret = fmt.Sprintf("%s %s", instName, RegisterName(n.DstReg))
	case OperandTypesNoneToMemory:
		ret = fmt.Sprintf("%s [%s + 0x%x]", instName, RegisterName(n.DstReg), n.DstConst)
	case OperandTypesNoneToBranch:
		ret = fmt.Sprintf("%s {%v}", instName, n.JumpTarget)
	case OperandTypesRegisterToRegister:
		ret = fmt.Sprintf("%s %s, %s", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg))
	case OperandTypesLeftShiftedRegisterToRegister:
		ret = fmt.Sprintf("%s (%s, %s << %d), %s", instName, RegisterName(n.SrcReg), RegisterName(n.SrcReg2), n.SrcConst, RegisterName(n.DstReg))
	case OperandTypesTwoRegistersToRegister:
		ret = fmt.Sprintf("%s (%s, %s), %s", instName, RegisterName(n.SrcReg), RegisterName(n.SrcReg2), RegisterName(n.DstReg))
	case OperandTypesThreeRegistersToRegister:
		ret = fmt.Sprintf("%s (%s, %s, %s), %s)", instName, RegisterName(n.SrcReg), RegisterName(n.SrcReg2), RegisterName(n.DstReg), RegisterName(n.DstReg2))
	case OperandTypesTwoRegistersToNone:
		ret = fmt.Sprintf("%s (%s, %s)", instName, RegisterName(n.SrcReg), RegisterName(n.SrcReg2))
	case OperandTypesRegisterAndConstToNone:
		ret = fmt.Sprintf("%s (%s, 0x%x)", instName, RegisterName(n.SrcReg), n.SrcConst)
	case OperandTypesRegisterToMemory:
		if n.DstReg2 != asm.NilRegister {
			ret = fmt.Sprintf("%s %s, [%s + %s]", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), RegisterName(n.DstReg2))
		} else {
			ret = fmt.Sprintf("%s %s, [%s + 0x%x]", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), n.DstConst)
		}
	case OperandTypesMemoryToRegister:
		if n.SrcReg2 != asm.NilRegister {
			ret = fmt.Sprintf("%s [%s + %s], %s", instName, RegisterName(n.SrcReg), RegisterName(n.SrcReg2), RegisterName(n.DstReg))
		} else {
			ret = fmt.Sprintf("%s [%s + 0x%x], %s", instName, RegisterName(n.SrcReg), n.SrcConst, RegisterName(n.DstReg))
		}
	case OperandTypesConstToRegister:
		ret = fmt.Sprintf("%s 0x%x, %s", instName, n.SrcConst, RegisterName(n.DstReg))
	case OperandTypesRegisterToVectorRegister:
		ret = fmt.Sprintf("%s %s, %s.%s[%d]", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), n.VectorArrangement, n.DstVectorIndex)
	case OperandTypesVectorRegisterToRegister:
		ret = fmt.Sprintf("%s %s.%s[%d], %s.%s[%d]", instName, RegisterName(n.SrcReg), n.VectorArrangement, n.SrcVectorIndex,
			RegisterName(n.DstReg), n.VectorArrangement, n.DstVectorIndex)
	case OperandTypesVectorRegisterToMemory:
		ret = fmt.Sprintf("%s %s.%s, [%s]", instName, RegisterName(n.SrcReg), n.VectorArrangement, RegisterName(n.DstReg))
	case OperandTypesMemoryToVectorRegister:
		ret = fmt.Sprintf("%s [%s], %s.%s", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), n.VectorArrangement)
	case OperandTypesVectorRegisterToVectorRegister:
		ret = fmt.Sprintf("%s %s.%[2]s, %s.%[2]s", instName, RegisterName(n.SrcReg), RegisterName(n.DstReg), n.VectorArrangement)
	case OperandTypesStaticConstToVectorRegister:
		ret = fmt.Sprintf("%s $%v %s.%s", instName, n.staticConst, RegisterName(n.DstReg), n.VectorArrangement)
	}
	return
}

// OperandType represents where an operand is placed for an instruction.
// Note: this is almost the same as obj.AddrType in GO assembler.
type OperandType byte

const (
	OperandTypeNone OperandType = iota
	OperandTypeRegister
	OperandTypeLeftShiftedRegister
	OperandTypeTwoRegisters
	OperandTypeThreeRegisters
	OperandTypeRegisterAndConst
	OperandTypeMemory
	OperandTypeConst
	OperandTypeBranch
	OperandTypeSIMDByte
	OperandTypeTwoSIMDBytes
	OperandTypeVectorRegister
	OperandTypeTwoVectorRegisters
	OperandTypeStaticConst
)

// String implements fmt.Stringer.
func (o OperandType) String() (ret string) {
	switch o {
	case OperandTypeNone:
		ret = "none"
	case OperandTypeRegister:
		ret = "register"
	case OperandTypeLeftShiftedRegister:
		ret = "left-shifted-register"
	case OperandTypeTwoRegisters:
		ret = "two-registers"
	case OperandTypeRegisterAndConst:
		ret = "register-and-const"
	case OperandTypeMemory:
		ret = "memory"
	case OperandTypeConst:
		ret = "const"
	case OperandTypeBranch:
		ret = "branch"
	case OperandTypeSIMDByte:
		ret = "simd-byte"
	case OperandTypeTwoSIMDBytes:
		ret = "two-simd-bytes"
	case OperandTypeVectorRegister:
		ret = "vector-register"
	case OperandTypeStaticConst:
		ret = "static-const"
	case OperandTypeTwoVectorRegisters:
		ret = "two-vector-registers"
	}
	return
}

// OperandTypes represents the only combinations of two OperandTypes used by wazero
type OperandTypes struct{ src, dst OperandType }

var (
	OperandTypesNoneToNone                         = OperandTypes{OperandTypeNone, OperandTypeNone}
	OperandTypesNoneToRegister                     = OperandTypes{OperandTypeNone, OperandTypeRegister}
	OperandTypesNoneToMemory                       = OperandTypes{OperandTypeNone, OperandTypeMemory}
	OperandTypesNoneToBranch                       = OperandTypes{OperandTypeNone, OperandTypeBranch}
	OperandTypesRegisterToRegister                 = OperandTypes{OperandTypeRegister, OperandTypeRegister}
	OperandTypesLeftShiftedRegisterToRegister      = OperandTypes{OperandTypeLeftShiftedRegister, OperandTypeRegister}
	OperandTypesTwoRegistersToRegister             = OperandTypes{OperandTypeTwoRegisters, OperandTypeRegister}
	OperandTypesThreeRegistersToRegister           = OperandTypes{OperandTypeThreeRegisters, OperandTypeRegister}
	OperandTypesTwoRegistersToNone                 = OperandTypes{OperandTypeTwoRegisters, OperandTypeNone}
	OperandTypesRegisterAndConstToNone             = OperandTypes{OperandTypeRegisterAndConst, OperandTypeNone}
	OperandTypesRegisterToMemory                   = OperandTypes{OperandTypeRegister, OperandTypeMemory}
	OperandTypesMemoryToRegister                   = OperandTypes{OperandTypeMemory, OperandTypeRegister}
	OperandTypesConstToRegister                    = OperandTypes{OperandTypeConst, OperandTypeRegister}
	OperandTypesRegisterToVectorRegister           = OperandTypes{OperandTypeRegister, OperandTypeVectorRegister}
	OperandTypesVectorRegisterToRegister           = OperandTypes{OperandTypeVectorRegister, OperandTypeRegister}
	OperandTypesMemoryToVectorRegister             = OperandTypes{OperandTypeMemory, OperandTypeVectorRegister}
	OperandTypesVectorRegisterToMemory             = OperandTypes{OperandTypeVectorRegister, OperandTypeMemory}
	OperandTypesVectorRegisterToVectorRegister     = OperandTypes{OperandTypeVectorRegister, OperandTypeVectorRegister}
	OperandTypesTwoVectorRegistersToVectorRegister = OperandTypes{OperandTypeTwoVectorRegisters, OperandTypeVectorRegister}
	OperandTypesStaticConstToVectorRegister        = OperandTypes{OperandTypeStaticConst, OperandTypeVectorRegister}
)

// String implements fmt.Stringer
func (o OperandTypes) String() string {
	return fmt.Sprintf("from:%s,to:%s", o.src, o.dst)
}

// AssemblerImpl implements Assembler.
type AssemblerImpl struct {
	asm.BaseAssemblerImpl
	Root, Current     *NodeImpl
	Buf               *bytes.Buffer
	temporaryRegister asm.Register
	nodeCount         int
	pool              *asm.StaticConstPool
	// MaxDisplacementForConstantPool is fixed to defaultMaxDisplacementForConstPool
	// but have it as a field here for testability.
	MaxDisplacementForConstantPool int
}

func NewAssemblerImpl(temporaryRegister asm.Register) *AssemblerImpl {
	return &AssemblerImpl{
		Buf: bytes.NewBuffer(nil), temporaryRegister: temporaryRegister,
		pool:                           asm.NewStaticConstPool(),
		MaxDisplacementForConstantPool: defaultMaxDisplacementForConstPool,
	}
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
	return n
}

// addNode appends the new node into the linked list.
func (a *AssemblerImpl) addNode(node *NodeImpl) {
	a.nodeCount++

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

// Assemble implements asm.AssemblerBase
func (a *AssemblerImpl) Assemble() ([]byte, error) {
	// arm64 has 32-bit fixed length instructions,
	// but note that some nodes are encoded as multiple instructions,
	// so the resulting binary might not be the size of count*8.
	a.Buf.Grow(a.nodeCount * 8)

	for n := a.Root; n != nil; n = n.Next {
		n.OffsetInBinaryField = uint64(a.Buf.Len())
		if err := a.EncodeNode(n); err != nil {
			return nil, err
		}
		a.maybeFlushConstPool(n.Next == nil)
	}

	code := a.Bytes()
	for _, cb := range a.OnGenerateCallbacks {
		if err := cb(code); err != nil {
			return nil, err
		}
	}
	return code, nil
}

const defaultMaxDisplacementForConstPool = (1 << 20) - 1 - 4 // -4 for unconditional branch to skip the constants.

// maybeFlushConstPool flushes the constant pool if endOfBinary or a boundary condition was met.
func (a *AssemblerImpl) maybeFlushConstPool(endOfBinary bool) {
	if a.pool.FirstUseOffsetInBinary == nil {
		return
	}

	// If endOfBinary = true, we no longer need to emit the instructions, therefore
	// flush all the constants.
	if endOfBinary ||
		// Also, if the offset between the first usage of the constant pool and
		// the first constant would exceed 2^20 -1(= 2MiB-1), which is the maximum offset
		// for LDR(literal)/ADR instruction, flush all the constants in the pool.
		(a.Buf.Len()+a.pool.PoolSizeInBytes-int(*a.pool.FirstUseOffsetInBinary)) >= a.MaxDisplacementForConstantPool {

		// Before emitting consts, we have to add br instruction to skip the const pool.
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1123-L1129
		skipOffset := a.pool.PoolSizeInBytes/4 + 1
		if a.pool.PoolSizeInBytes%4 != 0 {
			skipOffset++
		}
		if endOfBinary {
			// If this is the end of binary, we never reach this block,
			// so offset can be zero (which is the behavior of Go's assembler).
			skipOffset = 0
		}

		a.Buf.Write([]byte{
			byte(skipOffset),
			byte(skipOffset >> 8),
			byte(skipOffset >> 16),
			0x14,
		})

		// Then adding the consts into the binary.
		for _, c := range a.pool.Consts {
			c.SetOffsetInBinary(uint64(a.Buf.Len()))
			a.Buf.Write(c.Raw)
		}

		// arm64 instructions are 4-byte (32-bit) aligned, so we must pad the zero consts here.
		if pad := a.Buf.Len() % 4; pad != 0 {
			a.Buf.Write(make([]byte, 4-pad))
		}

		// After the flush, reset the constant pool.
		a.pool = asm.NewStaticConstPool()
	}
}

// Bytes returns the encoded binary.
//
// Exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) Bytes() []byte {
	// 16 bytes alignment to match our impl with golang-asm.
	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L62
	//
	// TODO: Delete after golang-asm removal.
	if pad := 16 - a.Buf.Len()%16; pad > 0 && pad != 16 {
		a.Buf.Write(make([]byte, pad))
	}
	return a.Buf.Bytes()
}

// EncodeNode encodes the given node into writer.
func (a *AssemblerImpl) EncodeNode(n *NodeImpl) (err error) {
	switch n.Types {
	case OperandTypesNoneToNone:
		err = a.EncodeNoneToNone(n)
	case OperandTypesNoneToRegister, OperandTypesNoneToMemory:
		err = a.EncodeJumpToRegister(n)
	case OperandTypesNoneToBranch:
		err = a.EncodeRelativeBranch(n)
	case OperandTypesRegisterToRegister:
		err = a.EncodeRegisterToRegister(n)
	case OperandTypesLeftShiftedRegisterToRegister:
		err = a.EncodeLeftShiftedRegisterToRegister(n)
	case OperandTypesTwoRegistersToRegister:
		err = a.EncodeTwoRegistersToRegister(n)
	case OperandTypesThreeRegistersToRegister:
		err = a.EncodeThreeRegistersToRegister(n)
	case OperandTypesTwoRegistersToNone:
		err = a.EncodeTwoRegistersToNone(n)
	case OperandTypesRegisterAndConstToNone:
		err = a.EncodeRegisterAndConstToNone(n)
	case OperandTypesRegisterToMemory:
		err = a.EncodeRegisterToMemory(n)
	case OperandTypesMemoryToRegister:
		err = a.EncodeMemoryToRegister(n)
	case OperandTypesConstToRegister:
		err = a.EncodeConstToRegister(n)
	case OperandTypesRegisterToVectorRegister:
		err = a.EncodeRegisterToVectorRegister(n)
	case OperandTypesVectorRegisterToRegister:
		err = a.EncodeVectorRegisterToRegister(n)
	case OperandTypesMemoryToVectorRegister:
		err = a.EncodeMemoryToVectorRegister(n)
	case OperandTypesVectorRegisterToMemory:
		err = a.EncodeVectorRegisterToMemory(n)
	case OperandTypesVectorRegisterToVectorRegister:
		err = a.EncodeVectorRegisterToVectorRegister(n)
	case OperandTypesStaticConstToVectorRegister:
		err = a.EncodeStaticConstToVectorRegister(n)
	case OperandTypesTwoVectorRegistersToVectorRegister:
		err = a.encodeTwoVectorRegistersToVectorRegister(n)
	default:
		err = fmt.Errorf("encoder undefined for [%s] operand type", n.Types)
	}
	if err != nil {
		err = fmt.Errorf("%w: %s", err, n) // Ensure the error is debuggable by including the string value.
	}
	return
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
func (a *AssemblerImpl) CompileJumpToMemory(jmpInstruction asm.Instruction, baseReg asm.Register) {
	n := a.newNode(jmpInstruction, OperandTypesNoneToMemory)
	n.DstReg = baseReg
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
	n := a.newNode(ADR, OperandTypesMemoryToRegister)
	n.DstReg = destinationRegister
	n.readInstructionAddressBeforeTargetInstruction = beforeAcquisitionTargetInstruction
}

// CompileMemoryWithRegisterOffsetToRegister implements Assembler.CompileMemoryWithRegisterOffsetToRegister
func (a *AssemblerImpl) CompileMemoryWithRegisterOffsetToRegister(
	instruction asm.Instruction,
	srcBaseReg, srcOffsetReg, dstReg asm.Register,
) {
	n := a.newNode(instruction, OperandTypesMemoryToRegister)
	n.DstReg = dstReg
	n.SrcReg = srcBaseReg
	n.SrcReg2 = srcOffsetReg
}

// CompileRegisterToMemoryWithRegisterOffset implements Assembler.CompileRegisterToMemoryWithRegisterOffset
func (a *AssemblerImpl) CompileRegisterToMemoryWithRegisterOffset(
	instruction asm.Instruction,
	srcReg, dstBaseReg, dstOffsetReg asm.Register,
) {
	n := a.newNode(instruction, OperandTypesRegisterToMemory)
	n.SrcReg = srcReg
	n.DstReg = dstBaseReg
	n.DstReg2 = dstOffsetReg
}

// CompileTwoRegistersToRegister implements Assembler.CompileTwoRegistersToRegister
func (a *AssemblerImpl) CompileTwoRegistersToRegister(instruction asm.Instruction, src1, src2, dst asm.Register) {
	n := a.newNode(instruction, OperandTypesTwoRegistersToRegister)
	n.SrcReg = src1
	n.SrcReg2 = src2
	n.DstReg = dst
}

// CompileThreeRegistersToRegister implements Assembler.CompileThreeRegistersToRegister
func (a *AssemblerImpl) CompileThreeRegistersToRegister(
	instruction asm.Instruction,
	src1, src2, src3, dst asm.Register,
) {
	n := a.newNode(instruction, OperandTypesThreeRegistersToRegister)
	n.SrcReg = src1
	n.SrcReg2 = src2
	n.DstReg = src3 // To minimize the size of NodeImpl struct, we reuse DstReg for the third source operand.
	n.DstReg2 = dst
}

// CompileTwoRegistersToNone implements Assembler.CompileTwoRegistersToNone
func (a *AssemblerImpl) CompileTwoRegistersToNone(instruction asm.Instruction, src1, src2 asm.Register) {
	n := a.newNode(instruction, OperandTypesTwoRegistersToNone)
	n.SrcReg = src1
	n.SrcReg2 = src2
}

// CompileRegisterAndConstToNone implements Assembler.CompileRegisterAndConstToNone
func (a *AssemblerImpl) CompileRegisterAndConstToNone(
	instruction asm.Instruction,
	src asm.Register,
	srcConst asm.ConstantValue,
) {
	n := a.newNode(instruction, OperandTypesRegisterAndConstToNone)
	n.SrcReg = src
	n.SrcConst = srcConst
}

// CompileLeftShiftedRegisterToRegister implements Assembler.CompileLeftShiftedRegisterToRegister
func (a *AssemblerImpl) CompileLeftShiftedRegisterToRegister(
	instruction asm.Instruction,
	shiftedSourceReg asm.Register,
	shiftNum asm.ConstantValue,
	srcReg, dstReg asm.Register,
) {
	n := a.newNode(instruction, OperandTypesLeftShiftedRegisterToRegister)
	n.SrcReg = srcReg
	n.SrcReg2 = shiftedSourceReg
	n.SrcConst = shiftNum
	n.DstReg = dstReg
}

// CompileConditionalRegisterSet implements Assembler.CompileConditionalRegisterSet
func (a *AssemblerImpl) CompileConditionalRegisterSet(cond asm.ConditionalRegisterState, dstReg asm.Register) {
	n := a.newNode(CSET, OperandTypesRegisterToRegister)
	n.SrcReg = conditionalRegisterStateToRegister(cond)
	n.DstReg = dstReg
}

// CompileMemoryToVectorRegister implements Assembler.CompileMemoryToVectorRegister
func (a *AssemblerImpl) CompileMemoryToVectorRegister(
	instruction asm.Instruction, srcBaseReg asm.Register, dstOffset asm.ConstantValue, dstReg asm.Register, arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesMemoryToVectorRegister)
	n.SrcReg = srcBaseReg
	n.SrcConst = dstOffset
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

// CompileMemoryWithRegisterOffsetToVectorRegister implements Assembler.CompileMemoryWithRegisterOffsetToVectorRegister
func (a *AssemblerImpl) CompileMemoryWithRegisterOffsetToVectorRegister(instruction asm.Instruction,
	srcBaseReg, srcOffsetRegister asm.Register, dstReg asm.Register, arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesMemoryToVectorRegister)
	n.SrcReg = srcBaseReg
	n.SrcReg2 = srcOffsetRegister
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

// CompileVectorRegisterToMemory implements Assembler.CompileVectorRegisterToMemory
func (a *AssemblerImpl) CompileVectorRegisterToMemory(
	instruction asm.Instruction, srcReg, dstBaseReg asm.Register, dstOffset asm.ConstantValue, arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesVectorRegisterToMemory)
	n.SrcReg = srcReg
	n.DstReg = dstBaseReg
	n.DstConst = dstOffset
	n.VectorArrangement = arrangement
}

// CompileVectorRegisterToMemoryWithRegisterOffset implements Assembler.CompileVectorRegisterToMemoryWithRegisterOffset
func (a *AssemblerImpl) CompileVectorRegisterToMemoryWithRegisterOffset(instruction asm.Instruction,
	srcReg, dstBaseReg, dstOffsetRegister asm.Register, arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesVectorRegisterToMemory)
	n.SrcReg = srcReg
	n.DstReg = dstBaseReg
	n.DstReg2 = dstOffsetRegister
	n.VectorArrangement = arrangement
}

// CompileRegisterToVectorRegister implements Assembler.CompileRegisterToVectorRegister
func (a *AssemblerImpl) CompileRegisterToVectorRegister(
	instruction asm.Instruction, srcReg, dstReg asm.Register, arrangement VectorArrangement, index VectorIndex) {
	n := a.newNode(instruction, OperandTypesRegisterToVectorRegister)
	n.SrcReg = srcReg
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
	n.DstVectorIndex = index
}

// CompileVectorRegisterToRegister implements Assembler.CompileVectorRegisterToRegister
func (a *AssemblerImpl) CompileVectorRegisterToRegister(instruction asm.Instruction, srcReg, dstReg asm.Register,
	arrangement VectorArrangement, index VectorIndex) {
	n := a.newNode(instruction, OperandTypesVectorRegisterToRegister)
	n.SrcReg = srcReg
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
	n.SrcVectorIndex = index
}

// CompileVectorRegisterToVectorRegister implements Assembler.CompileVectorRegisterToVectorRegister
func (a *AssemblerImpl) CompileVectorRegisterToVectorRegister(
	instruction asm.Instruction, srcReg, dstReg asm.Register, arrangement VectorArrangement, srcIndex, dstIndex VectorIndex) {
	n := a.newNode(instruction, OperandTypesVectorRegisterToVectorRegister)
	n.SrcReg = srcReg
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
	n.SrcVectorIndex = srcIndex
	n.DstVectorIndex = dstIndex
}

// CompileVectorRegisterToVectorRegisterWithConst implements Assembler.CompileVectorRegisterToVectorRegisterWithConst
func (a *AssemblerImpl) CompileVectorRegisterToVectorRegisterWithConst(instruction asm.Instruction,
	srcReg, dstReg asm.Register, arrangement VectorArrangement, c asm.ConstantValue) {
	n := a.newNode(instruction, OperandTypesVectorRegisterToVectorRegister)
	n.SrcReg = srcReg
	n.SrcConst = c
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

// CompileStaticConstToRegister implements Assembler.CompileStaticConstToVectorRegister
func (a *AssemblerImpl) CompileStaticConstToRegister(instruction asm.Instruction, c *asm.StaticConst, dstReg asm.Register) {
	n := a.newNode(instruction, OperandTypesMemoryToRegister)
	n.staticConst = c
	n.DstReg = dstReg
}

// CompileStaticConstToVectorRegister implements Assembler.CompileStaticConstToVectorRegister
func (a *AssemblerImpl) CompileStaticConstToVectorRegister(instruction asm.Instruction,
	c *asm.StaticConst, dstReg asm.Register, arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesStaticConstToVectorRegister)
	n.staticConst = c
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

// CompileTwoVectorRegistersToVectorRegister implements Assembler.CompileTwoVectorRegistersToVectorRegister.
func (a *AssemblerImpl) CompileTwoVectorRegistersToVectorRegister(instruction asm.Instruction, srcReg, srcReg2, dstReg asm.Register,
	arrangement VectorArrangement) {
	n := a.newNode(instruction, OperandTypesTwoVectorRegistersToVectorRegister)
	n.SrcReg = srcReg
	n.SrcReg2 = srcReg2
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

// CompileTwoVectorRegistersToVectorRegisterWithConst implements Assembler.CompileTwoVectorRegistersToVectorRegisterWithConst.
func (a *AssemblerImpl) CompileTwoVectorRegistersToVectorRegisterWithConst(instruction asm.Instruction,
	srcReg, srcReg2, dstReg asm.Register, arrangement VectorArrangement, c asm.ConstantValue) {
	n := a.newNode(instruction, OperandTypesTwoVectorRegistersToVectorRegister)
	n.SrcReg = srcReg
	n.SrcReg2 = srcReg2
	n.SrcConst = c
	n.DstReg = dstReg
	n.VectorArrangement = arrangement
}

func errorEncodingUnsupported(n *NodeImpl) error {
	return fmt.Errorf("%s is unsupported for %s type", InstructionName(n.Instruction), n.Types)
}

// EncodeNoneToNone is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeNoneToNone(n *NodeImpl) (err error) {
	if n.Instruction != NOP {
		err = errorEncodingUnsupported(n)
	}
	return
}

// EncodeJumpToRegister is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeJumpToRegister(n *NodeImpl) (err error) {
	// "Unconditional branch (register)" in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Branches--Exception-Generating-and-System-instructions
	var opc byte
	switch n.Instruction {
	case RET:
		opc = 0b0010
	case B:
		opc = 0b0000
	default:
		return errorEncodingUnsupported(n)
	}

	regBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return fmt.Errorf("invalid destination register: %w", err)
	}

	a.Buf.Write([]byte{
		0x00 | (regBits << 5),
		0x00 | (regBits >> 3),
		0b000_11111 | (opc << 5),
		0b1101011_0 | (opc >> 3),
	})
	return
}

// EncodeRelativeBranch is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeRelativeBranch(n *NodeImpl) (err error) {
	switch n.Instruction {
	case B, BCONDEQ, BCONDGE, BCONDGT, BCONDHI, BCONDHS, BCONDLE, BCONDLO, BCONDLS, BCONDLT, BCONDMI, BCONDNE, BCONDVS, BCONDPL:
	default:
		return errorEncodingUnsupported(n)
	}

	if n.JumpTarget == nil {
		return fmt.Errorf("branch target must be set for %s", InstructionName(n.Instruction))
	}

	// At this point, we don't yet know that target's branch, so emit the placeholder (4 bytes).
	a.Buf.Write([]byte{0, 0, 0, 0})

	a.AddOnGenerateCallBack(func(code []byte) error {
		var condBits byte
		const condBitsUnconditional = 0xff // Indicates this is not conditional jump.

		// https://developer.arm.com/documentation/den0024/a/CHDEEABE
		switch n.Instruction {
		case B:
			condBits = condBitsUnconditional
		case BCONDEQ:
			condBits = 0b0000
		case BCONDGE:
			condBits = 0b1010
		case BCONDGT:
			condBits = 0b1100
		case BCONDHI:
			condBits = 0b1000
		case BCONDHS:
			condBits = 0b0010
		case BCONDLE:
			condBits = 0b1101
		case BCONDLO:
			condBits = 0b0011
		case BCONDLS:
			condBits = 0b1001
		case BCONDLT:
			condBits = 0b1011
		case BCONDMI:
			condBits = 0b0100
		case BCONDPL:
			condBits = 0b0101
		case BCONDNE:
			condBits = 0b0001
		case BCONDVS:
			condBits = 0b0110
		}

		branchInstOffset := int64(n.OffsetInBinary())
		offset := int64(n.JumpTarget.OffsetInBinary()) - branchInstOffset
		if offset%4 != 0 {
			return errors.New("BUG: relative jump offset must be 4 bytes aligned")
		}

		branchInst := code[branchInstOffset : branchInstOffset+4]
		if condBits == condBitsUnconditional {
			imm26 := offset / 4
			const maxSignedInt26 int64 = 1<<25 - 1
			const minSignedInt26 int64 = -(1 << 25)
			if offset < minSignedInt26 || offset > maxSignedInt26 {
				// In theory this could happen if a Wasm binary has a huge single label (more than 128MB for a single block),
				// and in that case, we use load the offset into a register and do the register jump, but to avoid the complexity,
				// we impose this limit for now as that would be *unlikely* happen in practice.
				return fmt.Errorf("relative jump offset %d/4 must be within %d and %d", offset, minSignedInt26, maxSignedInt26)
			}
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/B--Branch-?lang=en
			branchInst[0] = byte(imm26)
			branchInst[1] = byte(imm26 >> 8)
			branchInst[2] = byte(imm26 >> 16)
			branchInst[3] = (byte(imm26 >> 24 & 0b000000_11)) | 0b000101_00
		} else {
			imm19 := offset / 4
			const maxSignedInt19 int64 = 1<<19 - 1
			const minSignedInt19 int64 = -(1 << 19)
			if offset < minSignedInt19 || offset > maxSignedInt19 {
				// This should be a bug in our compiler as the conditional jumps are only used in the small offsets (~a few bytes),
				// and if ever happens, compiler can be fixed.
				return fmt.Errorf("BUG: relative jump offset %d/4(=%d)must be within %d and %d", offset, imm19, minSignedInt19, maxSignedInt19)
			}
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/B-cond--Branch-conditionally-?lang=en
			branchInst[0] = (byte(imm19<<5) & 0b111_0_0000) | condBits
			branchInst[1] = byte(imm19 >> 3)
			branchInst[2] = byte(imm19 >> 11)
			branchInst[3] = 0b01010100
		}
		return nil
	})
	return
}

func checkRegisterToRegisterType(src, dst asm.Register, requireSrcInt, requireDstInt bool) (err error) {
	isSrcInt, isDstInt := isIntRegister(src), isIntRegister(dst)
	if isSrcInt && !requireSrcInt {
		err = fmt.Errorf("src requires float register but got %s", RegisterName(src))
	} else if !isSrcInt && requireSrcInt {
		err = fmt.Errorf("src requires int register but got %s", RegisterName(src))
	} else if isDstInt && !requireDstInt {
		err = fmt.Errorf("dst requires float register but got %s", RegisterName(dst))
	} else if !isDstInt && requireDstInt {
		err = fmt.Errorf("dst requires int register but got %s", RegisterName(dst))
	}
	return
}

// EncodeRegisterToRegister is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeRegisterToRegister(n *NodeImpl) (err error) {
	switch inst := n.Instruction; inst {
	case ADD, ADDW, SUB:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		var sfops byte
		switch inst {
		case ADD:
			sfops = 0b100
		case ADDW:
		case SUB:
			sfops = 0b110
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)
		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			dstRegBits >> 3,
			srcRegBits,
			(sfops << 5) | 0b01011,
		})
	case CLZ, CLZW, RBIT, RBITW:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		var sf, opcode byte
		switch inst {
		case CLZ:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CLZ--Count-Leading-Zeros-?lang=en
			sf, opcode = 0b1, 0b000_100
		case CLZW:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CLZ--Count-Leading-Zeros-?lang=en
			sf, opcode = 0b0, 0b000_100
		case RBIT:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/RBIT--Reverse-Bits-?lang=en
			sf, opcode = 0b1, 0b000_000
		case RBITW:
			// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/RBIT--Reverse-Bits-?lang=en
			sf, opcode = 0b0, 0b000_000
		}
		if inst == CLZ {
			sf = 1
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)
		a.Buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			opcode<<2 | (srcRegBits >> 3),
			0b110_00000,
			(sf << 7) | 0b0_1011010,
		})
	case CSET:
		if !isConditionalRegister(n.SrcReg) {
			return fmt.Errorf("CSET requires conditional register but got %s", RegisterName(n.SrcReg))
		}

		dstRegBits, err := intRegisterBits(n.DstReg)
		if err != nil {
			return err
		}

		// CSET encodes the conditional bits with its least significant bit inverted.
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CSET--Conditional-Set--an-alias-of-CSINC-?lang=en
		//
		// https://developer.arm.com/documentation/den0024/a/CHDEEABE
		var conditionalBits byte
		switch n.SrcReg {
		case RegCondEQ:
			conditionalBits = 0b0001
		case RegCondNE:
			conditionalBits = 0b0000
		case RegCondHS:
			conditionalBits = 0b0011
		case RegCondLO:
			conditionalBits = 0b0010
		case RegCondMI:
			conditionalBits = 0b0101
		case RegCondPL:
			conditionalBits = 0b0100
		case RegCondVS:
			conditionalBits = 0b0111
		case RegCondVC:
			conditionalBits = 0b0110
		case RegCondHI:
			conditionalBits = 0b1001
		case RegCondLS:
			conditionalBits = 0b1000
		case RegCondGE:
			conditionalBits = 0b1011
		case RegCondLT:
			conditionalBits = 0b1010
		case RegCondGT:
			conditionalBits = 0b1101
		case RegCondLE:
			conditionalBits = 0b1100
		case RegCondAL:
			conditionalBits = 0b1111
		case RegCondNV:
			conditionalBits = 0b1110
		}

		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CSET--Conditional-Set--an-alias-of-CSINC-?lang=en
		a.Buf.Write([]byte{
			0b111_00000 | dstRegBits,
			(conditionalBits << 4) | 0b0000_0111,
			0b100_11111,
			0b10011010,
		})

	case FABSD, FABSS, FNEGD, FNEGS, FSQRTD, FSQRTS, FCVTSD, FCVTDS, FRINTMD, FRINTMS,
		FRINTND, FRINTNS, FRINTPD, FRINTPS, FRINTZD, FRINTZS:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, false, false); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var tp, opcode byte
		switch inst {
		case FABSD:
			opcode, tp = 0b000001, 0b01
		case FABSS:
			opcode, tp = 0b000001, 0b00
		case FNEGD:
			opcode, tp = 0b000010, 0b01
		case FNEGS:
			opcode, tp = 0b000010, 0b00
		case FSQRTD:
			opcode, tp = 0b000011, 0b01
		case FSQRTS:
			opcode, tp = 0b000011, 0b00
		case FCVTSD:
			opcode, tp = 0b000101, 0b00
		case FCVTDS:
			opcode, tp = 0b000100, 0b01
		case FRINTMD:
			opcode, tp = 0b001010, 0b01
		case FRINTMS:
			opcode, tp = 0b001010, 0b00
		case FRINTND:
			opcode, tp = 0b001000, 0b01
		case FRINTNS:
			opcode, tp = 0b001000, 0b00
		case FRINTPD:
			opcode, tp = 0b001001, 0b01
		case FRINTPS:
			opcode, tp = 0b001001, 0b00
		case FRINTZD:
			opcode, tp = 0b001011, 0b01
		case FRINTZS:
			opcode, tp = 0b001011, 0b00
		}
		a.Buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			(opcode << 7) | 0b0_10000_00 | (srcRegBits >> 3),
			tp<<6 | 0b00_1_00000 | opcode>>1,
			0b0_00_11110,
		})

	case FADDD, FADDS, FDIVS, FDIVD, FMAXD, FMAXS, FMIND, FMINS, FMULS, FMULD:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, false, false); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		// "Floating-point data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var tp, opcode byte
		switch inst {
		case FADDD:
			opcode, tp = 0b0010, 0b01
		case FADDS:
			opcode, tp = 0b0010, 0b00
		case FDIVD:
			opcode, tp = 0b0001, 0b01
		case FDIVS:
			opcode, tp = 0b0001, 0b00
		case FMAXD:
			opcode, tp = 0b0100, 0b01
		case FMAXS:
			opcode, tp = 0b0100, 0b00
		case FMIND:
			opcode, tp = 0b0101, 0b01
		case FMINS:
			opcode, tp = 0b0101, 0b00
		case FMULS:
			opcode, tp = 0b0000, 0b00
		case FMULD:
			opcode, tp = 0b0000, 0b01
		}

		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			opcode<<4 | 0b0000_10_00 | (dstRegBits >> 3),
			tp<<6 | 0b00_1_00000 | srcRegBits,
			0b0001_1110,
		})

	case FCVTZSD, FCVTZSDW, FCVTZSS, FCVTZSSW, FCVTZUD, FCVTZUDW, FCVTZUS, FCVTZUSW:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, false, true); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		// "Conversion between floating-point and integer" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var sf, tp, opcode byte
		switch inst {
		case FCVTZSD: // Double to signed 64-bit
			sf, tp, opcode = 0b1, 0b01, 0b000
		case FCVTZSDW: // Double to signed 32-bit.
			sf, tp, opcode = 0b0, 0b01, 0b000
		case FCVTZSS: // Single to signed 64-bit.
			sf, tp, opcode = 0b1, 0b00, 0b000
		case FCVTZSSW: // Single to signed 32-bit.
			sf, tp, opcode = 0b0, 0b00, 0b000
		case FCVTZUD: // Double to unsigned 64-bit.
			sf, tp, opcode = 0b1, 0b01, 0b001
		case FCVTZUDW: // Double to unsigned 32-bit.
			sf, tp, opcode = 0b0, 0b01, 0b001
		case FCVTZUS: // Single to unsigned 64-bit.
			sf, tp, opcode = 0b1, 0b00, 0b001
		case FCVTZUSW: // Single to unsigned 32-bit.
			sf, tp, opcode = 0b0, 0b00, 0b001
		}

		a.Buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			0 | (srcRegBits >> 3),
			tp<<6 | 0b00_1_11_000 | opcode,
			sf<<7 | 0b0_0_0_11110,
		})

	case FMOVD, FMOVS:
		isSrcInt, isDstInt := isIntRegister(n.SrcReg), isIntRegister(n.DstReg)
		if isSrcInt && isDstInt {
			return errors.New("FMOV needs at least one of operands to be integer")
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMOV--register---Floating-point-Move-register-without-conversion-?lang=en
		if !isSrcInt && !isDstInt { // Float to float.
			var tp byte
			if inst == FMOVD {
				tp = 0b01
			}
			a.Buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				0b0_10000_00 | (srcRegBits >> 3),
				tp<<6 | 0b00_1_00000,
				0b000_11110,
			})
		} else if isSrcInt && !isDstInt { // Int to float.
			var tp, sf byte
			if inst == FMOVD {
				tp, sf = 0b01, 0b1
			}
			a.Buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				srcRegBits >> 3,
				tp<<6 | 0b00_1_00_111,
				sf<<7 | 0b0_00_11110,
			})
		} else { // Float to int.
			var tp, sf byte
			if inst == FMOVD {
				tp, sf = 0b01, 0b1
			}
			a.Buf.Write([]byte{
				(srcRegBits << 5) | dstRegBits,
				srcRegBits >> 3,
				tp<<6 | 0b00_1_00_110,
				sf<<7 | 0b0_00_11110,
			})
		}

	case MOVD, MOVWU:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)
		if n.SrcReg == RegRZR && inst == MOVD {
			// If this is 64-bit mov from zero register, then we encode this as MOVK.
			// See "Move wide (immediate)" in
			// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Immediate
			a.Buf.Write([]byte{
				dstRegBits,
				0x0,
				0b1000_0000,
				0b1_10_10010,
			})
		} else {
			// MOV can be encoded as ORR (shifted register): "ORR Wd, WZR, Wm".
			// https://developer.arm.com/documentation/100069/0609/A64-General-Instructions/MOV--register-
			var sf byte
			if inst == MOVD {
				sf = 0b1
			}
			a.Buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | srcRegBits,
				sf<<7 | 0b0_01_01010,
			})
		}

	case MRS:
		if n.SrcReg != RegFPSR {
			return fmt.Errorf("MRS has only support for FPSR register as a src but got %s", RegisterName(n.SrcReg))
		}

		// For how to specify FPSR register, see "Accessing FPSR" in:
		// https://developer.arm.com/documentation/ddi0595/2021-12/AArch64-Registers/FPSR--Floating-point-Status-Register?lang=en
		dstRegBits := registerBits(n.DstReg)
		a.Buf.Write([]byte{
			0b001<<5 | dstRegBits,
			0b0100<<4 | 0b0100,
			0b0011_0000 | 0b11<<3 | 0b011,
			0b1101_0101,
		})

	case MSR:
		if n.DstReg != RegFPSR {
			return fmt.Errorf("MSR has only support for FPSR register as a dst but got %s", RegisterName(n.SrcReg))
		}

		// For how to specify FPSR register, see "Accessing FPSR" in:
		// https://developer.arm.com/documentation/ddi0595/2021-12/AArch64-Registers/FPSR--Floating-point-Status-Register?lang=en
		srcRegBits := registerBits(n.SrcReg)
		a.Buf.Write([]byte{
			0b001<<5 | srcRegBits,
			0b0100<<4 | 0b0100,
			0b0001_0000 | 0b11<<3 | 0b011,
			0b1101_0101,
		})

	case MUL, MULW:
		// Multiplications are encoded as MADD (zero register, src, dst), dst = zero + (src * dst) = src * dst.
		// See "Data-processing (3 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		var sf byte
		if inst == MUL {
			sf = 0b1
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		a.Buf.Write([]byte{
			dstRegBits<<5 | dstRegBits,
			zeroRegisterBits<<2 | dstRegBits>>3,
			srcRegBits,
			sf<<7 | 0b11011,
		})

	case NEG, NEGW:
		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		// NEG is encoded as "SUB dst, XZR, src" = "dst = 0 - src"
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		var sf byte
		if inst == NEG {
			sf = 0b1
		}

		a.Buf.Write([]byte{
			(zeroRegisterBits << 5) | dstRegBits,
			zeroRegisterBits >> 3,
			srcRegBits,
			sf<<7 | 0b0_10_00000 | 0b0_00_01011,
		})

	case SDIV, SDIVW, UDIV, UDIVW:
		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf, opcode byte
		switch inst {
		case SDIV:
			sf, opcode = 0b1, 0b000011
		case SDIVW:
			sf, opcode = 0b0, 0b000011
		case UDIV:
			sf, opcode = 0b1, 0b000010
		case UDIVW:
			sf, opcode = 0b0, 0b000010
		}

		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			opcode<<2 | (dstRegBits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})

	case SCVTFD, SCVTFWD, SCVTFS, SCVTFWS, UCVTFD, UCVTFS, UCVTFWD, UCVTFWS:
		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)

		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, false); err != nil {
			return
		}

		// "Conversion between floating-point and integer" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en#floatdp1
		var sf, tp, opcode byte
		switch inst {
		case SCVTFD: // 64-bit integer to double
			sf, tp, opcode = 0b1, 0b01, 0b010
		case SCVTFWD: // 32-bit integer to double
			sf, tp, opcode = 0b0, 0b01, 0b010
		case SCVTFS: // 64-bit integer to single
			sf, tp, opcode = 0b1, 0b00, 0b010
		case SCVTFWS: // 32-bit integer to single
			sf, tp, opcode = 0b0, 0b00, 0b010
		case UCVTFD: // 64-bit to double
			sf, tp, opcode = 0b1, 0b01, 0b011
		case UCVTFWD: // 32-bit to double
			sf, tp, opcode = 0b0, 0b01, 0b011
		case UCVTFS: // 64-bit to single
			sf, tp, opcode = 0b1, 0b00, 0b011
		case UCVTFWS: // 32-bit to single
			sf, tp, opcode = 0b0, 0b00, 0b011
		}

		a.Buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			srcRegBits >> 3,
			tp<<6 | 0b00_1_00_000 | opcode,
			sf<<7 | 0b0_0_0_11110,
		})

	case SXTB, SXTBW, SXTH, SXTHW, SXTW:
		if err = checkRegisterToRegisterType(n.SrcReg, n.DstReg, true, true); err != nil {
			return
		}

		srcRegBits, dstRegBits := registerBits(n.SrcReg), registerBits(n.DstReg)
		if n.SrcReg == RegRZR {
			// If the source is zero register, we encode as MOV dst, zero.
			var sf byte
			if inst == MOVD {
				sf = 0b1
			}
			a.Buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | srcRegBits,
				sf<<7 | 0b0_01_01010,
			})
			return
		}

		// SXTB is encoded as "SBFM Wd, Wn, #0, #7"
		// https://developer.arm.com/documentation/dui0801/g/A64-General-Instructions/SXTB
		// SXTH is encoded as "SBFM Wd, Wn, #0, #15"
		// https://developer.arm.com/documentation/dui0801/g/A64-General-Instructions/SXTH
		// SXTW is encoded as "SBFM Xd, Xn, #0, #31"
		// https://developer.arm.com/documentation/dui0802/b/A64-General-Instructions/SXTW

		var n, sf, imms, opc byte
		switch inst {
		case SXTB:
			n, sf, imms = 0b1, 0b1, 0x7
		case SXTBW:
			n, sf, imms = 0b0, 0b0, 0x7
		case SXTH:
			n, sf, imms = 0b1, 0b1, 0xf
		case SXTHW:
			n, sf, imms = 0b0, 0b0, 0xf
		case SXTW:
			n, sf, imms = 0b1, 0b1, 0x1f
		}

		a.Buf.Write([]byte{
			(srcRegBits << 5) | dstRegBits,
			imms<<2 | (srcRegBits >> 3),
			n << 6,
			sf<<7 | opc<<5 | 0b10011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

// EncodeLeftShiftedRegisterToRegister is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeLeftShiftedRegisterToRegister(n *NodeImpl) (err error) {

	baseRegBits, err := intRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}
	shiftTargetRegBits, err := intRegisterBits(n.SrcReg2)
	if err != nil {
		return err
	}
	dstRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	switch n.Instruction {
	case ADD:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		const logicalLeftShiftBits = 0b00
		if n.SrcConst < 0 || n.SrcConst > 64 {
			return fmt.Errorf("shift amount must fit in unsigned 6-bit integer (0-64) but got %d", n.SrcConst)
		}
		shiftByte := byte(n.SrcConst)
		a.Buf.Write([]byte{
			(baseRegBits << 5) | dstRegBits,
			(shiftByte << 2) | (baseRegBits >> 3),
			(logicalLeftShiftBits << 6) | shiftTargetRegBits,
			0b1000_1011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

// EncodeTwoRegistersToRegister is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeTwoRegistersToRegister(n *NodeImpl) (err error) {
	switch inst := n.Instruction; inst {
	case AND, ANDW, ORR, ORRW, EOR, EORW:
		// See "Logical (shifted register)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.SrcReg), registerBits(n.SrcReg2), registerBits(n.DstReg)
		var sf, opc byte
		switch inst {
		case AND:
			sf, opc = 0b1, 0b00
		case ANDW:
			sf, opc = 0b0, 0b00
		case ORR:
			sf, opc = 0b1, 0b01
		case ORRW:
			sf, opc = 0b0, 0b01
		case EOR:
			sf, opc = 0b1, 0b10
		case EORW:
			sf, opc = 0b0, 0b10
		}
		a.Buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			srcReg2Bits >> 3,
			srcRegBits,
			sf<<7 | opc<<5 | 0b01010,
		})
	case ASR, ASRW, LSL, LSLW, LSR, LSRW, ROR, RORW:
		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.SrcReg), registerBits(n.SrcReg2), registerBits(n.DstReg)

		var sf, opcode byte
		switch inst {
		case ASR:
			sf, opcode = 0b1, 0b001010
		case ASRW:
			sf, opcode = 0b0, 0b001010
		case LSL:
			sf, opcode = 0b1, 0b001000
		case LSLW:
			sf, opcode = 0b0, 0b001000
		case LSR:
			sf, opcode = 0b1, 0b001001
		case LSRW:
			sf, opcode = 0b0, 0b001001
		case ROR:
			sf, opcode = 0b1, 0b001011
		case RORW:
			sf, opcode = 0b0, 0b001011
		}
		a.Buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			opcode<<2 | (srcReg2Bits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})
	case SDIV, SDIVW, UDIV, UDIVW:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.SrcReg), registerBits(n.SrcReg2), registerBits(n.DstReg)

		// See "Data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0602/2021-06/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf, opcode byte
		switch inst {
		case SDIV:
			sf, opcode = 0b1, 0b000011
		case SDIVW:
			sf, opcode = 0b0, 0b000011
		case UDIV:
			sf, opcode = 0b1, 0b000010
		case UDIVW:
			sf, opcode = 0b0, 0b000010
		}

		a.Buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			opcode<<2 | (srcReg2Bits >> 3),
			0b110_00000 | srcRegBits,
			sf<<7 | 0b0_00_11010,
		})
	case SUB, SUBW:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.SrcReg), registerBits(n.SrcReg2), registerBits(n.DstReg)

		// See "Add/subtract (shifted register)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		var sf byte
		if inst == SUB {
			sf = 0b1
		}

		a.Buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			srcReg2Bits >> 3,
			srcRegBits,
			sf<<7 | 0b0_10_01011,
		})
	case FSUBD, FSUBS:
		srcRegBits, srcReg2Bits, dstRegBits := registerBits(n.SrcReg), registerBits(n.SrcReg2), registerBits(n.DstReg)

		// See "Floating-point data-processing (2 source)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		var tp byte
		if inst == FSUBD {
			tp = 0b01
		}
		a.Buf.Write([]byte{
			(srcReg2Bits << 5) | dstRegBits,
			0b0011_10_00 | (srcReg2Bits >> 3),
			tp<<6 | 0b00_1_00000 | srcRegBits,
			0b0_00_11110,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

// EncodeThreeRegistersToRegister is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeThreeRegistersToRegister(n *NodeImpl) (err error) {
	switch n.Instruction {
	case MSUB, MSUBW:
		// Dst = Src2 - (Src1 * Src3)
		// "Data-processing (3 source)" in:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en
		src1RegBits, err := intRegisterBits(n.SrcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := intRegisterBits(n.SrcReg2)
		if err != nil {
			return err
		}
		src3RegBits, err := intRegisterBits(n.DstReg)
		if err != nil {
			return err
		}
		dstRegBits, err := intRegisterBits(n.DstReg2)
		if err != nil {
			return err
		}

		var sf byte // is zero for MSUBW (32-bit MSUB).
		if n.Instruction == MSUB {
			sf = 0b1
		}

		a.Buf.Write([]byte{
			(src3RegBits << 5) | dstRegBits,
			0b1_0000000 | (src2RegBits << 2) | (src3RegBits >> 3),
			src1RegBits,
			sf<<7 | 0b00_11011,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

// EncodeTwoRegistersToNone is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeTwoRegistersToNone(n *NodeImpl) (err error) {
	switch n.Instruction {
	case CMPW, CMP:
		// Compare on two registers is an alias for "SUBS (src1, src2) ZERO"
		// which can be encoded as SUBS (shifted registers) with zero shifting.
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Register?lang=en#addsub_shift
		src1RegBits, err := intRegisterBits(n.SrcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := intRegisterBits(n.SrcReg2)
		if err != nil {
			return err
		}

		var op byte
		if n.Instruction == CMP {
			op = 0b111
		} else {
			op = 0b011
		}

		a.Buf.Write([]byte{
			(src2RegBits << 5) | zeroRegisterBits,
			src2RegBits >> 3,
			src1RegBits,
			0b01011 | (op << 5),
		})
	case FCMPS, FCMPD:
		// "Floating-point compare" section in:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		src1RegBits, err := vectorRegisterBits(n.SrcReg)
		if err != nil {
			return err
		}
		src2RegBits, err := vectorRegisterBits(n.SrcReg2)
		if err != nil {
			return err
		}

		var ftype byte // is zero for FCMPS (single precision float compare).
		if n.Instruction == FCMPD {
			ftype = 0b01
		}
		a.Buf.Write([]byte{
			src2RegBits << 5,
			0b001000_00 | (src2RegBits >> 3),
			ftype<<6 | 0b1_00000 | src1RegBits,
			0b000_11110,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

// EncodeRegisterAndConstToNone is exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeRegisterAndConstToNone(n *NodeImpl) (err error) {
	if n.Instruction != CMP {
		return errorEncodingUnsupported(n)
	}

	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/CMP--immediate---Compare--immediate---an-alias-of-SUBS--immediate--?lang=en
	if n.SrcConst < 0 || n.SrcConst > 4095 {
		return fmt.Errorf("immediate for CMP must fit in 0 to 4095 but got %d", n.SrcConst)
	} else if n.SrcReg == RegRZR {
		return errors.New("zero register is not supported for CMP (immediate)")
	}

	srcRegBits, err := intRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	a.Buf.Write([]byte{
		(srcRegBits << 5) | zeroRegisterBits,
		(byte(n.SrcConst) << 2) | (srcRegBits >> 3),
		byte(n.SrcConst >> 6),
		0b111_10001,
	})
	return
}

func fitInSigned9Bits(v int64) bool {
	return v >= -256 && v <= 255
}

func (a *AssemblerImpl) encodeLoadOrStoreWithRegisterOffset(
	baseRegBits, offsetRegBits, targetRegBits byte, opcode, size, v byte) {
	// See "Load/store register (register offset)".
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_regoff
	a.Buf.Write([]byte{
		(baseRegBits << 5) | targetRegBits,
		0b011_010_00 | (baseRegBits >> 3),
		opcode<<6 | 0b00_1_00000 | offsetRegBits,
		size<<6 | v<<2 | 0b00_111_0_00,
	})
}

// validateMemoryOffset validates the memory offset if the given offset can be encoded in the assembler.
// In theory, offset can be any, but for simplicity of our homemade assembler, we limit the offset range
// that can be encoded enough for supporting compiler.
func validateMemoryOffset(offset int64) (err error) {
	if offset > 255 && offset%4 != 0 {
		// This is because we only have large offsets for load/store with Wasm value stack or reading type IDs, and its offset
		// is always multiplied by 4 or 8 (== the size of uint32 or uint64 == the type of wasm.FunctionTypeID or value stack in Go)
		err = fmt.Errorf("large memory offset (>255) must be a multiple of 4 but got %d", offset)
	} else if offset < -256 { // 9-bit signed integer's minimum = 2^8.
		err = fmt.Errorf("negative memory offset must be larget than or equal -256 but got %d", offset)
	} else if offset > 1<<31-1 {
		return fmt.Errorf("large memory offset must be less than %d but got %d", 1<<31-1, offset)
	}
	return
}

// encodeLoadOrStoreWithConstOffset encodes load/store instructions with the constant offset.
//
// Note: Encoding strategy intentionally matches the Go assembler: https://go.dev/doc/asm
func (a *AssemblerImpl) encodeLoadOrStoreWithConstOffset(
	baseRegBits, targetRegBits byte,
	offset int64,
	opcode, size, v byte,
	datasize, datasizeLog2 int64,
) (err error) {
	if err = validateMemoryOffset(offset); err != nil {
		return
	}

	if fitInSigned9Bits(offset) {
		// See "LDAPR/STLR (unscaled immediate)"
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldapstl_unscaled
		if offset < 0 || offset%datasize != 0 {
			// This case is encoded as one "unscaled signed store".
			a.Buf.Write([]byte{
				(baseRegBits << 5) | targetRegBits,
				byte(offset<<4) | (baseRegBits >> 3),
				opcode<<6 | (0b00_00_11111 & byte(offset>>4)),
				size<<6 | v<<2 | 0b00_1_11_0_00,
			})
			return
		}
	}

	// At this point we have the assumption that offset is positive and multiple of datasize.
	if offset < (1<<12)<<datasizeLog2 {
		// This case can be encoded as a single "unsigned immediate".
		m := offset / datasize
		a.Buf.Write([]byte{
			(baseRegBits << 5) | targetRegBits,
			(byte(m << 2)) | (baseRegBits >> 3),
			opcode<<6 | 0b00_111111&byte(m>>6),
			size<<6 | v<<2 | 0b00_1_11_0_01,
		})
		return
	}

	// Otherwise, we need multiple instructions.
	tmpRegBits := registerBits(a.temporaryRegister)
	offset32 := int32(offset)

	// Go's assembler adds a const into the const pool at this point,
	// regardless of its usage; e.g. if we enter the then block of the following if statement,
	// the const is not used but it is added into the const pool.
	var c = asm.NewStaticConst(make([]byte, 4))
	binary.LittleEndian.PutUint32(c.Raw, uint32(offset))
	a.pool.AddConst(c, uint64(a.Buf.Len()))

	// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3529-L3532
	// If the offset is within 24-bits, we can load it with two ADD instructions.
	hi := offset32 - (offset32 & (0xfff << uint(datasizeLog2)))
	if hi&^0xfff000 == 0 {
		var sfops byte = 0b100
		m := ((offset32 - hi) >> datasizeLog2) & 0xfff
		hi >>= 12

		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3534-L3535
		a.Buf.Write([]byte{
			(baseRegBits << 5) | tmpRegBits,
			(byte(hi) << 2) | (baseRegBits >> 3),
			0b01<<6 /* shift by 12 */ | byte(hi>>6),
			sfops<<5 | 0b10001,
		})

		a.Buf.Write([]byte{
			(tmpRegBits << 5) | targetRegBits,
			(byte(m << 2)) | (tmpRegBits >> 3),
			opcode<<6 | 0b00_111111&byte(m>>6),
			size<<6 | v<<2 | 0b00_1_11_0_01,
		})
	} else {
		// This case we load the const via ldr(literal) into tem register,
		// and the target const is placed after this instruction below.
		loadLiteralOffsetInBinary := uint64(a.Buf.Len())

		// First we emit the ldr(literal) with offset zero as we don't yet know the const's placement in the binary.
		// https://developer.arm.com/documentation/ddi0596/2020-12/Base-Instructions/LDR--literal---Load-Register--literal--
		a.Buf.Write([]byte{tmpRegBits, 0x0, 0x0, 0b00_011_0_00})

		// Set the callback for the constant, and we set properly the offset in the callback.

		c.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
			// ldr(literal) encodes offset divided by 4.
			offset := (int(offsetOfConst) - int(loadLiteralOffsetInBinary)) / 4
			bin := a.Buf.Bytes()
			bin[loadLiteralOffsetInBinary] |= byte(offset << 5)
			bin[loadLiteralOffsetInBinary+1] |= byte(offset >> 3)
			bin[loadLiteralOffsetInBinary+2] |= byte(offset >> 11)
		})

		// Then, load the constant with the register offset.
		// https://developer.arm.com/documentation/ddi0596/2020-12/Base-Instructions/LDR--register---Load-Register--register--
		a.Buf.Write([]byte{
			(baseRegBits << 5) | targetRegBits,
			0b011_010_00 | (baseRegBits >> 3),
			opcode<<6 | 0b00_1_00000 | tmpRegBits,
			size<<6 | v<<2 | 0b00_111_0_00,
		})
	}
	return
}

var storeOrLoadInstructionTable = map[asm.Instruction]struct {
	size, v                byte
	datasize, datasizeLog2 int64
	isTargetFloat          bool
}{
	MOVD:  {size: 0b11, v: 0x0, datasize: 8, datasizeLog2: 3},
	MOVW:  {size: 0b10, v: 0x0, datasize: 4, datasizeLog2: 2},
	MOVWU: {size: 0b10, v: 0x0, datasize: 4, datasizeLog2: 2},
	MOVH:  {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1},
	MOVHU: {size: 0b01, v: 0x0, datasize: 2, datasizeLog2: 1},
	MOVB:  {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0},
	MOVBU: {size: 0b00, v: 0x0, datasize: 1, datasizeLog2: 0},
	FMOVD: {size: 0b11, v: 0x1, datasize: 8, datasizeLog2: 3, isTargetFloat: true},
	FMOVS: {size: 0b10, v: 0x1, datasize: 4, datasizeLog2: 2, isTargetFloat: true},
}

// Exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeRegisterToMemory(n *NodeImpl) (err error) {
	inst, ok := storeOrLoadInstructionTable[n.Instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}

	var srcRegBits byte
	if inst.isTargetFloat {
		srcRegBits, err = vectorRegisterBits(n.SrcReg)
	} else {
		srcRegBits, err = intRegisterBits(n.SrcReg)
	}
	if err != nil {
		return
	}

	baseRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	const opcode = 0x00 // opcode for store instructions.
	if n.DstReg2 != asm.NilRegister {
		offsetRegBits, err := intRegisterBits(n.DstReg2)
		if err != nil {
			return err
		}
		a.encodeLoadOrStoreWithRegisterOffset(baseRegBits, offsetRegBits, srcRegBits, opcode, inst.size, inst.v)
	} else {
		err = a.encodeLoadOrStoreWithConstOffset(baseRegBits, srcRegBits, n.DstConst, opcode, inst.size, inst.v, inst.datasize, inst.datasizeLog2)
	}
	return
}

func (a *AssemblerImpl) encodeADR(n *NodeImpl) (err error) {
	dstRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	adrInstructionOffsetInBinary := uint64(a.Buf.Len())

	// At this point, we don't yet know the target offset to read from,
	// so we emit the ADR instruction with 0 offset, and replace later in the callback.
	// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
	a.Buf.Write([]byte{dstRegBits, 0x0, 0x0, 0b10000})

	// This case, the ADR's target offset is for the staticConst's initial address.
	if sc := n.staticConst; sc != nil {
		a.pool.AddConst(sc, adrInstructionOffsetInBinary)
		sc.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
			adrInstructionBytes := a.Buf.Bytes()[adrInstructionOffsetInBinary : adrInstructionOffsetInBinary+4]
			offset := int(offsetOfConst) - int(adrInstructionOffsetInBinary)

			// See https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
			adrInstructionBytes[3] |= byte(offset & 0b00000011 << 5)
			offset >>= 2
			adrInstructionBytes[0] |= byte(offset << 5)
			offset >>= 3
			adrInstructionBytes[1] |= byte(offset)
			offset >>= 8
			adrInstructionBytes[2] |= byte(offset)
		})
		return
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
			return fmt.Errorf("BUG: target instruction %s not found for ADR", InstructionName(n.readInstructionAddressBeforeTargetInstruction))
		}

		offset := targetNode.OffsetInBinary() - n.OffsetInBinary()
		if offset > math.MaxUint8 {
			// We could support up to 20-bit integer, but byte should be enough for our impl.
			// If the necessity comes up, we could fix the below to support larger offsets.
			return fmt.Errorf("BUG: too large offset for ADR")
		}

		// Now ready to write an offset byte.
		v := byte(offset)

		adrInstructionBytes := code[n.OffsetInBinary() : n.OffsetInBinary()+4]
		// According to the binary format of ADR instruction in arm64:
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ADR--Form-PC-relative-address-?lang=en
		//
		// The 0 to 1 bits live on 29 to 30 bits of the instruction.
		adrInstructionBytes[3] |= (v & 0b00000011) << 5
		// The 2 to 4 bits live on 5 to 7 bits of the instruction.
		adrInstructionBytes[0] |= (v & 0b00011100) << 3
		// The 5 to 7 bits live on 8 to 10 bits of the instruction.
		adrInstructionBytes[1] |= (v & 0b11100000) >> 5
		return nil
	})
	return
}

// Exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeMemoryToRegister(n *NodeImpl) (err error) {
	if n.Instruction == ADR {
		return a.encodeADR(n)
	}

	inst, ok := storeOrLoadInstructionTable[n.Instruction]
	if !ok {
		return errorEncodingUnsupported(n)
	}

	var dstRegBits byte
	if inst.isTargetFloat {
		dstRegBits, err = vectorRegisterBits(n.DstReg)
	} else {
		dstRegBits, err = intRegisterBits(n.DstReg)
	}
	if err != nil {
		return
	}
	baseRegBits, err := intRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	var opcode byte = 0b01 // opcode for load instructions.
	if n.Instruction == MOVW || n.Instruction == MOVH || n.Instruction == MOVB {
		// Sign-extend load (without "u" suffix except 64-bit MOVD) needs different opcode.
		opcode = 0b10
	}
	if n.SrcReg2 != asm.NilRegister {
		offsetRegBits, err := intRegisterBits(n.SrcReg2)
		if err != nil {
			return err
		}
		a.encodeLoadOrStoreWithRegisterOffset(baseRegBits, offsetRegBits, dstRegBits, opcode, inst.size, inst.v)
	} else {
		err = a.encodeLoadOrStoreWithConstOffset(baseRegBits, dstRegBits, n.SrcConst, opcode, inst.size, inst.v, inst.datasize, inst.datasizeLog2)
	}
	return
}

// const16bitAligned check if the value is on the 16-bit alignment.
// If so, returns the shift num divided by 16, and otherwise -1.
func const16bitAligned(v int64) (ret int) {
	ret = -1
	for s := 0; s < 64; s += 16 {
		if (uint64(v) &^ (uint64(0xffff) << uint(s))) == 0 {
			ret = s / 16
			break
		}
	}
	return
}

// isBitMaskImmediate determines if the value can be encoded as "bitmask immediate".
//
//	Such an immediate is a 32-bit or 64-bit pattern viewed as a vector of identical elements of size e = 2, 4, 8, 16, 32, or 64 bits.
//	Each element contains the same sub-pattern: a single run of 1 to e-1 non-zero bits, rotated by 0 to e-1 bits.
//
// See https://developer.arm.com/documentation/dui0802/b/A64-General-Instructions/MOV--bitmask-immediate-
func isBitMaskImmediate(x uint64) bool {
	// All zeros and ones are not "bitmask immediate" by defainition.
	if x == 0 || x == 0xffff_ffff_ffff_ffff {
		return false
	}

	switch {
	case x != x>>32|x<<32:
		// e = 64
	case x != x>>16|x<<48:
		// e = 32 (x == x>>32|x<<32).
		// e.g. 0x00ff_ff00_00ff_ff00
		x = uint64(int32(x))
	case x != x>>8|x<<56:
		// e = 16 (x == x>>16|x<<48).
		// e.g. 0x00ff_00ff_00ff_00ff
		x = uint64(int16(x))
	case x != x>>4|x<<60:
		// e = 8 (x == x>>8|x<<56).
		// e.g. 0x0f0f_0f0f_0f0f_0f0f
		x = uint64(int8(x))
	default:
		// e = 4 or 2.
		return true
	}
	return sequenceOfSetbits(x) || sequenceOfSetbits(^x)
}

// sequenceOfSetbits returns true if the number's binary representation is the sequence set bit (1).
// For example: 0b1110 -> true, 0b1010 -> false
func sequenceOfSetbits(x uint64) bool {
	y := getLowestBit(x)
	// If x is a sequence of set bit, this should results in the number
	// with only one set bit (i.e. power of two).
	y += x
	return (y-1)&y == 0
}

func getLowestBit(x uint64) uint64 {
	// See https://stackoverflow.com/questions/12247186/find-the-lowest-set-bit
	return x & (^x + 1)
}

func (a *AssemblerImpl) addOrSub64BitRegisters(sfops byte, src1RegBits byte, src2RegBits byte) {
	// src1Reg = src1Reg +/- src2Reg
	a.Buf.Write([]byte{
		(src1RegBits << 5) | src1RegBits,
		src1RegBits >> 3,
		src2RegBits,
		sfops<<5 | 0b01011,
	})
}

// See "Logical (immediate)" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Immediate
var logicalImmediate = map[asm.Instruction]struct {
	sf, opc  byte
	resolver func(imm asm.ConstantValue) (imms, immr, N byte, err error)
}{
	ANDIMM32: {sf: 0b0, opc: 0b00, resolver: func(imm asm.ConstantValue) (imms, immr, N byte, err error) {
		if !isBitMaskImmediate(uint64(imm)) {
			err = fmt.Errorf("const %d must be valid bitmask immediate for %s", imm, InstructionName(ANDIMM64))
			return
		}
		immr, imms, N = bitmaskImmediate(uint64(imm), false)
		return
	}},
	ANDIMM64: {sf: 0b1, opc: 0b00, resolver: func(imm asm.ConstantValue) (imms, immr, N byte, err error) {
		if !isBitMaskImmediate(uint64(imm)) {
			err = fmt.Errorf("const %d must be valid bitmask immediate for %s", imm, InstructionName(ANDIMM64))
			return
		}
		immr, imms, N = bitmaskImmediate(uint64(imm), true)
		return
	}},
}

func bitmaskImmediate(c uint64, is64bit bool) (immr, imms, N byte) {
	var size uint32
	switch {
	case c != c>>32|c<<32:
		size = 64
	case c != c>>16|c<<48:
		size = 32
		c = uint64(int32(c))
	case c != c>>8|c<<56:
		size = 16
		c = uint64(int16(c))
	case c != c>>4|c<<60:
		size = 8
		c = uint64(int8(c))
	case c != c>>2|c<<62:
		size = 4
		c = uint64(int64(c<<60) >> 60)
	default:
		size = 2
		c = uint64(int64(c<<62) >> 62)
	}

	neg := false
	if int64(c) < 0 {
		c = ^c
		neg = true
	}

	onesSize, nonZeroPos := getOnesSequenceSize(c)
	if neg {
		nonZeroPos = onesSize + nonZeroPos
		onesSize = size - onesSize
	}

	var mode byte = 32
	if is64bit {
		N, mode = 0b1, 64
	}

	immr = byte((size - nonZeroPos) & (size - 1) & uint32(mode-1))
	imms = byte((onesSize - 1) | 63&^(size<<1-1))
	return
}

// Exported for inter-op testing with golang-asm.
// TODO: unexport after golang-asm complete removal.
func (a *AssemblerImpl) EncodeConstToRegister(n *NodeImpl) (err error) {
	// Alias for readability.
	c := n.SrcConst

	dstRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	if log, ok := logicalImmediate[n.Instruction]; ok {
		// See "Logical (immediate)" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Immediate
		imms, immr, N, err := log.resolver(c)
		if err != nil {
			return err
		}

		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			imms<<2 | dstRegBits>>3,
			N<<6 | immr,
			log.sf<<7 | log.opc<<5 | 0b10010,
		})
		return nil
	}

	// TODO: refactor and generalize the following like ^ logicalImmediate, etc.
	switch inst := n.Instruction; inst {
	case ADD, ADDS, SUB, SUBS:
		var sfops byte
		if inst == ADD {
			sfops = 0b100
		} else if inst == ADDS {
			sfops = 0b101
		} else if inst == SUB {
			sfops = 0b110
		} else if inst == SUBS {
			sfops = 0b111
		}

		if c == 0 {
			// If the constant equals zero, we encode it as ADD (register) with zero register.
			a.addOrSub64BitRegisters(sfops, dstRegBits, zeroRegisterBits)
			return
		}

		if c >= 0 && (c <= 0xfff || (c&0xfff) == 0 && (uint64(c>>12) <= 0xfff)) {
			// If the const can be represented as "imm12" or "imm12 << 12": one instruction
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L2992

			if c <= 0xfff {
				a.Buf.Write([]byte{
					(dstRegBits << 5) | dstRegBits,
					(byte(c) << 2) | (dstRegBits >> 3),
					byte(c >> 6),
					sfops<<5 | 0b10001,
				})
			} else {
				c >>= 12
				a.Buf.Write([]byte{
					(dstRegBits << 5) | dstRegBits,
					(byte(c) << 2) | (dstRegBits >> 3),
					0b01<<6 /* shift by 12 */ | byte(c>>6),
					sfops<<5 | 0b10001,
				})
			}
			return
		}

		if t := const16bitAligned(c); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			//https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L4029
			tmpRegBits := registerBits(a.temporaryRegister)

			// MOVZ $c, tmpReg with shifting.
			a.load16bitAlignedConst(c>>(16*t), byte(t), tmpRegBits, false, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, dstRegBits, tmpRegBits)
			return
		} else if t := const16bitAligned(^c); t >= 0 {
			// Also if the reverse of the const can fit within 16-bit range, do the same ^^.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L4029
			tmpRegBits := registerBits(a.temporaryRegister)

			// MOVN $c, tmpReg with shifting.
			a.load16bitAlignedConst(^c>>(16*t), byte(t), tmpRegBits, true, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, dstRegBits, tmpRegBits)
			return
		}

		if uc := uint64(c); isBitMaskImmediate(uc) {
			// If the const can be represented as "bitmask immediate", we load it via ORR into temp register.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6570-L6583
			tmpRegBits := registerBits(a.temporaryRegister)
			// OOR $c, tmpReg
			a.loadConstViaBitMaskImmediate(uc, tmpRegBits, true)

			// ADD/SUB tmpReg, dstReg
			a.addOrSub64BitRegisters(sfops, dstRegBits, tmpRegBits)
			return
		}

		// If the value fits within 24-bit, then we emit two add instructions
		if 0 <= c && c <= 0xffffff && inst != SUBS && inst != ADDS {
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3849-L3862
			a.Buf.Write([]byte{
				(dstRegBits << 5) | dstRegBits,
				(byte(c) << 2) | (dstRegBits >> 3),
				byte(c & 0xfff >> 6),
				sfops<<5 | 0b10001,
			})
			c = c >> 12
			a.Buf.Write([]byte{
				(dstRegBits << 5) | dstRegBits,
				(byte(c) << 2) | (dstRegBits >> 3),
				0b01_000000 /* shift by 12 */ | byte(c>>6),
				sfops<<5 | 0b10001,
			})
			return
		}

		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L3163-L3203
		// Otherwise we use MOVZ and MOVNs for loading const into tmpRegister.
		tmpRegBits := registerBits(a.temporaryRegister)
		a.load64bitConst(c, tmpRegBits)
		a.addOrSub64BitRegisters(sfops, dstRegBits, tmpRegBits)
	case MOVW:
		if c == 0 {
			a.Buf.Write([]byte{
				(zeroRegisterBits << 5) | dstRegBits,
				zeroRegisterBits >> 3,
				0b000_00000 | zeroRegisterBits,
				0b0_01_01010,
			})
			return
		}

		// Following the logic here:
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1637
		c32 := uint32(c)
		ic := int64(c32)
		if ic >= 0 && (ic <= 0xfff || (ic&0xfff) == 0 && (uint64(ic>>12) <= 0xfff)) {
			if isBitMaskImmediate(uint64(c)) {
				a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, false)
				return
			}
		}

		if t := const16bitAligned(int64(c32)); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			a.load16bitAlignedConst(int64(c32)>>(16*t), byte(t), dstRegBits, false, false)
		} else if t := const16bitAligned(int64(^c32)); t >= 0 {
			// Also, if the reverse of the const can fit within 16-bit range, do the same ^^.
			a.load16bitAlignedConst(int64(^c32)>>(16*t), byte(t), dstRegBits, true, false)
		} else if isBitMaskImmediate(uint64(c)) {
			a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, false)
		} else {
			// Otherwise, we use MOVZ and MOVK to load it.
			// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6623-L6630
			c16 := uint16(c32)
			// MOVZ: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
			a.Buf.Write([]byte{
				(byte(c16) << 5) | dstRegBits,
				byte(c16 >> 3),
				1<<7 | byte(c16>>11),
				0b0_10_10010,
			})
			// MOVK: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVK
			c16 = uint16(c32 >> 16)
			if c16 != 0 {
				a.Buf.Write([]byte{
					(byte(c16) << 5) | dstRegBits,
					byte(c16 >> 3),
					1<<7 | 0b0_01_00000 /* shift by 16 */ | byte(c16>>11),
					0b0_11_10010,
				})
			}
		}
	case MOVD:
		// Following the logic here:
		// https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L1798-L1852
		if c >= 0 && (c <= 0xfff || (c&0xfff) == 0 && (uint64(c>>12) <= 0xfff)) {
			if isBitMaskImmediate(uint64(c)) {
				a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, true)
				return
			}
		}

		if t := const16bitAligned(c); t >= 0 {
			// If the const can fit within 16-bit alignment, for example, 0xffff, 0xffff_0000 or 0xffff_0000_0000_0000
			// We could load it into temporary with movk.
			a.load16bitAlignedConst(c>>(16*t), byte(t), dstRegBits, false, true)
		} else if t := const16bitAligned(^c); t >= 0 {
			// Also, if the reverse of the const can fit within 16-bit range, do the same ^^.
			a.load16bitAlignedConst((^c)>>(16*t), byte(t), dstRegBits, true, true)
		} else if isBitMaskImmediate(uint64(c)) {
			a.loadConstViaBitMaskImmediate(uint64(c), dstRegBits, true)
		} else {
			a.load64bitConst(c, dstRegBits)
		}
	case LSR:
		if c == 0 {
			err = errors.New("LSR with zero constant should be optimized out")
			return
		} else if c < 0 || c > 63 {
			err = fmt.Errorf("LSR requires immediate to be within 0 to 63, but got %d", c)
			return
		}

		// LSR(immediate) is an alias of UBFM
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LSR--immediate---Logical-Shift-Right--immediate---an-alias-of-UBFM-?lang=en
		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			0b111111_00 | dstRegBits>>3,
			0b01_000000 | byte(c),
			0b110_10011,
		})
	case LSL:
		if c == 0 {
			err = errors.New("LSL with zero constant should be optimized out")
			return
		} else if c < 0 || c > 63 {
			err = fmt.Errorf("LSL requires immediate to be within 0 to 63, but got %d", c)
			return
		}

		// LSL(immediate) is an alias of UBFM
		// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/LSL--immediate---Logical-Shift-Left--immediate---an-alias-of-UBFM-
		cb := byte(c)
		a.Buf.Write([]byte{
			(dstRegBits << 5) | dstRegBits,
			(0b111111-cb)<<2 | dstRegBits>>3,
			0b01_000000 | (64 - cb),
			0b110_10011,
		})

	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) movk(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVK
	a.Buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_11_10010,
	})
}

func (a *AssemblerImpl) movz(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
	a.Buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_10_10010,
	})
}

func (a *AssemblerImpl) movn(v uint64, shfitNum int, dstRegBits byte) {
	// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
	a.Buf.Write([]byte{
		(byte(v) << 5) | dstRegBits,
		byte(v >> 3),
		1<<7 | byte(shfitNum)<<5 | (0b000_11111 & byte(v>>11)),
		0b1_00_10010,
	})
}

// load64bitConst loads a 64-bit constant into the register, following the same logic to decide how to load large 64-bit
// consts as in the Go assembler.
//
// See https://github.com/golang/go/blob/release-branch.go1.15/src/cmd/internal/obj/arm64/asm7.go#L6632-L6759
func (a *AssemblerImpl) load64bitConst(c int64, dstRegBits byte) {
	var bits [4]uint64
	var zeros, negs int
	for i := 0; i < 4; i++ {
		bits[i] = uint64((c >> uint(i*16)) & 0xffff)
		if v := bits[i]; v == 0 {
			zeros++
		} else if v == 0xffff {
			negs++
		}
	}

	if zeros == 3 {
		// one MOVZ instruction.
		for i, v := range bits {
			if v != 0 {
				a.movz(v, i, dstRegBits)
			}
		}

	} else if negs == 3 {
		// one MOVN instruction.
		for i, v := range bits {
			if v != 0xffff {
				v = ^v
				a.movn(v, i, dstRegBits)
			}
		}

	} else if zeros == 2 {
		// one MOVZ then one OVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if negs == 2 {
		// one MOVN then one or two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				a.movn(v, i, dstRegBits)
				movn = true
			} else if v != 0xffff {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if zeros == 1 {
		// one MOVZ then two MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	} else if negs == 1 {
		// one MOVN then two MOVK.
		var movn bool
		for i, v := range bits { // Emit MOVN.
			if !movn && v != 0xffff {
				v = ^v
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
				a.movn(v, i, dstRegBits)
				movn = true
			} else if v != 0xffff {
				a.movk(v, i, dstRegBits)
			}
		}

	} else {
		// one MOVZ then tree MOVK.
		var movz bool
		for i, v := range bits {
			if !movz && v != 0 { // MOVZ.
				// https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
				a.movz(v, i, dstRegBits)
				movz = true
			} else if v != 0 {
				a.movk(v, i, dstRegBits)
			}
		}

	}
}

func (a *AssemblerImpl) load16bitAlignedConst(c int64, shiftNum byte, regBits byte, reverse bool, dst64bit bool) {
	var lastByte byte
	if reverse {
		// MOVN: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVZ
		lastByte = 0b0_00_10010
	} else {
		// MOVZ: https://developer.arm.com/documentation/dui0802/a/A64-General-Instructions/MOVN
		lastByte = 0b0_10_10010
	}
	if dst64bit {
		lastByte |= 0b1 << 7
	}
	a.Buf.Write([]byte{
		(byte(c) << 5) | regBits,
		byte(c >> 3),
		1<<7 | (shiftNum << 5) | byte(c>>11),
		lastByte,
	})
}

// loadConstViaBitMaskImmediate loads the constant with ORR (bitmask immediate).
// https://developer.arm.com/documentation/ddi0596/2021-12/Base-Instructions/ORR--immediate---Bitwise-OR--immediate--?lang=en
func (a *AssemblerImpl) loadConstViaBitMaskImmediate(c uint64, regBits byte, dst64bit bool) {
	var size uint32
	switch {
	case c != c>>32|c<<32:
		size = 64
	case c != c>>16|c<<48:
		size = 32
		c = uint64(int32(c))
	case c != c>>8|c<<56:
		size = 16
		c = uint64(int16(c))
	case c != c>>4|c<<60:
		size = 8
		c = uint64(int8(c))
	case c != c>>2|c<<62:
		size = 4
		c = uint64(int64(c<<60) >> 60)
	default:
		size = 2
		c = uint64(int64(c<<62) >> 62)
	}

	neg := false
	if int64(c) < 0 {
		c = ^c
		neg = true
	}

	onesSize, nonZeroPos := getOnesSequenceSize(c)
	if neg {
		nonZeroPos = onesSize + nonZeroPos
		onesSize = size - onesSize
	}

	// See the following article for understanding the encoding.
	// https://dinfuehr.github.io/blog/encoding-of-immediate-values-on-aarch64/
	var n byte
	var mode = 32
	if dst64bit && size == 64 {
		n = 0b1
		mode = 64
	}

	r := byte((size - nonZeroPos) & (size - 1) & uint32(mode-1))
	s := byte((onesSize - 1) | 63&^(size<<1-1))

	var sf byte
	if dst64bit {
		sf = 0b1
	}
	a.Buf.Write([]byte{
		(zeroRegisterBits << 5) | regBits,
		s<<2 | (zeroRegisterBits >> 3),
		n<<6 | r,
		sf<<7 | 0b0_01_10010,
	})
}

func getOnesSequenceSize(x uint64) (size, nonZeroPos uint32) {
	// Take 0b00111000 for example:
	y := getLowestBit(x)               // = 0b0000100
	nonZeroPos = setBitPos(y)          // = 2
	size = setBitPos(x+y) - nonZeroPos // = setBitPos(0b0100000) - 2 = 5 - 2 = 3
	return
}

func setBitPos(x uint64) (ret uint32) {
	for ; ; ret++ {
		if x == 0b1 {
			break
		}
		x = x >> 1
	}
	return
}

func checkArrangementIndexPair(arr VectorArrangement, index VectorIndex) (err error) {
	if arr == VectorArrangementNone {
		return nil
	}
	var valid bool
	switch arr {
	case VectorArrangement8B:
		valid = index < 8
	case VectorArrangement16B:
		valid = index < 16
	case VectorArrangement4H:
		valid = index < 4
	case VectorArrangement8H:
		valid = index < 8
	case VectorArrangement2S:
		valid = index < 2
	case VectorArrangement4S:
		valid = index < 4
	case VectorArrangement1D:
		valid = index < 1
	case VectorArrangement2D:
		valid = index < 2
	case VectorArrangementB:
		valid = index < 16
	case VectorArrangementH:
		valid = index < 8
	case VectorArrangementS:
		valid = index < 4
	case VectorArrangementD:
		valid = index < 2
	}
	if !valid {
		err = fmt.Errorf("invalid arrangement and index pair: %s[%d]", arr, index)
	}
	return
}

func (a *AssemblerImpl) EncodeMemoryToVectorRegister(n *NodeImpl) (err error) {
	srcBaseRegBits, err := intRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	dstVectorRegBits, err := vectorRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	switch n.Instruction {
	case VMOV: // translated as LDR(immediate,SIMD&FP)
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LDR--immediate--SIMD-FP---Load-SIMD-FP-Register--immediate-offset--?lang=en
		var size, opcode byte
		var dataSize, dataSizeLog2 int64
		switch n.VectorArrangement {
		case VectorArrangementB:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b01, 1, 0
		case VectorArrangementH:
			size, opcode, dataSize, dataSizeLog2 = 0b01, 0b01, 2, 1
		case VectorArrangementS:
			size, opcode, dataSize, dataSizeLog2 = 0b10, 0b01, 4, 2
		case VectorArrangementD:
			size, opcode, dataSize, dataSizeLog2 = 0b11, 0b01, 8, 3
		case VectorArrangementQ:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b11, 16, 4
		}
		const v = 1 // v as in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_pos
		if n.SrcReg2 != asm.NilRegister {
			offsetRegBits, err := intRegisterBits(n.SrcReg2)
			if err != nil {
				return err
			}
			a.encodeLoadOrStoreWithRegisterOffset(srcBaseRegBits, offsetRegBits, dstVectorRegBits, opcode, size, v)
		} else {
			err = a.encodeLoadOrStoreWithConstOffset(srcBaseRegBits, dstVectorRegBits,
				n.SrcConst, opcode, size, v, dataSize, dataSizeLog2)
		}
	case LD1R:
		if n.SrcReg2 != asm.NilRegister || n.SrcConst != 0 {
			return fmt.Errorf("offset for %s is not implemented", InstructionName(LD1R))
		}

		var size, q byte
		switch n.VectorArrangement {
		case VectorArrangement8B:
			size, q = 0b00, 0b0
		case VectorArrangement16B:
			size, q = 0b00, 0b1
		case VectorArrangement4H:
			size, q = 0b01, 0b0
		case VectorArrangement8H:
			size, q = 0b01, 0b1
		case VectorArrangement2S:
			size, q = 0b10, 0b0
		case VectorArrangement4S:
			size, q = 0b10, 0b1
		case VectorArrangement1D:
			size, q = 0b11, 0b0
		case VectorArrangement2D:
			size, q = 0b11, 0b1
		}

		// No offset encoding.
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/LD1R--Load-one-single-element-structure-and-Replicate-to-all-lanes--of-one-register--?lang=en#iclass_as_post_index
		a.Buf.Write([]byte{
			(srcBaseRegBits << 5) | dstVectorRegBits,
			0b11_000000 | size<<2 | srcBaseRegBits>>3,
			0b01_000000,
			q<<6 | 0b1101,
		})
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func arrangementSizeQ(arr VectorArrangement) (size, q byte) {
	switch arr {
	case VectorArrangement8B:
		size, q = 0b00, 0
	case VectorArrangement16B:
		size, q = 0b00, 1
	case VectorArrangement4H:
		size, q = 0b01, 0
	case VectorArrangement8H:
		size, q = 0b01, 1
	case VectorArrangement2S:
		size, q = 0b10, 0
	case VectorArrangement4S:
		size, q = 0b10, 1
	case VectorArrangement1D:
		size, q = 0b11, 0
	case VectorArrangement2D:
		size, q = 0b11, 1
	}
	return
}

func (a *AssemblerImpl) EncodeVectorRegisterToMemory(n *NodeImpl) (err error) {
	srcVectorRegBits, err := vectorRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	dstBaseRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	switch n.Instruction {
	case VMOV: // translated as STR(immediate,SIMD&FP)
		// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/STR--immediate--SIMD-FP---Store-SIMD-FP-register--immediate-offset--
		var size, opcode byte
		var dataSize, dataSizeLog2 int64
		switch n.VectorArrangement {
		case VectorArrangementB:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b00, 1, 0
		case VectorArrangementH:
			size, opcode, dataSize, dataSizeLog2 = 0b01, 0b00, 2, 1
		case VectorArrangementS:
			size, opcode, dataSize, dataSizeLog2 = 0b10, 0b00, 4, 2
		case VectorArrangementD:
			size, opcode, dataSize, dataSizeLog2 = 0b11, 0b00, 8, 3
		case VectorArrangementQ:
			size, opcode, dataSize, dataSizeLog2 = 0b00, 0b10, 16, 4
		}
		const v = 1 // v as in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Loads-and-Stores?lang=en#ldst_pos

		if n.DstReg2 != asm.NilRegister {
			offsetRegBits, err := intRegisterBits(n.DstReg2)
			if err != nil {
				return err
			}
			a.encodeLoadOrStoreWithRegisterOffset(dstBaseRegBits, offsetRegBits, srcVectorRegBits, opcode, size, v)
		} else {
			err = a.encodeLoadOrStoreWithConstOffset(dstBaseRegBits, srcVectorRegBits,
				n.DstConst, opcode, size, v, dataSize, dataSizeLog2)
		}
	default:
		return errorEncodingUnsupported(n)
	}
	return
}

func (a *AssemblerImpl) EncodeStaticConstToVectorRegister(n *NodeImpl) (err error) {
	if n.Instruction != VMOV {
		return errorEncodingUnsupported(n)
	}

	dstRegBits, err := vectorRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	// LDR (literal, SIMD&FP)
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/LDR--literal--SIMD-FP---Load-SIMD-FP-Register--PC-relative-literal--
	var opc byte
	var constLength int
	switch n.VectorArrangement {
	case VectorArrangementS:
		opc, constLength = 0b00, 4
	case VectorArrangementD:
		opc, constLength = 0b01, 8
	case VectorArrangementQ:
		opc, constLength = 0b10, 16
	}

	loadLiteralOffsetInBinary := uint64(a.Buf.Len())
	a.pool.AddConst(n.staticConst, loadLiteralOffsetInBinary)

	if len(n.staticConst.Raw) != constLength {
		return fmt.Errorf("invalid const length for %s: want %d but was %d",
			n.VectorArrangement, constLength, len(n.staticConst.Raw))
	}

	a.Buf.Write([]byte{dstRegBits, 0x0, 0x0, opc<<6 | 0b11100})
	n.staticConst.AddOffsetFinalizedCallback(func(offsetOfConst uint64) {
		// LDR (literal, SIMD&FP) encodes offset divided by 4.
		offset := (int(offsetOfConst) - int(loadLiteralOffsetInBinary)) / 4
		bin := a.Buf.Bytes()
		bin[loadLiteralOffsetInBinary] |= byte(offset << 5)
		bin[loadLiteralOffsetInBinary+1] |= byte(offset >> 3)
		bin[loadLiteralOffsetInBinary+2] |= byte(offset >> 11)
	})
	return
}

// advancedSIMDTwoRegisterMisc holds information to encode instructions as "Advanced SIMD two-register miscellaneous" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDTwoRegisterMisc = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/NOT--Bitwise-NOT--vector--?lang=en
	NOT: {u: 0b1, opcode: 0b00101,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FNEG--vector---Floating-point-Negate--vector--?lang=en
	VFNEG: {u: 0b1, opcode: 0b01111,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FABS--vector---Floating-point-Absolute-value--vector--?lang=en
	VFABS: {u: 0, opcode: 0b01111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSQRT--vector---Floating-point-Square-Root--vector--?lang=en
	VFSQRT: {u: 1, opcode: 0b11111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTM--vector---Floating-point-Round-to-Integral--toward-Minus-infinity--vector--?lang=en
	VFRINTM: {u: 0, opcode: 0b11001, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b01, q: 0b1},
		VectorArrangement4S: {size: 0b00, q: 0b1},
		VectorArrangement2S: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTN--vector---Floating-point-Round-to-Integral--to-nearest-with-ties-to-even--vector--?lang=en
	VFRINTN: {u: 0, opcode: 0b11000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b01, q: 0b1},
		VectorArrangement4S: {size: 0b00, q: 0b1},
		VectorArrangement2S: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTP--vector---Floating-point-Round-to-Integral--toward-Plus-infinity--vector--?lang=en
	VFRINTP: {u: 0, opcode: 0b11000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FRINTZ--vector---Floating-point-Round-to-Integral--toward-Zero--vector--?lang=en
	VFRINTZ: {u: 0, opcode: 0b11001, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {size: 0b11, q: 0b1},
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CNT--Population-Count-per-byte-?lang=en
	VCNT: {u: 0b0, opcode: 0b00101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B:  {size: 0b00, q: 0b0},
		VectorArrangement16B: {size: 0b00, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/NEG--vector---Negate--vector--?lang=en
	VNEG: {u: 0b1, opcode: 0b01011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ABS--Absolute-value--vector--?lang=en
	VABS: {u: 0b0, opcode: 0b01011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/REV64--Reverse-elements-in-64-bit-doublewords--vector--?lang=en
	REV64: {u: 0b0, opcode: 0b00000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/XTN--XTN2--Extract-Narrow-?lang=en
	XTN: {u: 0b0, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0, size: 0b10},
		VectorArrangement4S: {q: 0, size: 0b01},
		VectorArrangement8H: {q: 0, size: 0b00},
	}},
	SHLL: {u: 0b1, opcode: 0b10011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b00, size: 0b00},
		VectorArrangement4H: {q: 0b00, size: 0b01},
		VectorArrangement2S: {q: 0b00, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMEQ--zero---Compare-bitwise-Equal-to-zero--vector--?lang=en
	CMEQZERO: {u: 0b0, opcode: 0b01001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SADDLP--Signed-Add-Long-Pairwise-?lang=en
	SADDLP: {u: 0b0, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UADDLP--Unsigned-Add-Long-Pairwise-?lang=en
	UADDLP: {u: 0b1, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZS--vector--integer---Floating-point-Convert-to-Signed-integer--rounding-toward-Zero--vector--?lang=en
	VFCVTZS: {u: 0b0, opcode: 0b11011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
		VectorArrangement2D: {size: 0b11, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTZU--vector--integer---Floating-point-Convert-to-Unsigned-integer--rounding-toward-Zero--vector--?lang=en
	VFCVTZU: {u: 0b1, opcode: 0b11011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4S: {size: 0b10, q: 0b1},
		VectorArrangement2S: {size: 0b10, q: 0b0},
		VectorArrangement2D: {size: 0b11, q: 0b1},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN: {u: 0b0, opcode: 0b10100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},

	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTN--SQXTN2--Signed-saturating-extract-Narrow-?lang=en
	SQXTN2: {u: 0b0, opcode: 0b10100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQXTN--UQXTN2--Unsigned-saturating-extract-Narrow-?lang=en
	UQXTN: {u: 0b1, opcode: 0b10100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN: {u: 0b1, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQXTUN--SQXTUN2--Signed-saturating-extract-Unsigned-Narrow-?lang=en
	SQXTUN2: {u: 0b1, opcode: 0b10010, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SCVTF--vector--integer---Signed-integer-Convert-to-Floating-point--vector--?lang=en
	VSCVTF: {u: 0b0, opcode: 0b11101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0b1, size: 0b01},
		VectorArrangement4S: {q: 0b1, size: 0b00},
		VectorArrangement2S: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UCVTF--vector--integer---Unsigned-integer-Convert-to-Floating-point--vector--?lang=en
	VUCVTF: {u: 0b1, opcode: 0b11101, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2D: {q: 0b1, size: 0b01},
		VectorArrangement4S: {q: 0b1, size: 0b00},
		VectorArrangement2S: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTL--FCVTL2--Floating-point-Convert-to-higher-precision-Long--vector--?lang=en
	FCVTL: {u: 0b0, opcode: 0b10111, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {size: 0b01, q: 0b0},
		VectorArrangement4H: {size: 0b00, q: 0b0},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCVTN--FCVTN2--Floating-point-Convert-to-lower-precision-Narrow--vector--?lang=en
	FCVTN: {u: 0b0, opcode: 0b10110, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {size: 0b01, q: 0b0},
		VectorArrangement4H: {size: 0b00, q: 0b0},
	}},
}

// advancedSIMDThreeDifferent holds information to encode instructions as "Advanced SIMD three different" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDThreeDifferent = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMLAL--UMLAL2--vector---Unsigned-Multiply-Add-Long--vector--?lang=en
	VUMLAL: {u: 0b1, opcode: 0b1000, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement2S: {q: 0b0, size: 0b10},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement8B: {q: 0b0, size: 0b00},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL: {u: 0b0, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMULL--SMULL2--vector---Signed-Multiply-Long--vector--?lang=en
	SMULL2: {u: 0b0, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL: {u: 0b1, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B: {q: 0b0, size: 0b00},
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
	UMULL2: {u: 0b1, opcode: 0b1100, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {q: 0b1, size: 0b00},
		VectorArrangement8H:  {q: 0b1, size: 0b01},
		VectorArrangement4S:  {q: 0b1, size: 0b10},
	}},
}

// advancedSIMDThreeSame holds information to encode instructions as "Advanced SIMD three same" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDThreeSame = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/AND--vector---Bitwise-AND--vector--?lang=en
	VAND: {u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BSL--Bitwise-Select-?lang=en
	BSL: {u: 0b1, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b01, q: 0b1},
			VectorArrangement8B:  {size: 0b01, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/EOR--vector---Bitwise-Exclusive-OR--vector--?lang=en
	EOR: {u: 0b1, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ORR--vector--register---Bitwise-inclusive-OR--vector--register--?lang=en
	VORR: {u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b10, q: 0b1},
			VectorArrangement8B:  {size: 0b10, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BIC--vector--register---Bitwise-bit-Clear--vector--register--?lang=en
	BIC: {u: 0b0, opcode: 0b00011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b01, q: 0b1},
			VectorArrangement8B:  {size: 0b01, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FADD--vector---Floating-point-Add--vector--?lang=en
	VFADDS: {u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FADD--vector---Floating-point-Add--vector--?lang=en
	VFADDD: {u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSUB--vector---Floating-point-Subtract--vector--?lang=en
	VFSUBS: {u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FSUB--vector---Floating-point-Subtract--vector--?lang=en
	VFSUBD: {u: 0b0, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMAXP--Unsigned-Maximum-Pairwise-?lang=en
	UMAXP: {u: 0b1, opcode: 0b10100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMEQ--register---Compare-bitwise-Equal--vector--?lang=en
	CMEQ: {u: 0b1, opcode: 0b10001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/dui0801/g/A64-SIMD-Vector-Instructions/ADDP--vector-
	VADDP: {u: 0b0, opcode: 0b10111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADD--vector---Add--vector--?lang=en
	VADD: {u: 0, opcode: 0b10000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SUB--vector---Subtract--vector--?lang=en
	VSUB: {u: 1, opcode: 0b10000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	SSHL: {u: 0, opcode: 0b01000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHL--Signed-Shift-Left--register--?lang=en
	USHL: {u: 0b1, opcode: 0b01000, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGT--register---Compare-signed-Greater-than--vector--?lang=en
	CMGT: {u: 0b0, opcode: 0b00110, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHI--register---Compare-unsigned-Higher--vector--?lang=en
	CMHI: {u: 0b1, opcode: 0b00110, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMGE--register---Compare-signed-Greater-than-or-Equal--vector--?lang=en
	CMGE: {u: 0b0, opcode: 0b00111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/CMHS--register---Compare-unsigned-Higher-or-Same--vector--?lang=en
	CMHS: {u: 0b1, opcode: 0b00111, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMEQ--register---Floating-point-Compare-Equal--vector--?lang=en
	FCMEQ: {u: 0b0, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGT--register---Floating-point-Compare-Greater-than--vector--?lang=en
	FCMGT: {u: 0b1, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FCMGE--register---Floating-point-Compare-Greater-than-or-Equal--vector--?lang=en
	FCMGE: {u: 0b1, opcode: 0b11100,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMIN--vector---Floating-point-minimum--vector--?lang=en
	VFMIN: {u: 0b0, opcode: 0b11110,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b10, q: 0b1},
			VectorArrangement2S: {size: 0b10, q: 0b0},
			VectorArrangement2D: {size: 0b11, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMAX--vector---Floating-point-Maximum--vector--?lang=en
	VFMAX: {u: 0b0, opcode: 0b11110,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FMUL--vector---Floating-point-Multiply--vector--?lang=en
	VFMUL: {u: 0b1, opcode: 0b11011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/FDIV--vector---Floating-point-Divide--vector--?lang=en
	VFDIV: {u: 0b1, opcode: 0b11111,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement4S: {size: 0b00, q: 0b1},
			VectorArrangement2S: {size: 0b00, q: 0b0},
			VectorArrangement2D: {size: 0b01, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/MUL--vector---Multiply--vector--?lang=en
	VMUL: {u: 0b0, opcode: 0b10011, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQADD--Signed-saturating-Add-?lang=en
	VSQADD: {u: 0b0, opcode: 0b00001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQADD--Unsigned-saturating-Add-?lang=en
	VUQADD: {u: 0b1, opcode: 0b00001, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMIN--Signed-Minimum--vector--?lang=en
	SMIN: {u: 0b0, opcode: 0b01101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMAX--Signed-Maximum--vector--?lang=en
	SMAX: {u: 0b0, opcode: 0b01100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMIN--Unsigned-Minimum--vector--?lang=en
	UMIN: {u: 0b1, opcode: 0b01101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMAX--Unsigned-Maximum--vector--?lang=en
	UMAX: {u: 0b1, opcode: 0b01100, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/URHADD--Unsigned-Rounding-Halving-Add-?lang=en
	URHADD: {u: 0b1, opcode: 0b00010, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SQSUB--Signed-saturating-Subtract-?lang=en
	VSQSUB: {u: 0b0, opcode: 0b00101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UQSUB--Unsigned-saturating-Subtract-?lang=en
	VUQSUB: {u: 0b1, opcode: 0b00101, qAndSize: defaultQAndSize},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/BIT--Bitwise-Insert-if-True-?lang=en
	VBIT: {u: 0b1, opcode: 0b00011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement8B:  {q: 0b0, size: 0b10},
		VectorArrangement16B: {q: 0b1, size: 0b10},
	}},
	SQRDMULH: {u: 0b1, opcode: 0b10110, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement4H: {q: 0b0, size: 0b01},
		VectorArrangement8H: {q: 0b1, size: 0b01},
		VectorArrangement2S: {q: 0b0, size: 0b10},
		VectorArrangement4S: {q: 0b1, size: 0b10},
	}},
}

// aAndSize is a pair of "Q" and "size" that appear in https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
type qAndSize struct{ q, size byte }

// defaultQAndSize maps a vector arrangement to the default qAndSize which is encoded by many instructions.
var defaultQAndSize = map[VectorArrangement]qAndSize{
	VectorArrangement8B:  {size: 0b00, q: 0b0},
	VectorArrangement16B: {size: 0b00, q: 0b1},
	VectorArrangement4H:  {size: 0b01, q: 0b0},
	VectorArrangement8H:  {size: 0b01, q: 0b1},
	VectorArrangement2S:  {size: 0b10, q: 0b0},
	VectorArrangement4S:  {size: 0b10, q: 0b1},
	VectorArrangement1D:  {size: 0b11, q: 0b0},
	VectorArrangement2D:  {size: 0b11, q: 0b1},
}

// advancedSIMDAcrossLanes holds information to encode instructions as "Advanced SIMD across lanes" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDAcrossLanes = map[asm.Instruction]struct {
	u, opcode byte
	qAndSize  map[VectorArrangement]qAndSize
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADDV--Add-across-Vector-?lang=en
	ADDV: {u: 0b0, opcode: 0b11011,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
			VectorArrangement8H:  {size: 0b01, q: 0b1},
			VectorArrangement4H:  {size: 0b01, q: 0b0},
			VectorArrangement4S:  {size: 0b10, q: 0b1},
		},
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMINV--Unsigned-Minimum-across-Vector-?lang=en
	UMINV: {u: 0b1, opcode: 0b11010,
		qAndSize: map[VectorArrangement]qAndSize{
			VectorArrangement16B: {size: 0b00, q: 0b1},
			VectorArrangement8B:  {size: 0b00, q: 0b0},
			VectorArrangement8H:  {size: 0b01, q: 0b1},
			VectorArrangement4H:  {size: 0b01, q: 0b0},
			VectorArrangement4S:  {size: 0b10, q: 0b1},
		},
	},
	UADDLV: {u: 0b1, opcode: 0b00011, qAndSize: map[VectorArrangement]qAndSize{
		VectorArrangement16B: {size: 0b00, q: 0b1},
		VectorArrangement8B:  {size: 0b00, q: 0b0},
		VectorArrangement8H:  {size: 0b01, q: 0b1},
		VectorArrangement4H:  {size: 0b01, q: 0b0},
		VectorArrangement4S:  {size: 0b10, q: 0b1},
	}},
}

// advancedSIMDScalarPairwise holds information to encode instructions as "Advanced SIMD scalar pairwise" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDScalarPairwise = map[asm.Instruction]struct {
	u, opcode byte
	size      map[VectorArrangement]byte
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/ADDP--scalar---Add-Pair-of-elements--scalar--?lang=en
	ADDP: {u: 0b0, opcode: 0b11011, size: map[VectorArrangement]byte{VectorArrangement2D: 0b11}},
}

// advancedSIMDCopy holds information to encode instructions as "Advanced SIMD copy" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDCopy = map[asm.Instruction]struct {
	op byte
	// TODO: extract common implementation of resolver.
	resolver func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error)
}{
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/DUP--element---Duplicate-vector-element-to-vector-or-scalar-?lang=en
	DUPELEM: {op: 0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0000
		q = 0b1

		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(srcIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(srcIndex) << 4
		default:
			err = fmt.Errorf("unsupported arrangement for DUPELEM: %d", arr)
		}

		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/DUP--general---Duplicate-general-purpose-register-to-vector-?lang=en
	DUPGEN: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0001
		switch arr {
		case VectorArrangement8B:
			imm5 = 0b1
		case VectorArrangement16B:
			imm5 = 0b1
			q = 0b1
		case VectorArrangement4H:
			imm5 = 0b10
		case VectorArrangement8H:
			imm5 = 0b10
			q = 0b1
		case VectorArrangement2S:
			imm5 = 0b100
		case VectorArrangement4S:
			imm5 = 0b100
			q = 0b1
		case VectorArrangement2D:
			imm5 = 0b1000
			q = 0b1
		default:
			err = fmt.Errorf("unsupported arrangement for DUPGEN: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/INS--general---Insert-vector-element-from-general-purpose-register-?lang=en
	INSGEN: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4, q = 0b0011, 0b1
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(dstIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(dstIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(dstIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(dstIndex) << 4
		default:
			err = fmt.Errorf("unsupported arrangement for INSGEN: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/UMOV--Unsigned-Move-vector-element-to-general-purpose-register-?lang=en
	UMOV: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0111
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(srcIndex) << 3
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(srcIndex) << 4
			q = 0b1
		default:
			err = fmt.Errorf("unsupported arrangement for UMOV: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SMOV--Signed-Move-vector-element-to-general-purpose-register-?lang=en
	SMOV32: {op: 0b0, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		imm4 = 0b0101
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(srcIndex) << 1
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(srcIndex) << 2
		default:
			err = fmt.Errorf("unsupported arrangement for SMOV32: %s", arr)
		}
		return
	}},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/INS--element---Insert-vector-element-from-another-vector-element-?lang=en
	INSELEM: {op: 0b1, resolver: func(srcIndex, dstIndex VectorIndex, arr VectorArrangement) (imm5, imm4, q byte, err error) {
		q = 0b1
		switch arr {
		case VectorArrangementB:
			imm5 |= 0b1
			imm5 |= byte(dstIndex) << 1
			imm4 = byte(srcIndex)
		case VectorArrangementH:
			imm5 |= 0b10
			imm5 |= byte(dstIndex) << 2
			imm4 = byte(srcIndex) << 1
		case VectorArrangementS:
			imm5 |= 0b100
			imm5 |= byte(dstIndex) << 3
			imm4 = byte(srcIndex) << 2
		case VectorArrangementD:
			imm5 |= 0b1000
			imm5 |= byte(dstIndex) << 4
			imm4 = byte(srcIndex) << 3
		default:
			err = fmt.Errorf("unsupported arrangement for INSELEM: %d", arr)
		}
		return
	}},
}

// advancedSIMDTableLookup holds information to encode instructions as "Advanced SIMD table lookup" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDTableLookup = map[asm.Instruction]struct {
	op, op2, Len byte
	q            map[VectorArrangement]byte
}{
	TBL1: {op: 0, op2: 0, Len: 0b00, q: map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8B: 0b0}},
	TBL2: {op: 0, op2: 0, Len: 0b01, q: map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8B: 0b0}},
}

// advancedSIMDShiftByImmediate holds information to encode instructions as "Advanced SIMD shift by immediate" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDShiftByImmediate = map[asm.Instruction]struct {
	U, opcode   byte
	q           map[VectorArrangement]byte
	immResolver func(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error)
}{
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/SSHLL--SSHLL2--Signed-Shift-Left-Long--immediate--
	SSHLL: {U: 0b0, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/SSHLL--SSHLL2--Signed-Shift-Left-Long--immediate--
	SSHLL2: {U: 0b0, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/USHLL--USHLL2--Unsigned-Shift-Left-Long--immediate--
	USHLL: {U: 0b1, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2020-12/SIMD-FP-Instructions/USHLL--USHLL2--Unsigned-Shift-Left-Long--immediate--
	USHLL2: {U: 0b1, opcode: 0b10100,
		q:           map[VectorArrangement]byte{VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1},
		immResolver: immResolverForSIMDSiftLeftByImmediate,
	},
	// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/SSHR--Signed-Shift-Right--immediate--?lang=en
	SSHR: {U: 0b0, opcode: 0b00000,
		q: map[VectorArrangement]byte{
			VectorArrangement16B: 0b1, VectorArrangement8H: 0b1, VectorArrangement4S: 0b1, VectorArrangement2D: 0b1,
			VectorArrangement8B: 0b0, VectorArrangement4H: 0b0, VectorArrangement2S: 0b0,
		},
		immResolver: func(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error) {
			switch arr {
			case VectorArrangement16B, VectorArrangement8B:
				immh = 0b0001
				immb = 8 - byte(shiftAmount&0b111)
			case VectorArrangement8H, VectorArrangement4H:
				v := 16 - byte(shiftAmount&0b1111)
				immb = v & 0b111
				immh = 0b0010 | (v >> 3)
			case VectorArrangement4S, VectorArrangement2S:
				v := 32 - byte(shiftAmount&0b11111)
				immb = v & 0b111
				immh = 0b0100 | (v >> 3)
			case VectorArrangement2D:
				v := 64 - byte(shiftAmount&0b111111)
				immb = v & 0b111
				immh = 0b1000 | (v >> 3)
			default:
				err = fmt.Errorf("unsupported arrangement %s", arr)
			}
			return
		},
	},
}

// advancedSIMDPermute holds information to encode instructions as "Advanced SIMD permute" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
var advancedSIMDPermute = map[asm.Instruction]struct {
	opcode byte
}{
	ZIP1: {opcode: 0b011},
}

func immResolverForSIMDSiftLeftByImmediate(shiftAmount int64, arr VectorArrangement) (immh, immb byte, err error) {
	switch arr {
	case VectorArrangement16B, VectorArrangement8B:
		immb = byte(shiftAmount)
		immh = 0b0001
	case VectorArrangement8H, VectorArrangement4H:
		immb = byte(shiftAmount) & 0b111
		immh = 0b0010 | byte(shiftAmount>>3)
	case VectorArrangement4S, VectorArrangement2S:
		immb = byte(shiftAmount) & 0b111
		immh = 0b0100 | byte(shiftAmount>>3)
	default:
		err = fmt.Errorf("unsupported arrangement %s", arr)
	}
	return
}

// encodeAdvancedSIMDCopy encodes instruction as "Advanced SIMD copy" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDCopy(srcRegBits, dstRegBits, op, imm5, imm4, q byte) {
	a.Buf.Write([]byte{
		(srcRegBits << 5) | dstRegBits,
		imm4<<3 | 0b1<<2 | srcRegBits>>3,
		imm5,
		q<<6 | op<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDThreeSame encodes instruction as  "Advanced SIMD three same" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDThreeSame(src1, src2, dst, opcode, size, q, u byte) {
	a.Buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<3 | 1<<2 | src2>>3,
		size<<6 | 0b1<<5 | src1,
		q<<6 | u<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDThreeDifferent encodes instruction as  "Advanced SIMD three different" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDThreeDifferent(src1, src2, dst, opcode, size, q, u byte) {
	a.Buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<4 | src2>>3,
		size<<6 | 0b1<<5 | src1,
		q<<6 | u<<5 | 0b1110,
	})
}

// encodeAdvancedSIMDPermute encodes instruction as  "Advanced SIMD permute" in
// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
func (a *AssemblerImpl) encodeAdvancedSIMDPermute(src1, src2, dst, opcode, size, q byte) {
	a.Buf.Write([]byte{
		(src2 << 5) | dst,
		opcode<<4 | 0b1<<3 | src2>>3,
		size<<6 | src1,
		q<<6 | 0b1110,
	})
}

func (a *AssemblerImpl) EncodeVectorRegisterToVectorRegister(n *NodeImpl) (err error) {
	var srcVectorRegBits byte
	if n.SrcReg != RegRZR {
		srcVectorRegBits, err = vectorRegisterBits(n.SrcReg)
		if err != nil {
			return err
		}
	}

	dstVectorRegBits, err := vectorRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.Instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.SrcVectorIndex, n.DstVectorIndex, n.VectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcVectorRegBits, dstVectorRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}

	if scalarPairwise, ok := advancedSIMDScalarPairwise[n.Instruction]; ok {
		// See "Advanced SIMD scalar pairwise" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		size, ok := scalarPairwise.size[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.Buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			scalarPairwise.opcode<<4 | 1<<3 | srcVectorRegBits>>3,
			size<<6 | 0b11<<4 | scalarPairwise.opcode>>4,
			0b1<<6 | scalarPairwise.u<<5 | 0b11110,
		})
		return
	}

	if twoRegMisc, ok := advancedSIMDTwoRegisterMisc[n.Instruction]; ok {
		// See "Advanced SIMD two-register miscellaneous" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		qs, ok := twoRegMisc.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.Buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			twoRegMisc.opcode<<4 | 0b1<<3 | srcVectorRegBits>>3,
			qs.size<<6 | 0b1<<5 | twoRegMisc.opcode>>4,
			qs.q<<6 | twoRegMisc.u<<5 | 0b01110,
		})
		return nil
	}

	if threeSame, ok := advancedSIMDThreeSame[n.Instruction]; ok {
		qs, ok := threeSame.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.encodeAdvancedSIMDThreeSame(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, threeSame.opcode, qs.size, qs.q, threeSame.u)
		return nil
	}

	if threeDifferent, ok := advancedSIMDThreeDifferent[n.Instruction]; ok {
		qs, ok := threeDifferent.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.encodeAdvancedSIMDThreeDifferent(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, threeDifferent.opcode, qs.size, qs.q, threeDifferent.u)
		return nil
	}

	if acrossLanes, ok := advancedSIMDAcrossLanes[n.Instruction]; ok {
		// See "Advanced SIMD across lanes" in
		// https://developer.arm.com/documentation/ddi0596/2021-12/Index-by-Encoding/Data-Processing----Scalar-Floating-Point-and-Advanced-SIMD?lang=en
		qs, ok := acrossLanes.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.Buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			acrossLanes.opcode<<4 | 0b1<<3 | srcVectorRegBits>>3,
			qs.size<<6 | 0b11000<<1 | acrossLanes.opcode>>4,
			qs.q<<6 | acrossLanes.u<<5 | 0b01110,
		})
		return nil
	}

	if lookup, ok := advancedSIMDTableLookup[n.Instruction]; ok {
		q, ok := lookup.q[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.Buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			lookup.Len<<5 | lookup.op<<4 | srcVectorRegBits>>3,
			lookup.op2<<6 | dstVectorRegBits,
			q<<6 | 0b1110,
		})
		return
	}

	if shiftByImmediate, ok := advancedSIMDShiftByImmediate[n.Instruction]; ok {
		immh, immb, err := shiftByImmediate.immResolver(n.SrcConst, n.VectorArrangement)
		if err != nil {
			return err
		}

		q, ok := shiftByImmediate.q[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}

		a.Buf.Write([]byte{
			(srcVectorRegBits << 5) | dstVectorRegBits,
			shiftByImmediate.opcode<<3 | 0b1<<2 | srcVectorRegBits>>3,
			immh<<3 | immb,
			q<<6 | shiftByImmediate.U<<5 | 0b1111,
		})
		return nil
	}

	if permute, ok := advancedSIMDPermute[n.Instruction]; ok {
		size, q := arrangementSizeQ(n.VectorArrangement)
		a.encodeAdvancedSIMDPermute(srcVectorRegBits, dstVectorRegBits, dstVectorRegBits, permute.opcode, size, q)
		return
	}
	return errorEncodingUnsupported(n)
}

func (a *AssemblerImpl) encodeTwoVectorRegistersToVectorRegister(n *NodeImpl) (err error) {
	var srcRegBits, srcRegBits2, dstRegBits byte
	srcRegBits, err = vectorRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	srcRegBits2, err = vectorRegisterBits(n.SrcReg2)
	if err != nil {
		return err
	}

	dstRegBits, err = vectorRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	if threeSame, ok := advancedSIMDThreeSame[n.Instruction]; ok {
		qs, ok := threeSame.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.encodeAdvancedSIMDThreeSame(srcRegBits, srcRegBits2, dstRegBits, threeSame.opcode, qs.size, qs.q, threeSame.u)
		return nil
	}

	if threeDifferent, ok := advancedSIMDThreeDifferent[n.Instruction]; ok {
		qs, ok := threeDifferent.qAndSize[n.VectorArrangement]
		if !ok {
			return fmt.Errorf("unsupported vector arrangement %s for %s", n.VectorArrangement, InstructionName(n.Instruction))
		}
		a.encodeAdvancedSIMDThreeDifferent(srcRegBits, srcRegBits2, dstRegBits, threeDifferent.opcode, qs.size, qs.q, threeDifferent.u)
		return nil
	}

	if permute, ok := advancedSIMDPermute[n.Instruction]; ok {
		size, q := arrangementSizeQ(n.VectorArrangement)
		a.encodeAdvancedSIMDPermute(srcRegBits, srcRegBits2, dstRegBits, permute.opcode, size, q)
		return
	}

	if n.Instruction == EXT {
		// EXT is the only instruction in "Advanced SIMD extract", so inline the encoding here.
		// https://developer.arm.com/documentation/ddi0596/2021-12/SIMD-FP-Instructions/EXT--Extract-vector-from-pair-of-vectors-?lang=en
		var q, imm4 byte
		switch n.VectorArrangement {
		case VectorArrangement16B:
			imm4 = 0b1111 & byte(n.SrcConst)
			q = 0b1
		case VectorArrangement8B:
			imm4 = 0b111 & byte(n.SrcConst)
		default:
			return fmt.Errorf("invalid arrangement %s for EXT", n.VectorArrangement)
		}
		a.Buf.Write([]byte{
			(srcRegBits2 << 5) | dstRegBits,
			imm4<<3 | srcRegBits2>>3,
			srcRegBits,
			q<<6 | 0b101110,
		})
		return
	}
	return
}

func (a *AssemblerImpl) EncodeVectorRegisterToRegister(n *NodeImpl) (err error) {
	if err = checkArrangementIndexPair(n.VectorArrangement, n.SrcVectorIndex); err != nil {
		return
	}

	srcVecRegBits, err := vectorRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	dstRegBits, err := intRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.Instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.SrcVectorIndex, n.DstVectorIndex, n.VectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcVecRegBits, dstRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}
	return errorEncodingUnsupported(n)
}

func (a *AssemblerImpl) EncodeRegisterToVectorRegister(n *NodeImpl) (err error) {
	srcRegBits, err := intRegisterBits(n.SrcReg)
	if err != nil {
		return err
	}

	dstVectorRegBits, err := vectorRegisterBits(n.DstReg)
	if err != nil {
		return err
	}

	if simdCopy, ok := advancedSIMDCopy[n.Instruction]; ok {
		imm5, imm4, q, err := simdCopy.resolver(n.SrcVectorIndex, n.DstVectorIndex, n.VectorArrangement)
		if err != nil {
			return err
		}
		a.encodeAdvancedSIMDCopy(srcRegBits, dstVectorRegBits, simdCopy.op, imm5, imm4, q)
		return nil
	}
	return errorEncodingUnsupported(n)
}

var zeroRegisterBits byte = 0b11111

func isIntRegister(r asm.Register) bool {
	return RegR0 <= r && r <= RegRZR
}

func isVectorRegister(r asm.Register) bool {
	return RegV0 <= r && r <= RegV31
}

func isConditionalRegister(r asm.Register) bool {
	return RegCondEQ <= r && r <= RegCondNV
}

func intRegisterBits(r asm.Register) (ret byte, err error) {
	if !isIntRegister(r) {
		err = fmt.Errorf("%s is not integer", RegisterName(r))
	} else {
		ret = byte(r - RegR0)
	}
	return
}

func vectorRegisterBits(r asm.Register) (ret byte, err error) {
	if !isVectorRegister(r) {
		err = fmt.Errorf("%s is not vector", RegisterName(r))
	} else {
		ret = byte(r - RegV0)
	}
	return
}

func registerBits(r asm.Register) (ret byte) {
	if isIntRegister(r) {
		ret = byte(r - RegR0)
	} else {
		ret = byte(r - RegV0)
	}
	return
}
