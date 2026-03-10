# Exception Handling System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full exception handling system for Nxlang including try/catch/finally, defer statements, custom error types, and call stack information collection.

**Architecture:** Use stack-based exception handling table with per-frame defer linked lists. Compile-time generation of exception handler metadata, runtime VM stack unwinding with automatic defer execution and exception handler lookup.

**Tech Stack:** Go 1.22+, Nxlang parser, compiler, virtual machine, type system.

---

### Task 1: Add new bytecode instructions for exception handling

**Files:**
- Modify: `compiler/opcodes.go`

**Step 1: Add new opcode definitions**

```go
// Control flow
OpJmp          Opcode = 0x60 // Unconditional jump: OpJmp <offset>
OpJmpIfTrue    Opcode = 0x61 // Jump if top value is true: OpJmpIfTrue <offset>
OpJmpIfFalse   Opcode = 0x62 // Jump if top value is false: OpJmpIfFalse <offset>
OpCall         Opcode = 0x63 // Call function: OpCall <arg_count>
OpReturn       Opcode = 0x64 // Return from function with value
OpReturnVoid   Opcode = 0x65 // Return from function without value
OpClosure      Opcode = 0x66 // Create closure: OpClosure <func_index> <upvalue_count> <upvalue1> ... <upvalueN>
OpEnterTry     Opcode = 0x67 // Enter try block: OpEnterTry <catch_offset> <finally_offset>
OpExitTry      Opcode = 0x68 // Exit try block
OpDefer        Opcode = 0x69 // Register defer call: OpDefer <func_index> <arg_count>
```

**Step 2: Add opcode metadata to OpcodeTable**

```go
OpEnterTry:     {"ENTER_TRY", 4, 0, 0}, // 2-byte catch offset + 2-byte finally offset
OpExitTry:      {"EXIT_TRY", 0, 0, 0},
OpDefer:        {"DEFER", 3, 0, 0}, // 2-byte func index + 1-byte arg count
```

**Step 3: Verify compilation**

Run: `go build ./compiler`
Expected: No compilation errors

---

### Task 2: Extend VM frame structure to support defer and exception handling

**Files:**
- Modify: `vm/frame.go` (create if not exists)
- Modify: `vm/vm.go`

**Step 1: Create DeferCall structure**

```go
package vm

import "github.com/topxeq/nxlang/bytecode"

// DeferCall represents a deferred function call
type DeferCall struct {
	Function *bytecode.FunctionConstant
	Args     []interface{}
	PC       int // Program counter where defer was registered
}

// ExceptionHandler represents an active try/catch/finally handler
type ExceptionHandler struct {
	TryStart      int // Start offset of try block
	TryEnd        int // End offset of try block
	CatchOffset   int // Offset of catch block
	FinallyOffset int // Offset of finally block
	HasCatch      bool // Whether there is a catch block
	HasFinally    bool // Whether there is a finally block
}
```

**Step 2: Extend Frame structure**

```go
type Frame struct {
	fn           *bytecode.FunctionConstant
	pc           int // Program counter
	basePointer  int // Base pointer on stack
	localCount   int // Number of local variables
	deferStack   []*DeferCall // Stack of deferred calls
	handlerStack []*ExceptionHandler // Stack of active exception handlers
}
```

**Step 3: Update NewFrame constructor**

```go
func NewFrame(fn *bytecode.FunctionConstant, basePointer int) *Frame {
	return &Frame{
		fn:          fn,
		pc:          0,
		basePointer: basePointer,
		localCount:  fn.NumLocals,
		deferStack:  make([]*DeferCall, 0),
		handlerStack: make([]*ExceptionHandler, 0),
	}
}
```

**Step 4: Verify compilation**

Run: `go build ./vm`
Expected: No compilation errors

---

### Task 3: Implement exception throwing and basic VM error handling

**Files:**
- Modify: `vm/vm.go`
- Modify: `types/error.go`

**Step 1: Extend Error type with call stack support**

```go
// Error represents a runtime error
type Error struct {
	Message  string
	Code     int
	Stack    []*StackFrame
	Metadata map[string]interface{}
}

// StackFrame represents a single frame in the call stack
type StackFrame struct {
	FunctionName string
	Line         int
	Column       int
	SourceFile   string
}
```

