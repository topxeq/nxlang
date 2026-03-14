package bytecode

import (
	"bytes"
	"testing"
)

func TestMagicNumber(t *testing.T) {
	if Magic != 0x4E584230 {
		t.Errorf("Expected Magic 0x4E584230, got 0x%x", Magic)
	}
}

func TestVersion(t *testing.T) {
	if VersionMajor != 1 {
		t.Errorf("Expected VersionMajor 1, got %d", VersionMajor)
	}
	if VersionMinor != 0 {
		t.Errorf("Expected VersionMinor 0, got %d", VersionMinor)
	}
	if VersionPatch != 0 {
		t.Errorf("Expected VersionPatch 0, got %d", VersionPatch)
	}
	if FullVersion != 0x010000 {
		t.Errorf("Expected FullVersion 0x010000, got 0x%x", FullVersion)
	}
}

func TestNilConstant(t *testing.T) {
	c := NilConstant{}
	if c.Type() != ConstNil {
		t.Errorf("Expected type ConstNil, got %v", c.Type())
	}
}

func TestBoolConstant(t *testing.T) {
	c := BoolConstant{Value: true}
	if c.Type() != ConstBool {
		t.Errorf("Expected type ConstBool, got %v", c.Type())
	}
	if c.Value != true {
		t.Errorf("Expected value true, got %v", c.Value)
	}
}

func TestIntConstant(t *testing.T) {
	c := IntConstant{Value: 42}
	if c.Type() != ConstInt {
		t.Errorf("Expected type ConstInt, got %v", c.Type())
	}
	if c.Value != 42 {
		t.Errorf("Expected value 42, got %d", c.Value)
	}
}

func TestFloatConstant(t *testing.T) {
	c := FloatConstant{Value: 3.14}
	if c.Type() != ConstFloat {
		t.Errorf("Expected type ConstFloat, got %v", c.Type())
	}
	if c.Value != 3.14 {
		t.Errorf("Expected value 3.14, got %f", c.Value)
	}
}

func TestStringConstant(t *testing.T) {
	c := StringConstant{Value: "hello"}
	if c.Type() != ConstString {
		t.Errorf("Expected type ConstString, got %v", c.Type())
	}
	if c.Value != "hello" {
		t.Errorf("Expected value 'hello', got '%s'", c.Value)
	}
}

func TestCharConstant(t *testing.T) {
	c := CharConstant{Value: 'A'}
	if c.Type() != ConstChar {
		t.Errorf("Expected type ConstChar, got %v", c.Type())
	}
	if c.Value != 'A' {
		t.Errorf("Expected value 'A', got '%c'", c.Value)
	}
}

func TestFunctionConstant(t *testing.T) {
	c := FunctionConstant{
		Name:           "test",
		Instructions:   []byte{0x01, 0x02, 0x03},
		NumLocals:      5,
		NumParameters:  2,
		IsVariadic:     false,
		IsStatic:       false,
		AccessModifier: 0,
		Flags:          0,
		DefaultValues:  []int{1, 2},
	}
	if c.Type() != ConstFunction {
		t.Errorf("Expected type ConstFunction, got %v", c.Type())
	}
	if c.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", c.Name)
	}
	if c.NumLocals != 5 {
		t.Errorf("Expected NumLocals 5, got %d", c.NumLocals)
	}
	if c.NumParameters != 2 {
		t.Errorf("Expected NumParameters 2, got %d", c.NumParameters)
	}
}

func TestClassConstant(t *testing.T) {
	c := ClassConstant{
		Name:          "TestClass",
		SuperClass:    "Object",
		Interfaces:    []string{"Interface1"},
		Methods:       map[string]int{"method1": 0},
		StaticMethods: map[string]int{"staticMethod": 1},
	}
	if c.Type() != ConstClass {
		t.Errorf("Expected type ConstClass, got %v", c.Type())
	}
	if c.Name != "TestClass" {
		t.Errorf("Expected name 'TestClass', got '%s'", c.Name)
	}
}

