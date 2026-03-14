package types

import (
	"testing"
)

func TestToInt(t *testing.T) {
	tests := []struct {
		input    Object
		expected Int
		hasError bool
	}{
		{Int(123), Int(123), false},
		{Float(3.14), Int(3), false},
		{Bool(true), Int(1), false},
		{Bool(false), Int(0), false},
		{String("123"), Int(123), false},
		{String("abc"), Int(0), true},
		{Byte(65), Int(65), false},
		{Char(65), Int(65), false},
		{UInt(123), Int(123), false},
	}

	for _, tt := range tests {
		result, err := ToInt(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ToInt(%v) expected error, got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ToInt(%v) unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("ToInt(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input    Object
		expected Float
		hasError bool
	}{
		{Int(123), Float(123), false},
		{Float(3.14), Float(3.14), false},
		{Bool(true), Float(1), false},
		{Bool(false), Float(0), false},
		{String("3.14"), Float(3.14), false},
		{String("abc"), Float(0), true},
		{Byte(65), Float(65), false},
		{UInt(123), Float(123), false},
	}

	for _, tt := range tests {
		result, err := ToFloat(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ToFloat(%v) expected error, got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ToFloat(%v) unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("ToFloat(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		input    Object
		expected bool
	}{
		{Int(1), true},
		{Int(0), false},
		{Float(3.14), true},
		{Float(0), false},
		{Bool(true), true},
		{Bool(false), false},
		{String("hello"), true},
		{String(""), false},
		{UndefinedValue, false},
		{NullValue, false},
	}

	for _, tt := range tests {
		result := ToBool(tt.input)
		if result != tt.expected {
			t.Errorf("ToBool(%v) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestToByte(t *testing.T) {
	tests := []struct {
		input    Object
		expected Byte
		hasError bool
	}{
		{Int(65), Byte(65), false},
		{Int(255), Byte(255), false},
		{Int(256), Byte(0), true},
		{Int(-1), Byte(0), true},
		{Float(65.5), Byte(65), false},
		{String("A"), Byte(65), false},
		{Char(65), Byte(65), false},
		{String("AB"), Byte(0), true},
	}

	for _, tt := range tests {
		result, err := ToByte(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ToByte(%v) expected error, got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ToByte(%v) unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("ToByte(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestToChar(t *testing.T) {
	tests := []struct {
		input    Object
		expected Char
		hasError bool
	}{
		{Int(65), Char(65), false},
		{Char(20013), Char(20013), false},
		{String("A"), Char(65), false},
		{String("中"), Char(20013), false},
		{String("AB"), Char(0), true},
		{Float(65.5), Char(65), false},
	}

	for _, tt := range tests {
		result, err := ToChar(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ToChar(%v) expected error, got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ToChar(%v) unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("ToChar(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestToUint(t *testing.T) {
	tests := []struct {
		input    Object
		expected UInt
		hasError bool
	}{
		{Int(123), UInt(123), false},
		{Int(-1), UInt(0), true},
		{Float(3.14), UInt(3), false},
		{Float(-3.14), UInt(0), true},
		{Bool(true), UInt(1), false},
		{Bool(false), UInt(0), false},
		{String("456"), UInt(456), false},
		{Byte(65), UInt(65), false},
	}

	for _, tt := range tests {
		result, err := ToUint(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ToUint(%v) expected error, got none", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ToUint(%v) unexpected error: %v", tt.input, err)
			} else if result != tt.expected {
				t.Errorf("ToUint(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		input    Object
		expected string
	}{
		{Int(123), "123"},
		{Float(3.14), "3.14"},
		{Bool(true), "true"},
		{Bool(false), "false"},
		{String("hello"), "hello"},
		{Byte(65), "65"},
		{Char(65), "A"},
		{UndefinedValue, "undefined"},
		{NullValue, "null"},
	}

	for _, tt := range tests {
		result := ToString(tt.input)
		if string(result) != tt.expected {
			t.Errorf("ToString(%v) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestTypeCodeConsistency(t *testing.T) {
	tests := []struct {
		obj      Object
		typeName string
		typeCode uint8
	}{
		{UndefinedValue, "undefined", TypeUndefined},
		{NullValue, "null", TypeNull},
		{Bool(true), "bool", TypeBool},
		{Int(123), "int", TypeInt},
		{Float(3.14), "float", TypeFloat},
		{String("hello"), "string", TypeString},
		{Byte(65), "byte", TypeByte},
		{Char(65), "char", TypeChar},
		{UInt(123), "uint", TypeUInt},
	}

	for _, tt := range tests {
		if tt.obj.TypeName() != tt.typeName {
			t.Errorf("TypeName mismatch: got %s, expected %s", tt.obj.TypeName(), tt.typeName)
		}
		if tt.obj.TypeCode() != tt.typeCode {
			t.Errorf("TypeCode mismatch for %s: got %d, expected %d", tt.typeName, tt.obj.TypeCode(), tt.typeCode)
		}
	}
}

func TestObjectEquals(t *testing.T) {
	// Test Int equals
	if !Int(5).Equals(Int(5)) {
		t.Error("Int(5) should equal Int(5)")
	}
	if Int(5).Equals(Int(6)) {
		t.Error("Int(5) should not equal Int(6)")
	}

	// Test Float equals
	if !Float(3.14).Equals(Float(3.14)) {
		t.Error("Float(3.14) should equal Float(3.14)")
	}

	// Test String equals
	if !String("hello").Equals(String("hello")) {
		t.Error("String('hello') should equal String('hello')")
	}

	// Test Bool equals
	if !Bool(true).Equals(Bool(true)) {
		t.Error("Bool(true) should equal Bool(true)")
	}

	// Test cross-type comparisons
	if Int(5).Equals(String("5")) {
		t.Error("Int(5) should not equal String('5')")
	}
}

func TestObjectStr(t *testing.T) {
	tests := []struct {
		obj      Object
		expected string
	}{
		{Int(123), "123"},
		{Float(3.14), "3.14"},
		{Bool(true), "true"},
		{Bool(false), "false"},
		{String("hello"), "hello"},
		{NullValue, "null"},
		{UndefinedValue, "undefined"},
	}

	for _, tt := range tests {
		if tt.obj.ToStr() != tt.expected {
			t.Errorf("ToStr() = %s, expected %s", tt.obj.ToStr(), tt.expected)
		}
	}
}

func TestNullUndefined(t *testing.T) {
	// Test Null
	if NullValue.TypeCode() != TypeNull {
		t.Error("NullValue should have TypeNull")
	}
	if NullValue.TypeName() != "null" {
		t.Error("NullValue should have typeName 'null'")
	}
	if !NullValue.Equals(NullValue) {
		t.Error("NullValue should equal itself")
	}

	// Test Undefined
	if UndefinedValue.TypeCode() != TypeUndefined {
		t.Error("UndefinedValue should have TypeUndefined")
	}
	if UndefinedValue.TypeName() != "undefined" {
		t.Error("UndefinedValue should have typeName 'undefined'")
	}
}

func TestByteOperations(t *testing.T) {
	b := Byte(65)

	if b.TypeCode() != TypeByte {
		t.Error("Byte should have TypeByte")
	}
	if b.TypeName() != "byte" {
		t.Error("Byte should have typeName 'byte'")
	}
	if b.ToStr() != "65" {
		t.Errorf("Byte ToStr = %s, expected 65", b.ToStr())
	}
}

func TestCharOperations(t *testing.T) {
	c := Char(65) // 'A'

	if c.TypeCode() != TypeChar {
		t.Error("Char should have TypeChar")
	}
	if c.TypeName() != "char" {
		t.Error("Char should have typeName 'char'")
	}
	if c.ToStr() != "A" {
		t.Errorf("Char ToStr = %s, expected A", c.ToStr())
	}

	// Test Unicode character
	unicode := Char(20013) // '中'
	if unicode.ToStr() != "中" {
		t.Errorf("Unicode Char ToStr = %s, expected 中", unicode.ToStr())
	}
}

func TestUIntOperations(t *testing.T) {
	u := UInt(123456)

	if u.TypeCode() != TypeUInt {
		t.Error("UInt should have TypeUInt")
	}
	if u.TypeName() != "uint" {
		t.Error("UInt should have typeName 'uint'")
	}
	if !u.Equals(UInt(123456)) {
		t.Error("UInt should equal same value")
	}
}
