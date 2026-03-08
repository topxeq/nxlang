package types

import (
	"fmt"
	"github.com/topxeq/nxlang/bytecode"
	"strconv"
)

// Undefined represents the undefined value
type Undefined struct{}

var UndefinedValue = &Undefined{}

// TypeCode implements Object interface
func (u *Undefined) TypeCode() uint8 { return TypeUndefined }

// TypeName implements Object interface
func (u *Undefined) TypeName() string { return typeNames[TypeUndefined] }

// ToStr implements Object interface
func (u *Undefined) ToStr() string { return "undefined" }

// Equals implements Object interface
func (u *Undefined) Equals(other Object) bool {
	_, ok := other.(*Undefined)
	return ok
}

// Null represents the null value
type Null struct{}

var NullValue = &Null{}

// TypeCode implements Object interface
func (n *Null) TypeCode() uint8 { return TypeNull }

// TypeName implements Object interface
func (n *Null) TypeName() string { return typeNames[TypeNull] }

// ToStr implements Object interface
func (n *Null) ToStr() string { return "null" }

// Equals implements Object interface
func (n *Null) Equals(other Object) bool {
	_, ok := other.(*Null)
	return ok
}

// Bool represents a boolean value
type Bool bool

// TypeCode implements Object interface
func (b Bool) TypeCode() uint8 { return TypeBool }

// TypeName implements Object interface
func (b Bool) TypeName() string { return typeNames[TypeBool] }

// ToStr implements Object interface
func (b Bool) ToStr() string { return strconv.FormatBool(bool(b)) }

// Equals implements Object interface
func (b Bool) Equals(other Object) bool {
	otherBool, ok := other.(Bool)
	if !ok {
		return false
	}
	return b == otherBool
}

// Byte represents a byte value (uint8)
type Byte byte

// TypeCode implements Object interface
func (b Byte) TypeCode() uint8 { return TypeByte }

// TypeName implements Object interface
func (b Byte) TypeName() string { return typeNames[TypeByte] }

// ToStr implements Object interface
func (b Byte) ToStr() string { return strconv.Itoa(int(b)) }

// Equals implements Object interface
func (b Byte) Equals(other Object) bool {
	otherByte, ok := other.(Byte)
	if !ok {
		return false
	}
	return b == otherByte
}

// Char represents a Unicode character
type Char rune

// TypeCode implements Object interface
func (c Char) TypeCode() uint8 { return TypeChar }

// TypeName implements Object interface
func (c Char) TypeName() string { return typeNames[TypeChar] }

// ToStr implements Object interface
func (c Char) ToStr() string { return string(c) }

// Equals implements Object interface
func (c Char) Equals(other Object) bool {
	// 支持和Char比较
	if otherChar, ok := other.(Char); ok {
		return c == otherChar
	}
	// 支持和Int比较
	if otherInt, ok := other.(Int); ok {
		return rune(c) == rune(otherInt)
	}
	return false
}

// Int represents an integer value (int64)
type Int int64

// TypeCode implements Object interface
func (i Int) TypeCode() uint8 { return TypeInt }

// TypeName implements Object interface
func (i Int) TypeName() string { return typeNames[TypeInt] }

// ToStr implements Object interface
func (i Int) ToStr() string { return strconv.FormatInt(int64(i), 10) }

// Equals implements Object interface
func (i Int) Equals(other Object) bool {
	// 支持和Int比较
	if otherInt, ok := other.(Int); ok {
		return i == otherInt
	}
	// 支持和Char比较
	if otherChar, ok := other.(Char); ok {
		return rune(i) == rune(otherChar)
	}
	return false
}

// UInt represents an unsigned integer value (uint64)
type UInt uint64

// TypeCode implements Object interface
func (u UInt) TypeCode() uint8 { return TypeUInt }

// TypeName implements Object interface
func (u UInt) TypeName() string { return typeNames[TypeUInt] }

// ToStr implements Object interface
func (u UInt) ToStr() string { return strconv.FormatUint(uint64(u), 10) }

// Equals implements Object interface
func (u UInt) Equals(other Object) bool {
	otherUInt, ok := other.(UInt)
	if !ok {
		return false
	}
	return u == otherUInt
}

// Float represents a floating point value (float64)
type Float float64

// TypeCode implements Object interface
func (f Float) TypeCode() uint8 { return TypeFloat }