func TestInterfaceConstant(t *testing.T) {
	c := InterfaceConstant{
		Name:    "TestInterface",
		Methods: map[string][]string{"method1": []string{"param1"}},
	}
	if c.Type() != ConstInterface {
		t.Errorf("Expected type ConstInterface, got %v", c.Type())
	}
	if c.Name != "TestInterface" {
		t.Errorf("Expected name 'TestInterface', got '%s'", c.Name)
	}
}

func TestBytecode(t *testing.T) {
	bc := Bytecode{
		Constants: []Constant{
			&IntConstant{Value: 42},
			&StringConstant{Value: "hello"},
			&BoolConstant{Value: true},
		},
		MainFunc:   0,
		SourceFile: "test.nx",
		LineNumberTable: []LineInfo{
			{Offset: 0, Line: 1, Column: 0},
			{Offset: 5, Line: 2, Column: 0},
		},
	}

	if len(bc.Constants) != 3 {
		t.Errorf("Expected 3 constants, got %d", len(bc.Constants))
	}
	if bc.MainFunc != 0 {
		t.Errorf("Expected MainFunc 0, got %d", bc.MainFunc)
	}
	if bc.SourceFile != "test.nx" {
		t.Errorf("Expected SourceFile 'test.nx', got '%s'", bc.SourceFile)
	}
	if len(bc.LineNumberTable) != 2 {
		t.Errorf("Expected 2 line info entries, got %d", len(bc.LineNumberTable))
	}
}

func TestLineInfo(t *testing.T) {
	info := LineInfo{
		Offset: 10,
		Line:   5,
		Column: 3,
	}

	if info.Offset != 10 {
		t.Errorf("Expected Offset 10, got %d", info.Offset)
	}
	if info.Line != 5 {
		t.Errorf("Expected Line 5, got %d", info.Line)
	}
	if info.Column != 3 {
		t.Errorf("Expected Column 3, got %d", info.Column)
	}
}

func TestHeader(t *testing.T) {
	header := Header{
		Magic:      Magic,
		Version:    FullVersion,
		Flags:      0,
		ConstCount: 10,
		CodeSize:   100,
		LineCount:  5,
	}

	if header.Magic != Magic {
		t.Errorf("Expected Magic 0x%x, got 0x%x", Magic, header.Magic)
	}
	if header.Version != FullVersion {
		t.Errorf("Expected Version 0x%x, got 0x%x", FullVersion, header.Version)
	}
	if header.ConstCount != 10 {
		t.Errorf("Expected ConstCount 10, got %d", header.ConstCount)
	}
}

func TestWriterNewWriter(t *testing.T) {
	w := NewWriter()
	if w == nil {
		t.Error("Expected non-nil Writer")
	}
	if w.buf == nil {
		t.Error("Expected non-nil buffer")
	}
}

func TestReaderNewReader(t *testing.T) {
	r := NewReader(bytes.NewReader(nil))
	if r == nil {
		t.Error("Expected non-nil Reader")
	}
}

func TestConstantTypeValues(t *testing.T) {
	tests := []struct {
		ct       ConstantType
		expected byte
		name     string
	}{
		{ConstNil, 0x00, "ConstNil"},
		{ConstBool, 0x01, "ConstBool"},
		{ConstInt, 0x02, "ConstInt"},
		{ConstFloat, 0x03, "ConstFloat"},
		{ConstString, 0x04, "ConstString"},
		{ConstFunction, 0x05, "ConstFunction"},
		{ConstClass, 0x06, "ConstClass"},
		{ConstChar, 0x07, "ConstChar"},
		{ConstInterface, 0x08, "ConstInterface"},
	}

	for _, tt := range tests {
		if byte(tt.ct) != tt.expected {
			t.Errorf("Expected %s = 0x%x, got 0x%x", tt.name, tt.expected, byte(tt.ct))
		}
	}
}

func TestNewReaderFromBytes(t *testing.T) {
	data := []byte{0x00}
	r := NewReaderFromBytes(data)
	if r == nil {
		t.Error("Expected non-nil Reader")
	}
}

func TestNewReaderFromBytesEmpty(t *testing.T) {
	data := []byte{}
	r := NewReaderFromBytes(data)
	if r == nil {
		t.Error("Expected non-nil Reader")
	}
}