**Step 2: Implement OpThrow handling in VM run loop**

Add this case to the VM instruction switch:

```go
case compiler.OpThrow:
	errObj := vm.stack.Pop()
	err, ok := errObj.(*types.Error)
	if !ok {
		// Convert to error object
		err = types.NewError(errObj.ToStr(), 0, vm.currentFrame().pc, "")
	}
	// Collect call stack
	err.Stack = vm.collectCallStack()
	vm.lastError = err
	// Begin unwinding
	return vm.unwindException(err)
```

**Step 3: Implement collectCallStack stub method**

```go
func (vm *VM) collectCallStack() []*types.StackFrame {
	stack := make([]*types.StackFrame, 0)
	for i := vm.framePointer - 1; i >= 0; i-- {
		frame := vm.frames[i]
		stack = append(stack, &types.StackFrame{
			FunctionName: frame.fn.Name,
			Line:         vm.getLineNumber(frame, frame.pc),
		})
	}
	return stack
}
```

**Step 4: Implement unwindException stub method**

```go
func (vm *VM) unwindException(err *types.Error) error {
	// Unwind frames until we find an exception handler
	for vm.framePointer > 0 {
		frame := vm.currentFrame()

		// Execute all defers in current frame
		if err := vm.runDefers(frame); err != nil {
			return err
		}

		// Look for exception handler
		for i := len(frame.handlerStack) - 1; i >= 0; i-- {
			handler := frame.handlerStack[i]
			if frame.pc >= handler.TryStart && frame.pc <= handler.TryEnd {
				// Found handler
				if handler.HasCatch {
					// Push error to stack
					vm.stack.Push(err)
					// Jump to catch block
					frame.pc = handler.CatchOffset
					return nil
				}
				if handler.HasFinally {
					// Run finally then rethrow
					frame.pc = handler.FinallyOffset
					return nil
				}
			}
		}

		// No handler in this frame, pop it
		vm.framePointer--
	}

	// Unhandled exception
	return fmt.Errorf("unhandled exception: %s\nStack:\n%s", err.Message, formatStack(err.Stack))
}
```

**Step 5: Verify compilation**

Run: `go build ./vm`
Expected: No compilation errors

---

### Task 4: Implement try/catch/finally compilation in compiler

**Files:**
- Modify: `compiler/compiler.go`

**Step 1: Add TryStatement compilation case**

Add to the Compile method switch:

```go
case *parser.TryStatement:
	// Enter try block
	tryStart := len(c.currentInstructions())
	catchOffsetPlaceholder := c.emit(compiler.OpEnterTry, 0, 0) // Placeholder for offsets
	finallyOffsetPlaceholder := catchOffsetPlaceholder + 2

	// Compile try block
	if err := c.Compile(n.TryBlock); err != nil {
		return err
	}

	// Exit try block normally
	c.emit(compiler.OpExitTry)
	afterTryOffset := len(c.currentInstructions())

	// Compile catch block
	var catchOffset int
	if n.Catch != nil {
		catchOffset = len(c.currentInstructions())
		// Catch block receives error as parameter
		if n.Catch.Param != nil {
			// Store error in catch parameter
			symbol := c.symbolTable.Define(n.Catch.Param.Value)
			c.emit(compiler.OpStoreLocal, symbol.Index)
		} else {
			// Discard error
			c.emit(compiler.OpPop)
		}
		if err := c.Compile(n.Catch.CatchBlock); err != nil {
			return err
		}
	}
	afterCatchOffset := len(c.currentInstructions())

	// Compile finally block
	var finallyOffset int
	if n.Finally != nil {
		finallyOffset = len(c.currentInstructions())
		if err := c.Compile(n.Finally.FinallyBlock); err != nil {
			return err
		}
	}
	afterFinallyOffset := len(c.currentInstructions())

	// Patch offsets
	c.patchOperand(catchOffsetPlaceholder, catchOffset)
	c.patchOperand(finallyOffsetPlaceholder, finallyOffset)

	return nil
```

**Step 2: Implement patchOperand helper method**

```go
// patchOperand patches the operand at the given offset
func (c *Compiler) patchOperand(offset int, value int) {
	instructions := c.currentInstructions()
	// 16-bit little-endian
	instructions[offset] = byte(value & 0xff)
	instructions[offset+1] = byte((value >> 8) & 0xff)
}
```

