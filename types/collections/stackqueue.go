package collections

import (
	"fmt"

	"github.com/topxeq/nxlang/types"
)

// Stack represents a LIFO (last-in first-out) data structure
type Stack struct {
	elements []types.Object
}

// NewStack creates a new empty stack
func NewStack() *Stack {
	return &Stack{
		elements: []types.Object{},
	}
}

// TypeCode implements types.Object interface
func (s *Stack) TypeCode() uint8 {
	return types.TypeStack
}

// TypeName implements types.Object interface
func (s *Stack) TypeName() string {
	return "stack"
}

// ToStr implements types.Object interface
func (s *Stack) ToStr() string {
	return fmt.Sprintf("Stack[%d elements]", len(s.elements))
}

// Equals implements types.Object interface
func (s *Stack) Equals(other types.Object) bool {
	otherStack, ok := other.(*Stack)
	if !ok {
		return false
	}
	if len(s.elements) != len(otherStack.elements) {
		return false
	}
	for i, elem := range s.elements {
		if !elem.Equals(otherStack.elements[i]) {
			return false
		}
	}
	return true
}

// Push adds an element to the top of the stack
func (s *Stack) Push(value types.Object) {
	s.elements = append(s.elements, value)
}

// Pop removes and returns the top element from the stack
func (s *Stack) Pop() types.Object {
	if len(s.elements) == 0 {
		return types.UndefinedValue
	}
	value := s.elements[len(s.elements)-1]
	s.elements = s.elements[:len(s.elements)-1]
	return value
}

// Peek returns the top element without removing it
func (s *Stack) Peek() types.Object {
	if len(s.elements) == 0 {
		return types.UndefinedValue
	}
	return s.elements[len(s.elements)-1]
}

// Len returns the number of elements in the stack
func (s *Stack) Len() int {
	return len(s.elements)
}

// IsEmpty returns true if the stack is empty
func (s *Stack) IsEmpty() bool {
	return len(s.elements) == 0
}

// Clear removes all elements from the stack
func (s *Stack) Clear() {
	s.elements = []types.Object{}
}

// Queue represents a FIFO (first-in first-out) data structure
type Queue struct {
	elements []types.Object
}

// NewQueue creates a new empty queue
func NewQueue() *Queue {
	return &Queue{
		elements: []types.Object{},
	}
}

// TypeCode implements types.Object interface
func (q *Queue) TypeCode() uint8 {
	return types.TypeQueue
}

// TypeName implements types.Object interface
func (q *Queue) TypeName() string {
	return "queue"
}

// ToStr implements types.Object interface
func (q *Queue) ToStr() string {
	return fmt.Sprintf("Queue[%d elements]", len(q.elements))
}

// Equals implements types.Object interface
func (q *Queue) Equals(other types.Object) bool {
	otherQueue, ok := other.(*Queue)
	if !ok {
		return false
	}
	if len(q.elements) != len(otherQueue.elements) {
		return false
	}
	for i, elem := range q.elements {
		if !elem.Equals(otherQueue.elements[i]) {
			return false
		}
	}
	return true
}

// Enqueue adds an element to the end of the queue
func (q *Queue) Enqueue(value types.Object) {
	q.elements = append(q.elements, value)
}

// Dequeue removes and returns the first element from the queue
func (q *Queue) Dequeue() types.Object {
	if len(q.elements) == 0 {
		return types.UndefinedValue
	}
	value := q.elements[0]
	q.elements = q.elements[1:]
	return value
}

// Peek returns the first element without removing it
func (q *Queue) Peek() types.Object {
	if len(q.elements) == 0 {
		return types.UndefinedValue
	}
	return q.elements[0]
}

// Len returns the number of elements in the queue
func (q *Queue) Len() int {
	return len(q.elements)
}

// IsEmpty returns true if the queue is empty
func (q *Queue) IsEmpty() bool {
	return len(q.elements) == 0
}

// Clear removes all elements from the queue
func (q *Queue) Clear() {
	q.elements = []types.Object{}
}
