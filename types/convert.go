package types

import (
	"fmt"
	"strconv"
)

// ToBool converts an Object to a boolean value following Nxlang conversion rules
func ToBool(obj Object) bool {
	switch v := obj.(type) {
	case *Undefined:
		return false
	case *Null:
		return false
	case Bool:
		return bool(v)
	case Byte:
		return v != 0
	case Char:
		return v != 0
	case Int:
		return v != 0
	case UInt:
		return v != 0
	case Float:
		return v != 0
	case String:
		return len(v) > 0
	default:
		return true // All other objects are truthy
	}
}

// ToInt converts an Object to an Int value following Nxlang conversion rules
func ToInt(obj Object) (Int, *Error) {
	switch v := obj.(type) {
	case Bool:
		if v {
			return Int(1), nil
		}
		return Int(0), nil
	case Byte:
		return Int(v), nil
	case Char:
		return Int(v), nil
	case Int:
		return v, nil
	case UInt:
		return Int(v), nil
	case Float:
		return Int(v), nil
	case String:
		num, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, NewError(fmt.Sprintf("cannot convert string '%s' to int", v), 0, 0, "")
		}
		return Int(num), nil
	default:
		return 0, NewError(fmt.Sprintf("cannot convert %s to int", obj.TypeName()), 0, 0, "")
	}
}

// ToFloat converts an Object to a Float value following Nxlang conversion rules
func ToFloat(obj Object) (Float, *Error) {
	switch v := obj.(type) {
	case Bool:
		if v {
			return Float(1.0), nil
		}
		return Float(0.0), nil
	case Byte:
		return Float(v), nil
	case Char:
		return Float(v), nil
	case Int:
		return Float(v), nil
	case UInt:
		return Float(v), nil
	case Float:
		return v, nil
	case String:
		num, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return 0, NewError(fmt.Sprintf("cannot convert string '%s' to float", v), 0, 0, "")
		}
		return Float(num), nil
	default:
		return 0, NewError(fmt.Sprintf("cannot convert %s to float", obj.TypeName()), 0, 0, "")
	}
}

// ToString converts an Object to a String value
func ToString(obj Object) String {
	return String(obj.ToStr())
}

// ToByte converts an Object to a Byte value
func ToByte(obj Object) (Byte, *Error) {
	i, err := ToInt(obj)
	if err != nil {
		return 0, err
	}
	if i < 0 || i > 255 {
		return 0, NewError(fmt.Sprintf("int %d out of byte range (0-255)", i), 0, 0, "")
	}
	return Byte(i), nil
}

// ToChar converts an Object to a Char value
func ToChar(obj Object) (Char, *Error) {
	switch v := obj.(type) {
	case Char:
		return v, nil
	case String:
		if len(v) == 0 {
			return 0, NewError("cannot convert empty string to char", 0, 0, "")
		}
		return Char([]rune(string(v))[0]), nil
	default:
		i, err := ToInt(obj)
		if err != nil {
			return 0, err
		}
		return Char(i), nil
	}
}