**Step 3: Verify compilation**

Run: `go build ./compiler`
Expected: No compilation errors

---

### Task 5: Implement defer statement compilation in compiler

**Files:**
- Modify: `compiler/compiler.go`

**Step 1: Add DeferStatement compilation case**

Add to the Compile method switch:

```go
case *parser.DeferStatement:
	// Compile the call expression
	callExpr := n.Call
	if err := c.Compile(callExpr.Function); err != nil {
		return err
	}
	// Compile arguments
	for _, arg := range callExpr.Arguments {
		if err := c.Compile(arg); err != nil {
			return err
		}
	}
	// Emit defer instruction
	funcConst := c.constants[len(c.constants)-1-len(callExpr.Arguments)].(*bytecode.FunctionConstant)
	funcIdx := c.addConstant(funcConst)
	c.emit(compiler.OpDefer, funcIdx, len(callExpr.Arguments))
	// Pop function and args from stack (they are stored in defer record)
	for i := 0; i < len(callExpr.Arguments) + 1; i++ {
		c.emit(compiler.OpPop)
	}
	return nil
```

**Step 2: Verify compilation**

Run: `go build ./compiler`
Expected: No compilation errors

---

### Task 6: Implement VM exception handling flow (try/catch/finally execution)

**Files:**
- Modify: `vm/vm.go`

**Step 1: Implement OpEnterTry handling**

Add to VM instruction switch:

```go
case compiler.OpEnterTry:
	catchOffset := int(code[ip+1]) | (int(code[ip+2]) << 8)
	finallyOffset := int(code[ip+3]) | (int(code[ip+4]) << 8)
	ip += 4

	handler := &ExceptionHandler{
		TryStart:      ip,
		TryEnd:        0, // Will be set by OpExitTry
		CatchOffset:   catchOffset,
		FinallyOffset: finallyOffset,
		HasCatch:      catchOffset != 0,
		HasFinally:    finallyOffset != 0,
	}
	frame.handlerStack = append(frame.handlerStack, handler)
```

**Step 2: Implement OpExitTry handling**

```go
case compiler.OpExitTry:
	// Pop handler from stack
	if len(frame.handlerStack) > 0 {
		handler := frame.handlerStack[len(frame.handlerStack)-1]
		handler.TryEnd = ip
		frame.handlerStack = frame.handlerStack[:len(frame.handlerStack)-1]
	}
```

**Step 3: Complete unwindException implementation**

Add finally block execution logic, ensure proper rethrowing after finally.

**Step 4: Verify compilation**

Run: `go build ./vm`
Expected: No compilation errors

---

### Task 7: Implement defer execution logic in VM

**Files:**
- Modify: `vm/vm.go`

**Step 1: Implement OpDefer handling**

Add to VM instruction switch:

```go
case compiler.OpDefer:
	funcIdx := int(code[ip+1]) | (int(code[ip+2]) << 8)
	argCount := int(code[ip+3])
	ip += 3

	fn := vm.constants[funcIdx].(*bytecode.FunctionConstant)
	args := make([]interface{}, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = vm.stack.Pop()
	}
	// Pop function itself
	vm.stack.Pop()

	// Add to defer stack
	deferCall := &DeferCall{
		Function: fn,
		Args:     args,
		PC:       ip,
	}
	frame.deferStack = append(frame.deferStack, deferCall)
```

**Step 2: Implement runDefers method**

```go
func (vm *VM) runDefers(frame *Frame) error {
	// Execute defers in reverse order
	for i := len(frame.deferStack) - 1; i >= 0; i-- {
		deferCall := frame.deferStack[i]
		// Push args to stack
		for _, arg := range deferCall.Args {
			vm.stack.Push(arg)
		}
		// Push function
		vm.stack.Push(deferCall.Function)
		// Call the function
		if err := vm.callFunction(deferCall.Function, len(deferCall.Args)); err != nil {
			return err
		}
		// Run the function
		if err := vm.runFrame(frame); err != nil {
			return err
		}
	}
	// Clear defer stack
	frame.deferStack = nil
	return nil
}
```

**Step 3: Run defers on normal function return**

Add defer execution to OpReturn and OpReturnVoid handling.

