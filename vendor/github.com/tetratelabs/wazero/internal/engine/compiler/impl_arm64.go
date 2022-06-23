// This file implements the compiler for arm64 target.
// Please refer to https://developer.arm.com/documentation/102374/latest/
// if unfamiliar with arm64 instructions and semantics.
//
// Note: we use arm64 pkg as the assembler (github.com/twitchyliquid64/golang-asm/obj/arm64)
// which has different notation from the original arm64 assembly. For example,
// 64-bit variant ldr, str, stur are all corresponding to arm64.MOVD.
// Please refer to https://pkg.go.dev/cmd/internal/obj/garm64.

package compiler

import (
	"errors"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/asm"
	"github.com/tetratelabs/wazero/internal/asm/arm64"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wazeroir"
)

type arm64Compiler struct {
	assembler arm64.Assembler
	ir        *wazeroir.CompilationResult
	// locationStack holds the state of wazeroir virtual stack.
	// and each item is either placed in register or the actual memory stack.
	locationStack *runtimeValueLocationStack
	// labels maps a label (Ex. ".L1_then") to *arm64LabelInfo.
	labels map[string]*arm64LabelInfo
	// stackPointerCeil is the greatest stack pointer value (from runtimeValueLocationStack) seen during compilation.
	stackPointerCeil uint64
	// onStackPointerCeilDeterminedCallBack hold a callback which are called when the ceil of stack pointer is determined before generating native code.
	onStackPointerCeilDeterminedCallBack func(stackPointerCeil uint64)
}

func newArm64Compiler(ir *wazeroir.CompilationResult) (compiler, error) {
	return &arm64Compiler{
		assembler:     arm64.NewAssemblerImpl(arm64ReservedRegisterForTemporary),
		locationStack: newRuntimeValueLocationStack(),
		ir:            ir,
		labels:        map[string]*arm64LabelInfo{},
	}, nil
}

var (
	arm64UnreservedVectorRegisters = []asm.Register{
		arm64.RegV0, arm64.RegV1, arm64.RegV2, arm64.RegV3,
		arm64.RegV4, arm64.RegV5, arm64.RegV6, arm64.RegV7, arm64.RegV8,
		arm64.RegV9, arm64.RegV10, arm64.RegV11, arm64.RegV12, arm64.RegV13,
		arm64.RegV14, arm64.RegV15, arm64.RegV16, arm64.RegV17, arm64.RegV18,
		arm64.RegV19, arm64.RegV20, arm64.RegV21, arm64.RegV22, arm64.RegV23,
		arm64.RegV24, arm64.RegV25, arm64.RegV26, arm64.RegV27, arm64.RegV28,
		arm64.RegV29, arm64.RegV30, arm64.RegV31,
	}

	// Note (see arm64 section in https://go.dev/doc/asm):
	// * RegR18 is reserved as a platform register, and we don't use it in Compiler.
	// * RegR28 is reserved for Goroutine by Go runtime, and we don't use it in Compiler.
	arm64UnreservedGeneralPurposeRegisters = []asm.Register{ // nolint
		arm64.RegR3, arm64.RegR4, arm64.RegR5, arm64.RegR6, arm64.RegR7, arm64.RegR8,
		arm64.RegR9, arm64.RegR10, arm64.RegR11, arm64.RegR12, arm64.RegR13,
		arm64.RegR14, arm64.RegR15, arm64.RegR16, arm64.RegR17, arm64.RegR19,
		arm64.RegR20, arm64.RegR21, arm64.RegR22, arm64.RegR23, arm64.RegR24,
		arm64.RegR25, arm64.RegR26, arm64.RegR29, arm64.RegR30,
	}
)

const (
	// arm64ReservedRegisterForCallEngine holds the pointer to callEngine instance (i.e. *callEngine as uintptr)
	arm64ReservedRegisterForCallEngine = arm64.RegR0
	// arm64ReservedRegisterForStackBasePointerAddress holds stack base pointer's address (callEngine.stackBasePointer) in the current function call.
	arm64ReservedRegisterForStackBasePointerAddress = arm64.RegR1
	// arm64ReservedRegisterForMemory holds the pointer to the memory slice's data (i.e. &memory.Buffer[0] as uintptr).
	arm64ReservedRegisterForMemory = arm64.RegR2
	// arm64ReservedRegisterForTemporary is the temporary register which is available at any point of execution, but its content shouldn't be supposed to live beyond the single operation.
	// Note: we choose R27 as that is the temporary register used in Go's assembler.
	arm64ReservedRegisterForTemporary = arm64.RegR27
)

var (
	arm64CallingConventionModuleInstanceAddressRegister = arm64.RegR29
)

const (
	// arm64CallEngineArchContextCompilerCallReturnAddressOffset is the offset of archContext.nativeCallReturnAddress in callEngine.
	arm64CallEngineArchContextCompilerCallReturnAddressOffset = 136
	// arm64CallEngineArchContextMinimum32BitSignedIntOffset is the offset of archContext.minimum32BitSignedIntAddress in callEngine.
	arm64CallEngineArchContextMinimum32BitSignedIntOffset = 144
	// arm64CallEngineArchContextMinimum64BitSignedIntOffset is the offset of archContext.minimum64BitSignedIntAddress in callEngine.
	arm64CallEngineArchContextMinimum64BitSignedIntOffset = 152
)

func isZeroRegister(r asm.Register) bool {
	return r == arm64.RegRZR
}

// compile implements compiler.compile for the arm64 architecture.
func (c *arm64Compiler) compile() (code []byte, stackPointerCeil uint64, err error) {
	// c.stackPointerCeil tracks the stack pointer ceiling (max seen) value across all runtimeValueLocationStack(s)
	// used for all labels (via setLocationStack), excluding the current one.
	// Hence, we check here if the final block's max one exceeds the current c.stackPointerCeil.
	stackPointerCeil = c.stackPointerCeil
	if stackPointerCeil < c.locationStack.stackPointerCeil {
		stackPointerCeil = c.locationStack.stackPointerCeil
	}

	// Now that the ceil of stack pointer is determined, we are invoking the callback.
	// Note: this must be called before Assemble() below.
	if c.onStackPointerCeilDeterminedCallBack != nil {
		c.onStackPointerCeilDeterminedCallBack(stackPointerCeil)
	}

	var original []byte
	original, err = c.assembler.Assemble()
	if err != nil {
		return
	}

	code, err = platform.MmapCodeSegment(original)
	if err != nil {
		return
	}
	return
}

// arm64LabelInfo holds a wazeroir label specific information in this function.
type arm64LabelInfo struct {
	// initialInstruction is the initial instruction for this label so other block can branch into it.
	initialInstruction asm.Node
	// initialStack is the initial value location stack from which we start compiling this label.
	initialStack *runtimeValueLocationStack
	// labelBeginningCallbacks holds callbacks should to be called with initialInstruction
	labelBeginningCallbacks []func(asm.Node)
}

func (c *arm64Compiler) label(labelKey string) *arm64LabelInfo {
	ret, ok := c.labels[labelKey]
	if ok {
		return ret
	}
	c.labels[labelKey] = &arm64LabelInfo{}
	return c.labels[labelKey]
}

func (c *arm64Compiler) pushRuntimeValueLocationOnRegister(reg asm.Register, vt runtimeValueType) (ret *runtimeValueLocation) {
	ret = c.locationStack.pushRuntimeValueLocationOnRegister(reg, vt)
	c.markRegisterUsed(reg)
	return
}

func (c *arm64Compiler) pushVectorRuntimeValueLocationOnRegister(reg asm.Register) {
	c.locationStack.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeV128Lo)
	c.locationStack.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeV128Hi)
	c.markRegisterUsed(reg)
}

func (c *arm64Compiler) markRegisterUsed(regs ...asm.Register) {
	for _, reg := range regs {
		if !isZeroRegister(reg) && reg != asm.NilRegister {
			c.locationStack.markRegisterUsed(reg)
		}
	}
}

func (c *arm64Compiler) markRegisterUnused(regs ...asm.Register) {
	for _, reg := range regs {
		if !isZeroRegister(reg) && reg != asm.NilRegister {
			c.locationStack.markRegisterUnused(reg)
		}
	}
}

func (c *arm64Compiler) String() (ret string) { return }

// pushFunctionParams pushes any function parameters onto the stack, setting appropriate register types.
func (c *arm64Compiler) pushFunctionParams() {
	for _, t := range c.ir.Signature.Params {
		loc := c.locationStack.pushRuntimeValueLocationOnStack()
		switch t {
		case wasm.ValueTypeI32:
			loc.valueType = runtimeValueTypeI32
		case wasm.ValueTypeI64, wasm.ValueTypeFuncref, wasm.ValueTypeExternref:
			loc.valueType = runtimeValueTypeI64
		case wasm.ValueTypeF32:
			loc.valueType = runtimeValueTypeF32
		case wasm.ValueTypeF64:
			loc.valueType = runtimeValueTypeF64
		case wasm.ValueTypeV128:
			loc.valueType = runtimeValueTypeV128Lo
			hi := c.locationStack.pushRuntimeValueLocationOnStack()
			hi.valueType = runtimeValueTypeV128Hi
		default:
			panic("BUG")
		}
	}
}

// compilePreamble implements compiler.compilePreamble for the arm64 architecture.
func (c *arm64Compiler) compilePreamble() error {
	// The assembler skips the first instruction so we intentionally add NOP here.
	// TODO: delete after #233
	c.assembler.CompileStandAlone(arm64.NOP)

	c.pushFunctionParams()

	// Check if it's necessary to grow the value stack before entering function body.
	if err := c.compileMaybeGrowValueStack(); err != nil {
		return err
	}

	if err := c.compileModuleContextInitialization(); err != nil {
		return err
	}

	// We must initialize the stack base pointer register so that we can manipulate the stack properly.
	c.compileReservedStackBasePointerRegisterInitialization()

	c.compileReservedMemoryRegisterInitialization()
	return nil
}

// compileMaybeGrowValueStack adds instructions to check the necessity to grow the value stack,
// and if so, make the builtin function call to do so. These instructions are called in the function's
// preamble.
func (c *arm64Compiler) compileMaybeGrowValueStack() error {
	tmpRegs, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 2)
	if !found {
		return fmt.Errorf("BUG: all the registers should be free at this point")
	}
	tmpX, tmpY := tmpRegs[0], tmpRegs[1]

	// "tmpX = len(ce.valueStack)"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextValueStackLenOffset,
		tmpX,
	)

	// "tmpY = ce.stackBasePointer"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineValueStackContextStackBasePointerOffset,
		tmpY,
	)

	// "tmpX = tmpX - tmpY", in other words "tmpX = len(ce.valueStack) - ce.stackBasePointer"
	c.assembler.CompileRegisterToRegister(
		arm64.SUB,
		tmpY,
		tmpX,
	)

	// "tmpY = stackPointerCeil"
	loadStackPointerCeil := c.assembler.CompileConstToRegister(
		arm64.MOVD,
		math.MaxInt32,
		tmpY,
	)
	// At this point of compilation, we don't know the value of stack point ceil,
	// so we lazily resolve the value later.
	c.onStackPointerCeilDeterminedCallBack = func(stackPointerCeil uint64) { loadStackPointerCeil.AssignSourceConstant(int64(stackPointerCeil)) }

	// Compare tmpX (len(ce.valueStack) - ce.stackBasePointer) and tmpY (ce.stackPointerCeil)
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, tmpX, tmpY)

	// If ceil > valueStackLen - stack base pointer, we need to grow the stack by calling builtin Go function.
	brIfValueStackOK := c.assembler.CompileJump(arm64.BCONDLS)
	if err := c.compileCallGoFunction(nativeCallStatusCodeCallBuiltInFunction, builtinFunctionIndexGrowValueStack); err != nil {
		return err
	}

	// Otherwise, skip calling it.
	c.assembler.SetJumpTargetOnNext(brIfValueStackOK)

	c.markRegisterUnused(tmpRegs...)
	return nil
}

// returnFunction emits instructions to return from the current function frame.
// If the current frame is the bottom, the code goes back to the Go code with nativeCallStatusCodeReturned status.
// Otherwise, we branch into the caller's return address.
func (c *arm64Compiler) compileReturnFunction() error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	// arm64CallingConventionModuleInstanceAddressRegister holds the module intstance's address
	// so mark it used so that it won't be used as a free register.
	c.locationStack.markRegisterUsed(arm64CallingConventionModuleInstanceAddressRegister)
	defer c.locationStack.markRegisterUnused(arm64CallingConventionModuleInstanceAddressRegister)

	tmpRegs, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 3)
	if !found {
		return fmt.Errorf("BUG: all the registers should be free at this point")
	}

	// Alias for readability.
	callFramePointerReg, callFrameStackTopAddressRegister, tmpReg := tmpRegs[0], tmpRegs[1], tmpRegs[2]

	// First we decrement the call-frame stack pointer.
	c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset, callFramePointerReg)
	c.assembler.CompileConstToRegister(arm64.SUBS, 1, callFramePointerReg)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, callFramePointerReg, arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset)

	// Next we compare the decremented call frame stack pointer with zero.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, callFramePointerReg, arm64.RegRZR)

	// If the values are identical, we return back to the Go code with returned status.
	brIfNotEqual := c.assembler.CompileJump(arm64.BCONDNE)
	c.compileExitFromNativeCode(nativeCallStatusCodeReturned)

	// Otherwise, we have to jump to the caller's return address.
	c.assembler.SetJumpTargetOnNext(brIfNotEqual)

	// First, we have to calculate the caller callFrame's absolute address to acquire the return address.
	//
	// "tmpReg = &ce.callFrameStack[0]"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackElement0AddressOffset,
		tmpReg,
	)
	// "callFrameStackTopAddressRegister = tmpReg + callFramePointerReg << ${callFrameDataSizeMostSignificantSetBit}"
	c.assembler.CompileLeftShiftedRegisterToRegister(
		arm64.ADD,
		callFramePointerReg, callFrameDataSizeMostSignificantSetBit,
		tmpReg,
		callFrameStackTopAddressRegister,
	)

	// At this point, we have
	//
	//      [......., ra.caller, rb.caller, rc.caller, _, ra.current, rb.current, rc.current, _, ...]  <- call frame stack's data region (somewhere in the memory)
	//                                                  ^
	//                               callFrameStackTopAddressRegister
	//                   (absolute address of &callFrameStack[ce.callFrameStackPointer])
	//
	// where:
	//      ra.* = callFrame.returnAddress
	//      rb.* = callFrame.returnStackBasePointer
	//      rc.* = callFrame.code
	//      _  = callFrame's padding (see comment on callFrame._ field.)
	//
	// What we have to do in the following is that
	//   1) Set ce.valueStackContext.stackBasePointer to the value on "rb.caller".
	//   2) Load rc.caller.moduleInstanceAddress into arm64CallingConventionModuleInstanceAddressRegister.
	//   3) Jump into the address of "ra.caller".

	// 1) Set ce.valueStackContext.stackBasePointer to the value on "rb.caller".
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		// "rb.caller" is below the top address.
		callFrameStackTopAddressRegister, -(callFrameDataSize - callFrameReturnStackBasePointerOffset),
		tmpReg)
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		tmpReg,
		arm64ReservedRegisterForCallEngine, callEngineValueStackContextStackBasePointerOffset)

	// 2) Load rc.caller.moduleInstanceAddress into arm64CallingConventionModuleInstanceAddressRegister.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		// "rb.caller" is below the top address.
		callFrameStackTopAddressRegister, -(callFrameDataSize - callFrameFunctionOffset),
		arm64CallingConventionModuleInstanceAddressRegister)
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64CallingConventionModuleInstanceAddressRegister, functionModuleInstanceAddressOffset,
		arm64CallingConventionModuleInstanceAddressRegister)

	// 3) Branch into the address of "ra.caller".
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		// "rb.caller" is below the top address.
		callFrameStackTopAddressRegister, -(callFrameDataSize - callFrameReturnAddressOffset),
		tmpReg)
	c.assembler.CompileJumpToMemory(arm64.B, tmpReg)

	c.markRegisterUnused(tmpRegs...)
	return nil
}

// compileExitFromNativeCode adds instructions to give the control back to ce.exec with the given status code.
func (c *arm64Compiler) compileExitFromNativeCode(status nativeCallStatusCode) {
	// Write the current stack pointer to the ce.stackPointer.
	c.assembler.CompileConstToRegister(arm64.MOVD, int64(c.locationStack.sp), arm64ReservedRegisterForTemporary)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64ReservedRegisterForTemporary, arm64ReservedRegisterForCallEngine,
		callEngineValueStackContextStackPointerOffset)

	if status != 0 {
		c.assembler.CompileConstToRegister(arm64.MOVW, int64(status), arm64ReservedRegisterForTemporary)
		c.assembler.CompileRegisterToMemory(arm64.MOVWU, arm64ReservedRegisterForTemporary, arm64ReservedRegisterForCallEngine, callEngineExitContextNativeCallStatusCodeOffset)
	} else {
		// If the status == 0, we use zero register to store zero.
		c.assembler.CompileRegisterToMemory(arm64.MOVWU, arm64.RegRZR, arm64ReservedRegisterForCallEngine, callEngineExitContextNativeCallStatusCodeOffset)
	}

	// The return address to the Go code is stored in archContext.compilerReturnAddress which
	// is embedded in ce. We load the value to the tmpRegister, and then
	// invoke RET with that register.
	c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine, arm64CallEngineArchContextCompilerCallReturnAddressOffset, arm64ReservedRegisterForTemporary)

	c.assembler.CompileJumpToRegister(arm64.RET, arm64ReservedRegisterForTemporary)
}

// compileHostFunction implements compiler.compileHostFunction for the arm64 architecture.
func (c *arm64Compiler) compileHostFunction() error {
	// The assembler skips the first instruction so we intentionally add NOP here.
	// TODO: delete after #233
	c.assembler.CompileStandAlone(arm64.NOP)

	// First we must update the location stack to reflect the number of host function inputs.
	c.pushFunctionParams()

	if err := c.compileCallGoFunction(nativeCallStatusCodeCallHostFunction, 0); err != nil {
		return err
	}
	return c.compileReturnFunction()
}

// setLocationStack sets the given runtimeValueLocationStack to .locationStack field,
// while allowing us to track runtimeValueLocationStack.stackPointerCeil across multiple stacks.
// This is called when we branch into different block.
func (c *arm64Compiler) setLocationStack(newStack *runtimeValueLocationStack) {
	if c.stackPointerCeil < c.locationStack.stackPointerCeil {
		c.stackPointerCeil = c.locationStack.stackPointerCeil
	}
	c.locationStack = newStack
}

// arm64Compiler implements compiler.arm64Compiler for the arm64 architecture.
func (c *arm64Compiler) compileLabel(o *wazeroir.OperationLabel) (skipThisLabel bool) {
	labelKey := o.Label.String()
	arm64LabelInfo := c.label(labelKey)

	// If initialStack is not set, that means this label has never been reached.
	if arm64LabelInfo.initialStack == nil {
		skipThisLabel = true
		return
	}

	// We use NOP as a beginning of instructions in a label.
	// This should be eventually optimized out by assembler.
	labelBegin := c.assembler.CompileStandAlone(arm64.NOP)

	// Save the instructions so that backward branching
	// instructions can branch to this label.
	arm64LabelInfo.initialInstruction = labelBegin

	// Set the initial stack.
	c.setLocationStack(arm64LabelInfo.initialStack)

	// Invoke callbacks to notify the forward branching
	// instructions can properly branch to this label.
	for _, cb := range arm64LabelInfo.labelBeginningCallbacks {
		cb(labelBegin)
	}
	return false
}

