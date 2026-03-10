package collections

import (
	"fmt"
	"strings"

	"github.com/topxeq/nxlang/types"
)

// Seq represents a self-growing sequence (auto-incrementing array)
// It automatically grows when accessing indices beyond its current length
type Seq struct {
	elements []types.Object
}

// NewSeq creates a new empty sequence
func NewSeq() *Seq {
	return &Seq{
		elements: []types.Object{},
	}
}

// NewSeqWithCapacity creates a new sequence with initial capacity
func NewSeqWithCapacity(capacity int) *Seq {
	return &Seq{
		elements: make([]types.Object, 0, capacity),
	}
}

// NewSeqWithElements creates a new sequence with the given elements
func NewSeqWithElements(elements []types.Object) *Seq {
	return &Seq{
		elements: elements,
	}
}

// TypeCode implements types.Object interface
func (s *Seq) TypeCode() uint8 {
	return types.TypeSeq
}

// TypeName implements types.Object interface
func (s *Seq) TypeName() string {
	return "seq"
}

// ToStr implements types.Object interface
func (s *Seq) ToStr() string {
	elements := make([]string, len(s.elements))
	for i, elem := range s.elements {
		elements[i] = elem.ToStr()
	}
	return fmt.Sprintf("seq[%s]", strings.Join(elements, ", "))
}

// Equals implements types.Object interface
func (s *Seq) Equals(other types.Object) bool {
	otherSeq, ok := other.(*Seq)
	if !ok {
		return false
	}
	if len(s.elements) != len(otherSeq.elements) {
		return false
	}
	for i, elem := range s.elements {
		if !elem.Equals(otherSeq.elements[i]) {
			return false
		}
	}
	return true
}

// Len returns the number of elements in the sequence
func (s *Seq) Len() int {
	return len(s.elements)
}

// Get returns the element at the given index
// If index is beyond current length, returns undefined (auto-growth on read returns undefined)
func (s *Seq) Get(index int) types.Object {
	if index < 0 {
		return types.NewError(fmt.Sprintf("seq index out of range: %d", index), 0, 0, "")
	}
	if index >= len(s.elements) {
		return types.UndefinedValue
	}
	return s.elements[index]
}

// GetAuto returns the element at the given index, auto-growing if needed
func (s *Seq) GetAuto(index int) types.Object {
	if index < 0 {
		return types.NewError(fmt.Sprintf("seq index out of range: %d", index), 0, 0, "")
	}
	// Auto-grow the sequence if needed
	for len(s.elements) <= index {
		s.elements = append(s.elements, types.UndefinedValue)
	}
	return s.elements[index]
}

// Set sets the element at the given index
// Auto-grows the sequence if index is beyond current length
func (s *Seq) Set(index int, value types.Object) types.Object {
	if index < 0 {
		return types.NewError(fmt.Sprintf("seq index out of range: %d", index), 0, 0, "")
	}
	// Auto-grow the sequence if needed
	for len(s.elements) <= index {
		s.elements = append(s.elements, types.UndefinedValue)
	}
	s.elements[index] = value
	return types.UndefinedValue
}

// Append adds an element to the end of the sequence
func (s *Seq) Append(value types.Object) *Seq {
	s.elements = append(s.elements, value)
	return s
}

// AppendMany adds multiple elements to the end of the sequence
func (s *Seq) AppendMany(values ...types.Object) *Seq {
	s.elements = append(s.elements, values...)
	return s
}

// Pop removes and returns the last element
func (s *Seq) Pop() types.Object {
	if len(s.elements) == 0 {
		return types.UndefinedValue
	}
	last := s.elements[len(s.elements)-1]
	s.elements = s.elements[:len(s.elements)-1]
	return last
}

// Clear removes all elements from the sequence
func (s *Seq) Clear() {
	s.elements = []types.Object{}
}

// Resize changes the size of the sequence
// If new size is larger, new elements are initialized to undefined
// If new size is smaller, elements are removed from the end
func (s *Seq) Resize(newSize int) {
	if newSize < 0 {
		return
	}
	if newSize > len(s.elements) {
		for len(s.elements) < newSize {
			s.elements = append(s.elements, types.UndefinedValue)
		}
	} else if newSize < len(s.elements) {
		s.elements = s.elements[:newSize]
	}
}

// Fill fills the sequence with a given value
func (s *Seq) Fill(value types.Object, count int) *Seq {
	for i := 0; i < count; i++ {
		s.elements = append(s.elements, value)
	}
	return s
}

// Range returns a new sequence with elements from start to end (exclusive)
func (s *Seq) Range(start, end int) *Seq {
	if start < 0 {
		start = 0
	}
	if end > len(s.elements) {
		end = len(s.elements)
	}
	if start >= end {
		return NewSeq()
	}
	result := NewSeq()
	result.elements = make([]types.Object, end-start)
	copy(result.elements, s.elements[start:end])
	return result
}

// ForEach calls the given function for each element
func (s *Seq) ForEach(fn func(types.Object, int)) {
	for i, elem := range s.elements {
		fn(elem, i)
	}
}

// Map returns a new sequence with the results of applying the function to each element
func (s *Seq) Map(fn func(types.Object, int) types.Object) *Seq {
	result := NewSeq()
	for i, elem := range s.elements {
		result.Append(fn(elem, i))
	}
	return result
}

// Filter returns a new sequence with elements that pass the test
func (s *Seq) Filter(fn func(types.Object, int) bool) *Seq {
	result := NewSeq()
	for i, elem := range s.elements {
		if fn(elem, i) {
			result.Append(elem)
		}
	}
	return result
}

// Find returns the first element that passes the test
func (s *Seq) Find(fn func(types.Object, int) bool) types.Object {
	for i, elem := range s.elements {
		if fn(elem, i) {
			return elem
		}
	}
	return types.UndefinedValue
}

// FindIndex returns the index of the first element that passes the test
func (s *Seq) FindIndex(fn func(types.Object, int) bool) int {
	for i, elem := range s.elements {
		if fn(elem, i) {
			return i
		}
	}
	return -1
}

// Includes checks if the sequence contains a value
func (s *Seq) Includes(value types.Object) bool {
	for _, elem := range s.elements {
		if elem.Equals(value) {
			return true
		}
	}
	return false
}

// IndexOf returns the first index of a value
func (s *Seq) IndexOf(value types.Object) int {
	for i, elem := range s.elements {
		if elem.Equals(value) {
			return i
		}
	}
	return -1
}

// Join joins all elements with a separator
func (s *Seq) Join(sep string) string {
	elements := make([]string, len(s.elements))
	for i, elem := range s.elements {
		elements[i] = elem.ToStr()
	}
	return strings.Join(elements, sep)
}

// Reverse reverses the sequence in place
func (s *Seq) Reverse() *Seq {
	for i, j := 0, len(s.elements)-1; i < j; i, j = i+1, j-1 {
		s.elements[i], s.elements[j] = s.elements[j], s.elements[i]
	}
	return s
}

// Elements returns the underlying elements slice (for direct access)
func (s *Seq) Elements() []types.Object {
	return s.elements
}