func TestWriterBytes(t *testing.T) {
	w := NewWriter()
	// Bytes() returns nil when nothing is written yet, which is expected
	// After writing, it should return non-nil
	bc := &Bytecode{
		Constants: []Constant{&IntConstant{Value: 42}},
		MainFunc:  0,
	}
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if w.Bytes() == nil {
		t.Error("Expected non-nil bytes after write")
	}
}

func TestWriteAndReadNilConstant(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&NilConstant{},
		},
		MainFunc: 0,
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(read.Constants) != 1 {
		t.Errorf("Expected 1 constant, got %d", len(read.Constants))
	}
	if read.Constants[0].Type() != ConstNil {
		t.Errorf("Expected ConstNil, got %v", read.Constants[0].Type())
	}
}

func TestWriteAndReadBoolConstant(t *testing.T) {
	tests := []bool{true, false}

	for _, val := range tests {
		bc := &Bytecode{
			Constants: []Constant{
				&BoolConstant{Value: val},
			},
			MainFunc: 0,
		}

		w := NewWriter()
		if err := w.Write(bc); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		r := NewReaderFromBytes(w.Bytes())
		read, err := r.Read()
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		c, ok := read.Constants[0].(*BoolConstant)
		if !ok {
			t.Fatalf("Expected *BoolConstant, got %T", read.Constants[0])
		}
		if c.Value != val {
			t.Errorf("Expected %v, got %v", val, c.Value)
		}
	}
}

func TestWriteAndReadIntConstant(t *testing.T) {
	tests := []int64{0, 42, -42, 9223372036854775807, -9223372036854775808}

	for _, val := range tests {
		bc := &Bytecode{
			Constants: []Constant{
				&IntConstant{Value: val},
			},
			MainFunc: 0,
		}

		w := NewWriter()
		if err := w.Write(bc); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		r := NewReaderFromBytes(w.Bytes())
		read, err := r.Read()
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		c, ok := read.Constants[0].(*IntConstant)
		if !ok {
			t.Fatalf("Expected *IntConstant, got %T", read.Constants[0])
		}
		if c.Value != val {
			t.Errorf("Expected %d, got %d", val, c.Value)
		}
	}
}

func TestWriteAndReadFloatConstant(t *testing.T) {
	tests := []float64{0.0, 3.14, -3.14, 1.7976931348623157e+308, -1.7976931348623157e+308}

	for _, val := range tests {
		bc := &Bytecode{
			Constants: []Constant{
				&FloatConstant{Value: val},
			},
			MainFunc: 0,
		}

		w := NewWriter()
		if err := w.Write(bc); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		r := NewReaderFromBytes(w.Bytes())
		read, err := r.Read()
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		c, ok := read.Constants[0].(*FloatConstant)
		if !ok {
			t.Fatalf("Expected *FloatConstant, got %T", read.Constants[0])
		}
		if c.Value != val {
			t.Errorf("Expected %f, got %f", val, c.Value)
		}
	}
}

func TestWriteAndReadStringConstant(t *testing.T) {
	tests := []string{"", "hello", "hello world", "你好世界", "🎉"}

	for _, val := range tests {
		bc := &Bytecode{
			Constants: []Constant{
				&StringConstant{Value: val},
			},
			MainFunc: 0,
		}

		w := NewWriter()
		if err := w.Write(bc); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		r := NewReaderFromBytes(w.Bytes())
		read, err := r.Read()
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		c, ok := read.Constants[0].(*StringConstant)
		if !ok {
			t.Fatalf("Expected *StringConstant, got %T", read.Constants[0])
		}
		if c.Value != val {
			t.Errorf("Expected %q, got %q", val, c.Value)
		}
	}
}