// compileUnreachable implements compiler.compileUnreachable for the arm64 architecture.
func (c *arm64Compiler) compileUnreachable() error {
	c.compileExitFromNativeCode(nativeCallStatusCodeUnreachable)
	return nil
}

// compileSwap implements compiler.compileSwap for the arm64 architecture.
func (c *arm64Compiler) compileSwap(o *wazeroir.OperationSwap) error {
	yIndex := int(c.locationStack.sp) - 1 - o.Depth
	var x, y *runtimeValueLocation
	if o.IsTargetVector {
		x, y = c.locationStack.stack[c.locationStack.sp-2], c.locationStack.stack[yIndex]
	} else {
		x, y = c.locationStack.peek(), c.locationStack.stack[yIndex]
	}

	if err := c.compileEnsureOnRegister(x); err != nil {
		return err
	}

	if err := c.compileEnsureOnRegister(y); err != nil {
		return err
	}

	x.register, y.register = y.register, x.register
	if o.IsTargetVector {
		x, y = c.locationStack.peek(), c.locationStack.stack[yIndex+1]
		x.register, y.register = y.register, x.register
	}
	return nil
}

// compileGlobalGet implements compiler.compileGlobalGet for the arm64 architecture.
func (c *arm64Compiler) compileGlobalGet(o *wazeroir.OperationGlobalGet) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	wasmValueType := c.ir.Globals[o.Index].ValType
	isV128 := wasmValueType == wasm.ValueTypeV128
	// Get the address of globals[index] into intReg.
	intReg, err := c.compileReadGlobalAddress(o.Index)
	if err != nil {
		return err
	}

	if isV128 {
		resultReg, err := c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
		c.assembler.CompileConstToRegister(arm64.ADD, globalInstanceValueOffset, intReg)
		c.assembler.CompileMemoryToVectorRegister(arm64.VMOV, intReg, 0,
			resultReg, arm64.VectorArrangementQ)

		c.pushVectorRuntimeValueLocationOnRegister(resultReg)
	} else {

		var intMov, floatMov = arm64.NOP, arm64.NOP
		var vt runtimeValueType
		switch wasmValueType {
		case wasm.ValueTypeI32:
			intMov = arm64.MOVWU
			vt = runtimeValueTypeI32
		case wasm.ValueTypeI64, wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
			intMov = arm64.MOVD
			vt = runtimeValueTypeI64
		case wasm.ValueTypeF32:
			intMov = arm64.MOVWU
			floatMov = arm64.FMOVS
			vt = runtimeValueTypeF32
		case wasm.ValueTypeF64:
			intMov = arm64.MOVD
			floatMov = arm64.FMOVD
			vt = runtimeValueTypeF64
		}

		// "intReg = [intReg + globalInstanceValueOffset] (== globals[index].Val)"
		c.assembler.CompileMemoryToRegister(
			intMov,
			intReg, globalInstanceValueOffset,
			intReg,
		)

		// If the value type is float32 or float64, we have to move the value
		// further into the float register.
		resultReg := intReg
		if floatMov != arm64.NOP {
			resultReg, err = c.allocateRegister(registerTypeVector)
			if err != nil {
				return err
			}
			c.assembler.CompileRegisterToRegister(floatMov, intReg, resultReg)
		}

		c.pushRuntimeValueLocationOnRegister(resultReg, vt)
	}
	return nil
}

// compileGlobalSet implements compiler.compileGlobalSet for the arm64 architecture.
func (c *arm64Compiler) compileGlobalSet(o *wazeroir.OperationGlobalSet) error {
	wasmValueType := c.ir.Globals[o.Index].ValType
	isV128 := wasmValueType == wasm.ValueTypeV128

	var val *runtimeValueLocation
	if isV128 {
		val = c.locationStack.popV128()
	} else {
		val = c.locationStack.pop()
	}
	if err := c.compileEnsureOnRegister(val); err != nil {
		return err
	}

	globalInstanceAddressRegister, err := c.compileReadGlobalAddress(o.Index)
	if err != nil {
		return err
	}

	if isV128 {
		c.assembler.CompileVectorRegisterToMemory(arm64.VMOV,
			val.register, globalInstanceAddressRegister, globalInstanceValueOffset,
			arm64.VectorArrangementQ)
	} else {
		var mov asm.Instruction
		switch c.ir.Globals[o.Index].ValType {
		case wasm.ValueTypeI32:
			mov = arm64.MOVWU
		case wasm.ValueTypeI64, wasm.ValueTypeExternref, wasm.ValueTypeFuncref:
			mov = arm64.MOVD
		case wasm.ValueTypeF32:
			mov = arm64.FMOVS
		case wasm.ValueTypeF64:
			mov = arm64.FMOVD
		}

		// At this point "globalInstanceAddressRegister = globals[index]".
		// Therefore, this means "globals[index].Val = val.register"
		c.assembler.CompileRegisterToMemory(
			mov,
			val.register,
			globalInstanceAddressRegister, globalInstanceValueOffset,
		)
	}

	c.markRegisterUnused(val.register)
	return nil
}

// compileReadGlobalAddress adds instructions to store the absolute address of the global instance at globalIndex into a register
func (c *arm64Compiler) compileReadGlobalAddress(globalIndex uint32) (destinationRegister asm.Register, err error) {
	// TODO: rethink about the type used in store `globals []*GlobalInstance`.
	// If we use `[]GlobalInstance` instead, we could reduce one MOV instruction here.

	destinationRegister, err = c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return
	}

	// "destinationRegister = globalIndex * 8"
	c.assembler.CompileConstToRegister(
		// globalIndex is an index to []*GlobalInstance, therefore
		// we have to multiply it by the size of *GlobalInstance == the pointer size == 8.
		arm64.MOVD, int64(globalIndex)*8, destinationRegister,
	)

	// "arm64ReservedRegisterForTemporary = &globals[0]"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextGlobalElement0AddressOffset,
		arm64ReservedRegisterForTemporary,
	)

	// "destinationRegister = [arm64ReservedRegisterForTemporary + destinationRegister] (== globals[globalIndex])".
	c.assembler.CompileMemoryWithRegisterOffsetToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForTemporary, destinationRegister,
		destinationRegister,
	)
	return
}

// compileBr implements compiler.compileBr for the arm64 architecture.
func (c *arm64Compiler) compileBr(o *wazeroir.OperationBr) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()
	return c.compileBranchInto(o.Target)
}

// compileBrIf implements compiler.compileBrIf for the arm64 architecture.
func (c *arm64Compiler) compileBrIf(o *wazeroir.OperationBrIf) error {
	cond := c.locationStack.pop()

	var conditionalBR asm.Node
	if cond.onConditionalRegister() {
		// If the cond is on a conditional register, it corresponds to one of "conditional codes"
		// https://developer.arm.com/documentation/dui0801/a/Condition-Codes/Condition-code-suffixes
		// Here we represent the conditional codes by using arm64.COND_** registers, and that means the
		// conditional jump can be performed if we use arm64.B**.
		// For example, if we have arm64.CondEQ on cond, that means we performed compileEq right before
		// this compileBrIf and BrIf can be achieved by arm64.BCONDEQ.
		var brInst asm.Instruction
		switch cond.conditionalRegister {
		case arm64.CondEQ:
			brInst = arm64.BCONDEQ
		case arm64.CondNE:
			brInst = arm64.BCONDNE
		case arm64.CondHS:
			brInst = arm64.BCONDHS
		case arm64.CondLO:
			brInst = arm64.BCONDLO
		case arm64.CondMI:
			brInst = arm64.BCONDMI
		case arm64.CondHI:
			brInst = arm64.BCONDHI
		case arm64.CondLS:
			brInst = arm64.BCONDLS
		case arm64.CondGE:
			brInst = arm64.BCONDGE
		case arm64.CondLT:
			brInst = arm64.BCONDLT
		case arm64.CondGT:
			brInst = arm64.BCONDGT
		case arm64.CondLE:
			brInst = arm64.BCONDLE
		default:
			// BUG: This means that we use the cond.conditionalRegister somewhere in this file,
			// but not covered in switch ^. That shouldn't happen.
			return fmt.Errorf("unsupported condition for br_if: %v", cond.conditionalRegister)
		}
		conditionalBR = c.assembler.CompileJump(brInst)
	} else {
		// If the value is not on the conditional register, we compare the value with the zero register,
		// and then do the conditional BR if the value doesn't equal zero.
		if err := c.compileEnsureOnRegister(cond); err != nil {
			return err
		}
		// Compare the value with zero register. Note that the value is ensured to be i32 by function validation phase,
		// so we use CMPW (32-bit compare) here.
		c.assembler.CompileTwoRegistersToNone(arm64.CMPW, cond.register, arm64.RegRZR)

		conditionalBR = c.assembler.CompileJump(arm64.BCONDNE)

		c.markRegisterUnused(cond.register)
	}

	// Emit the code for branching into else branch.
	// We save and clone the location stack because we might end up modifying it inside of branchInto,
	// and we have to avoid affecting the code generation for Then branch afterwards.
	saved := c.locationStack
	c.setLocationStack(saved.clone())
	if err := c.compileDropRange(o.Else.ToDrop); err != nil {
		return err
	}
	if err := c.compileBranchInto(o.Else.Target); err != nil {
		return err
	}

	// Now ready to emit the code for branching into then branch.
	// Retrieve the original value location stack so that the code below won't be affected by the Else branch ^^.
	c.setLocationStack(saved)
	// We branch into here from the original conditional BR (conditionalBR).
	c.assembler.SetJumpTargetOnNext(conditionalBR)
	if err := c.compileDropRange(o.Then.ToDrop); err != nil {
		return err
	}
	return c.compileBranchInto(o.Then.Target)
}

func (c *arm64Compiler) compileBranchInto(target *wazeroir.BranchTarget) error {
	if target.IsReturnTarget() {
		return c.compileReturnFunction()
	} else {
		labelKey := target.String()
		if c.ir.LabelCallers[labelKey] > 1 {
			// We can only re-use register state if when there's a single call-site.
			// Release existing values on registers to the stack if there's multiple ones to have
			// the consistent value location state at the beginning of label.
			if err := c.compileReleaseAllRegistersToStack(); err != nil {
				return err
			}
		}
		// Set the initial stack of the target label, so we can start compiling the label
		// with the appropriate value locations. Note we clone the stack here as we maybe
		// manipulate the stack before compiler reaches the label.
		targetLabel := c.label(labelKey)
		if targetLabel.initialStack == nil {
			targetLabel.initialStack = c.locationStack.clone()
		}

		br := c.assembler.CompileJump(arm64.B)
		c.assignBranchTarget(labelKey, br)
		return nil
	}
}

// assignBranchTarget assigns the given label's initial instruction to the destination of br.
func (c *arm64Compiler) assignBranchTarget(labelKey string, br asm.Node) {
	target := c.label(labelKey)
	if target.initialInstruction != nil {
		br.AssignJumpTarget(target.initialInstruction)
	} else {
		// This case, the target label hasn't been compiled yet, so we append the callback and assign
		// the target instruction when compileLabel is called for the label.
		target.labelBeginningCallbacks = append(target.labelBeginningCallbacks, func(labelInitialInstruction asm.Node) {
			br.AssignJumpTarget(labelInitialInstruction)
		})
	}
}

// compileBrTable implements compiler.compileBrTable for the arm64 architecture.
func (c *arm64Compiler) compileBrTable(o *wazeroir.OperationBrTable) error {
	// If the operation only consists of the default target, we branch into it and return early.
	if len(o.Targets) == 0 {
		loc := c.locationStack.pop()
		if loc.onRegister() {
			c.markRegisterUnused(loc.register)
		}
		if err := c.compileDropRange(o.Default.ToDrop); err != nil {
			return err
		}
		return c.compileBranchInto(o.Default.Target)
	}

	index := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(index); err != nil {
		return err
	}

	if isZeroRegister(index.register) {
		reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		index.setRegister(reg)
		c.markRegisterUsed(reg)

		// Zero the value on a picked register.
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, reg)
	}

	tmpReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// Load the branch table's length.
	// "tmpReg = len(o.Targets)"
	c.assembler.CompileConstToRegister(arm64.MOVW, int64(len(o.Targets)), tmpReg)
	// Compare the length with offset.
	c.assembler.CompileTwoRegistersToNone(arm64.CMPW, tmpReg, index.register)
	// If the value exceeds the length, we will branch into the default target (corresponding to len(o.Targets) index).
	brDefaultIndex := c.assembler.CompileJump(arm64.BCONDLO)
	c.assembler.CompileRegisterToRegister(arm64.MOVWU, tmpReg, index.register)
	c.assembler.SetJumpTargetOnNext(brDefaultIndex)

	// We prepare the asm.StaticConst which holds the offset of
	// each target's first instruction (incl. default)
	// relative to the beginning of label tables.
	//
	// For example, if we have targets=[L0, L1] and default=L_DEFAULT,
	// we emit the code like this at [Emit the code for each target and default branch] below.
	//
	// L0:
	//  0x123001: XXXX, ...
	//  .....
	// L1:
	//  0x123005: YYY, ...
	//  .....
	// L_DEFAULT:
	//  0x123009: ZZZ, ...
	//
	// then offsetData becomes like [0x0, 0x5, 0x8].
	// By using this offset list, we could jump into the label for the index by
	// "jmp offsetData[index]+0x123001" and "0x123001" can be acquired by "LEA"
	// instruction.
	//
	// Note: We store each offset of 32-bit unsigned integer as 4 consecutive bytes. So more precisely,
	// the above example's offsetData would be [0x0, 0x0, 0x0, 0x0, 0x5, 0x0, 0x0, 0x0, 0x8, 0x0, 0x0, 0x0].
	//
	// Note: this is similar to how GCC implements Switch statements in C.
	offsetData := asm.NewStaticConst(make([]byte, 4*(len(o.Targets)+1)))

	// "tmpReg = &offsetData[0]"
	c.assembler.CompileStaticConstToRegister(arm64.ADR, offsetData, tmpReg)

	// "index.register = tmpReg + (index.register << 2) (== &offsetData[offset])"
	c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD, index.register, 2, tmpReg, index.register)

	// "index.register = *index.register (== offsetData[offset])"
	c.assembler.CompileMemoryToRegister(arm64.MOVW, index.register, 0, index.register)

	// Now we read the address of the beginning of the jump table.
	// In the above example, this corresponds to reading the address of 0x123001.
	c.assembler.CompileReadInstructionAddress(tmpReg, arm64.B)

	// Now we have the address of L0 in tmp register, and the offset to the target label in the index.register.
	// So we could achieve the br_table jump by adding them and jump into the resulting address.
	c.assembler.CompileRegisterToRegister(arm64.ADD, tmpReg, index.register)

	c.assembler.CompileJumpToMemory(arm64.B, index.register)

	// We no longer need the index's register, so mark it unused.
	c.markRegisterUnused(index.register)

	// [Emit the code for each targets and default branch]
	labelInitialInstructions := make([]asm.Node, len(o.Targets)+1)
	saved := c.locationStack
	for i := range labelInitialInstructions {
		// Emit the initial instruction of each target where
		// we use NOP as we don't yet know the next instruction in each label.
		init := c.assembler.CompileStandAlone(arm64.NOP)
		labelInitialInstructions[i] = init

		var locationStack *runtimeValueLocationStack
		var target *wazeroir.BranchTargetDrop
		if i < len(o.Targets) {
			target = o.Targets[i]
			// Clone the location stack so the branch-specific code doesn't
			// affect others.
			locationStack = saved.clone()
		} else {
			target = o.Default
			// If this is the default branch, we use the original one
			// as this is the last code in this block.
			locationStack = saved
		}
		c.setLocationStack(locationStack)
		if err := c.compileDropRange(target.ToDrop); err != nil {
			return err
		}
		if err := c.compileBranchInto(target.Target); err != nil {
			return err
		}
	}

	c.assembler.BuildJumpTable(offsetData, labelInitialInstructions)
	return nil
}

// compileCall implements compiler.compileCall for the arm64 architecture.
func (c *arm64Compiler) compileCall(o *wazeroir.OperationCall) error {
	tp := c.ir.Types[c.ir.Functions[o.FunctionIndex]]
	return c.compileCallImpl(o.FunctionIndex, asm.NilRegister, tp)
}

