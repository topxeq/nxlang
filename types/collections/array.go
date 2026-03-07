package collections

import (
	"fmt"
	"strings"

	"github.com/topxeq/nxlang/types"
)

// Array represents a dynamic array of objects
type Array struct {
	Elements []types.Object
}

// NewArray creates a new empty array
func NewArray() *Array {
	return &Array{
		Elements: []types.Object{},
	}
}

// NewArrayWithElements creates a new array with the given elements
func NewArrayWithElements(elements []types.Object) *Array {
	return &Array{
		Elements: elements,
	}
}

// TypeCode implements types.Object interface
func (a *Array) TypeCode() uint8 {
	return types.TypeArray
}

// TypeName implements types.Object interface
func (a *Array) TypeName() string {
	return "array"
}

// ToStr implements types.Object interface
func (a *Array) ToStr() string {
	elements := make([]string, len(a.Elements))
	for i, elem := range a.Elements {
		elements[i] = elem.ToStr()
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}

// Equals implements types.Object interface
func (a *Array) Equals(other types.Object) bool {
	otherArr, ok := other.(*Array)
	if !ok {
		return false
	}
	if len(a.Elements) != len(otherArr.Elements) {
		return false
	}
	for i, elem := range a.Elements {
		if !elem.Equals(otherArr.Elements[i]) {
			return false
		}
	}
	return true
}

// Len returns the number of elements in the array
func (a *Array) Len() int {
	return len(a.Elements)
}

// Get returns the element at the given index
func (a *Array) Get(index int) types.Object {
	if index < 0 || index >= len(a.Elements) {
		return types.UndefinedValue
	}
	return a.Elements[index]
}

// Set sets the element at the given index
func (a *Array) Set(index int, value types.Object) {
	if index < 0 {
		return
	}
	// Grow array if needed
	for index >= len(a.Elements) {
		a.Elements = append(a.Elements, types.UndefinedValue)
	}
	a.Elements[index] = value
}

// Append adds an element to the end of the array
func (a *Array) Append(value types.Object) {
	a.Elements = append(a.Elements, value)
}

// Insert inserts an element at the given index
func (a *Array) Insert(index int, value types.Object) {
	if index < 0 || index > len(a.Elements) {
		return
	}
	a.Elements = append(a.Elements[:index], append([]types.Object{value}, a.Elements[index:]...)...)
}

// Remove removes and returns the element at the given index
func (a *Array) Remove(index int) types.Object {
	if index < 0 || index >= len(a.Elements) {
		return types.UndefinedValue
	}
	value := a.Elements[index]
	a.Elements = append(a.Elements[:index], a.Elements[index+1:]...)
	return value
}

// Clear removes all elements from the array
func (a *Array) Clear() {
	a.Elements = []types.Object{}
}
