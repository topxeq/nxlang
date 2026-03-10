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
