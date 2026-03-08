package vm

import (
	"fmt"

	"github.com/topxeq/nxlang/types"
)

// Stack represents the operand stack used by the VM
type Stack struct {
	elements []types.Object
	sp       int // Stack pointer (points to next free slot)
}

const MaxStackSize = 1024 * 1024 // 1MB stack

// NewStack creates a new empty stack
func NewStack() *Stack {
	return &Stack{
		elements: make([]types.Object, MaxStackSize),
		sp:       0,
	}
}

// Push adds an element to the top of the stack
func (s *Stack) Push(o types.Object) error {
	if s.sp >= MaxStackSize {
		return &StackOverflowError{}
	}
	s.elements[s.sp] = o
	s.sp++
	return nil
}

// Pop removes and returns the top element from the stack
func (s *Stack) Pop() types.Object {
	if s.sp == 0 {
		panic("stack underflow")
	}
	s.sp--
	o := s.elements[s.sp]
	s.elements[s.sp] = nil // Help garbage collection
	return o
}

// PopSafe pops element with error instead of panic
func (s *Stack) PopSafe() (types.Object, error) {
	if s.sp == 0 {
		return nil, &StackUnderflowError{}
	}
	s.sp--
	o := s.elements[s.sp]
	s.elements[s.sp] = nil
	return o, nil
}

// Peek returns the top element without removing it
func (s *Stack) Peek() types.Object {
	if s.sp == 0 {
		panic("stack underflow")
	}
	return s.elements[s.sp-1]
}

// PeekSafe peeks element with error instead of panic
func (s *Stack) PeekSafe() (types.Object, error) {
	if s.sp == 0 {
		return nil, &StackUnderflowError{}
	}
	return s.elements[s.sp-1], nil
}

// PeekN returns the nth element from the top (0 = top) without removing it
func (s *Stack) PeekN(n int) types.Object {
	if s.sp-n-1 < 0 {
		panic(fmt.Sprintf("stack underflow when peeking %d elements", n))
	}
	return s.elements[s.sp-n-1]
}

// PeekNSafe peeks nth element with error instead of panic
func (s *Stack) PeekNSafe(n int) (types.Object, error) {
	if s.sp-n-1 < 0 {
		return nil, &StackUnderflowError{}
	}
	return s.elements[s.sp-n-1], nil
}

// PopN removes and returns the top n elements (in reverse order: top first)
func (s *Stack) PopN(n int) []types.Object {
	elements := make([]types.Object, n)
	for i := 0; i < n; i++ {
		elements[i] = s.Pop()
	}
	// Reverse to get the correct order
	for i, j := 0, len(elements)-1; i < j; i, j = i+1, j-1 {
		elements[i], elements[j] = elements[j], elements[i]
	}
	return elements
}

// Size returns the number of elements on the stack
func (s *Stack) Size() int {
	return s.sp
}

// SetSize sets the stack size to the specified value
func (s *Stack) SetSize(size int) {
	if size < 0 {
		size = 0
	} else if size > MaxStackSize {
		size = MaxStackSize
	}
	// Clear elements between new size and old size
	for i := size; i < s.sp; i++ {
		s.elements[i] = nil
	}
	s.sp = size
}

// Clear removes all elements from the stack
func (s *Stack) Clear() {
	for i := 0; i < s.sp; i++ {
		s.elements[i] = nil
	}
	s.sp = 0
}

// StackOverflowError is returned when the stack exceeds its maximum size
type StackOverflowError struct{}

func (e *StackOverflowError) Error() string {
	return "stack overflow"
}

// StackUnderflowError is returned when trying to pop from an empty stack
type StackUnderflowError struct{}

func (e *StackUnderflowError) Error() string {
	return "stack underflow"
}
