package vm

import (
	"testing"

	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/types"
)

func TestStackNew(t *testing.T) {
	s := NewStack()
	if s == nil {
		t.Fatal("Expected non-nil stack")
	}
	if s.Size() != 0 {
		t.Errorf("Expected empty stack, got size %d", s.Size())
	}
}

func TestStackPushPop(t *testing.T) {
	s := NewStack()

	// Push elements
	if err := s.Push(types.Int(1)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := s.Push(types.Int(2)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := s.Push(types.Int(3)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if s.Size() != 3 {
		t.Errorf("Expected size 3, got %d", s.Size())
	}

	// Pop elements
	val := s.Pop()
	if !val.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", val)
	}

	val = s.Pop()
	if !val.Equals(types.Int(2)) {
		t.Errorf("Expected 2, got %v", val)
	}

	val = s.Pop()
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}

	if s.Size() != 0 {
		t.Errorf("Expected empty stack, got size %d", s.Size())
	}
}

func TestStackPeek(t *testing.T) {
	s := NewStack()
	s.Push(types.Int(1))
	s.Push(types.Int(2))

	val := s.Peek()
	if !val.Equals(types.Int(2)) {
		t.Errorf("Expected 2, got %v", val)
	}

	// Peek should not remove the element
	if s.Size() != 2 {
		t.Errorf("Expected size 2 after peek, got %d", s.Size())
	}
}

func TestStackPeekN(t *testing.T) {
	s := NewStack()
	s.Push(types.Int(1))
	s.Push(types.Int(2))
	s.Push(types.Int(3))

	// PeekN(0) should return top
	val := s.PeekN(0)
	if !val.Equals(types.Int(3)) {
		t.Errorf("PeekN(0) expected 3, got %v", val)
	}

	// PeekN(1) should return second from top
	val = s.PeekN(1)
	if !val.Equals(types.Int(2)) {
		t.Errorf("PeekN(1) expected 2, got %v", val)
	}

	// PeekN(2) should return third from top
	val = s.PeekN(2)
	if !val.Equals(types.Int(1)) {
		t.Errorf("PeekN(2) expected 1, got %v", val)
	}
}

func TestStackPopN(t *testing.T) {
	s := NewStack()
	s.Push(types.Int(1))
	s.Push(types.Int(2))
	s.Push(types.Int(3))

	elements := s.PopN(2)
	if len(elements) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(elements))
	}
	// PopN returns in correct order (not reversed)
	if !elements[0].Equals(types.Int(2)) {
		t.Errorf("Expected 2, got %v", elements[0])
	}
	if !elements[1].Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", elements[1])
	}
}

func TestStackClear(t *testing.T) {
	s := NewStack()
	s.Push(types.Int(1))
	s.Push(types.Int(2))
	s.Clear()

	if s.Size() != 0 {
		t.Errorf("Expected empty stack after clear, got size %d", s.Size())
	}
}

func TestStackSetSize(t *testing.T) {
	s := NewStack()
	s.Push(types.Int(1))
	s.Push(types.Int(2))
	s.Push(types.Int(3))

	s.SetSize(1)
	if s.Size() != 1 {
		t.Errorf("Expected size 1, got %d", s.Size())
	}

	// Test negative size
	s.SetSize(-1)
	if s.Size() != 0 {
		t.Errorf("Expected size 0 for negative input, got %d", s.Size())
	}
}

func TestStackPopSafe(t *testing.T) {
	s := NewStack()

	// Pop from empty stack
	_, err := s.PopSafe()
	if err == nil {
		t.Error("Expected error for pop from empty stack")
	}

	s.Push(types.Int(1))
	val, err := s.PopSafe()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}
}

func TestStackPeekSafe(t *testing.T) {
	s := NewStack()

	// Peek from empty stack
	_, err := s.PeekSafe()
	if err == nil {
		t.Error("Expected error for peek from empty stack")
	}

	s.Push(types.Int(1))
	val, err := s.PeekSafe()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}
}