// compileCallImpl implements compiler.compileCall and compiler.compileCallIndirect for the arm64 architecture.
func (c *arm64Compiler) compileCallImpl(index wasm.Index, targetFunctionAddressRegister asm.Register, functype *wasm.FunctionType) error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	freeRegisters, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 5)
	if !found {
		return fmt.Errorf("BUG: all registers except indexReg should be free at this point")
	}
	c.markRegisterUsed(freeRegisters...)

	// Alias for readability.
	callFrameStackPointerRegister, callFrameStackTopAddressRegister, targetFunctionRegister, oldStackBasePointer,
		tmp := freeRegisters[0], freeRegisters[1], freeRegisters[2], freeRegisters[3], freeRegisters[4]

	// First, we have to check if we need to grow the callFrame stack.
	//
	// "callFrameStackPointerRegister = ce.callFrameStackPointer"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset,
		callFrameStackPointerRegister)
	// "tmp = len(ce.callFrameStack)"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackLenOffset,
		tmp,
	)
	// Compare tmp(len(ce.callFrameStack)) with callFrameStackPointerRegister(ce.callFrameStackPointer).
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, tmp, callFrameStackPointerRegister)
	brIfCallFrameStackOK := c.assembler.CompileJump(arm64.BCONDNE)

	// If these values equal, we need to grow the callFrame stack.
	// For call_indirect, we need to push the value back to the register.
	if !isNilRegister(targetFunctionAddressRegister) {
		// If we need to get the target funcaddr from register (call_indirect case), we must save it before growing the
		// call-frame stack, as the register is not saved across function calls.
		savedOffsetLocation := c.pushRuntimeValueLocationOnRegister(targetFunctionAddressRegister, runtimeValueTypeI64)
		c.compileReleaseRegisterToStack(savedOffsetLocation)
	}

	if err := c.compileCallGoFunction(nativeCallStatusCodeCallBuiltInFunction, builtinFunctionIndexGrowCallFrameStack); err != nil {
		return err
	}

	// For call_indirect, we need to push the value back to the register.
	if !isNilRegister(targetFunctionAddressRegister) {
		// Since this is right after callGoFunction, we have to initialize the stack base pointer
		// to properly load the value on memory stack.
		c.compileReservedStackBasePointerRegisterInitialization()

		savedOffsetLocation := c.locationStack.pop()
		savedOffsetLocation.setRegister(targetFunctionAddressRegister)
		c.compileLoadValueOnStackToRegister(savedOffsetLocation)
	}

	// On the function return, we again have to set ce.callFrameStackPointer into callFrameStackPointerRegister.
	// "callFrameStackPointerRegister = ce.callFrameStackPointer"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset,
		callFrameStackPointerRegister)

	// Now that we ensured callFrameStack length is enough.
	c.assembler.SetJumpTargetOnNext(brIfCallFrameStackOK)
	c.compileCalcCallFrameStackTopAddress(callFrameStackPointerRegister, callFrameStackTopAddressRegister)

	// At this point, we have:
	//
	//    [..., ra.current, rb.current, rc.current, _, ra.next, rb.next, rc.next, ...]  <- call frame stack's data region (somewhere in the memory)
	//                                               ^
	//                              callFrameStackTopAddressRegister
	//                (absolute address of &callFrame[ce.callFrameStackPointer]])
	//
	// where:
	//      ra.* = callFrame.returnAddress
	//      rb.* = callFrame.returnStackBasePointer
	//      rc.* = callFrame.code
	//      _  = callFrame's padding (see comment on callFrame._ field.)
	//
	// In the following comment, we use the notations in the above example.
	//
	// What we have to do in the following is that
	//   1) Set rb.current so that we can return back to this function properly.
	//   2) Set ce.valueStackContext.stackBasePointer for the next function.
	//   3) Set rc.next to specify which function is executed on the current call frame (needs to make Go function calls).
	//   4) Set ra.current so that we can return back to this function properly.

	// 1) Set rb.current so that we can return back to this function properly.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineValueStackContextStackBasePointerOffset,
		oldStackBasePointer)
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		oldStackBasePointer,
		// "rb.current" is BELOW the top address. See the above example for detail.
		callFrameStackTopAddressRegister, -(callFrameDataSize - callFrameReturnStackBasePointerOffset))

	// 2) Set ce.valueStackContext.stackBasePointer for the next function.
	//
	// At this point, oldStackBasePointer holds the old stack base pointer. We could get the new frame's
	// stack base pointer by "old stack base pointer + old stack pointer - # of function params"
	// See the comments in ce.pushCallFrame which does exactly the same calculation in Go.
	if offset := int64(c.locationStack.sp) - int64(functype.ParamNumInUint64); offset > 0 {
		c.assembler.CompileConstToRegister(arm64.ADD, offset, oldStackBasePointer)
		c.assembler.CompileRegisterToMemory(arm64.MOVD,
			oldStackBasePointer,
			arm64ReservedRegisterForCallEngine, callEngineValueStackContextStackBasePointerOffset)
	}

	// 3) Set rc.next to specify which function is executed on the current call frame.
	//
	// First, we read the address of the first item of ce.functions slice (= &ce.functions[0])
	// into tmp.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextFunctionsElement0AddressOffset,
		tmp)

	// Next, read the index of the target function (= &ce.functions[offset])
	// into codeIndexRegister.
	if isNilRegister(targetFunctionAddressRegister) {
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			tmp, int64(index)*8, // * 8 because the size of *function equals 8 bytes.
			targetFunctionRegister)
	} else {
		targetFunctionRegister = targetFunctionAddressRegister
	}

	// Finally, we are ready to write the address of the target function's *function into the new call-frame.
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		targetFunctionRegister,
		callFrameStackTopAddressRegister, callFrameFunctionOffset)

	// 4) Set ra.current so that we can return back to this function properly.
	//
	// First, Get the return address into the tmp.
	c.assembler.CompileReadInstructionAddress(tmp, arm64.B)
	// Then write the address into the call-frame.
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		tmp,
		// "ra.current" is BELOW the top address. See the above example for detail.
		callFrameStackTopAddressRegister, -(callFrameDataSize - callFrameReturnAddressOffset),
	)

	// Everything is done to make function call now: increment the call-frame stack pointer.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset,
		tmp)
	c.assembler.CompileConstToRegister(arm64.ADD, 1, tmp)
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		tmp,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset)

	if targetFunctionRegister == arm64CallingConventionModuleInstanceAddressRegister {
		// This case we must move the value on targetFunctionAddressRegister to another register, otherwise
		// the address (jump target below) will be modified and result in segfault.
		// See #526.
		c.assembler.CompileRegisterToRegister(arm64.MOVD, targetFunctionRegister, tmp)
		targetFunctionRegister = tmp
	}

	// Also, we have to put the code's moduleInstance address into arm64CallingConventionModuleInstanceAddressRegister.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		targetFunctionRegister, functionModuleInstanceAddressOffset,
		arm64CallingConventionModuleInstanceAddressRegister,
	)

	// Then, br into the target function's initial address.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		targetFunctionRegister, functionCodeInitialAddressOffset,
		tmp)

	c.assembler.CompileJumpToMemory(arm64.B, tmp)

	// All the registers used are temporary so we mark them unused.
	c.markRegisterUnused(freeRegisters...)

	// We consumed the function parameters from the stack after call.
	for i := 0; i < functype.ParamNumInUint64; i++ {
		c.locationStack.pop()
	}

	// Also, the function results were pushed by the call.
	for _, t := range functype.Results {
		loc := c.locationStack.pushRuntimeValueLocationOnStack()
		switch t {
		case wasm.ValueTypeI32:
			loc.valueType = runtimeValueTypeI32
		case wasm.ValueTypeI64, wasm.ValueTypeFuncref, wasm.ValueTypeExternref:
			loc.valueType = runtimeValueTypeI64
		case wasm.ValueTypeF32:
			loc.valueType = runtimeValueTypeF32
		case wasm.ValueTypeF64:
			loc.valueType = runtimeValueTypeF64
		case wasm.ValueTypeV128:
			loc.valueType = runtimeValueTypeV128Lo
			hi := c.locationStack.pushRuntimeValueLocationOnStack()
			hi.valueType = runtimeValueTypeV128Hi
		}
	}

	if err := c.compileModuleContextInitialization(); err != nil {
		return err
	}

	// On the function return, we initialize the state for this function.
	c.compileReservedStackBasePointerRegisterInitialization()

	c.compileReservedMemoryRegisterInitialization()
	return nil
}

// compileCalcCallFrameStackTopAddress adds instructions to set the absolute address of
// ce.callFrameStack[callFrameStackPointerRegister] into destinationRegister.
func (c *arm64Compiler) compileCalcCallFrameStackTopAddress(callFrameStackPointerRegister, destinationRegister asm.Register) {
	// "destinationRegister = &ce.callFrameStack[0]"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackElement0AddressOffset,
		destinationRegister)
	// "destinationRegister += callFrameStackPointerRegister << $callFrameDataSizeMostSignificantSetBit"
	c.assembler.CompileLeftShiftedRegisterToRegister(
		arm64.ADD,
		callFrameStackPointerRegister, callFrameDataSizeMostSignificantSetBit,
		destinationRegister,
		destinationRegister,
	)
}

// compileCallIndirect implements compiler.compileCallIndirect for the arm64 architecture.
func (c *arm64Compiler) compileCallIndirect(o *wazeroir.OperationCallIndirect) error {
	offset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(offset); err != nil {
		return err
	}

	if isZeroRegister(offset.register) {
		reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		offset.setRegister(reg)
		c.markRegisterUsed(reg)

		// Zero the value on a picked register.
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, reg)
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.markRegisterUsed(tmp)

	tmp2, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.markRegisterUsed(tmp2)

	// First, we need to check if the offset doesn't exceed the length of table.
	// "tmp = &Tables[0]"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		tmp,
	)
	// tmp = [tmp + TableIndex*8] = [&Tables[0] + TableIndex*sizeOf(*tableInstance)] = Tables[tableIndex]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		tmp, int64(o.TableIndex)*8,
		tmp,
	)
	// tmp2 = [tmp + tableInstanceTableLenOffset] = len(Tables[tableIndex])
	c.assembler.CompileMemoryToRegister(arm64.MOVD, tmp, tableInstanceTableLenOffset, tmp2)

	// "cmp tmp2, offset"
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, tmp2, offset.register)

	// If it exceeds len(table), we exit the execution.
	brIfOffsetOK := c.assembler.CompileJump(arm64.BCONDLO)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)

	// Otherwise, we proceed to do function type check.
	c.assembler.SetJumpTargetOnNext(brIfOffsetOK)

	// We need to obtains the absolute address of table element.
	// "tmp = &Tables[tableIndex].table[0]"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		tmp, tableInstanceTableOffset,
		tmp,
	)
	// "offset = tmp + (offset << pointerSizeLog2) (== &table[offset])"
	// Here we left shifting by 3 in order to get the offset in bytes,
	// and the table element type is uintptr which is 8 bytes.
	c.assembler.CompileLeftShiftedRegisterToRegister(
		arm64.ADD,
		offset.register, pointerSizeLog2,
		tmp,
		offset.register,
	)

	// "offset = (*offset) (== table[offset])"
	c.assembler.CompileMemoryToRegister(arm64.MOVD, offset.register, 0, offset.register)

	// Check if the value of table[offset] equals zero, meaning that the target element is uninitialized.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, offset.register)
	brIfInitialized := c.assembler.CompileJump(arm64.BCONDNE)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)

	c.assembler.SetJumpTargetOnNext(brIfInitialized)
	// Next we check the type matches, i.e. table[offset].source.TypeID == targetFunctionType.
	// "tmp = table[offset].source ( == *FunctionInstance type)"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		offset.register, functionSourceOffset,
		tmp,
	)
	// "tmp = [tmp + functionInstanceTypeIDOffset] (== table[offset].source.TypeID)"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVW, tmp, functionInstanceTypeIDOffset,
		tmp,
	)
	// "tmp2 = ModuleInstance.TypeIDs[index]"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextTypeIDsElement0AddressOffset,
		tmp2)
	c.assembler.CompileMemoryToRegister(arm64.MOVWU, tmp2, int64(o.TypeIndex)*4, tmp2)

	// Compare these two values, and if they equal, we are ready to make function call.
	c.assembler.CompileTwoRegistersToNone(arm64.CMPW, tmp, tmp2)
	brIfTypeMatched := c.assembler.CompileJump(arm64.BCONDEQ)
	c.compileExitFromNativeCode(nativeCallStatusCodeTypeMismatchOnIndirectCall)

	c.assembler.SetJumpTargetOnNext(brIfTypeMatched)

	targetFunctionType := c.ir.Types[o.TypeIndex]
	if err := c.compileCallImpl(0, offset.register, targetFunctionType); err != nil {
		return err
	}

	// The offset register should be marked as un-used as we consumed in the function call.
	c.markRegisterUnused(offset.register, tmp, tmp2)
	return nil
}

// compileDrop implements compiler.compileDrop for the arm64 architecture.
func (c *arm64Compiler) compileDrop(o *wazeroir.OperationDrop) error {
	return c.compileDropRange(o.Depth)
}

// compileDropRange is the implementation of compileDrop. See compiler.compileDrop.
func (c *arm64Compiler) compileDropRange(r *wazeroir.InclusiveRange) error {
	if r == nil {
		return nil
	} else if r.Start == 0 {
		// When the drop starts from the top of the stack, mark all registers unused.
		for i := 0; i <= r.End; i++ {
			if loc := c.locationStack.pop(); loc.onRegister() {
				c.markRegisterUnused(loc.register)
			}
		}
		return nil
	}

	// Below, we might end up moving a non-top value first which
	// might result in changing the flag value.
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	// Save the live values because we pop and release values in drop range below.
	liveValues := c.locationStack.stack[c.locationStack.sp-uint64(r.Start) : c.locationStack.sp]
	c.locationStack.sp -= uint64(r.Start)

	// Note: drop target range is inclusive.
	dropNum := r.End - r.Start + 1

	// Then mark all registers used by drop targets unused.
	for i := 0; i < dropNum; i++ {
		if loc := c.locationStack.pop(); loc.onRegister() {
			c.markRegisterUnused(loc.register)
		}
	}

	for _, live := range liveValues {
		// If the value is on a memory, we have to move it to a register,
		// otherwise the memory location is overridden by other values
		// after this drop instruction.
		if err := c.compileEnsureOnRegister(live); err != nil {
			return err
		}
		// Update the runtime memory stack location by pushing onto the location stack.
		c.locationStack.push(live)
	}
	return nil
}

// compileSelect implements compiler.compileSelect for the arm64 architecture.
func (c *arm64Compiler) compileSelect() error {
	cv, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	c.markRegisterUsed(cv.register)

	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) && isZeroRegister(x2.register) {
		// If both values are zero, the result is always zero.
		c.pushRuntimeValueLocationOnRegister(arm64.RegRZR, x1.valueType)
		c.markRegisterUnused(cv.register)
		return nil
	}

	// In the following, we emit the code so that x1's register contains the chosen value
	// no matter which of original x1 or x2 is selected.
	//
	// If x1 is currently on zero register, we cannot place the result because
	// "MOV arm64.RegRZR x2.register" results in arm64.RegRZR regardless of the value.
	// So we explicitly assign a general purpose register to x1 here.
	if isZeroRegister(x1.register) {
		// Mark x2 and cv's registers are used so they won't be chosen.
		c.markRegisterUsed(x2.register)
		// Pick the non-zero register for x1.
		x1Reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		x1.setRegister(x1Reg)
		// And zero our the picked register.
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, x1Reg)
	}

	// At this point, x1 is non-zero register, and x2 is either general purpose or zero register.

	c.assembler.CompileTwoRegistersToNone(arm64.CMPW, arm64.RegRZR, cv.register)
	brIfNotZero := c.assembler.CompileJump(arm64.BCONDNE)

	// If cv == 0, we move the value of x2 to the x1.register.

	switch x1.valueType {
	case runtimeValueTypeI32:
		// TODO: use 32-bit mov
		c.assembler.CompileRegisterToRegister(arm64.MOVD, x2.register, x1.register)
	case runtimeValueTypeI64:
		c.assembler.CompileRegisterToRegister(arm64.MOVD, x2.register, x1.register)
	case runtimeValueTypeF32:
		// TODO: use 32-bit mov
		c.assembler.CompileRegisterToRegister(arm64.FMOVD, x2.register, x1.register)
	case runtimeValueTypeF64:
		c.assembler.CompileRegisterToRegister(arm64.FMOVD, x2.register, x1.register)
	default:
		return errors.New("TODO: implement vector type select")
	}

	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)

	// Otherwise, nothing to do for select.
	c.assembler.SetJumpTargetOnNext(brIfNotZero)

	// Only x1.register is reused.
	c.markRegisterUnused(cv.register, x2.register)
	return nil
}

// compilePick implements compiler.compilePick for the arm64 architecture.
func (c *arm64Compiler) compilePick(o *wazeroir.OperationPick) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	pickTarget := c.locationStack.stack[c.locationStack.sp-1-uint64(o.Depth)]
	pickedRegister, err := c.allocateRegister(pickTarget.getRegisterType())
	if err != nil {
		return err
	}

	if pickTarget.onRegister() { // Copy the value to the pickedRegister.
		switch pickTarget.valueType {
		case runtimeValueTypeI32:
			c.assembler.CompileRegisterToRegister(arm64.MOVD, pickTarget.register, pickedRegister)
		case runtimeValueTypeI64:
			c.assembler.CompileRegisterToRegister(arm64.MOVD, pickTarget.register, pickedRegister)
		case runtimeValueTypeF32:
			// TODO: use 32-bit mov.
			c.assembler.CompileRegisterToRegister(arm64.FMOVD, pickTarget.register, pickedRegister)
		case runtimeValueTypeF64:
			c.assembler.CompileRegisterToRegister(arm64.FMOVD, pickTarget.register, pickedRegister)
		case runtimeValueTypeV128Lo:
			c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VORR,
				pickTarget.register, pickTarget.register, pickedRegister, arm64.VectorArrangement16B)
		case runtimeValueTypeV128Hi:
			panic("BUG") // since pick target must point to the lower 64-bits of vectors.
		}
	} else if pickTarget.onStack() {
		// Temporarily assign a register to the pick target, and then load the value.
		pickTarget.setRegister(pickedRegister)
		c.compileLoadValueOnStackToRegister(pickTarget)

		// After the load, we revert the register assignment to the pick target.
		pickTarget.setRegister(asm.NilRegister)
		if o.IsTargetVector {
			hi := c.locationStack.stack[pickTarget.stackPointer+1]
			hi.setRegister(asm.NilRegister)
		}
	}

	// Now we have the value of the target on the pickedRegister,
	// so push the location.
	c.pushRuntimeValueLocationOnRegister(pickedRegister, pickTarget.valueType)
	if o.IsTargetVector {
		c.pushRuntimeValueLocationOnRegister(pickedRegister, runtimeValueTypeV128Hi)
	}
	return nil
}

// compileAdd implements compiler.compileAdd for the arm64 architecture.
func (c *arm64Compiler) compileAdd(o *wazeroir.OperationAdd) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// Addition can be nop if one of operands is zero.
	if isZeroRegister(x1.register) {
		c.pushRuntimeValueLocationOnRegister(x2.register, x1.valueType)
		return nil
	} else if isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		inst = arm64.ADDW
	case wazeroir.UnsignedTypeI64:
		inst = arm64.ADD
	case wazeroir.UnsignedTypeF32:
		inst = arm64.FADDS
	case wazeroir.UnsignedTypeF64:
		inst = arm64.FADDD
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)
	// The result is placed on a register for x1, so record it.
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileSub implements compiler.compileSub for the arm64 architecture.
func (c *arm64Compiler) compileSub(o *wazeroir.OperationSub) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// If both of registers are zeros, this can be nop and push the zero register.
	if isZeroRegister(x1.register) && isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(arm64.RegRZR, x1.valueType)
		return nil
	}

	// At this point, at least one of x1 or x2 registers is non zero.
	// Choose the non-zero register as destination.
	var destinationReg = x1.register
	if isZeroRegister(x1.register) {
		destinationReg = x2.register
	}

	var inst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		inst = arm64.SUBW
		vt = runtimeValueTypeI32
	case wazeroir.UnsignedTypeI64:
		inst = arm64.SUB
		vt = runtimeValueTypeI64
	case wazeroir.UnsignedTypeF32:
		inst = arm64.FSUBS
		vt = runtimeValueTypeF32
	case wazeroir.UnsignedTypeF64:
		inst = arm64.FSUBD
		vt = runtimeValueTypeF64
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, destinationReg)
	c.pushRuntimeValueLocationOnRegister(destinationReg, vt)
	return nil
}

// compileMul implements compiler.compileMul for the arm64 architecture.
func (c *arm64Compiler) compileMul(o *wazeroir.OperationMul) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// Multiplication can be done by putting a zero register if one of operands is zero.
	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(arm64.RegRZR, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		inst = arm64.MULW
		vt = runtimeValueTypeI32
	case wazeroir.UnsignedTypeI64:
		inst = arm64.MUL
		vt = runtimeValueTypeI64
	case wazeroir.UnsignedTypeF32:
		inst = arm64.FMULS
		vt = runtimeValueTypeF32
	case wazeroir.UnsignedTypeF64:
		inst = arm64.FMULD
		vt = runtimeValueTypeF64
	}

	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)
	// The result is placed on a register for x1, so record it.
	c.pushRuntimeValueLocationOnRegister(x1.register, vt)
	return nil
}

