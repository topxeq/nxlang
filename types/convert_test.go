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
