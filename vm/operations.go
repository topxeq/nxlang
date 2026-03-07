package vm

import (
	"fmt"
	"math"

	"github.com/topxeq/nxlang/types"
)

// addObjects performs addition of two objects
func (vm *VM) addObjects(a, b types.Object) (types.Object, error) {
	// String concatenation
	if a.TypeCode() == types.TypeString || b.TypeCode() == types.TypeString {
		return types.String(a.ToStr() + b.ToStr()), nil
	}

	// Numeric addition
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt + bInt, nil
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		return types.Float(af + bf), nil
	}

	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s + %s", a.TypeName(), b.TypeName()), 0)
}

// subObjects performs subtraction of two objects
func (vm *VM) subObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt - bInt, nil
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		return types.Float(af - bf), nil
	}

	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s - %s", a.TypeName(), b.TypeName()), 0)
}

// mulObjects performs multiplication of two objects
func (vm *VM) mulObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt * bInt, nil
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		return types.Float(af * bf), nil
	}

	// String repetition
	if str, ok := a.(types.String); ok {
		if count, ok := b.(types.Int); ok && count >= 0 {
			result := ""
			for i := 0; i < int(count); i++ {
				result += string(str)
			}
			return types.String(result), nil
		}
	}
	if str, ok := b.(types.String); ok {
		if count, ok := a.(types.Int); ok && count >= 0 {
			result := ""
			for i := 0; i < int(count); i++ {
				result += string(str)
			}
			return types.String(result), nil
		}
	}

	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s * %s", a.TypeName(), b.TypeName()), 0)
}

// divObjects performs division of two objects
func (vm *VM) divObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		if bInt == 0 {
			return nil, vm.newError("division by zero", 0)
		}
		return types.Float(float64(aInt) / float64(bInt)), nil
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		if bf == 0 {
			return nil, vm.newError("division by zero", 0)
		}

		return types.Float(af / bf), nil
	}

	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s / %s", a.TypeName(), b.TypeName()), 0)
}

// modObjects performs modulo operation on two objects
func (vm *VM) modObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		if bInt == 0 {
			return nil, vm.newError("modulo by zero", 0)
		}
		return aInt % bInt, nil
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		if bf == 0 {
			return nil, vm.newError("modulo by zero", 0)
		}

		// Use math.Mod for float modulo
		return types.Float(math.Mod(af, bf)), nil
	}

	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s %% %s", a.TypeName(), b.TypeName()), 0)
}

// negObject performs negation of an object
func (vm *VM) negObject(a types.Object) (types.Object, error) {
	switch val := a.(type) {
	case types.Int:
		return -val, nil
	case types.Float:
		return -val, nil
	default:
		return nil, vm.newError(fmt.Sprintf("unsupported operation: -%s", a.TypeName()), 0)
	}
}

// bitNotObject performs bitwise NOT operation on an object
func (vm *VM) bitNotObject(a types.Object) (types.Object, error) {
	if val, ok := a.(types.Int); ok {
		return types.Int(^val), nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: ~%s", a.TypeName()), 0)
}

// bitAndObjects performs bitwise AND operation on two objects
func (vm *VM) bitAndObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt & bInt, nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s & %s", a.TypeName(), b.TypeName()), 0)
}

// bitOrObjects performs bitwise OR operation on two objects
func (vm *VM) bitOrObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt | bInt, nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s | %s", a.TypeName(), b.TypeName()), 0)
}

// bitXorObjects performs bitwise XOR operation on two objects
func (vm *VM) bitXorObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt ^ bInt, nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s ^ %s", a.TypeName(), b.TypeName()), 0)
}

// shiftLeftObjects performs left shift operation on two objects
func (vm *VM) shiftLeftObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt << bInt, nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s << %s", a.TypeName(), b.TypeName()), 0)
}

// shiftRightObjects performs right shift operation on two objects
func (vm *VM) shiftRightObjects(a, b types.Object) (types.Object, error) {
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		return aInt >> bInt, nil
	}
	return nil, vm.newError(fmt.Sprintf("unsupported operation: %s >> %s", a.TypeName(), b.TypeName()), 0)
}

// compareObjects compares two objects using the specified operator
func (vm *VM) compareObjects(a, b types.Object, op int) (types.Object, error) {
	// Compare numbers
	aInt, aIsInt := a.(types.Int)
	bInt, bIsInt := b.(types.Int)
	if aIsInt && bIsInt {
		switch op {
		case lt:
			return types.Bool(aInt < bInt), nil
		case lte:
			return types.Bool(aInt <= bInt), nil
		case gt:
			return types.Bool(aInt > bInt), nil
		case gte:
			return types.Bool(aInt >= bInt), nil
		}
	}

	aFloat, aIsFloat := a.(types.Float)
	bFloat, bIsFloat := b.(types.Float)
	if (aIsInt || aIsFloat) && (bIsInt || bIsFloat) {
		af := float64(0.0)
		if aIsInt {
			af = float64(aInt)
		} else {
			af = float64(aFloat)
		}

		bf := float64(0.0)
		if bIsInt {
			bf = float64(bInt)
		} else {
			bf = float64(bFloat)
		}

		switch op {
		case lt:
			return types.Bool(af < bf), nil
		case lte:
			return types.Bool(af <= bf), nil
		case gt:
			return types.Bool(af > bf), nil
		case gte:
			return types.Bool(af >= bf), nil
		}
	}

	// Compare strings
	aStr, aIsStr := a.(types.String)
	bStr, bIsStr := b.(types.String)
	if aIsStr && bIsStr {
		switch op {
		case lt:
			return types.Bool(aStr < bStr), nil
		case lte:
			return types.Bool(aStr <= bStr), nil
		case gt:
			return types.Bool(aStr > bStr), nil
		case gte:
			return types.Bool(aStr >= bStr), nil
		}
	}

	// Compare booleans
	aBool, aIsBool := a.(types.Bool)
	bBool, bIsBool := b.(types.Bool)
	if aIsBool && bIsBool {
		aVal := 0
		if aBool {
			aVal = 1
		}
		bVal := 0
		if bBool {
			bVal = 1
		}
		switch op {
		case lt:
			return types.Bool(aVal < bVal), nil
		case lte:
			return types.Bool(aVal <= bVal), nil
		case gt:
			return types.Bool(aVal > bVal), nil
		case gte:
			return types.Bool(aVal >= bVal), nil
		}
	}

	return nil, vm.newError(fmt.Sprintf("unsupported comparison: %s %s %s",
		a.TypeName(), opToString(op), b.TypeName()), 0)
}

// opToString converts a comparison operator to string
func opToString(op int) string {
	switch op {
	case lt:
		return "<"
	case lte:
		return "<="
	case gt:
		return ">"
	case gte:
		return ">="
	default:
		return "??"
	}
}