// compileClz implements compiler.compileClz for the arm64 architecture.
func (c *arm64Compiler) compileClz(o *wazeroir.OperationClz) error {
	v, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	if isZeroRegister(v.register) {
		// If the target is zero register, the result is always 32 (or 64 for 64-bits),
		// so we allocate a register and put the const on it.
		reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		var vt runtimeValueType
		if o.Type == wazeroir.UnsignedInt32 {
			vt = runtimeValueTypeI32
			c.assembler.CompileConstToRegister(arm64.MOVW, 32, reg)
		} else {
			vt = runtimeValueTypeI64
			c.assembler.CompileConstToRegister(arm64.MOVD, 64, reg)
		}
		c.pushRuntimeValueLocationOnRegister(reg, vt)
		return nil
	}

	reg := v.register
	var vt runtimeValueType
	if o.Type == wazeroir.UnsignedInt32 {
		vt = runtimeValueTypeI32
		c.assembler.CompileRegisterToRegister(arm64.CLZW, reg, reg)
	} else {
		vt = runtimeValueTypeI64
		c.assembler.CompileRegisterToRegister(arm64.CLZ, reg, reg)
	}
	c.pushRuntimeValueLocationOnRegister(reg, vt)
	return nil
}

// compileCtz implements compiler.compileCtz for the arm64 architecture.
func (c *arm64Compiler) compileCtz(o *wazeroir.OperationCtz) error {
	v, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	reg := v.register
	if isZeroRegister(reg) {
		// If the target is zero register, the result is always 32 (or 64 for 64-bits),
		// so we allocate a register and put the const on it.
		reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		var vt runtimeValueType
		if o.Type == wazeroir.UnsignedInt32 {
			vt = runtimeValueTypeI32
			c.assembler.CompileConstToRegister(arm64.MOVW, 32, reg)
		} else {
			vt = runtimeValueTypeI64
			c.assembler.CompileConstToRegister(arm64.MOVD, 64, reg)
		}
		c.pushRuntimeValueLocationOnRegister(reg, vt)
		return nil
	}

	// Since arm64 doesn't have an instruction directly counting trailing zeros,
	// we reverse the bits first, and then do CLZ, which is exactly the same as
	// gcc implements __builtin_ctz for arm64.
	var vt runtimeValueType
	if o.Type == wazeroir.UnsignedInt32 {
		vt = runtimeValueTypeI32
		c.assembler.CompileRegisterToRegister(arm64.RBITW, reg, reg)
		c.assembler.CompileRegisterToRegister(arm64.CLZW, reg, reg)
	} else {
		vt = runtimeValueTypeI64
		c.assembler.CompileRegisterToRegister(arm64.RBIT, reg, reg)
		c.assembler.CompileRegisterToRegister(arm64.CLZ, reg, reg)
	}
	c.pushRuntimeValueLocationOnRegister(reg, vt)
	return nil
}

// compilePopcnt implements compiler.compilePopcnt for the arm64 architecture.
func (c *arm64Compiler) compilePopcnt(o *wazeroir.OperationPopcnt) error {
	v, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	reg := v.register
	if isZeroRegister(reg) {
		c.pushRuntimeValueLocationOnRegister(reg, v.valueType)
		return nil
	}

	freg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// arm64 doesn't have an instruction for population count on scalar register,
	// so we use the vector one (VCNT).
	// This exactly what the official Go implements bits.OneCount.
	// For example, "func () int { return bits.OneCount(10) }" is compiled as
	//
	//    MOVD    $10, R0 ;; Load 10.
	//    FMOVD   R0, F0
	//    VCNT    V0.B8, V0.B8
	//    UADDLV  V0.B8, V0
	//
	var movInst asm.Instruction
	if o.Type == wazeroir.UnsignedInt32 {
		movInst = arm64.FMOVS
	} else {
		movInst = arm64.FMOVD
	}
	c.assembler.CompileRegisterToRegister(movInst, reg, freg)
	c.assembler.CompileVectorRegisterToVectorRegister(arm64.VCNT, freg, freg,
		arm64.VectorArrangement16B, arm64.VectorIndexNone, arm64.VectorIndexNone)
	c.assembler.CompileVectorRegisterToVectorRegister(arm64.UADDLV, freg, freg, arm64.VectorArrangement8B,
		arm64.VectorIndexNone, arm64.VectorIndexNone)

	c.assembler.CompileRegisterToRegister(movInst, freg, reg)

	c.pushRuntimeValueLocationOnRegister(reg, v.valueType)
	return nil
}

// compileDiv implements compiler.compileDiv for the arm64 architecture.
func (c *arm64Compiler) compileDiv(o *wazeroir.OperationDiv) error {
	dividend, divisor, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// If the divisor is on the zero register, exit from the function deterministically.
	if isZeroRegister(divisor.register) {
		// Push any value so that the subsequent instruction can have a consistent location stack state.
		c.locationStack.pushRuntimeValueLocationOnStack()
		c.compileExitFromNativeCode(nativeCallStatusIntegerDivisionByZero)
		return nil
	}

	var inst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		inst = arm64.UDIVW
		if err := c.compileIntegerDivPrecheck(true, false, dividend.register, divisor.register); err != nil {
			return err
		}
		vt = runtimeValueTypeI32
	case wazeroir.SignedTypeUint64:
		if err := c.compileIntegerDivPrecheck(false, false, dividend.register, divisor.register); err != nil {
			return err
		}
		inst = arm64.UDIV
		vt = runtimeValueTypeI64
	case wazeroir.SignedTypeInt32:
		if err := c.compileIntegerDivPrecheck(true, true, dividend.register, divisor.register); err != nil {
			return err
		}
		inst = arm64.SDIVW
		vt = runtimeValueTypeI32
	case wazeroir.SignedTypeInt64:
		if err := c.compileIntegerDivPrecheck(false, true, dividend.register, divisor.register); err != nil {
			return err
		}
		inst = arm64.SDIV
		vt = runtimeValueTypeI64
	case wazeroir.SignedTypeFloat32:
		inst = arm64.FDIVS
		vt = runtimeValueTypeF32
	case wazeroir.SignedTypeFloat64:
		inst = arm64.FDIVD
		vt = runtimeValueTypeF64
	}

	c.assembler.CompileRegisterToRegister(inst, divisor.register, dividend.register)

	c.pushRuntimeValueLocationOnRegister(dividend.register, vt)
	return nil
}

// compileIntegerDivPrecheck adds instructions to check if the divisor and dividend are sound for division operation.
// First, this adds instructions to check if the divisor equals zero, and if so, exits the function.
// Plus, for signed divisions, check if the result might result in overflow or not.
func (c *arm64Compiler) compileIntegerDivPrecheck(is32Bit, isSigned bool, dividend, divisor asm.Register) error {
	// We check the divisor value equals zero.
	var cmpInst, movInst asm.Instruction
	var minValueOffsetInVM int64
	if is32Bit {
		cmpInst = arm64.CMPW
		movInst = arm64.MOVW
		minValueOffsetInVM = arm64CallEngineArchContextMinimum32BitSignedIntOffset
	} else {
		cmpInst = arm64.CMP
		movInst = arm64.MOVD
		minValueOffsetInVM = arm64CallEngineArchContextMinimum64BitSignedIntOffset
	}
	c.assembler.CompileTwoRegistersToNone(cmpInst, arm64.RegRZR, divisor)

	// If it is zero, we exit with nativeCallStatusIntegerDivisionByZero.
	brIfDivisorNonZero := c.assembler.CompileJump(arm64.BCONDNE)
	c.compileExitFromNativeCode(nativeCallStatusIntegerDivisionByZero)

	// Otherwise, we proceed.
	c.assembler.SetJumpTargetOnNext(brIfDivisorNonZero)

	// If the operation is a signed integer div, we have to do an additional check on overflow.
	if isSigned {
		// For signed division, we have to have branches for "math.MinInt{32,64} / -1"
		// case which results in the overflow.

		// First, we compare the divisor with -1.
		c.assembler.CompileConstToRegister(movInst, -1, arm64ReservedRegisterForTemporary)
		c.assembler.CompileTwoRegistersToNone(cmpInst, arm64ReservedRegisterForTemporary, divisor)

		// If they not equal, we skip the following check.
		brIfDivisorNonMinusOne := c.assembler.CompileJump(arm64.BCONDNE)

		// Otherwise, we further check if the dividend equals math.MinInt32 or MinInt64.
		c.assembler.CompileMemoryToRegister(
			movInst,
			arm64ReservedRegisterForCallEngine, minValueOffsetInVM,
			arm64ReservedRegisterForTemporary,
		)
		c.assembler.CompileTwoRegistersToNone(cmpInst, arm64ReservedRegisterForTemporary, dividend)

		// If they not equal, we are safe to execute the division.
		brIfDividendNotMinInt := c.assembler.CompileJump(arm64.BCONDNE)

		// Otherwise, we raise overflow error.
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)

		c.assembler.SetJumpTargetOnNext(brIfDivisorNonMinusOne, brIfDividendNotMinInt)
	}
	return nil
}

// compileRem implements compiler.compileRem for the arm64 architecture.
func (c *arm64Compiler) compileRem(o *wazeroir.OperationRem) error {
	dividend, divisor, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	dividendReg := dividend.register
	divisorReg := divisor.register

	// If the divisor is on the zero register, exit from the function deterministically.
	if isZeroRegister(divisor.register) {
		// Push any value so that the subsequent instruction can have a consistent location stack state.
		c.locationStack.pushRuntimeValueLocationOnStack()
		c.compileExitFromNativeCode(nativeCallStatusIntegerDivisionByZero)
		return nil
	}

	var divInst, msubInst, cmpInst asm.Instruction
	switch o.Type {
	case wazeroir.SignedUint32:
		divInst = arm64.UDIVW
		msubInst = arm64.MSUBW
		cmpInst = arm64.CMPW
	case wazeroir.SignedUint64:
		divInst = arm64.UDIV
		msubInst = arm64.MSUB
		cmpInst = arm64.CMP
	case wazeroir.SignedInt32:
		divInst = arm64.SDIVW
		msubInst = arm64.MSUBW
		cmpInst = arm64.CMPW
	case wazeroir.SignedInt64:
		divInst = arm64.SDIV
		msubInst = arm64.MSUB
		cmpInst = arm64.CMP
	}

	// We check the divisor value equals zero.
	c.assembler.CompileTwoRegistersToNone(cmpInst, arm64.RegRZR, divisorReg)

	// If it is zero, we exit with nativeCallStatusIntegerDivisionByZero.
	brIfDivisorNonZero := c.assembler.CompileJump(arm64.BCONDNE)
	c.compileExitFromNativeCode(nativeCallStatusIntegerDivisionByZero)

	// Otherwise, we proceed.
	c.assembler.SetJumpTargetOnNext(brIfDivisorNonZero)

	// Temporarily mark them used to allocate a result register while keeping these values.
	c.markRegisterUsed(dividend.register, divisor.register)

	resultReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// arm64 doesn't have an instruction for rem, we use calculate it by two instructions: UDIV (SDIV for signed) and MSUB.
	// This exactly the same code that Clang emits.
	// [input: x0=dividend, x1=divisor]
	// >> UDIV x2, x0, x1
	// >> MSUB x3, x2, x1, x0
	// [result: x2=quotient, x3=remainder]
	//
	c.assembler.CompileTwoRegistersToRegister(divInst, divisorReg, dividendReg, resultReg)
	// ResultReg = dividendReg - (divisorReg * resultReg)
	c.assembler.CompileThreeRegistersToRegister(msubInst, divisorReg, dividendReg, resultReg, resultReg)

	c.markRegisterUnused(dividend.register, divisor.register)
	c.pushRuntimeValueLocationOnRegister(resultReg, dividend.valueType)
	return nil
}

// compileAnd implements compiler.compileAnd for the arm64 architecture.
func (c *arm64Compiler) compileAnd(o *wazeroir.OperationAnd) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// If either of the registers x1 or x2 is zero,
	// the result will always be zero.
	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(arm64.RegRZR, x1.valueType)
		return nil
	}

	// At this point, at least one of x1 or x2 registers is non zero.
	// Choose the non-zero register as destination.
	var destinationReg = x1.register
	if isZeroRegister(x1.register) {
		destinationReg = x2.register
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.ANDW
	case wazeroir.UnsignedInt64:
		inst = arm64.AND
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, destinationReg)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileOr implements compiler.compileOr for the arm64 architecture.
func (c *arm64Compiler) compileOr(o *wazeroir.OperationOr) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) {
		c.pushRuntimeValueLocationOnRegister(x2.register, x2.valueType)
		return nil
	}
	if isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.ORRW
	case wazeroir.UnsignedInt64:
		inst = arm64.ORR
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileXor implements compiler.compileXor for the arm64 architecture.
func (c *arm64Compiler) compileXor(o *wazeroir.OperationXor) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	// At this point, at least one of x1 or x2 registers is non zero.
	// Choose the non-zero register as destination.
	var destinationReg = x1.register
	if isZeroRegister(x1.register) {
		destinationReg = x2.register
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.EORW
	case wazeroir.UnsignedInt64:
		inst = arm64.EOR
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, destinationReg)
	c.pushRuntimeValueLocationOnRegister(destinationReg, x1.valueType)
	return nil
}

// compileShl implements compiler.compileShl for the arm64 architecture.
func (c *arm64Compiler) compileShl(o *wazeroir.OperationShl) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.LSLW
	case wazeroir.UnsignedInt64:
		inst = arm64.LSL
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileShr implements compiler.compileShr for the arm64 architecture.
func (c *arm64Compiler) compileShr(o *wazeroir.OperationShr) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.SignedInt32:
		inst = arm64.ASRW
	case wazeroir.SignedInt64:
		inst = arm64.ASR
	case wazeroir.SignedUint32:
		inst = arm64.LSRW
	case wazeroir.SignedUint64:
		inst = arm64.LSR
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileRotl implements compiler.compileRotl for the arm64 architecture.
func (c *arm64Compiler) compileRotl(o *wazeroir.OperationRotl) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst, neginst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.RORW
		neginst = arm64.NEGW
	case wazeroir.UnsignedInt64:
		inst = arm64.ROR
		neginst = arm64.NEG
	}

	// Arm64 doesn't have rotate left instruction.
	// The shift amount needs to be converted to a negative number, similar to assembly output of bits.RotateLeft.
	c.assembler.CompileRegisterToRegister(neginst, x2.register, x2.register)

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileRotr implements compiler.compileRotr for the arm64 architecture.
func (c *arm64Compiler) compileRotr(o *wazeroir.OperationRotr) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	if isZeroRegister(x1.register) || isZeroRegister(x2.register) {
		c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
		return nil
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.RORW
	case wazeroir.UnsignedInt64:
		inst = arm64.ROR
	}

	c.assembler.CompileTwoRegistersToRegister(inst, x2.register, x1.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileAbs implements compiler.compileAbs for the arm64 architecture.
func (c *arm64Compiler) compileAbs(o *wazeroir.OperationAbs) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FABSS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FABSD, runtimeValueTypeF64)
	}
}

// compileNeg implements compiler.compileNeg for the arm64 architecture.
func (c *arm64Compiler) compileNeg(o *wazeroir.OperationNeg) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FNEGS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FNEGD, runtimeValueTypeF64)
	}
}

// compileCeil implements compiler.compileCeil for the arm64 architecture.
func (c *arm64Compiler) compileCeil(o *wazeroir.OperationCeil) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FRINTPS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FRINTPD, runtimeValueTypeF64)
	}
}

// compileFloor implements compiler.compileFloor for the arm64 architecture.
func (c *arm64Compiler) compileFloor(o *wazeroir.OperationFloor) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FRINTMS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FRINTMD, runtimeValueTypeF64)
	}
}

// compileTrunc implements compiler.compileTrunc for the arm64 architecture.
func (c *arm64Compiler) compileTrunc(o *wazeroir.OperationTrunc) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FRINTZS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FRINTZD, runtimeValueTypeF64)
	}
}

// compileNearest implements compiler.compileNearest for the arm64 architecture.
func (c *arm64Compiler) compileNearest(o *wazeroir.OperationNearest) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FRINTNS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FRINTND, runtimeValueTypeF64)
	}
}

// compileSqrt implements compiler.compileSqrt for the arm64 architecture.
func (c *arm64Compiler) compileSqrt(o *wazeroir.OperationSqrt) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleUnop(arm64.FSQRTS, runtimeValueTypeF32)
	} else {
		return c.compileSimpleUnop(arm64.FSQRTD, runtimeValueTypeF64)
	}
}

// compileMin implements compiler.compileMin for the arm64 architecture.
func (c *arm64Compiler) compileMin(o *wazeroir.OperationMin) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleFloatBinop(arm64.FMINS)
	} else {
		return c.compileSimpleFloatBinop(arm64.FMIND)
	}
}

// compileMax implements compiler.compileMax for the arm64 architecture.
func (c *arm64Compiler) compileMax(o *wazeroir.OperationMax) error {
	if o.Type == wazeroir.Float32 {
		return c.compileSimpleFloatBinop(arm64.FMAXS)
	} else {
		return c.compileSimpleFloatBinop(arm64.FMAXD)
	}
}

func (c *arm64Compiler) compileSimpleFloatBinop(inst asm.Instruction) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}
	c.assembler.CompileRegisterToRegister(inst, x2.register, x1.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileCopysign implements compiler.compileCopysign for the arm64 architecture.
func (c *arm64Compiler) compileCopysign(o *wazeroir.OperationCopysign) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var fmov asm.Instruction
	var minValueOffsetInVM int64
	if o.Type == wazeroir.Float32 {
		fmov = arm64.FMOVS
		minValueOffsetInVM = arm64CallEngineArchContextMinimum32BitSignedIntOffset
	} else {
		fmov = arm64.FMOVD
		minValueOffsetInVM = arm64CallEngineArchContextMinimum64BitSignedIntOffset
	}

	c.markRegisterUsed(x1.register, x2.register)
	freg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	// This is exactly the same code emitted by GCC for "__builtin_copysign":
	//
	//    mov     x0, -9223372036854775808
	//    fmov    d2, x0
	//    vbit    v0.8b, v1.8b, v2.8b
	//
	// "mov freg, -9223372036854775808 (stored at ce.minimum64BitSignedInt)"
	c.assembler.CompileMemoryToRegister(
		fmov,
		arm64ReservedRegisterForCallEngine, minValueOffsetInVM,
		freg,
	)

	// VBIT inserts each bit from the first operand into the destination if the corresponding bit of the second operand is 1,
	// otherwise it leaves the destination bit unchanged.
	// See https://developer.arm.com/documentation/dui0801/g/Advanced-SIMD-Instructions--32-bit-/VBIT
	//
	// For how to specify "V0.B8" (SIMD register arrangement), see
	// * https://github.com/twitchyliquid64/golang-asm/blob/v0.15.1/obj/link.go#L172-L177
	// * https://github.com/golang/go/blob/739328c694d5e608faa66d17192f0a59f6e01d04/src/cmd/compile/internal/arm64/ssa.go#L972
	//
	// "vbit vreg.8b, x2vreg.8b, x1vreg.8b" == "inserting 64th bit of x2 into x1".
	c.assembler.CompileTwoVectorRegistersToVectorRegister(arm64.VBIT,
		freg, x2.register, x1.register, arm64.VectorArrangement16B)

	c.markRegisterUnused(x2.register)
	c.pushRuntimeValueLocationOnRegister(x1.register, x1.valueType)
	return nil
}

