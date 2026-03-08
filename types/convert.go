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

// ConvertToType converts an object to the specified target type, following Nxlang conversion rules
// Returns the converted object and an error if conversion is not possible
func ConvertToType(targetType uint8, value Object) (Object, *Error) {
	switch targetType {
	case TypeUndefined:
		return UndefinedValue, nil
	case TypeNull:
		return NullValue, nil
	case TypeBool:
		return Bool(ToBool(value)), nil
	case TypeByte:
		return ToByte(value)
	case TypeChar:
		return ToChar(value)
	case TypeInt:
		return ToInt(value)
	case TypeUInt:
		i, err := ToInt(value)
		if err != nil {
			return nil, err
		}
		return UInt(i), nil
	case TypeFloat:
		return ToFloat(value)
	case TypeString:
		return ToString(value), nil
	default:
		// For non-primitive types, return the value as-is if types match
		if value.TypeCode() == targetType {
			return value, nil
		}
		return nil, NewError(fmt.Sprintf("cannot convert %s to target type %s", value.TypeName(), typeNames[targetType]), 0, 0, "")
	}
}

// AutoConvert converts right value to match left value type (left-side priority rule)
// Used for binary operations where right operand should be converted to left operand type
// Returns (convertedLeft, convertedRight, error)
func AutoConvert(left, right Object) (Object, Object, *Error) {
	leftType := left.TypeCode()
	rightType := right.TypeCode()

	// Same type, no conversion needed
	if leftType == rightType {
		return left, right, nil
	}

	// String concatenation special case: if either is string, convert both to string
	if leftType == TypeString || rightType == TypeString {
		return ToString(left), ToString(right), nil
	}

	// Numeric type promotion hierarchy: Byte < Char < Int < UInt < Float
	typeOrder := map[uint8]int{
		TypeByte:  1,
		TypeChar:  2,
		TypeInt:   3,
		TypeUInt:  4,
		TypeFloat: 5,
	}

	leftOrder, leftIsNumeric := typeOrder[leftType]
	rightOrder, rightIsNumeric := typeOrder[rightType]

	if leftIsNumeric && rightIsNumeric {
		// Both numeric, promote to higher type
		if leftOrder > rightOrder {
			// Convert right to left type
			convertedRight, err := ConvertToType(leftType, right)
			if err != nil {
				return nil, nil, err
			}
			return left, convertedRight, nil
		} else if rightOrder > leftOrder {
			// Convert left to right type
			convertedLeft, err := ConvertToType(rightType, left)
			if err != nil {
				return nil, nil, err
			}
			return convertedLeft, right, nil
		}
		// Same order, should have same type, already handled above
	}

	// Bool conversion
	if leftType == TypeBool {
		return left, Bool(ToBool(right)), nil
	}
	if rightType == TypeBool {
		return Bool(ToBool(left)), right, nil
	}

	// If no conversion rule applies, return as-is (will fail in operation)
	return left, right, nil
}
