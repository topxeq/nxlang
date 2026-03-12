package types

import (
	"testing"
)

func TestErrorCreation(t *testing.T) {
	err := NewError("test error", 10, 5, "test.nx")
	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
	if err.Line != 10 {
		t.Errorf("Expected line 10, got %d", err.Line)
	}
	if err.Column != 5 {
		t.Errorf("Expected column 5, got %d", err.Column)
	}
	if err.Filename != "test.nx" {
		t.Errorf("Expected filename 'test.nx', got '%s'", err.Filename)
	}
	if err.ErrorType != "Error" {
		t.Errorf("Expected ErrorType 'Error', got '%s'", err.ErrorType)
	}
}

func TestErrorWithCode(t *testing.T) {
	err := NewErrorWithCode("test error", 10, 5, "test.nx", "let x = 1")
	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
	if err.Code != "let x = 1" {
		t.Errorf("Expected code 'let x = 1', got '%s'", err.Code)
	}
}

func TestErrorWithStack(t *testing.T) {
	stack := []StackFrame{
		{FunctionName: "main", Line: 1, Column: 0, Filename: "test.nx"},
		{FunctionName: "foo", Line: 5, Column: 2, Filename: "test.nx"},
	}
	err := NewErrorWithStack("test error", 10, 5, "test.nx", stack)
	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
	if len(err.Stack) != 2 {
		t.Errorf("Expected stack length 2, got %d", len(err.Stack))
	}
	if err.Stack[0].FunctionName != "main" {
		t.Errorf("Expected first stack frame function name 'main', got '%s'", err.Stack[0].FunctionName)
	}
}

func TestErrorString(t *testing.T) {
	err := NewError("test error", 10, 5, "test.nx")
	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}
}

func TestErrorToString(t *testing.T) {
	err := NewError("test error", 10, 5, "test.nx")
	errStr := err.ToStr()
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}
}

func TestErrorTypeName(t *testing.T) {
	err := NewError("test error", 10, 5, "test.nx")
	if err.TypeName() != "Error" {
		t.Errorf("Expected type name 'Error', got '%s'", err.TypeName())
	}
}

func TestErrorTypeCode(t *testing.T) {
	err := NewError("test error", 10, 5, "test.nx")
	if err.TypeCode() != TypeError {
		t.Errorf("Expected type code %d, got %d", TypeError, err.TypeCode())
	}
}

func TestErrorEquals(t *testing.T) {
	err1 := NewError("test error", 10, 5, "test.nx")
	err2 := NewError("test error", 10, 5, "test.nx")
	err3 := NewError("different error", 10, 5, "test.nx")

	if !err1.Equals(err2) {
		t.Error("Expected err1 to equal err2")
	}
	if err1.Equals(err3) {
		t.Error("Expected err1 to not equal err3")
	}
	if err1.Equals(Int(1)) {
		t.Error("Expected err1 to not equal Int(1)")
	}
}

func TestErrorEqualsWithProperties(t *testing.T) {
	err1 := NewError("test error", 10, 5, "test.nx")
	err1.Properties["custom"] = String("value")

	err2 := NewError("test error", 10, 5, "test.nx")
	err2.Properties["custom"] = String("value")

	if !err1.Equals(err2) {
		t.Error("Expected err1 to equal err2 with same properties")
	}
}

func TestCustomError(t *testing.T) {
	err := NewCustomError("CustomError", "custom error", 10, 5, "test.nx")
	if err.ErrorType != "CustomError" {
		t.Errorf("Expected ErrorType 'CustomError', got '%s'", err.ErrorType)
	}
	if err.Message != "custom error" {
		t.Errorf("Expected message 'custom error', got '%s'", err.Message)
	}
}