// compileI32WrapFromI64 implements compiler.compileI32WrapFromI64 for the arm64 architecture.
func (c *arm64Compiler) compileI32WrapFromI64() error {
	return c.compileSimpleUnop(arm64.MOVWU, runtimeValueTypeI64)
}

// compileITruncFromF implements compiler.compileITruncFromF for the arm64 architecture.
func (c *arm64Compiler) compileITruncFromF(o *wazeroir.OperationITruncFromF) error {
	// Clear the floating point status register (FPSR).
	c.assembler.CompileRegisterToRegister(arm64.MSR, arm64.RegRZR, arm64.RegFPSR)

	var vt runtimeValueType
	var convinst asm.Instruction
	var is32bitFloat = o.InputType == wazeroir.Float32
	if is32bitFloat && o.OutputType == wazeroir.SignedInt32 {
		convinst = arm64.FCVTZSSW
		vt = runtimeValueTypeI32
	} else if is32bitFloat && o.OutputType == wazeroir.SignedInt64 {
		convinst = arm64.FCVTZSS
		vt = runtimeValueTypeI64
	} else if !is32bitFloat && o.OutputType == wazeroir.SignedInt32 {
		convinst = arm64.FCVTZSDW
		vt = runtimeValueTypeI32
	} else if !is32bitFloat && o.OutputType == wazeroir.SignedInt64 {
		convinst = arm64.FCVTZSD
		vt = runtimeValueTypeI64
	} else if is32bitFloat && o.OutputType == wazeroir.SignedUint32 {
		convinst = arm64.FCVTZUSW
		vt = runtimeValueTypeI32
	} else if is32bitFloat && o.OutputType == wazeroir.SignedUint64 {
		convinst = arm64.FCVTZUS
		vt = runtimeValueTypeI64
	} else if !is32bitFloat && o.OutputType == wazeroir.SignedUint32 {
		convinst = arm64.FCVTZUDW
		vt = runtimeValueTypeI32
	} else if !is32bitFloat && o.OutputType == wazeroir.SignedUint64 {
		convinst = arm64.FCVTZUD
		vt = runtimeValueTypeI64
	}

	source, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	destinationReg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(convinst, source.register, destinationReg)
	c.pushRuntimeValueLocationOnRegister(destinationReg, vt)

	if !o.NonTrapping {
		// Obtain the floating point status register value into the general purpose register,
		// so that we can check if the conversion resulted in undefined behavior.
		c.assembler.CompileRegisterToRegister(arm64.MRS, arm64.RegFPSR, arm64ReservedRegisterForTemporary)
		// Check if the conversion was undefined by comparing the status with 1.
		// See https://developer.arm.com/documentation/ddi0595/2020-12/AArch64-Registers/FPSR--Floating-point-Status-Register
		c.assembler.CompileRegisterAndConstToNone(arm64.CMP, arm64ReservedRegisterForTemporary, 1)

		brOK := c.assembler.CompileJump(arm64.BCONDNE)

		// If so, exit the execution with errors depending on whether or not the source value is NaN.
		var floatcmp asm.Instruction
		if is32bitFloat {
			floatcmp = arm64.FCMPS
		} else {
			floatcmp = arm64.FCMPD
		}
		c.assembler.CompileTwoRegistersToNone(floatcmp, source.register, source.register)
		// VS flag is set if at least one of values for FCMP is NaN.
		// https://developer.arm.com/documentation/dui0801/g/Condition-Codes/Comparison-of-condition-code-meanings-in-integer-and-floating-point-code
		brIfSourceNaN := c.assembler.CompileJump(arm64.BCONDVS)

		// If the source value is not NaN, the operation was overflow.
		c.compileExitFromNativeCode(nativeCallStatusIntegerOverflow)

		// Otherwise, the operation was invalid as this is trying to convert NaN to integer.
		c.assembler.SetJumpTargetOnNext(brIfSourceNaN)
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidFloatToIntConversion)

		// Otherwise, we branch into the next instruction.
		c.assembler.SetJumpTargetOnNext(brOK)
	}
	return nil
}

// compileFConvertFromI implements compiler.compileFConvertFromI for the arm64 architecture.
func (c *arm64Compiler) compileFConvertFromI(o *wazeroir.OperationFConvertFromI) error {
	var convinst asm.Instruction
	if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedInt32 {
		convinst = arm64.SCVTFWS
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedInt64 {
		convinst = arm64.SCVTFS
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedInt32 {
		convinst = arm64.SCVTFWD
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedInt64 {
		convinst = arm64.SCVTFD
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedUint32 {
		convinst = arm64.UCVTFWS
	} else if o.OutputType == wazeroir.Float32 && o.InputType == wazeroir.SignedUint64 {
		convinst = arm64.UCVTFS
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedUint32 {
		convinst = arm64.UCVTFWD
	} else if o.OutputType == wazeroir.Float64 && o.InputType == wazeroir.SignedUint64 {
		convinst = arm64.UCVTFD
	}

	var vt runtimeValueType
	if o.OutputType == wazeroir.Float32 {
		vt = runtimeValueTypeF32
	} else {
		vt = runtimeValueTypeF64
	}
	return c.compileSimpleConversion(convinst, registerTypeVector, vt)
}

// compileF32DemoteFromF64 implements compiler.compileF32DemoteFromF64 for the arm64 architecture.
func (c *arm64Compiler) compileF32DemoteFromF64() error {
	return c.compileSimpleUnop(arm64.FCVTDS, runtimeValueTypeF32)
}

// compileF64PromoteFromF32 implements compiler.compileF64PromoteFromF32 for the arm64 architecture.
func (c *arm64Compiler) compileF64PromoteFromF32() error {
	return c.compileSimpleUnop(arm64.FCVTSD, runtimeValueTypeF64)
}

// compileI32ReinterpretFromF32 implements compiler.compileI32ReinterpretFromF32 for the arm64 architecture.
func (c *arm64Compiler) compileI32ReinterpretFromF32() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeI32
		return nil
	}
	return c.compileSimpleConversion(arm64.FMOVS, registerTypeGeneralPurpose, runtimeValueTypeI32)
}

// compileI64ReinterpretFromF64 implements compiler.compileI64ReinterpretFromF64 for the arm64 architecture.
func (c *arm64Compiler) compileI64ReinterpretFromF64() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeI64
		return nil
	}
	return c.compileSimpleConversion(arm64.FMOVD, registerTypeGeneralPurpose, runtimeValueTypeI64)
}

// compileF32ReinterpretFromI32 implements compiler.compileF32ReinterpretFromI32 for the arm64 architecture.
func (c *arm64Compiler) compileF32ReinterpretFromI32() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeF32
		return nil
	}
	return c.compileSimpleConversion(arm64.FMOVS, registerTypeVector, runtimeValueTypeF32)
}

// compileF64ReinterpretFromI64 implements compiler.compileF64ReinterpretFromI64 for the arm64 architecture.
func (c *arm64Compiler) compileF64ReinterpretFromI64() error {
	if peek := c.locationStack.peek(); peek.onStack() {
		// If the value is on the stack, this is no-op as there is nothing to do for converting type.
		peek.valueType = runtimeValueTypeF64
		return nil
	}
	return c.compileSimpleConversion(arm64.FMOVD, registerTypeVector, runtimeValueTypeF64)
}

func (c *arm64Compiler) compileSimpleConversion(inst asm.Instruction, destinationRegType registerType, resultRuntimeValueType runtimeValueType) error {
	source, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	destinationReg, err := c.allocateRegister(destinationRegType)
	if err != nil {
		return err
	}

	c.assembler.CompileRegisterToRegister(inst, source.register, destinationReg)
	c.pushRuntimeValueLocationOnRegister(destinationReg, resultRuntimeValueType)
	return nil
}

// compileExtend implements compiler.compileExtend for the arm64 architecture.
func (c *arm64Compiler) compileExtend(o *wazeroir.OperationExtend) error {
	if o.Signed {
		return c.compileSimpleUnop(arm64.SXTW, runtimeValueTypeI64)
	} else {
		return c.compileSimpleUnop(arm64.MOVWU, runtimeValueTypeI64)
	}
}

// compileSignExtend32From8 implements compiler.compileSignExtend32From8 for the arm64 architecture.
func (c *arm64Compiler) compileSignExtend32From8() error {
	return c.compileSimpleUnop(arm64.SXTBW, runtimeValueTypeI32)
}

// compileSignExtend32From16 implements compiler.compileSignExtend32From16 for the arm64 architecture.
func (c *arm64Compiler) compileSignExtend32From16() error {
	return c.compileSimpleUnop(arm64.SXTHW, runtimeValueTypeI32)
}

// compileSignExtend64From8 implements compiler.compileSignExtend64From8 for the arm64 architecture.
func (c *arm64Compiler) compileSignExtend64From8() error {
	return c.compileSimpleUnop(arm64.SXTB, runtimeValueTypeI64)
}

// compileSignExtend64From16 implements compiler.compileSignExtend64From16 for the arm64 architecture.
func (c *arm64Compiler) compileSignExtend64From16() error {
	return c.compileSimpleUnop(arm64.SXTH, runtimeValueTypeI64)
}

// compileSignExtend64From32 implements compiler.compileSignExtend64From32 for the arm64 architecture.
func (c *arm64Compiler) compileSignExtend64From32() error {
	return c.compileSimpleUnop(arm64.SXTW, runtimeValueTypeI64)
}

func (c *arm64Compiler) compileSimpleUnop(inst asm.Instruction, resultRuntimeValueType runtimeValueType) error {
	v, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	reg := v.register
	c.assembler.CompileRegisterToRegister(inst, reg, reg)
	c.pushRuntimeValueLocationOnRegister(reg, resultRuntimeValueType)
	return nil
}

// compileEq implements compiler.compileEq for the arm64 architecture.
func (c *arm64Compiler) compileEq(o *wazeroir.OperationEq) error {
	return c.emitEqOrNe(true, o.Type)
}

// compileNe implements compiler.compileNe for the arm64 architecture.
func (c *arm64Compiler) compileNe(o *wazeroir.OperationNe) error {
	return c.emitEqOrNe(false, o.Type)
}

// emitEqOrNe implements compiler.compileEq and compiler.compileNe for the arm64 architecture.
func (c *arm64Compiler) emitEqOrNe(isEq bool, unsignedType wazeroir.UnsignedType) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	switch unsignedType {
	case wazeroir.UnsignedTypeI32:
		inst = arm64.CMPW
	case wazeroir.UnsignedTypeI64:
		inst = arm64.CMP
	case wazeroir.UnsignedTypeF32:
		inst = arm64.FCMPS
	case wazeroir.UnsignedTypeF64:
		inst = arm64.FCMPD
	}

	c.assembler.CompileTwoRegistersToNone(inst, x2.register, x1.register)

	// Push the comparison result as a conditional register value.
	cond := arm64.CondNE
	if isEq {
		cond = arm64.CondEQ
	}
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(cond)
	return nil
}

// compileEqz implements compiler.compileEqz for the arm64 architecture.
func (c *arm64Compiler) compileEqz(o *wazeroir.OperationEqz) error {
	x1, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	switch o.Type {
	case wazeroir.UnsignedInt32:
		inst = arm64.CMPW
	case wazeroir.UnsignedInt64:
		inst = arm64.CMP
	}

	c.assembler.CompileTwoRegistersToNone(inst, arm64.RegRZR, x1.register)

	// Push the comparison result as a conditional register value.
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(arm64.CondEQ)
	return nil
}

// compileLt implements compiler.compileLt for the arm64 architecture.
func (c *arm64Compiler) compileLt(o *wazeroir.OperationLt) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	var conditionalRegister asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondLO
	case wazeroir.SignedTypeUint64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondLO
	case wazeroir.SignedTypeInt32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondLT
	case wazeroir.SignedTypeInt64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondLT
	case wazeroir.SignedTypeFloat32:
		inst = arm64.FCMPS
		conditionalRegister = arm64.CondMI
	case wazeroir.SignedTypeFloat64:
		inst = arm64.FCMPD
		conditionalRegister = arm64.CondMI
	}

	c.assembler.CompileTwoRegistersToNone(inst, x2.register, x1.register)

	// Push the comparison result as a conditional register value.
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(conditionalRegister)
	return nil
}

// compileGt implements compiler.compileGt for the arm64 architecture.
func (c *arm64Compiler) compileGt(o *wazeroir.OperationGt) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	var conditionalRegister asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondHI
	case wazeroir.SignedTypeUint64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondHI
	case wazeroir.SignedTypeInt32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondGT
	case wazeroir.SignedTypeInt64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondGT
	case wazeroir.SignedTypeFloat32:
		inst = arm64.FCMPS
		conditionalRegister = arm64.CondGT
	case wazeroir.SignedTypeFloat64:
		inst = arm64.FCMPD
		conditionalRegister = arm64.CondGT
	}

	c.assembler.CompileTwoRegistersToNone(inst, x2.register, x1.register)

	// Push the comparison result as a conditional register value.
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(conditionalRegister)
	return nil
}

// compileLe implements compiler.compileLe for the arm64 architecture.
func (c *arm64Compiler) compileLe(o *wazeroir.OperationLe) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	var conditionalRegister asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondLS
	case wazeroir.SignedTypeUint64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondLS
	case wazeroir.SignedTypeInt32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondLE
	case wazeroir.SignedTypeInt64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondLE
	case wazeroir.SignedTypeFloat32:
		inst = arm64.FCMPS
		conditionalRegister = arm64.CondLS
	case wazeroir.SignedTypeFloat64:
		inst = arm64.FCMPD
		conditionalRegister = arm64.CondLS
	}

	c.assembler.CompileTwoRegistersToNone(inst, x2.register, x1.register)

	// Push the comparison result as a conditional register value.
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(conditionalRegister)
	return nil
}

// compileGe implements compiler.compileGe for the arm64 architecture.
func (c *arm64Compiler) compileGe(o *wazeroir.OperationGe) error {
	x1, x2, err := c.popTwoValuesOnRegisters()
	if err != nil {
		return err
	}

	var inst asm.Instruction
	var conditionalRegister asm.ConditionalRegisterState
	switch o.Type {
	case wazeroir.SignedTypeUint32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondHS
	case wazeroir.SignedTypeUint64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondHS
	case wazeroir.SignedTypeInt32:
		inst = arm64.CMPW
		conditionalRegister = arm64.CondGE
	case wazeroir.SignedTypeInt64:
		inst = arm64.CMP
		conditionalRegister = arm64.CondGE
	case wazeroir.SignedTypeFloat32:
		inst = arm64.FCMPS
		conditionalRegister = arm64.CondGE
	case wazeroir.SignedTypeFloat64:
		inst = arm64.FCMPD
		conditionalRegister = arm64.CondGE
	}

	c.assembler.CompileTwoRegistersToNone(inst, x2.register, x1.register)

	// Push the comparison result as a conditional register value.
	c.locationStack.pushRuntimeValueLocationOnConditionalRegister(conditionalRegister)
	return nil
}

// compileLoad implements compiler.compileLoad for the arm64 architecture.
func (c *arm64Compiler) compileLoad(o *wazeroir.OperationLoad) error {
	var (
		isFloat           bool
		loadInst          asm.Instruction
		targetSizeInBytes int64
		vt                runtimeValueType
	)

	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		loadInst = arm64.MOVWU
		targetSizeInBytes = 32 / 8
		vt = runtimeValueTypeI32
	case wazeroir.UnsignedTypeI64:
		loadInst = arm64.MOVD
		targetSizeInBytes = 64 / 8
		vt = runtimeValueTypeI64
	case wazeroir.UnsignedTypeF32:
		loadInst = arm64.FMOVS
		isFloat = true
		targetSizeInBytes = 32 / 8
		vt = runtimeValueTypeF32
	case wazeroir.UnsignedTypeF64:
		loadInst = arm64.FMOVD
		isFloat = true
		targetSizeInBytes = 64 / 8
		vt = runtimeValueTypeF64
	}
	return c.compileLoadImpl(o.Arg.Offset, loadInst, targetSizeInBytes, isFloat, vt)
}

// compileLoad8 implements compiler.compileLoad8 for the arm64 architecture.
func (c *arm64Compiler) compileLoad8(o *wazeroir.OperationLoad8) error {
	var loadInst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedInt32:
		// TODO used 32-bit variant.
		loadInst = arm64.MOVB
		vt = runtimeValueTypeI32
	case wazeroir.SignedInt64:
		loadInst = arm64.MOVB
		vt = runtimeValueTypeI64
	case wazeroir.SignedUint32:
		loadInst = arm64.MOVBU
		vt = runtimeValueTypeI32
	case wazeroir.SignedUint64:
		loadInst = arm64.MOVBU
		vt = runtimeValueTypeI64
	}
	return c.compileLoadImpl(o.Arg.Offset, loadInst, 1, false, vt)
}

// compileLoad16 implements compiler.compileLoad16 for the arm64 architecture.
func (c *arm64Compiler) compileLoad16(o *wazeroir.OperationLoad16) error {
	var loadInst asm.Instruction
	var vt runtimeValueType
	switch o.Type {
	case wazeroir.SignedInt32:
		// TODO used 32-bit variant.
		loadInst = arm64.MOVH
		vt = runtimeValueTypeI32
	case wazeroir.SignedInt64:
		loadInst = arm64.MOVH
		vt = runtimeValueTypeI64
	case wazeroir.SignedUint32:
		loadInst = arm64.MOVHU
		vt = runtimeValueTypeI32
	case wazeroir.SignedUint64:
		loadInst = arm64.MOVHU
		vt = runtimeValueTypeI64
	}
	return c.compileLoadImpl(o.Arg.Offset, loadInst, 16/8, false, vt)
}

// compileLoad32 implements compiler.compileLoad32 for the arm64 architecture.
func (c *arm64Compiler) compileLoad32(o *wazeroir.OperationLoad32) error {
	var loadInst asm.Instruction
	if o.Signed {
		loadInst = arm64.MOVW
	} else {
		loadInst = arm64.MOVWU
	}
	return c.compileLoadImpl(o.Arg.Offset, loadInst, 32/8, false, runtimeValueTypeI64)
}

// compileLoadImpl implements compileLoadImpl* variants for arm64 architecture.
func (c *arm64Compiler) compileLoadImpl(offsetArg uint32, loadInst asm.Instruction,
	targetSizeInBytes int64, isFloat bool, resultRuntimeValueType runtimeValueType) error {
	offsetReg, err := c.compileMemoryAccessOffsetSetup(offsetArg, targetSizeInBytes)
	if err != nil {
		return err
	}

	resultRegister := offsetReg
	if isFloat {
		resultRegister, err = c.allocateRegister(registerTypeVector)
		if err != nil {
			return err
		}
	}

	// "resultRegister = [arm64ReservedRegisterForMemory + offsetReg]"
	// In other words, "resultRegister = memory.Buffer[offset: offset+targetSizeInBytes]"
	c.assembler.CompileMemoryWithRegisterOffsetToRegister(
		loadInst,
		arm64ReservedRegisterForMemory, offsetReg,
		resultRegister,
	)

	c.pushRuntimeValueLocationOnRegister(resultRegister, resultRuntimeValueType)
	return nil
}