// TypeName implements Object interface
func (f Float) TypeName() string { return typeNames[TypeFloat] }

// ToStr implements Object interface
func (f Float) ToStr() string { return strconv.FormatFloat(float64(f), 'g', -1, 64) }

// Equals implements Object interface
func (f Float) Equals(other Object) bool {
	otherFloat, ok := other.(Float)
	if !ok {
		return false
	}
	return f == otherFloat
}

// String represents a string value
type String string

// TypeCode implements Object interface
func (s String) TypeCode() uint8 { return TypeString }

// TypeName implements Object interface
func (s String) TypeName() string { return typeNames[TypeString] }

// ToStr implements Object interface
func (s String) ToStr() string { return string(s) }

// Equals implements Object interface
func (s String) Equals(other Object) bool {
	otherStr, ok := other.(String)
	if !ok {
		return false
	}
	return s == otherStr
}

// Function represents a compiled function
type Function struct {
	Name          string
	NumLocals     int
	NumParameters int
	IsVariadic    bool
	IsStatic      bool // Whether this is a static method
	DefaultValues []int // Indices of default values in constant pool
	Instructions  []byte
	ConstantPool  []bytecode.Constant // Constant pool this function belongs to
}

// TypeCode implements Object interface
func (f *Function) TypeCode() uint8 { return TypeFunction }

// TypeName implements Object interface
func (f *Function) TypeName() string { return typeNames[TypeFunction] }

// ToStr implements Object interface
func (f *Function) ToStr() string { return fmt.Sprintf("[function %s]", f.Name) }

// Equals implements Object interface
func (f *Function) Equals(other Object) bool {
	otherFunc, ok := other.(*Function)
	if !ok {
		return false
	}
	return f == otherFunc
}

// NativeFunction represents a native Go function callable from Nxlang
type NativeFunction struct {
	Fn func(args ...Object) Object
}

func (nf *NativeFunction) TypeCode() uint8          { return TypeNativeFunc }
func (nf *NativeFunction) TypeName() string          { return "nativeFunc" }
func (nf *NativeFunction) ToStr() string             { return "[native function]" }
func (nf *NativeFunction) Equals(other Object) bool { return nf == other }

// Class represents a compiled class definition
type Class struct {
	Name       string
	SuperClass *Class
	Methods    map[string]*Function // Map of method name to function
}

func (c *Class) TypeCode() uint8          { return TypeClass }
func (c *Class) TypeName() string          { return "class" }
func (c *Class) ToStr() string             { return fmt.Sprintf("[class %s]", c.Name) }
func (c *Class) Equals(other Object) bool { return c == other }

// Instance represents an instance of a class
type Instance struct {
	Class      *Class
	Properties map[string]Object // Object properties
}

func (i *Instance) TypeCode() uint8          { return TypeObject }
func (i *Instance) TypeName() string          { return i.Class.Name }
func (i *Instance) ToStr() string             { return fmt.Sprintf("[object %s]", i.Class.Name) }
func (i *Instance) Equals(other Object) bool { return i == other }

// BoundMethod represents a method bound to a specific instance
type BoundMethod struct {
	Instance *Instance // The instance this method is bound to
	Method   *Function // The actual function to call
}

func (bm *BoundMethod) TypeCode() uint8          { return TypeBoundMethod }
func (bm *BoundMethod) TypeName() string          { return "bound_method" }
func (bm *BoundMethod) ToStr() string             { return fmt.Sprintf("[bound method %s.%s]", bm.Instance.Class.Name, bm.Method.Name) }
func (bm *BoundMethod) Equals(other Object) bool {
	otherBM, ok := other.(*BoundMethod)
	return ok && bm.Instance == otherBM.Instance && bm.Method == otherBM.Method
}

// SuperReference represents a reference to superclass with bound instance
type SuperReference struct {
	Instance *Instance // The current instance
	Super    *Class    // The superclass
}

func (sr *SuperReference) TypeCode() uint8          { return TypeSuperReference }
func (sr *SuperReference) TypeName() string          { return "super_reference" }
func (sr *SuperReference) ToStr() string             { return fmt.Sprintf("[super reference for %s]", sr.Instance.Class.Name) }
func (sr *SuperReference) Equals(other Object) bool {
	otherSR, ok := other.(*SuperReference)
	return ok && sr.Instance == otherSR.Instance && sr.Super == otherSR.Super
}
