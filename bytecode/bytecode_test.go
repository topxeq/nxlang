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