// compileStore implements compiler.compileStore for the arm64 architecture.
func (c *arm64Compiler) compileStore(o *wazeroir.OperationStore) error {
	var movInst asm.Instruction
	var targetSizeInBytes int64
	switch o.Type {
	case wazeroir.UnsignedTypeI32:
		movInst = arm64.MOVW
		targetSizeInBytes = 32 / 8
	case wazeroir.UnsignedTypeI64:
		movInst = arm64.MOVD
		targetSizeInBytes = 64 / 8
	case wazeroir.UnsignedTypeF32:
		movInst = arm64.FMOVS
		targetSizeInBytes = 32 / 8
	case wazeroir.UnsignedTypeF64:
		movInst = arm64.FMOVD
		targetSizeInBytes = 64 / 8
	}
	return c.compileStoreImpl(o.Arg.Offset, movInst, targetSizeInBytes)
}

// compileStore8 implements compiler.compileStore8 for the arm64 architecture.
func (c *arm64Compiler) compileStore8(o *wazeroir.OperationStore8) error {
	return c.compileStoreImpl(o.Arg.Offset, arm64.MOVB, 1)
}

// compileStore16 implements compiler.compileStore16 for the arm64 architecture.
func (c *arm64Compiler) compileStore16(o *wazeroir.OperationStore16) error {
	return c.compileStoreImpl(o.Arg.Offset, arm64.MOVH, 16/8)
}

// compileStore32 implements compiler.compileStore32 for the arm64 architecture.
func (c *arm64Compiler) compileStore32(o *wazeroir.OperationStore32) error {
	return c.compileStoreImpl(o.Arg.Offset, arm64.MOVW, 32/8)
}

// compileStoreImpl implements compleStore* variants for arm64 architecture.
func (c *arm64Compiler) compileStoreImpl(offsetArg uint32, storeInst asm.Instruction, targetSizeInBytes int64) error {
	val, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	// Mark temporarily used as compileMemoryAccessOffsetSetup might try allocating register.
	c.markRegisterUsed(val.register)

	offsetReg, err := c.compileMemoryAccessOffsetSetup(offsetArg, targetSizeInBytes)
	if err != nil {
		return err
	}

	// "[arm64ReservedRegisterForMemory + offsetReg] = val.register"
	// In other words, "memory.Buffer[offset: offset+targetSizeInBytes] = val.register"
	c.assembler.CompileRegisterToMemoryWithRegisterOffset(
		storeInst, val.register,
		arm64ReservedRegisterForMemory, offsetReg,
	)

	c.markRegisterUnused(val.register)
	return nil
}

// compileMemoryAccessOffsetSetup pops the top value from the stack (called "base"), stores "base + offsetArg + targetSizeInBytes"
// into a register, and returns the stored register. We call the result "offset" because we access the memory
// as memory.Buffer[offset: offset+targetSizeInBytes].
//
// Note: this also emits the instructions to check the out of bounds memory access.
// In other words, if the offset+targetSizeInBytes exceeds the memory size, the code exits with nativeCallStatusCodeMemoryOutOfBounds status.
func (c *arm64Compiler) compileMemoryAccessOffsetSetup(offsetArg uint32, targetSizeInBytes int64) (offsetRegister asm.Register, err error) {
	base, err := c.popValueOnRegister()
	if err != nil {
		return 0, err
	}

	offsetRegister = base.register
	if isZeroRegister(base.register) {
		offsetRegister, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, offsetRegister)
	}

	if offsetConst := int64(offsetArg) + targetSizeInBytes; offsetConst <= math.MaxUint32 {
		// "offsetRegister = base + offsetArg + targetSizeInBytes"
		c.assembler.CompileConstToRegister(arm64.ADD, offsetConst, offsetRegister)
	} else {
		// If the offset const is too large, we exit with nativeCallStatusCodeMemoryOutOfBounds.
		c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
		return
	}

	// "arm64ReservedRegisterForTemporary = len(memory.Buffer)"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
		arm64ReservedRegisterForTemporary)

	// Check if offsetRegister(= base+offsetArg+targetSizeInBytes) > len(memory.Buffer).
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, offsetRegister)
	boundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If offsetRegister(= base+offsetArg+targetSizeInBytes) exceeds the memory length,
	//  we exit the function with nativeCallStatusCodeMemoryOutOfBounds.
	c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)

	// Otherwise, we subtract targetSizeInBytes from offsetRegister.
	c.assembler.SetJumpTargetOnNext(boundsOK)
	c.assembler.CompileConstToRegister(arm64.SUB, targetSizeInBytes, offsetRegister)
	return offsetRegister, nil
}

// compileMemoryGrow implements compileMemoryGrow variants for arm64 architecture.
func (c *arm64Compiler) compileMemoryGrow() error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	if err := c.compileCallGoFunction(nativeCallStatusCodeCallBuiltInFunction, builtinFunctionIndexMemoryGrow); err != nil {
		return err
	}

	// After return, we re-initialize reserved registers just like preamble of functions.
	c.compileReservedStackBasePointerRegisterInitialization()
	c.compileReservedMemoryRegisterInitialization()
	return nil
}

// compileMemorySize implements compileMemorySize variants for arm64 architecture.
func (c *arm64Compiler) compileMemorySize() error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	reg, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// "reg = len(memory.Buffer)"
	c.assembler.CompileMemoryToRegister(
		arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
		reg,
	)

	// memory.size loads the page size of memory, so we have to divide by the page size.
	// "reg = reg >> wasm.MemoryPageSizeInBits (== reg / wasm.MemoryPageSize) "
	c.assembler.CompileConstToRegister(
		arm64.LSR,
		wasm.MemoryPageSizeInBits,
		reg,
	)

	c.pushRuntimeValueLocationOnRegister(reg, runtimeValueTypeI32)
	return nil
}

// compileCallGoFunction adds instructions to call a Go function whose address equals the addr parameter.
// compilerStatus is set before making call, and it should be either nativeCallStatusCodeCallBuiltInFunction or
// nativeCallStatusCodeCallHostFunction.
func (c *arm64Compiler) compileCallGoFunction(compilerStatus nativeCallStatusCode, builtinFunction wasm.Index) error {
	// Release all the registers as our calling convention requires the caller-save.
	if err := c.compileReleaseAllRegistersToStack(); err != nil {
		return err
	}

	freeRegs, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 4)
	if !found {
		return fmt.Errorf("BUG: all registers except indexReg should be free at this point")
	}
	c.markRegisterUsed(freeRegs...)

	// Alias these free tmp registers for readability.
	tmp, currentCallFrameStackPointerRegister, currentCallFrameTopAddressRegister, returnAddressRegister :=
		freeRegs[0], freeRegs[1], freeRegs[2], freeRegs[3]

	if compilerStatus == nativeCallStatusCodeCallBuiltInFunction {
		// Set the target function address to ce.functionCallAddress
		// "tmp = $index"
		c.assembler.CompileConstToRegister(
			arm64.MOVD,
			int64(builtinFunction),
			tmp,
		)
		// "[arm64ReservedRegisterForCallEngine + callEngineExitContextFunctionCallAddressOffset] = tmp"
		// In other words, "ce.functionCallAddress = tmp (== $addr)"
		c.assembler.CompileRegisterToMemory(
			arm64.MOVW,
			tmp,
			arm64ReservedRegisterForCallEngine, callEngineExitContextBuiltinFunctionCallAddressOffset,
		)
	}

	// Next, we have to set the return address into callFrameStack[ce.callFrameStackPointer-1].returnAddress.
	//
	// "currentCallFrameStackPointerRegister = ce.callFrameStackPointer"
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextCallFrameStackPointerOffset,
		currentCallFrameStackPointerRegister)

	// Set the address of callFrameStack[ce.callFrameStackPointer] into currentCallFrameTopAddressRegister.
	c.compileCalcCallFrameStackTopAddress(currentCallFrameStackPointerRegister, currentCallFrameTopAddressRegister)

	// Set the return address (after RET in c.compileExitFromNativeCode below) into returnAddressRegister.
	c.assembler.CompileReadInstructionAddress(returnAddressRegister, arm64.RET)

	// Write returnAddressRegister into callFrameStack[ce.callFrameStackPointer-1].returnAddress.
	c.assembler.CompileRegisterToMemory(
		arm64.MOVD,
		returnAddressRegister,
		// callFrameStack[ce.callFrameStackPointer-1] is below of currentCallFrameTopAddressRegister.
		currentCallFrameTopAddressRegister, -(callFrameDataSize - callFrameReturnAddressOffset),
	)

	c.markRegisterUnused(freeRegs...)
	c.compileExitFromNativeCode(compilerStatus)
	return nil
}

// compileConstI32 implements compiler.compileConstI32 for the arm64 architecture.
func (c *arm64Compiler) compileConstI32(o *wazeroir.OperationConstI32) error {
	return c.compileIntConstant(true, uint64(o.Value))
}

// compileConstI64 implements compiler.compileConstI64 for the arm64 architecture.
func (c *arm64Compiler) compileConstI64(o *wazeroir.OperationConstI64) error {
	return c.compileIntConstant(false, o.Value)
}

// compileIntConstant adds instructions to load an integer constant.
// is32bit is true if the target value is originally 32-bit const, false otherwise.
// value holds the (zero-extended for 32-bit case) load target constant.
func (c *arm64Compiler) compileIntConstant(is32bit bool, value uint64) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	var inst asm.Instruction
	var vt runtimeValueType
	if is32bit {
		inst = arm64.MOVW
		vt = runtimeValueTypeI32
	} else {
		inst = arm64.MOVD
		vt = runtimeValueTypeI64
	}

	if value == 0 {
		c.pushRuntimeValueLocationOnRegister(arm64.RegRZR, vt)
	} else {
		// Take a register to load the value.
		reg, err := c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}

		c.assembler.CompileConstToRegister(inst, int64(value), reg)

		c.pushRuntimeValueLocationOnRegister(reg, vt)
	}
	return nil
}

// compileConstF32 implements compiler.compileConstF32 for the arm64 architecture.
func (c *arm64Compiler) compileConstF32(o *wazeroir.OperationConstF32) error {
	return c.compileFloatConstant(true, uint64(math.Float32bits(o.Value)))
}

// compileConstF64 implements compiler.compileConstF64 for the arm64 architecture.
func (c *arm64Compiler) compileConstF64(o *wazeroir.OperationConstF64) error {
	return c.compileFloatConstant(false, math.Float64bits(o.Value))
}

// compileFloatConstant adds instructions to load a float constant.
// is32bit is true if the target value is originally 32-bit const, false otherwise.
// value holds the (zero-extended for 32-bit case) bit representation of load target float constant.
func (c *arm64Compiler) compileFloatConstant(is32bit bool, value uint64) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	// Take a register to load the value.
	reg, err := c.allocateRegister(registerTypeVector)
	if err != nil {
		return err
	}

	tmpReg := arm64.RegRZR
	if value != 0 {
		tmpReg = arm64ReservedRegisterForTemporary
		var inst asm.Instruction
		if is32bit {
			inst = arm64.MOVW
		} else {
			inst = arm64.MOVD
		}
		c.assembler.CompileConstToRegister(inst, int64(value), tmpReg)
	}

	// Use FMOV instruction to move the value on integer register into the float one.
	var inst asm.Instruction
	var vt runtimeValueType
	if is32bit {
		vt = runtimeValueTypeF32
		inst = arm64.FMOVS
	} else {
		vt = runtimeValueTypeF64
		inst = arm64.FMOVD
	}
	c.assembler.CompileRegisterToRegister(inst, tmpReg, reg)

	c.pushRuntimeValueLocationOnRegister(reg, vt)
	return nil
}

// compileMemoryInit implements compiler.compileMemoryInit for the arm64 architecture.
func (c *arm64Compiler) compileMemoryInit(o *wazeroir.OperationMemoryInit) error {
	return c.compileInitImpl(false, o.DataIndex, 0)
}

// compileInitImpl implements compileTableInit and compileMemoryInit.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *arm64Compiler) compileInitImpl(isTable bool, index, tableIndex uint32) error {
	outOfBoundsErrorStatus := nativeCallStatusCodeMemoryOutOfBounds
	if isTable {
		outOfBoundsErrorStatus = nativeCallStatusCodeInvalidTableAccess
	}

	copySize, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	c.markRegisterUsed(copySize.register)

	sourceOffset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	if isZeroRegister(sourceOffset.register) {
		sourceOffset.register, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, sourceOffset.register)
	}
	c.markRegisterUsed(sourceOffset.register)

	destinationOffset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	if isZeroRegister(destinationOffset.register) {
		destinationOffset.register, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, destinationOffset.register)
	}
	c.markRegisterUsed(destinationOffset.register)

	var tableInstanceAddressReg = asm.NilRegister
	if isTable {
		tableInstanceAddressReg, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.markRegisterUsed(tableInstanceAddressReg)
	}

	if !isZeroRegister(copySize.register) {
		// sourceOffset += size.
		c.assembler.CompileRegisterToRegister(arm64.ADD, copySize.register, sourceOffset.register)
		// destinationOffset += size.
		c.assembler.CompileRegisterToRegister(arm64.ADD, copySize.register, destinationOffset.register)
	}

	instanceAddr, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	if isTable {
		c.compileLoadElemInstanceAddress(index, instanceAddr)
	} else {
		c.compileLoadDataInstanceAddress(index, instanceAddr)
	}

	// Check data instance bounds.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		instanceAddr, 8, // DataInstance and Element instance holds the length is stored at offset 8.
		arm64ReservedRegisterForTemporary)

	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, sourceOffset.register)
	sourceBoundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If not, raise out of bounds memory access error.
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)

	c.assembler.SetJumpTargetOnNext(sourceBoundsOK)

	// Check destination bounds.
	if isTable {
		// arm64ReservedRegisterForTemporary = &tables[0]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
			arm64ReservedRegisterForTemporary)
		// tableInstanceAddressReg = arm64ReservedRegisterForTemporary + tableIndex*8
		//                         = &tables[0] + sizeOf(*tableInstance)*8
		//                         = &tables[tableIndex]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, int64(tableIndex)*8,
			tableInstanceAddressReg)
		// arm64ReservedRegisterForTemporary = [tableInstanceAddressReg+tableInstanceTableLenOffset] = len(tables[tableIndex])
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			tableInstanceAddressReg, tableInstanceTableLenOffset,
			arm64ReservedRegisterForTemporary)
	} else {
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
			arm64ReservedRegisterForTemporary)
	}

	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, destinationOffset.register)
	destinationBoundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If not, raise out of bounds memory access error.
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)

	// Otherwise, ready to copy the value from source to destination.
	c.assembler.SetJumpTargetOnNext(destinationBoundsOK)

	if !isZeroRegister(copySize.register) {
		// If the size equals zero, we can skip the entire instructions beflow.
		c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, copySize.register)
		skipCopyJump := c.assembler.CompileJump(arm64.BCONDEQ)

		var movInst asm.Instruction
		var movSize int64
		if isTable {
			movInst = arm64.MOVD
			movSize = 8

			// arm64ReservedRegisterForTemporary = &Table[0]
			c.assembler.CompileMemoryToRegister(arm64.MOVD, tableInstanceAddressReg,
				tableInstanceTableOffset, arm64ReservedRegisterForTemporary)
			// destinationOffset = (destinationOffset<< pointerSizeLog2) + arm64ReservedRegisterForTemporary
			c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
				destinationOffset.register, pointerSizeLog2,
				arm64ReservedRegisterForTemporary, destinationOffset.register)

			// arm64ReservedRegisterForTemporary = &ElementInstance.References[0]
			c.assembler.CompileMemoryToRegister(arm64.MOVD, instanceAddr, 0, arm64ReservedRegisterForTemporary)
			// sourceOffset = (sourceOffset<< pointerSizeLog2) + arm64ReservedRegisterForTemporary
			c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
				sourceOffset.register, pointerSizeLog2,
				arm64ReservedRegisterForTemporary, sourceOffset.register)

			// copySize = copySize << pointerSizeLog2
			c.assembler.CompileConstToRegister(arm64.LSL, pointerSizeLog2, copySize.register)
		} else {
			movInst = arm64.MOVBU
			movSize = 1

			// destinationOffset += memory buffer's absolute address.
			c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, destinationOffset.register)

			// sourceOffset += data buffer's absolute address.
			c.assembler.CompileMemoryToRegister(arm64.MOVD, instanceAddr, 0, arm64ReservedRegisterForTemporary)
			c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForTemporary, sourceOffset.register)

		}

		// Negate the counter.
		c.assembler.CompileRegisterToRegister(arm64.NEG, copySize.register, copySize.register)

		beginCopyLoop := c.assembler.CompileStandAlone(arm64.NOP)

		// arm64ReservedRegisterForTemporary = [sourceOffset + (size.register)]
		c.assembler.CompileMemoryWithRegisterOffsetToRegister(movInst,
			sourceOffset.register, copySize.register,
			arm64ReservedRegisterForTemporary)
		// [destinationOffset + (size.register)] = arm64ReservedRegisterForTemporary.
		c.assembler.CompileRegisterToMemoryWithRegisterOffset(movInst,
			arm64ReservedRegisterForTemporary,
			destinationOffset.register, copySize.register,
		)

		// Decrement the size counter and if the value is still negative, continue the loop.
		c.assembler.CompileConstToRegister(arm64.ADDS, movSize, copySize.register)
		c.assembler.CompileJump(arm64.BCONDMI).AssignJumpTarget(beginCopyLoop)

		c.assembler.SetJumpTargetOnNext(skipCopyJump)
	}

	c.markRegisterUnused(copySize.register, sourceOffset.register,
		destinationOffset.register, instanceAddr, tableInstanceAddressReg)
	return nil
}

// compileDataDrop implements compiler.compileDataDrop for the arm64 architecture.
func (c *arm64Compiler) compileDataDrop(o *wazeroir.OperationDataDrop) error {
	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.compileLoadDataInstanceAddress(o.DataIndex, tmp)

	// Clears the content of DataInstance[o.DataIndex] (== []byte type).
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 0)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 8)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 16)
	return nil
}

func (c *arm64Compiler) compileLoadDataInstanceAddress(dataIndex uint32, dst asm.Register) {
	// dst = dataIndex * dataInstanceStructSize
	c.assembler.CompileConstToRegister(arm64.MOVD, int64(dataIndex)*dataInstanceStructSize, dst)

	// arm64ReservedRegisterForTemporary = &moduleInstance.DataInstances[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextDataInstancesElement0AddressOffset,
		arm64ReservedRegisterForTemporary,
	)

	// dst = arm64ReservedRegisterForTemporary + dst
	//     = &moduleInstance.DataInstances[0] + dataIndex*dataInstanceStructSize
	//     = &moduleInstance.DataInstances[dataIndex]
	c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForTemporary, dst)
}

// compileMemoryCopy implements compiler.compileMemoryCopy for the arm64 architecture.
func (c *arm64Compiler) compileMemoryCopy() error {
	return c.compileCopyImpl(false, 0, 0)
}