func TestStackPeekNSafe(t *testing.T) {
	s := NewStack()

	// PeekN from empty stack
	_, err := s.PeekNSafe(0)
	if err == nil {
		t.Error("Expected error for peek from empty stack")
	}

	s.Push(types.Int(1))
	val, err := s.PeekNSafe(0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}

	// PeekN beyond stack size
	_, err = s.PeekNSafe(5)
	if err == nil {
		t.Error("Expected error for peek beyond stack size")
	}
}

func TestStackErrors(t *testing.T) {
	overflow := &StackOverflowError{}
	if overflow.Error() != "stack overflow" {
		t.Errorf("Expected 'stack overflow', got %q", overflow.Error())
	}

	underflow := &StackUnderflowError{}
	if underflow.Error() != "stack underflow" {
		t.Errorf("Expected 'stack underflow', got %q", underflow.Error())
	}
}

func TestFrameNew(t *testing.T) {
	fn := &bytecode.FunctionConstant{
		Name:          "test",
		Instructions:  []byte{0x01, 0x00, 0x00, 0x00},
		NumLocals:     2,
		NumParameters: 1,
	}

	f := NewFrame(fn, 0)
	if f == nil {
		t.Fatal("Expected non-nil frame")
	}
	if f.Name() != "test" {
		t.Errorf("Expected name 'test', got %q", f.Name())
	}
	if len(f.Instructions()) != 4 {
		t.Errorf("Expected 4 instruction bytes, got %d", len(f.Instructions()))
	}
}

func TestFrameReadUint8(t *testing.T) {
	fn := &bytecode.FunctionConstant{
		Instructions: []byte{0x01, 0x02, 0x03},
	}

	f := NewFrame(fn, 0)

	val := f.ReadUint8()
	if val != 0x01 {
		t.Errorf("Expected 0x01, got 0x%02x", val)
	}

	val = f.ReadUint8()
	if val != 0x02 {
		t.Errorf("Expected 0x02, got 0x%02x", val)
	}
}

func TestFrameReadUint16(t *testing.T) {
	fn := &bytecode.FunctionConstant{
		Instructions: []byte{0x01, 0x02, 0x03, 0x04},
	}

	f := NewFrame(fn, 0)

	val := f.ReadUint16()
	if val != 0x0102 {
		t.Errorf("Expected 0x0102, got 0x%04x", val)
	}

	val = f.ReadUint16()
	if val != 0x0304 {
		t.Errorf("Expected 0x0304, got 0x%04x", val)
	}
}

func TestFrameCurrentInstruction(t *testing.T) {
	fn := &bytecode.FunctionConstant{
		Instructions: []byte{0x01, 0x02, 0x03},
	}

	f := NewFrame(fn, 0)

	if f.CurrentInstruction() != 0x01 {
		t.Errorf("Expected 0x01, got 0x%02x", f.CurrentInstruction())
	}

	f.ReadUint8()
	if f.CurrentInstruction() != 0x02 {
		t.Errorf("Expected 0x02, got 0x%02x", f.CurrentInstruction())
	}
}

func TestNewFrameFromFunction(t *testing.T) {
	fn := &types.Function{
		Name:          "testFunc",
		Instructions:  []byte{0x01, 0x02},
		NumLocals:     3,
		NumParameters: 2,
	}

	args := []types.Object{types.Int(10), types.Int(20)}
	f := NewFrameFromFunction(fn, 0, args)

	if f == nil {
		t.Fatal("Expected non-nil frame")
	}
	if f.Name() != "testFunc" {
		t.Errorf("Expected name 'testFunc', got %q", f.Name())
	}
	if len(f.locals) < 2 {
		t.Errorf("Expected at least 2 locals, got %d", len(f.locals))
	}
	if !f.locals[0].Equals(types.Int(10)) {
		t.Errorf("Expected local[0] = 10, got %v", f.locals[0])
	}
	if !f.locals[1].Equals(types.Int(20)) {
		t.Errorf("Expected local[1] = 20, got %v", f.locals[1])
	}
}

func TestVMAddObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	tests := []struct {
		a, b     types.Object
		expected types.Object
	}{
		{types.Int(1), types.Int(2), types.Int(3)},
		{types.Int(1), types.Float(2.5), types.Float(3.5)},
		{types.Float(1.5), types.Int(2), types.Float(3.5)},
		{types.Float(1.5), types.Float(2.5), types.Float(4.0)},
		{types.String("hello"), types.String(" world"), types.String("hello world")},
		{types.String("value: "), types.Int(42), types.String("value: 42")},
		{types.Int(42), types.String(" items"), types.String("42 items")},
	}

	for _, tt := range tests {
		result, err := vm.addObjects(tt.a, tt.b)
		if err != nil {
			t.Errorf("addObjects(%v, %v) error: %v", tt.a, tt.b, err)
			continue
		}
		if !result.Equals(tt.expected) {
			t.Errorf("addObjects(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestVMSubObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	tests := []struct {
		a, b     types.Object
		expected types.Object
	}{
		{types.Int(5), types.Int(3), types.Int(2)},
		{types.Int(5), types.Float(2.5), types.Float(2.5)},
		{types.Float(5.5), types.Int(2), types.Float(3.5)},
		{types.Float(5.5), types.Float(2.5), types.Float(3.0)},
	}

	for _, tt := range tests {
		result, err := vm.subObjects(tt.a, tt.b)
		if err != nil {
			t.Errorf("subObjects(%v, %v) error: %v", tt.a, tt.b, err)
			continue
		}
		if !result.Equals(tt.expected) {
			t.Errorf("subObjects(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestVMMulObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	tests := []struct {
		a, b     types.Object
		expected types.Object
	}{
		{types.Int(3), types.Int(4), types.Int(12)},
		{types.Int(3), types.Float(2.5), types.Float(7.5)},
		{types.Float(2.5), types.Int(4), types.Float(10.0)},
		{types.Float(2.5), types.Float(2.0), types.Float(5.0)},
		{types.String("ab"), types.Int(3), types.String("ababab")},
		{types.Int(3), types.String("x"), types.String("xxx")},
	}

	for _, tt := range tests {
		result, err := vm.mulObjects(tt.a, tt.b)
		if err != nil {
			t.Errorf("mulObjects(%v, %v) error: %v", tt.a, tt.b, err)
			continue
		}
		if !result.Equals(tt.expected) {
			t.Errorf("mulObjects(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestVMDivObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	tests := []struct {
		a, b     types.Object
		expected types.Object
	}{
		{types.Int(10), types.Int(2), types.Int(5)},
		{types.Int(7), types.Int(2), types.Float(3.5)},
		{types.Int(10), types.Float(4.0), types.Float(2.5)},
		{types.Float(10.0), types.Int(4), types.Float(2.5)},
		{types.Float(10.0), types.Float(4.0), types.Float(2.5)},
	}

	for _, tt := range tests {
		result, err := vm.divObjects(tt.a, tt.b, 0)
		if err != nil {
			t.Errorf("divObjects(%v, %v) error: %v", tt.a, tt.b, err)
			continue
		}
		if !result.Equals(tt.expected) {
			t.Errorf("divObjects(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}

	// Test division by zero
	_, err := vm.divObjects(types.Int(1), types.Int(0), 0)
	if err == nil {
		t.Error("Expected division by zero error")
	}
}

func TestVMModObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	tests := []struct {
		a, b     types.Object
		expected types.Object
	}{
		{types.Int(10), types.Int(3), types.Int(1)},
		{types.Int(15), types.Int(4), types.Int(3)},
		{types.Float(10.5), types.Float(3.0), types.Float(1.5)},
	}

	for _, tt := range tests {
		result, err := vm.modObjects(tt.a, tt.b, 0)
		if err != nil {
			t.Errorf("modObjects(%v, %v) error: %v", tt.a, tt.b, err)
			continue
		}
		if !result.Equals(tt.expected) {
			t.Errorf("modObjects(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}

	// Test modulo by zero
	_, err := vm.modObjects(types.Int(1), types.Int(0), 0)
	if err == nil {
		t.Error("Expected modulo by zero error")
	}
}

func TestVMBitwiseOperations(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	// AND
	result, err := vm.bitAndObjects(types.Int(0b1100), types.Int(0b1010))
	if err != nil || !result.Equals(types.Int(0b1000)) {
		t.Errorf("bitAndObjects error: %v, result: %v", err, result)
	}

	// OR
	result, err = vm.bitOrObjects(types.Int(0b1100), types.Int(0b1010))
	if err != nil || !result.Equals(types.Int(0b1110)) {
		t.Errorf("bitOrObjects error: %v, result: %v", err, result)
	}

	// XOR
	result, err = vm.bitXorObjects(types.Int(0b1100), types.Int(0b1010))
	if err != nil || !result.Equals(types.Int(0b0110)) {
		t.Errorf("bitXorObjects error: %v, result: %v", err, result)
	}

	// Shift left
	result, err = vm.shiftLeftObjects(types.Int(4), types.Int(2))
	if err != nil || !result.Equals(types.Int(16)) {
		t.Errorf("shiftLeftObjects error: %v, result: %v", err, result)
	}

	// Shift right
	result, err = vm.shiftRightObjects(types.Int(16), types.Int(2))
	if err != nil || !result.Equals(types.Int(4)) {
		t.Errorf("shiftRightObjects error: %v, result: %v", err, result)
	}

	// NOT
	result, err = vm.bitNotObject(types.Int(0), 0)
	if err != nil {
		t.Errorf("bitNotObject error: %v", err)
	}
}

func TestVMNegObject(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	result, err := vm.negObject(types.Int(5), 0)
	if err != nil || !result.Equals(types.Int(-5)) {
		t.Errorf("negObject(Int(5)) error: %v, result: %v", err, result)
	}

	result, err = vm.negObject(types.Float(3.14), 0)
	if err != nil || !result.Equals(types.Float(-3.14)) {
		t.Errorf("negObject(Float(3.14)) error: %v, result: %v", err, result)
	}
}

func TestVMCompareObjects(t *testing.T) {
	vm := NewVM(&bytecode.Bytecode{
		Constants: []bytecode.Constant{
			&bytecode.FunctionConstant{Name: "main", Instructions: []byte{}},
		},
		MainFunc: 0,
	})

	// Int comparisons
	tests := []struct {
		a, b   types.Object
		op     int
		expect bool
	}{
		{types.Int(1), types.Int(2), lt, true},
		{types.Int(1), types.Int(1), lte, true},
		{types.Int(2), types.Int(1), gt, true},
		{types.Int(1), types.Int(1), gte, true},
		{types.Float(1.5), types.Float(2.5), lt, true},
		{types.String("a"), types.String("b"), lt, true},
		{types.Bool(false), types.Bool(true), lt, true},
	}

	for _, tt := range tests {
		result, err := vm.compareObjects(tt.a, tt.b, tt.op)
		if err != nil {
			t.Errorf("compareObjects error: %v", err)
			continue
		}
		if !result.Equals(types.Bool(tt.expect)) {
			t.Errorf("compareObjects(%v, %v, %d) = %v, expected %v", tt.a, tt.b, tt.op, result, tt.expect)
		}
	}
}

func TestOpToString(t *testing.T) {
	tests := []struct {
		op       int
		expected string
	}{
		{lt, "<"},
		{lte, "<="},
		{gt, ">"},
		{gte, ">="},
		{999, "??"},
	}

	for _, tt := range tests {
		result := opToString(tt.op)
		if result != tt.expected {
			t.Errorf("opToString(%d) = %q, expected %q", tt.op, result, tt.expected)
		}
	}
}