**Step 4: Verify compilation**

Run: `go build ./vm`
Expected: No compilation errors

---

### Task 8: Extend Error type with custom fields and call stack support

**Files:**
- Modify: `types/error.go`

**Step 1: Implement full Error type with all methods**

```go
package types

import "fmt"

// Error represents a runtime error with call stack and metadata
type Error struct {
	Message  string
	Code     int
	Stack    []*StackFrame
	Metadata map[string]interface{}
}

// StackFrame represents a single entry in the call stack
type StackFrame struct {
	FunctionName string
	Line         int
	Column       int
	SourceFile   string
}

// NewError creates a new basic error
func NewError(message string, code int, line int, sourceFile string) *Error {
	return &Error{
		Message:  message,
		Code:     code,
		Stack:    []*StackFrame{},
		Metadata: make(map[string]interface{}),
	}
}

// NewCustomError creates a new error with custom metadata
func NewCustomError(message string, code int, metadata map[string]interface{}) *Error {
	return &Error{
		Message:  message,
		Code:     code,
		Stack:    []*StackFrame{},
		Metadata: metadata,
	}
}

// Type returns the type code
func (e *Error) Type() TypeCode {
	return TypeError
}

// TypeName returns the type name
func (e *Error) TypeName() string {
	return "error"
}

// ToStr converts the error to string
func (e *Error) ToStr() string {
	return fmt.Sprintf("Error: %s (code: %d)", e.Message, e.Code)
}

// FullString returns the error with stack trace
func (e *Error) FullString() string {
	str := e.ToStr() + "\nStack trace:\n"
	for i, frame := range e.Stack {
		str += fmt.Sprintf("  %d: %s at line %d\n", i+1, frame.FunctionName, frame.Line)
	}
	return str
}
```

**Step 2: Verify compilation**

Run: `go build ./types`
Expected: No compilation errors

---

### Task 9: Implement call stack collection functionality

**Files:**
- Modify: `vm/vm.go`

**Step 1: Implement getLineNumber method**

```go
func (vm *VM) getLineNumber(frame *Frame, pc int) int {
	// Use debug information from function constant
	if frame.fn.LineTable != nil && len(frame.fn.LineTable) > 0 {
		// Binary search for line number
		low, high := 0, len(frame.fn.LineTable)
		for low < high {
			mid := (low + high) / 2
			if frame.fn.LineTable[mid].Offset > pc {
				high = mid
			} else {
				low = mid + 1
			}
		}
		if low > 0 {
			return frame.fn.LineTable[low-1].Line
		}
	}
	return 0
}
```

**Step 2: Update collectCallStack with full information**

Add column and source file information collection.

**Step 3: Add debug line table generation to compiler**

Modify compiler to generate line number information for each instruction.

**Step 4: Verify compilation**

Run: `go build ./vm ./compiler`
Expected: No compilation errors

---

### Task 10: Add built-in functions for custom error creation

**Files:**
- Modify: `vm/builtins.go` (create if not exists)

**Step 1: Add error() built-in function**

```go
vm.globals["error"] = &types.NativeFunction{
	Fn: func(args ...types.Object) types.Object {
		if len(args) == 0 {
			return types.NewError("error() requires at least a message argument", 0, 0, "")
		}

		message := args[0].ToStr()
		code := 0
		metadata := make(map[string]interface{})

		if len(args) >= 2 {
			if codeObj, ok := args[1].(types.Int); ok {
				code = int(codeObj)
			}
		}

		if len(args) >= 3 {
			if mapObj, ok := args[2].(*collections.Map); ok {
				for k, v := range mapObj.Items() {
					metadata[k] = v
				}
			}
		}

		err := types.NewCustomError(message, code, metadata)
		// VM will fill in stack trace when thrown
		return err
	},
}
```

**Step 2: Add stackTrace() built-in function**

```go
vm.globals["stackTrace"] = &types.NativeFunction{
	Fn: func(args ...types.Object) types.Object {
		stack := vm.collectCallStack()
		arr := collections.NewArray()
		for _, frame := range stack {
			frameMap := collections.NewMap()
			frameMap.Set("function", types.String(frame.FunctionName))
			frameMap.Set("line", types.Int(frame.Line))
			frameMap.Set("column", types.Int(frame.Column))
			frameMap.Set("file", types.String(frame.SourceFile))
			arr.Append(frameMap)
		}
		return arr
	},
}
```