// compileCopyImpl implements compileTableCopy and compileMemoryCopy.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *arm64Compiler) compileCopyImpl(isTable bool, srcTableIndex, dstTableIndex uint32) error {
	outOfBoundsErrorStatus := nativeCallStatusCodeMemoryOutOfBounds
	if isTable {
		outOfBoundsErrorStatus = nativeCallStatusCodeInvalidTableAccess
	}

	copySize, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	c.markRegisterUsed(copySize.register)

	sourceOffset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	if isZeroRegister(sourceOffset.register) {
		sourceOffset.register, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, sourceOffset.register)
	}
	c.markRegisterUsed(sourceOffset.register)

	destinationOffset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	if isZeroRegister(destinationOffset.register) {
		destinationOffset.register, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, destinationOffset.register)
	}
	c.markRegisterUsed(destinationOffset.register)

	if !isZeroRegister(copySize.register) {
		// sourceOffset += size.
		c.assembler.CompileRegisterToRegister(arm64.ADD, copySize.register, sourceOffset.register)
		// destinationOffset += size.
		c.assembler.CompileRegisterToRegister(arm64.ADD, copySize.register, destinationOffset.register)
	}

	if isTable {
		// arm64ReservedRegisterForTemporary = &tables[0]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = arm64ReservedRegisterForTemporary + srcTableIndex*8
		//                                   = &tables[0] + sizeOf(*tableInstance)*8
		//                                   = &tables[srcTableIndex]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, int64(srcTableIndex)*8,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForTemporary+tableInstanceTableLenOffset] = len(tables[srcTableIndex])
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
			arm64ReservedRegisterForTemporary)
	} else {
		// arm64ReservedRegisterForTemporary = len(memoryInst.Buffer).
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
			arm64ReservedRegisterForTemporary)
	}

	// Check memory len >= sourceOffset.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, sourceOffset.register)
	sourceBoundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If not, raise out of bounds memory access error.
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)

	c.assembler.SetJumpTargetOnNext(sourceBoundsOK)

	// Otherwise, check memory len >= destinationOffset.
	if isTable {
		// arm64ReservedRegisterForTemporary = &tables[0]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = arm64ReservedRegisterForTemporary + dstTableIndex*8
		//                                   = &tables[0] + sizeOf(*tableInstance)*8
		//                                   = &tables[dstTableIndex]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, int64(dstTableIndex)*8,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForTemporary+tableInstanceTableLenOffset] = len(tables[dstTableIndex])
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
			arm64ReservedRegisterForTemporary)
	}

	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, destinationOffset.register)
	destinationBoundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If not, raise out of bounds memory access error.
	c.compileExitFromNativeCode(outOfBoundsErrorStatus)

	// Otherwise, ready to copy the value from source to destination.
	c.assembler.SetJumpTargetOnNext(destinationBoundsOK)

	var movInst asm.Instruction
	var movSize int64
	if isTable {
		movInst = arm64.MOVD
		movSize = 8
	} else {
		movInst = arm64.MOVBU
		movSize = 1
	}

	// If the size equals zero, we can skip the entire instructions beflow.
	if !isZeroRegister(copySize.register) {
		c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, copySize.register)
		skipCopyJump := c.assembler.CompileJump(arm64.BCONDEQ)

		// If source offet < destination offset: for (i = size-1; i >= 0; i--) dst[i] = src[i];
		c.assembler.CompileTwoRegistersToNone(arm64.CMP, sourceOffset.register, destinationOffset.register)
		destLowerThanSourceJump := c.assembler.CompileJump(arm64.BCONDLS)
		var endJump asm.Node
		{
			// sourceOffset -= size.
			c.assembler.CompileRegisterToRegister(arm64.SUB, copySize.register, sourceOffset.register)
			// destinationOffset -= size.
			c.assembler.CompileRegisterToRegister(arm64.SUB, copySize.register, destinationOffset.register)

			if isTable {
				// arm64ReservedRegisterForTemporary = &Tables[dstTableIndex].Table[0]
				c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine,
					callEngineModuleContextTablesElement0AddressOffset, arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, int64(dstTableIndex)*8,
					arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
					arm64ReservedRegisterForTemporary)
				// destinationOffset = (destinationOffset<< pointerSizeLog2) + &Table[dstTableIndex].Table[0]
				c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
					destinationOffset.register, pointerSizeLog2,
					arm64ReservedRegisterForTemporary, destinationOffset.register)

				// arm64ReservedRegisterForTemporary = &Tables[srcTableIndex]
				c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine,
					callEngineModuleContextTablesElement0AddressOffset, arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, int64(srcTableIndex)*8,
					arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
					arm64ReservedRegisterForTemporary)
				// sourceOffset = (sourceOffset<< 3) + &Table[0]
				c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
					sourceOffset.register, pointerSizeLog2,
					arm64ReservedRegisterForTemporary, sourceOffset.register)

				// copySize = copySize << pointerSizeLog2 as each element has 8 bytes and we copy one by one.
				c.assembler.CompileConstToRegister(arm64.LSL, pointerSizeLog2, copySize.register)
			} else {
				// sourceOffset += memory buffer's absolute address.
				c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, sourceOffset.register)
				// destinationOffset += memory buffer's absolute address.
				c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, destinationOffset.register)
			}

			beginCopyLoop := c.assembler.CompileStandAlone(arm64.NOP)

			// size -= 1
			c.assembler.CompileConstToRegister(arm64.SUBS, movSize, copySize.register)

			// arm64ReservedRegisterForTemporary = [sourceOffset + (size.register)]
			c.assembler.CompileMemoryWithRegisterOffsetToRegister(movInst,
				sourceOffset.register, copySize.register,
				arm64ReservedRegisterForTemporary)
			// [destinationOffset + (size.register)] = arm64ReservedRegisterForTemporary.
			c.assembler.CompileRegisterToMemoryWithRegisterOffset(movInst,
				arm64ReservedRegisterForTemporary,
				destinationOffset.register, copySize.register,
			)

			// If the value on the copySize.register is not equal zero, continue the loop.
			c.assembler.CompileJump(arm64.BCONDNE).AssignJumpTarget(beginCopyLoop)

			// Otherwise, exit the loop.
			endJump = c.assembler.CompileJump(arm64.B)
		}

		// Else (destination offet < source offset): for (i = 0; i < size; i++) dst[counter-1-i] = src[counter-1-i];
		c.assembler.SetJumpTargetOnNext(destLowerThanSourceJump)
		{

			if isTable {
				// arm64ReservedRegisterForTemporary = &Tables[dstTableIndex].Table[0]
				c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine,
					callEngineModuleContextTablesElement0AddressOffset, arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, int64(dstTableIndex)*8,
					arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
					arm64ReservedRegisterForTemporary)
				// destinationOffset = (destinationOffset<< interfaceDataySizeLog2) + &Table[dstTableIndex].Table[0]
				c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
					destinationOffset.register, pointerSizeLog2,
					arm64ReservedRegisterForTemporary, destinationOffset.register)

				// arm64ReservedRegisterForTemporary = &Tables[srcTableIndex]
				c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine,
					callEngineModuleContextTablesElement0AddressOffset, arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, int64(srcTableIndex)*8,
					arm64ReservedRegisterForTemporary)
				c.assembler.CompileMemoryToRegister(arm64.MOVD,
					arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
					arm64ReservedRegisterForTemporary)
				// sourceOffset = (sourceOffset<< 3) + &Table[0]
				c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
					sourceOffset.register, pointerSizeLog2,
					arm64ReservedRegisterForTemporary, sourceOffset.register)

				// copySize = copySize << pointerSizeLog2 as each element has 8 bytes and we copy one by one.
				c.assembler.CompileConstToRegister(arm64.LSL, pointerSizeLog2, copySize.register)
			} else {
				// sourceOffset += memory buffer's absolute address.
				c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, sourceOffset.register)
				// destinationOffset += memory buffer's absolute address.
				c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, destinationOffset.register)
			}

			// Negate the counter.
			c.assembler.CompileRegisterToRegister(arm64.NEG, copySize.register, copySize.register)

			beginCopyLoop := c.assembler.CompileStandAlone(arm64.NOP)

			// arm64ReservedRegisterForTemporary = [sourceOffset + (size.register)]
			c.assembler.CompileMemoryWithRegisterOffsetToRegister(movInst,
				sourceOffset.register, copySize.register,
				arm64ReservedRegisterForTemporary)
			// [destinationOffset + (size.register)] = arm64ReservedRegisterForTemporary.
			c.assembler.CompileRegisterToMemoryWithRegisterOffset(movInst,
				arm64ReservedRegisterForTemporary,
				destinationOffset.register, copySize.register,
			)

			// size += 1
			c.assembler.CompileConstToRegister(arm64.ADDS, movSize, copySize.register)
			c.assembler.CompileJump(arm64.BCONDMI).AssignJumpTarget(beginCopyLoop)
		}
		c.assembler.SetJumpTargetOnNext(skipCopyJump, endJump)
	}

	// Mark all of the operand registers.
	c.markRegisterUnused(copySize.register, sourceOffset.register, destinationOffset.register)

	return nil
}

// compileMemoryFill implements compiler.compileMemoryCopy for the arm64 architecture.
func (c *arm64Compiler) compileMemoryFill() error {
	return c.compileFillImpl(false, 0)
}

// compileFillImpl implements TableFill and MemoryFill.
//
// TODO: the compiled code in this function should be reused and compile at once as
// the code is independent of any module.
func (c *arm64Compiler) compileFillImpl(isTable bool, tableIndex uint32) error {
	fillSize, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	c.markRegisterUsed(fillSize.register)

	value, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	c.markRegisterUsed(value.register)

	destinationOffset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}
	if isZeroRegister(destinationOffset.register) {
		destinationOffset.register, err = c.allocateRegister(registerTypeGeneralPurpose)
		if err != nil {
			return err
		}
		c.assembler.CompileRegisterToRegister(arm64.MOVD, arm64.RegRZR, destinationOffset.register)
	}
	c.markRegisterUsed(destinationOffset.register)

	// destinationOffset += size.
	c.assembler.CompileRegisterToRegister(arm64.ADD, fillSize.register, destinationOffset.register)

	if isTable {
		// arm64ReservedRegisterForTemporary = &tables[0]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = arm64ReservedRegisterForTemporary + srcTableIndex*8
		//                                   = &tables[0] + sizeOf(*tableInstance)*8
		//                                   = &tables[srcTableIndex]
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, int64(tableIndex)*8,
			arm64ReservedRegisterForTemporary)
		// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForTemporary+tableInstanceTableLenOffset] = len(tables[srcTableIndex])
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
			arm64ReservedRegisterForTemporary)
	} else {
		// arm64ReservedRegisterForTemporary = len(memoryInst.Buffer).
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
			arm64ReservedRegisterForTemporary)
	}

	// Check  len >= destinationOffset.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64ReservedRegisterForTemporary, destinationOffset.register)
	destinationBoundsOK := c.assembler.CompileJump(arm64.BCONDLS)

	// If not, raise the runtime error.
	if isTable {
		c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	} else {
		c.compileExitFromNativeCode(nativeCallStatusCodeMemoryOutOfBounds)
	}

	// Otherwise, ready to copy the value from destination to source.
	c.assembler.SetJumpTargetOnNext(destinationBoundsOK)

	// If the size equals zero, we can skip the entire instructions beflow.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64.RegRZR, fillSize.register)
	skipCopyJump := c.assembler.CompileJump(arm64.BCONDEQ)

	// destinationOffset -= size.
	c.assembler.CompileRegisterToRegister(arm64.SUB, fillSize.register, destinationOffset.register)

	var movInst asm.Instruction
	var movSize int64
	if isTable {
		movInst = arm64.MOVD
		movSize = 8

		// arm64ReservedRegisterForTemporary = &Tables[dstTableIndex].Table[0]
		c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine,
			callEngineModuleContextTablesElement0AddressOffset, arm64ReservedRegisterForTemporary)
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, int64(tableIndex)*8,
			arm64ReservedRegisterForTemporary)
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
			arm64ReservedRegisterForTemporary)
		// destinationOffset = (destinationOffset<< pointerSizeLog2) + &Table[dstTableIndex].Table[0]
		c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
			destinationOffset.register, pointerSizeLog2,
			arm64ReservedRegisterForTemporary, destinationOffset.register)

		// copySize = copySize << pointerSizeLog2 as each element has 8 bytes and we copy one by one.
		c.assembler.CompileConstToRegister(arm64.LSL, pointerSizeLog2, fillSize.register)
	} else {
		movInst = arm64.MOVBU
		movSize = 1

		// destinationOffset += memory buffer's absolute address.
		c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForMemory, destinationOffset.register)
	}

	// Naively implement the copy with "for loop" by copying byte one by one.
	beginCopyLoop := c.assembler.CompileStandAlone(arm64.NOP)

	// size -= 1
	c.assembler.CompileConstToRegister(arm64.SUBS, movSize, fillSize.register)

	// [destinationOffset + (size.register)] = arm64ReservedRegisterForTemporary.
	c.assembler.CompileRegisterToMemoryWithRegisterOffset(movInst,
		value.register,
		destinationOffset.register, fillSize.register,
	)

	// If the value on the copySizeRgister.register is not equal zero, continue the loop.
	continueJump := c.assembler.CompileJump(arm64.BCONDNE)
	continueJump.AssignJumpTarget(beginCopyLoop)

	// Mark all of the operand registers.
	c.markRegisterUnused(fillSize.register, value.register, destinationOffset.register)

	c.assembler.SetJumpTargetOnNext(skipCopyJump)
	return nil
}

// compileTableInit implements compiler.compileTableInit for the arm64 architecture.
func (c *arm64Compiler) compileTableInit(o *wazeroir.OperationTableInit) error {
	return c.compileInitImpl(true, o.ElemIndex, o.TableIndex)
}

// compileTableCopy implements compiler.compileTableCopy for the arm64 architecture.
func (c *arm64Compiler) compileTableCopy(o *wazeroir.OperationTableCopy) error {
	return c.compileCopyImpl(true, o.SrcTableIndex, o.DstTableIndex)
}

// compileElemDrop implements compiler.compileElemDrop for the arm64 architecture.
func (c *arm64Compiler) compileElemDrop(o *wazeroir.OperationElemDrop) error {
	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	c.compileLoadElemInstanceAddress(o.ElemIndex, tmp)

	// Clears the content of ElementInstances[o.ElemIndex] (== []interface{} type).
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 0)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 8)
	c.assembler.CompileRegisterToMemory(arm64.MOVD, arm64.RegRZR, tmp, 16)
	return nil
}

func (c *arm64Compiler) compileLoadElemInstanceAddress(elemIndex uint32, dst asm.Register) {
	// dst = dataIndex * elementInstanceStructSize
	c.assembler.CompileConstToRegister(arm64.MOVD, int64(elemIndex)*elementInstanceStructSize, dst)

	// arm64ReservedRegisterForTemporary = &moduleInstance.ElementInstances[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextElementInstancesElement0AddressOffset,
		arm64ReservedRegisterForTemporary,
	)

	// dst = arm64ReservedRegisterForTemporary + dst
	//     = &moduleInstance.ElementInstances[0] + elemIndex*elementInstanceStructSize
	//     = &moduleInstance.ElementInstances[elemIndex]
	c.assembler.CompileRegisterToRegister(arm64.ADD, arm64ReservedRegisterForTemporary, dst)
}

// compileRefFunc implements compiler.compileRefFunc for the arm64 architecture.
func (c *arm64Compiler) compileRefFunc(o *wazeroir.OperationRefFunc) error {
	ref, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForCallEngine + callEngineModuleContextFunctionsElement0AddressOffset]
	//                                   = &moduleEngine.functions[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextFunctionsElement0AddressOffset,
		arm64ReservedRegisterForTemporary)

	// ref = [arm64ReservedRegisterForTemporary +  int64(o.FunctionIndex)*8]
	//     = [&moduleEngine.functions[0] + sizeOf(*function) * index]
	//     = moduleEngine.functions[index]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, int64(o.FunctionIndex)*8, // * 8 because the size of *code equals 8 bytes.
		ref,
	)

	c.pushRuntimeValueLocationOnRegister(ref, runtimeValueTypeI64)
	return nil
}

// compileTableGet implements compiler.compileTableGet for the arm64 architecture.
func (c *arm64Compiler) compileTableGet(o *wazeroir.OperationTableGet) error {
	ref, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.markRegisterUsed(ref)

	offset, err := c.popValueOnRegister()
	if err != nil {
		return err
	}

	// arm64ReservedRegisterForTemporary = &tables[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		arm64ReservedRegisterForTemporary)
	// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForTemporary + TableIndex*8]
	//                                   = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
	//                                   = [&tables[TableIndex]] = tables[TableIndex].
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, int64(o.TableIndex)*8,
		arm64ReservedRegisterForTemporary)

	// Out of bounds check.
	// ref = [&tables[TableIndex] + tableInstanceTableLenOffset] = len(tables[TableIndex])
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
		ref,
	)
	// "cmp ref, offset"
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, ref, offset.register)

	// If it exceeds len(table), we exit the execution.
	brIfBoundsOK := c.assembler.CompileJump(arm64.BCONDLO)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(brIfBoundsOK)

	// ref = [&tables[TableIndex] + tableInstanceTableOffset] = &tables[TableIndex].References[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
		ref,
	)

	// ref = (offset << pointerSizeLog2) + ref
	//     = &tables[TableIndex].References[0] + sizeOf(uintptr) * offset
	//     = &tables[TableIndex].References[offset]
	c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
		offset.register, pointerSizeLog2, ref, ref)

	// ref = [&tables[TableIndex]] = load the Reference's pointer as uint64.
	c.assembler.CompileMemoryToRegister(arm64.MOVD, ref, 0, ref)

	c.pushRuntimeValueLocationOnRegister(ref, runtimeValueTypeI64) // table elements are opaque 64-bit at runtime.
	return nil
}

// compileTableSet implements compiler.compileTableSet for the arm64 architecture.
func (c *arm64Compiler) compileTableSet(o *wazeroir.OperationTableSet) error {
	ref := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(ref); err != nil {
		return err
	}

	offset := c.locationStack.pop()
	if err := c.compileEnsureOnRegister(offset); err != nil {
		return err
	}

	tmp, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}

	// arm64ReservedRegisterForTemporary = &tables[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		arm64ReservedRegisterForTemporary)
	// arm64ReservedRegisterForTemporary = arm64ReservedRegisterForTemporary + TableIndex*8
	//                                   = &tables[0] + TableIndex*sizeOf(*tableInstance)
	//                                   = &tables[TableIndex]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, int64(o.TableIndex)*8,
		arm64ReservedRegisterForTemporary)

	// Out of bounds check.
	// tmp = [&tables[TableIndex] + tableInstanceTableLenOffset] = len(tables[TableIndex])
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
		tmp,
	)
	// "cmp tmp, offset"
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, tmp, offset.register)

	// If it exceeds len(table), we exit the execution.
	brIfBoundsOK := c.assembler.CompileJump(arm64.BCONDLO)
	c.compileExitFromNativeCode(nativeCallStatusCodeInvalidTableAccess)
	c.assembler.SetJumpTargetOnNext(brIfBoundsOK)

	// tmp = [&tables[TableIndex] + tableInstanceTableOffset] = &tables[TableIndex].References[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, tableInstanceTableOffset,
		tmp,
	)

	// tmp = (offset << pointerSizeLog2) + tmp
	//     = &tables[TableIndex].References[0] + sizeOf(uintptr) * offset
	//     = &tables[TableIndex].References[offset]
	c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD, offset.register, pointerSizeLog2, tmp, tmp)

	// Set the reference's raw pointer.
	c.assembler.CompileRegisterToMemory(arm64.MOVD, ref.register, tmp, 0)

	c.markRegisterUnused(offset.register, ref.register, tmp)
	return nil
}

