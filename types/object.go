package types

// Object is the core interface implemented by all Nxlang values
type Object interface {
	// TypeCode returns the fixed type code for this object
	TypeCode() uint8

	// TypeName returns the human-readable type name
	TypeName() string

	// ToStr returns the string representation of the object
	ToStr() string

	// Equals checks if this object is equal to another object
	Equals(other Object) bool
}

// IsErr checks if an object is an error type
func IsErr(obj Object) bool {
	_, ok := obj.(*Error)
	return ok
}

// General type conversion functions for Nxlang runtime
func TypeCode(obj Object) Int {
	return Int(obj.TypeCode())
}

func TypeName(obj Object) String {
	return String(obj.TypeName())
}

func ToStr(obj Object) String {
	return String(obj.ToStr())
}
