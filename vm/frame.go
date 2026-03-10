package vm

import (
	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/types"
)

// Frame represents a call frame (activation record) for a function execution
type Frame struct {
	fn         *bytecode.FunctionConstant
	ip         int // Instruction pointer (points to next instruction to execute)
	basePointer int // Base pointer (points to the start of the frame on the stack)
	locals     []types.Object
}

const MaxCallStackDepth = 1024 // Maximum depth of call stack (prevents stack overflow from infinite recursion)

// NewFrame creates a new call frame for a function
func NewFrame(fn *bytecode.FunctionConstant, basePointer int) *Frame {
	// Ensure locals array is at least large enough to hold all parameters
	localCount := fn.NumLocals
	if localCount < fn.NumParameters {
		localCount = fn.NumParameters
	}
	return &Frame{
		fn:         fn,
		ip:         0,
		basePointer: basePointer,
		locals:     make([]types.Object, localCount),
	}
}

// Instructions returns the bytecode instructions of the function
func (f *Frame) Instructions() []byte {
	return f.fn.Instructions
}

// Name returns the name of the function
func (f *Frame) Name() string {
	return f.fn.Name
}

// CurrentInstruction returns the opcode at the current IP
func (f *Frame) CurrentInstruction() byte {
	if f.ip >= len(f.fn.Instructions) {
		return 0 // NOP
	}
	return f.fn.Instructions[f.ip]
}

// ReadUint8 reads an 8-bit operand from the instruction stream and advances IP
func (f *Frame) ReadUint8() uint8 {
	if f.ip >= len(f.fn.Instructions) {
		return 0
	}
	val := f.fn.Instructions[f.ip]
	f.ip++
	return val
}

// ReadUint16 reads a 16-bit operand from the instruction stream and advances IP
func (f *Frame) ReadUint16() uint16 {
	if f.ip+1 >= len(f.fn.Instructions) {
		return 0
	}
	val := uint16(f.fn.Instructions[f.ip])<<8 | uint16(f.fn.Instructions[f.ip+1])
	f.ip += 2
	return val
}

// NewFrameFromFunction creates a new frame from a runtime Function object
func NewFrameFromFunction(fn *types.Function, basePointer int, args []types.Object) *Frame {
	// Create bytecode.FunctionConstant from types.Function
	bcFn := &bytecode.FunctionConstant{
		Name:          fn.Name,
		NumLocals:     fn.NumLocals,
		NumParameters: fn.NumParameters,
		IsVariadic:    fn.IsVariadic,
		Instructions:  fn.Instructions,
	}

	frame := NewFrame(bcFn, basePointer)

	// Copy function's constant pool for access during execution
	// Set up arguments as initial locals
	for i := 0; i < len(args) && i < len(frame.locals); i++ {
		frame.locals[i] = args[i]
	}

	return frame
}