// compileTableGrow implements compiler.compileTableGrow for the arm64 architecture.
func (c *arm64Compiler) compileTableGrow(o *wazeroir.OperationTableGrow) error {
	c.maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister()

	// Pushes the table index.
	if err := c.compileConstI32(&wazeroir.OperationConstI32{Value: o.TableIndex}); err != nil {
		return err
	}

	// Table grow cannot be done in assembly just like memory grow as it involves with allocation in Go.
	// Therefore, call out to the built function for this purpose.
	if err := c.compileCallGoFunction(nativeCallStatusCodeCallBuiltInFunction, builtinFunctionIndexTableGrow); err != nil {
		return err
	}

	// TableGrow consumes three values (table index, number of items, initial value).
	for i := 0; i < 3; i++ {
		c.locationStack.pop()
	}

	// Then, the previous length was pushed as the result.
	c.locationStack.pushRuntimeValueLocationOnStack()

	// After return, we re-initialize reserved registers just like preamble of functions.
	c.compileReservedStackBasePointerRegisterInitialization()
	return nil
}

// compileTableSize implements compiler.compileTableSize for the arm64 architecture.
func (c *arm64Compiler) compileTableSize(o *wazeroir.OperationTableSize) error {
	result, err := c.allocateRegister(registerTypeGeneralPurpose)
	if err != nil {
		return err
	}
	c.markRegisterUsed(result)

	// arm64ReservedRegisterForTemporary = &tables[0]
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		arm64ReservedRegisterForTemporary)
	// arm64ReservedRegisterForTemporary = [arm64ReservedRegisterForTemporary + TableIndex*8]
	//                                   = [&tables[0] + TableIndex*sizeOf(*tableInstance)]
	//                                   = [&tables[TableIndex]] = tables[TableIndex].
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, int64(o.TableIndex)*8,
		arm64ReservedRegisterForTemporary)

	// result = [&tables[TableIndex] + tableInstanceTableLenOffset] = len(tables[TableIndex])
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForTemporary, tableInstanceTableLenOffset,
		result,
	)

	c.pushRuntimeValueLocationOnRegister(result, runtimeValueTypeI32)
	return nil
}

// compileTableFill implements compiler.compileTableFill for the arm64 architecture.
func (c *arm64Compiler) compileTableFill(o *wazeroir.OperationTableFill) error {
	return c.compileFillImpl(true, o.TableIndex)
}

// popTwoValuesOnRegisters pops two values from the location stacks, ensures
// these two values are located on registers, and mark them unused.
//
// TODO: we’d usually prefix this with compileXXX as this might end up emitting instructions,
// but the name seems awkward.
func (c *arm64Compiler) popTwoValuesOnRegisters() (x1, x2 *runtimeValueLocation, err error) {
	x2 = c.locationStack.pop()
	if err = c.compileEnsureOnRegister(x2); err != nil {
		return
	}

	x1 = c.locationStack.pop()
	if err = c.compileEnsureOnRegister(x1); err != nil {
		return
	}

	c.markRegisterUnused(x2.register)
	c.markRegisterUnused(x1.register)
	return
}

// popValueOnRegister pops one value from the location stack, ensures
// that it is located on a register, and mark it unused.
//
// TODO: we’d usually prefix this with compileXXX as this might end up emitting instructions,
// but the name seems awkward.
func (c *arm64Compiler) popValueOnRegister() (v *runtimeValueLocation, err error) {
	v = c.locationStack.pop()
	if err = c.compileEnsureOnRegister(v); err != nil {
		return
	}

	c.markRegisterUnused(v.register)
	return
}

// compileEnsureOnRegister emits instructions to ensure that a value is located on a register.
func (c *arm64Compiler) compileEnsureOnRegister(loc *runtimeValueLocation) error {
	if loc.onStack() {
		reg, err := c.allocateRegister(loc.getRegisterType())
		if err != nil {
			return err
		}

		// Record that the value holds the register and the register is marked used.
		loc.setRegister(reg)
		c.markRegisterUsed(reg)

		c.compileLoadValueOnStackToRegister(loc)
	} else if loc.onConditionalRegister() {
		c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc)
	}
	return nil
}

// maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister moves the top value on the stack
// if the value is located on a conditional register.
//
// This is usually called at the beginning of methods on compiler interface where we possibly
// compile instructions without saving the conditional register value.
// compile* functions without calling this function is saving the conditional
// value to the stack or register by invoking ensureOnGeneralPurposeRegister for the top.
func (c *arm64Compiler) maybeCompileMoveTopConditionalToFreeGeneralPurposeRegister() {
	if c.locationStack.sp > 0 {
		if loc := c.locationStack.peek(); loc.onConditionalRegister() {
			c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc)
		}
	}
}

// loadConditionalRegisterToGeneralPurposeRegister saves the conditional register value
// to a general purpose register.
func (c *arm64Compiler) compileLoadConditionalRegisterToGeneralPurposeRegister(loc *runtimeValueLocation) {
	// There must be always at least one free register at this point, as the conditional register located value
	// is always pushed after consuming at least one value (eqz) or two values for most cases (gt, ge, etc.).
	reg, _ := c.locationStack.takeFreeRegister(registerTypeGeneralPurpose)
	c.markRegisterUsed(reg)

	c.assembler.CompileConditionalRegisterSet(loc.conditionalRegister, reg)

	// Record that now the value is located on a general purpose register.
	loc.setRegister(reg)
}

// compileLoadValueOnStackToRegister emits instructions to load the value located on the stack to the assigned register.
func (c *arm64Compiler) compileLoadValueOnStackToRegister(loc *runtimeValueLocation) {
	switch loc.valueType {
	case runtimeValueTypeI32:
		// TODO: use 32-bit mov.
		c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForStackBasePointerAddress,
			int64(loc.stackPointer)*8, loc.register)
	case runtimeValueTypeI64:
		c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForStackBasePointerAddress,
			int64(loc.stackPointer)*8, loc.register)
	case runtimeValueTypeF32:
		// TODO: use 32-bit mov.
		c.assembler.CompileMemoryToRegister(arm64.FMOVD, arm64ReservedRegisterForStackBasePointerAddress,
			int64(loc.stackPointer)*8, loc.register)
	case runtimeValueTypeF64:
		c.assembler.CompileMemoryToRegister(arm64.FMOVD, arm64ReservedRegisterForStackBasePointerAddress,
			int64(loc.stackPointer)*8, loc.register)
	case runtimeValueTypeV128Lo:
		c.assembler.CompileMemoryToVectorRegister(arm64.VMOV,
			arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8, loc.register,
			arm64.VectorArrangementQ)
		// Higher 64-bits are loaded as well ^^.
		hi := c.locationStack.stack[loc.stackPointer+1]
		hi.setRegister(loc.register)
	case runtimeValueTypeV128Hi:
		panic("BUG: V128Hi must be be loaded to a register along with V128Lo")
	}
}

// allocateRegister returns an unused register of the given type. The register will be taken
// either from the free register pool or by spilling an used register. If we allocate an used register,
// this adds an instruction to write the value on a register back to memory stack region.
// Note: resulting registers are NOT marked as used so the call site should mark it used if necessary.
//
// TODO: we’d usually prefix this with compileXXX as this might end up emitting instructions,
// but the name seems awkward.
func (c *arm64Compiler) allocateRegister(t registerType) (reg asm.Register, err error) {
	var ok bool
	// Try to get the unused register.
	reg, ok = c.locationStack.takeFreeRegister(t)
	if ok {
		return
	}

	// If not found, we have to steal the register.
	stealTarget, ok := c.locationStack.takeStealTargetFromUsedRegister(t)
	if !ok {
		err = fmt.Errorf("cannot steal register")
		return
	}

	// Release the steal target register value onto stack location.
	reg = stealTarget.register
	c.compileReleaseRegisterToStack(stealTarget)
	return
}

// compileReleaseAllRegistersToStack adds instructions to store all the values located on
// either general purpose or conditional registers onto the memory stack.
// See releaseRegisterToStack.
func (c *arm64Compiler) compileReleaseAllRegistersToStack() error {
	for i := uint64(0); i < c.locationStack.sp; i++ {
		if loc := c.locationStack.stack[i]; loc.onRegister() {
			c.compileReleaseRegisterToStack(loc)
		} else if loc.onConditionalRegister() {
			c.compileLoadConditionalRegisterToGeneralPurposeRegister(loc)
			c.compileReleaseRegisterToStack(loc)
		}
	}
	return nil
}

// releaseRegisterToStack adds an instruction to write the value on a register back to memory stack region.
func (c *arm64Compiler) compileReleaseRegisterToStack(loc *runtimeValueLocation) {
	switch loc.valueType {
	case runtimeValueTypeI32:
		// Use 64-bit mov as all the values are represented as uint64 in Go, so we have to clear out the higher bits.
		c.assembler.CompileRegisterToMemory(arm64.MOVD, loc.register, arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8)
	case runtimeValueTypeI64:
		c.assembler.CompileRegisterToMemory(arm64.MOVD, loc.register, arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8)
	case runtimeValueTypeF32:
		// Use 64-bit mov as all the values are represented as uint64 in Go, so we have to clear out the higher bits.
		c.assembler.CompileRegisterToMemory(arm64.FMOVD, loc.register, arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8)
	case runtimeValueTypeF64:
		c.assembler.CompileRegisterToMemory(arm64.FMOVD, loc.register, arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8)
	case runtimeValueTypeV128Lo:
		c.assembler.CompileVectorRegisterToMemory(arm64.VMOV,
			loc.register, arm64ReservedRegisterForStackBasePointerAddress, int64(loc.stackPointer)*8,
			arm64.VectorArrangementQ)
		// Higher 64-bits are released as well ^^.
		hi := c.locationStack.stack[loc.stackPointer+1]
		c.locationStack.releaseRegister(hi)
	case runtimeValueTypeV128Hi:
		panic("BUG: V128Hi must be released to the stack along with V128Lo")
	}

	// Mark the register is free.
	c.locationStack.releaseRegister(loc)
}

// compileReservedStackBasePointerRegisterInitialization adds instructions to initialize arm64ReservedRegisterForStackBasePointerAddress
// so that it points to the absolute address of the stack base for this function.
func (c *arm64Compiler) compileReservedStackBasePointerRegisterInitialization() {
	// First, load the address of the first element in the value stack into arm64ReservedRegisterForStackBasePointerAddress temporarily.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineGlobalContextValueStackElement0AddressOffset,
		arm64ReservedRegisterForStackBasePointerAddress)

	// Next we move the base pointer (ce.stackBasePointer) to arm64ReservedRegisterForTemporary.
	c.assembler.CompileMemoryToRegister(arm64.MOVD,
		arm64ReservedRegisterForCallEngine, callEngineValueStackContextStackBasePointerOffset,
		arm64ReservedRegisterForTemporary)

	// Finally, we calculate "arm64ReservedRegisterForStackBasePointerAddress + arm64ReservedRegisterForTemporary << 3"
	// where we shift tmpReg by 3 because stack pointer is an index in the []uint64
	// so we must multiply the value by the size of uint64 = 8 bytes.
	c.assembler.CompileLeftShiftedRegisterToRegister(arm64.ADD,
		arm64ReservedRegisterForTemporary, 3, arm64ReservedRegisterForStackBasePointerAddress,
		arm64ReservedRegisterForStackBasePointerAddress)
}

func (c *arm64Compiler) compileReservedMemoryRegisterInitialization() {
	if c.ir.HasMemory {
		// "arm64ReservedRegisterForMemory = ce.MemoryElement0Address"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemoryElement0AddressOffset,
			arm64ReservedRegisterForMemory,
		)
	}
}

// compileModuleContextInitialization adds instructions to initialize ce.moduleContext's fields based on
// ce.moduleContext.ModuleInstanceAddress.
// This is called in two cases: in function preamble, and on the return from (non-Go) function calls.
func (c *arm64Compiler) compileModuleContextInitialization() error {
	c.markRegisterUsed(arm64CallingConventionModuleInstanceAddressRegister)
	defer c.markRegisterUnused(arm64CallingConventionModuleInstanceAddressRegister)

	regs, found := c.locationStack.takeFreeRegisters(registerTypeGeneralPurpose, 2)
	if !found {
		return fmt.Errorf("BUG: all the registers should be free at this point")
	}
	c.markRegisterUsed(regs...)

	// Alias these free registers for readability.
	tmpX, tmpY := regs[0], regs[1]

	// "tmpX = ce.ModuleInstanceAddress"
	c.assembler.CompileMemoryToRegister(arm64.MOVD, arm64ReservedRegisterForCallEngine, callEngineModuleContextModuleInstanceAddressOffset, tmpX)

	// If the module instance address stays the same, we could skip the entire code below.
	c.assembler.CompileTwoRegistersToNone(arm64.CMP, arm64CallingConventionModuleInstanceAddressRegister, tmpX)
	brIfModuleUnchanged := c.assembler.CompileJump(arm64.BCONDEQ)

	// Otherwise, update the moduleEngine.moduleContext.ModuleInstanceAddress.
	c.assembler.CompileRegisterToMemory(arm64.MOVD,
		arm64CallingConventionModuleInstanceAddressRegister,
		arm64ReservedRegisterForCallEngine, callEngineModuleContextModuleInstanceAddressOffset,
	)

	// Also, we have to update the following fields:
	// * callEngine.moduleContext.globalElement0Address
	// * callEngine.moduleContext.memoryElement0Address
	// * callEngine.moduleContext.memorySliceLen
	// * callEngine.moduleContext.tableElement0Address
	// * callEngine.moduleContext.tableSliceLen
	// * callEngine.moduleContext.functionsElement0Address
	// * callEngine.moduleContext.typeIDsElement0Address
	// * callEngine.moduleContext.dataInstancesElement0Address
	// * callEngine.moduleContext.elementInstancesElement0Address

	// Update globalElement0Address.
	//
	// Note: if there's global.get or set instruction in the function, the existence of the globals
	// is ensured by function validation at module instantiation phase, and that's why it is ok to
	// skip the initialization if the module's globals slice is empty.
	if len(c.ir.Globals) > 0 {
		// "tmpX = &moduleInstance.Globals[0]"
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceGlobalsOffset,
			tmpX,
		)

		// "ce.GlobalElement0Address = tmpX (== &moduleInstance.Globals[0])"
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD, tmpX,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextGlobalElement0AddressOffset,
		)
	}

	// Update memoryElement0Address and memorySliceLen.
	//
	// Note: if there's memory instruction in the function, memory instance must be non-nil.
	// That is ensured by function validation at module instantiation phase, and that's
	// why it is ok to skip the initialization if the module's memory instance is nil.
	if c.ir.HasMemory {
		// "tmpX = moduleInstance.Memory"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceMemoryOffset,
			tmpX,
		)

		// First, we write the memory length into ce.MemorySliceLen.
		//
		// "tmpY = [tmpX + memoryInstanceBufferLenOffset] (== len(memory.Buffer))"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			tmpX, memoryInstanceBufferLenOffset,
			tmpY,
		)
		// "ce.MemorySliceLen = tmpY".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpY,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemorySliceLenOffset,
		)

		// Next write ce.memoryElement0Address.
		//
		// "tmpY = *tmpX (== &memory.Buffer[0])"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			tmpX, memoryInstanceBufferOffset,
			tmpY,
		)
		// "ce.memoryElement0Address = tmpY".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpY,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextMemoryElement0AddressOffset,
		)
	}

	// Update tableElement0Address, tableSliceLen and typeIDsElement0Address.
	//
	// Note: if there's table instruction in the function, the existence of the table
	// is ensured by function validation at module instantiation phase, and that's
	// why it is ok to skip the initialization if the module's table doesn't exist.
	if c.ir.HasTable {
		// "tmpX = &tables[0] (type of **wasm.Table)"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceTablesOffset,
			tmpX,
		)

		// Update ce.tableElement0Address.
		// "ce.tableElement0Address = tmpX".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpX,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextTablesElement0AddressOffset,
		)

		// Finally, we put &ModuleInstance.TypeIDs[0] into moduleContext.typeIDsElement0Address.
		c.assembler.CompileMemoryToRegister(arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceTypeIDsOffset, tmpX)
		c.assembler.CompileRegisterToMemory(arm64.MOVD,
			tmpX, arm64ReservedRegisterForCallEngine, callEngineModuleContextTypeIDsElement0AddressOffset)
	}

	// Update callEngine.moduleContext.functionsElement0Address
	{
		// "tmpX = [moduleInstanceAddressRegister + moduleInstanceEngineOffset + interfaceDataOffset] (== *moduleEngine)"
		//
		// Go's interface is laid out on memory as two quad words as struct {tab, data uintptr}
		// where tab points to the interface table, and the latter points to the actual
		// implementation of interface. This case, we extract "data" pointer as *moduleEngine.
		// See the following references for detail:
		// * https://research.swtch.com/interfaces
		// * https://github.com/golang/go/blob/release-branch.go1.17/src/runtime/runtime2.go#L207-L210
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceEngineOffset+interfaceDataOffset,
			tmpX,
		)

		// "tmpY = [tmpX + moduleEngineFunctionsOffset] (== &moduleEngine.functions[0])"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			tmpX, moduleEngineFunctionsOffset,
			tmpY,
		)

		// "callEngine.moduleContext.functionsElement0Address = tmpY".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpY,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextFunctionsElement0AddressOffset,
		)
	}

	// Update dataInstancesElement0Address.
	if c.ir.NeedsAccessToDataInstances {
		// "tmpX = &moduleInstance.DataInstances[0]"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceDataInstancesOffset,
			tmpX,
		)
		// "callEngine.moduleContext.dataInstancesElement0Address = tmpX".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpX,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextDataInstancesElement0AddressOffset,
		)
	}

	// Update callEngine.moduleContext.elementInstancesElement0Address
	if c.ir.NeedsAccessToElementInstances {
		// "tmpX = &moduleInstance.DataInstances[0]"
		c.assembler.CompileMemoryToRegister(
			arm64.MOVD,
			arm64CallingConventionModuleInstanceAddressRegister, moduleInstanceElementInstancesOffset,
			tmpX,
		)
		// "callEngine.moduleContext.dataInstancesElement0Address = tmpX".
		c.assembler.CompileRegisterToMemory(
			arm64.MOVD,
			tmpX,
			arm64ReservedRegisterForCallEngine, callEngineModuleContextElementInstancesElement0AddressOffset,
		)
	}

	c.assembler.SetJumpTargetOnNext(brIfModuleUnchanged)
	c.markRegisterUnused(regs...)
	return nil
}