func TestWriteAndReadFunctionConstant(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&FunctionConstant{
				Name:           "testFunc",
				Instructions:   []byte{0x01, 0x02, 0x03},
				NumLocals:      5,
				NumParameters:  2,
				IsVariadic:     true,
				IsStatic:       true,
				AccessModifier: 1,
				Flags:          3,
				DefaultValues:  []int{0, 1},
			},
		},
		MainFunc: 0,
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	c, ok := read.Constants[0].(*FunctionConstant)
	if !ok {
		t.Fatalf("Expected *FunctionConstant, got %T", read.Constants[0])
	}
	if c.Name != "testFunc" {
		t.Errorf("Expected name 'testFunc', got %q", c.Name)
	}
	if len(c.Instructions) != 3 {
		t.Errorf("Expected 3 instructions, got %d", len(c.Instructions))
	}
	if c.NumLocals != 5 {
		t.Errorf("Expected NumLocals 5, got %d", c.NumLocals)
	}
	if c.NumParameters != 2 {
		t.Errorf("Expected NumParameters 2, got %d", c.NumParameters)
	}
	if !c.IsVariadic {
		t.Error("Expected IsVariadic to be true")
	}
	if !c.IsStatic {
		t.Error("Expected IsStatic to be true")
	}
	if c.AccessModifier != 1 {
		t.Errorf("Expected AccessModifier 1, got %d", c.AccessModifier)
	}
	if c.Flags != 3 {
		t.Errorf("Expected Flags 3, got %d", c.Flags)
	}
	if len(c.DefaultValues) != 2 {
		t.Errorf("Expected 2 DefaultValues, got %d", len(c.DefaultValues))
	}
}

func TestWriteAndReadClassConstant(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&ClassConstant{
				Name:          "TestClass",
				SuperClass:    "BaseClass",
				Interfaces:    []string{"I1", "I2"},
				Methods:       map[string]int{"method1": 0, "method2": 1},
				StaticMethods: map[string]int{"staticMethod": 2},
			},
		},
		MainFunc: 0,
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	c, ok := read.Constants[0].(*ClassConstant)
	if !ok {
		t.Fatalf("Expected *ClassConstant, got %T", read.Constants[0])
	}
	if c.Name != "TestClass" {
		t.Errorf("Expected name 'TestClass', got %q", c.Name)
	}
	if c.SuperClass != "BaseClass" {
		t.Errorf("Expected SuperClass 'BaseClass', got %q", c.SuperClass)
	}
	if len(c.Interfaces) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(c.Interfaces))
	}
	if len(c.Methods) != 2 {
		t.Errorf("Expected 2 methods, got %d", len(c.Methods))
	}
	if len(c.StaticMethods) != 1 {
		t.Errorf("Expected 1 static method, got %d", len(c.StaticMethods))
	}
}

func TestWriteAndReadInterfaceConstant(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&InterfaceConstant{
				Name:    "TestInterface",
				Methods: map[string][]string{"method1": {"a", "b"}, "method2": {}},
			},
		},
		MainFunc: 0,
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	c, ok := read.Constants[0].(*InterfaceConstant)
	if !ok {
		t.Fatalf("Expected *InterfaceConstant, got %T", read.Constants[0])
	}
	if c.Name != "TestInterface" {
		t.Errorf("Expected name 'TestInterface', got %q", c.Name)
	}
	if len(c.Methods) != 2 {
		t.Errorf("Expected 2 methods, got %d", len(c.Methods))
	}
}

func TestWriteAndReadBytecodeWithSourceFile(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&IntConstant{Value: 42},
		},
		MainFunc:   0,
		SourceFile: "test.nx",
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if read.SourceFile != "test.nx" {
		t.Errorf("Expected SourceFile 'test.nx', got %q", read.SourceFile)
	}
}

func TestWriteAndReadBytecodeWithLineNumberTable(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&IntConstant{Value: 42},
		},
		MainFunc: 0,
		LineNumberTable: []LineInfo{
			{Offset: 0, Line: 1, Column: 0},
			{Offset: 5, Line: 2, Column: 4},
			{Offset: 10, Line: 3, Column: 8},
		},
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(read.LineNumberTable) != 3 {
		t.Errorf("Expected 3 line info entries, got %d", len(read.LineNumberTable))
	}
	for i, info := range read.LineNumberTable {
		expected := bc.LineNumberTable[i]
		if info.Offset != expected.Offset || info.Line != expected.Line || info.Column != expected.Column {
			t.Errorf("LineInfo[%d] mismatch: expected %+v, got %+v", i, expected, info)
		}
	}
}