**Step 3: Verify compilation**

Run: `go build ./vm`
Expected: No compilation errors

---

### Task 11: Comprehensive testing of all exception handling features

**Files:**
- Create: `test_exception_handling.nx`

**Step 1: Write comprehensive test script**

```nx
pln("=== Testing Exception Handling System ===")

// Test 1: Basic try/catch
pln("\nTest 1: Basic try/catch")
try {
    pln("Inside try block")
    throw error("Test error", 1001)
    pln("This should not execute")
} catch (e) {
    pln("Caught error:", e.Message)
    pln("Error code:", e.Code)
}

// Test 2: try/catch/finally
pln("\nTest 2: try/catch/finally")
let finallyExecuted = false
try {
    throw error("Test with finally")
} catch (e) {
    pln("Caught error in test 2")
} finally {
    pln("Finally block executed")
    finallyExecuted = true
}
pln("finallyExecuted:", finallyExecuted)

// Test 3: try with finally only
pln("\nTest 3: try with finally only")
let finallyRun = false
try {
    pln("Try block without catch")
} finally {
    pln("Finally runs even without catch")
    finallyRun = true
}
pln("finallyRun:", finallyRun)

// Test 4: defer statement
pln("\nTest 4: defer statement")
let deferExecuted = false
func testDefer() {
    defer {
        pln("Defer executed")
        deferExecuted = true
    }
    pln("Inside testDefer function")
    return "value"
}
result = testDefer()
pln("Function returned:", result)
pln("deferExecuted:", deferExecuted)

// Test 5: defer with arguments
pln("\nTest 5: defer with arguments")
let deferValue = 0
func testDeferArgs() {
    let x = 10
    defer pln("Deferred value:", x)
    x = 20
    pln("x inside function:", x)
}
testDeferArgs()

// Test 6: Multiple defers execute in reverse order
pln("\nTest 6: Multiple defers")
func testMultipleDefers() {
    defer pln("Defer 1")
    defer pln("Defer 2")
    defer pln("Defer 3")
    pln("Inside function")
}
testMultipleDefers()

// Test 7: Nested try/catch
pln("\nTest 7: Nested try/catch")
try {
    try {
        throw error("Nested error")
    } catch (e) {
        pln("Inner catch caught:", e.Message)
        throw error("Rethrown error", 2001)
    }
} catch (e) {
    pln("Outer catch caught:", e.Message)
}

// Test 8: Custom error metadata
pln("\nTest 8: Custom error metadata")
try {
    throw error("Validation failed", 400, {"field": "email", "value": "invalid-email"})
} catch (e) {
    pln("Error field:", e.metadata.field)
    pln("Error value:", e.metadata.value)
}

// Test 9: Defer executes even when exception is thrown
pln("\nTest 9: Defer on exception")
let deferOnException = false
func testDeferException() {
    defer {
        pln("Defer runs even after exception")
        deferOnException = true
    }
    throw error("Exception in function")
}
try {
    testDeferException()
} catch (e) {
    pln("Caught exception from function")
}
pln("deferOnException:", deferOnException)

// Test 10: Call stack collection
pln("\nTest 10: Call stack")
func funcC() {
    throw error("Deep error")
}
func funcB() {
    funcC()
}
func funcA() {
    funcB()
}
try {
    funcA()
} catch (e) {
    pln("Call stack length:", len(e.stack))
    for i, frame in e.stack {
        pln("Frame", i, ":", frame.function, "line", frame.line)
    }
}

pln("\n✅ All exception handling tests completed")
```

**Step 2: Run the test**

Run: `go build -o nx ./cmd/nx && ./nx test_exception_handling.nx`
Expected: All tests pass, output matches expected behavior

---

### Task 12: Documentation and final verification

**Files:**
- Update: `README.md`
- Update: `docs/language-spec.md` (create if needed)

**Step 1: Add exception handling documentation to language spec**

Document try/catch/finally syntax, defer syntax, error creation, and call stack format.

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: All existing tests still pass

**Step 3: Build final binary**

Run: `go build -o nx ./cmd/nx`
Expected: Build successful