func TestWriteAndReadComplexBytecode(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{
			&IntConstant{Value: 42},
			&FloatConstant{Value: 3.14},
			&StringConstant{Value: "hello"},
			&BoolConstant{Value: true},
			&NilConstant{},
			&FunctionConstant{
				Name:          "main",
				Instructions:  []byte{0x01, 0x00, 0x00, 0x00},
				NumLocals:     1,
				NumParameters: 0,
			},
		},
		MainFunc:   5,
		SourceFile: "complex.nx",
		LineNumberTable: []LineInfo{
			{Offset: 0, Line: 1, Column: 0},
		},
	}

	w := NewWriter()
	if err := w.Write(bc); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	r := NewReaderFromBytes(w.Bytes())
	read, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(read.Constants) != 6 {
		t.Errorf("Expected 6 constants, got %d", len(read.Constants))
	}
	if read.MainFunc != 5 {
		t.Errorf("Expected MainFunc 5, got %d", read.MainFunc)
	}
}

func TestReadInvalidMagic(t *testing.T) {
	// Create a valid bytecode then corrupt the magic number
	bc := &Bytecode{
		Constants: []Constant{&IntConstant{Value: 42}},
		MainFunc:  0,
	}

	w := NewWriter()
	w.Write(bc)
	data := w.Bytes()

	// Corrupt magic number
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x00
	data[3] = 0x00

	r := NewReaderFromBytes(data)
	_, err := r.Read()
	if err == nil {
		t.Error("Expected error for invalid magic number")
	}
}

func TestReadInvalidVersion(t *testing.T) {
	// Create bytecode with wrong major version
	bc := &Bytecode{
		Constants: []Constant{&IntConstant{Value: 42}},
		MainFunc:  0,
	}

	w := NewWriter()
	w.Write(bc)
	data := w.Bytes()

	// The version is stored as a uint32 at offset 4-7 (after magic) in little-endian
	// FullVersion = VersionMajor << 16 = 0x00010000 for version 1.0.0
	// In little-endian: data[4]=0x00, data[5]=0x00, data[6]=0x01, data[7]=0x00
	// data[6] contains the major version
	data[6] = 0x02 // Change major version to 2

	r := NewReaderFromBytes(data)
	_, err := r.Read()
	if err == nil {
		t.Error("Expected error for incompatible version")
	}
}

func TestReadEmptyData(t *testing.T) {
	r := NewReaderFromBytes([]byte{})
	_, err := r.Read()
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

func TestReadTruncatedData(t *testing.T) {
	// Create valid bytecode then truncate it
	bc := &Bytecode{
		Constants: []Constant{&IntConstant{Value: 42}},
		MainFunc:  0,
	}

	w := NewWriter()
	w.Write(bc)
	data := w.Bytes()

	// Truncate to just header
	truncated := data[:16]

	r := NewReaderFromBytes(truncated)
	_, err := r.Read()
	if err == nil {
		t.Error("Expected error for truncated data")
	}
}

func TestWriteInvalidConstantType(t *testing.T) {
	// Create an invalid constant type by using an unknown type code
	// We'll create a custom constant that returns an invalid type
	bc := &Bytecode{
		Constants: []Constant{&invalidConstant{}},
		MainFunc:  0,
	}

	w := NewWriter()
	err := w.Write(bc)
	if err == nil {
		t.Error("Expected error for invalid constant type")
	}
}

// invalidConstant is a test helper that returns an invalid constant type
type invalidConstant struct{}

func (c *invalidConstant) Type() ConstantType { return ConstantType(0xFF) }

func TestWriteTo(t *testing.T) {
	bc := &Bytecode{
		Constants: []Constant{&IntConstant{Value: 42}},
		MainFunc:  0,
	}

	w := NewWriter()
	w.Write(bc)

	var buf bytes.Buffer
	n, err := w.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if n == 0 {
		t.Error("Expected non-zero bytes written")
	}
}

func TestErrInvalidConstantType(t *testing.T) {
	err := &ErrInvalidConstantType{Type: ConstantType(0xFF)}
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
	expected := "invalid constant type: " + string(ConstantType(0xFF))
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}
