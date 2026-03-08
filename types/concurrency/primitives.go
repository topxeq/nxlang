package concurrency

import (
	"sync"

	"github.com/topxeq/nxlang/types"
)

// Mutex represents a mutual exclusion lock
type Mutex struct {
	mu sync.Mutex
}

// NewMutex creates a new mutex
func NewMutex() *Mutex {
	return &Mutex{}
}

// TypeCode implements types.Object interface
func (m *Mutex) TypeCode() uint8 {
	return types.TypeMutex
}

// TypeName implements types.Object interface
func (m *Mutex) TypeName() string {
	return "mutex"
}

// ToStr implements types.Object interface
func (m *Mutex) ToStr() string {
	return "[mutex]"
}

// Equals implements types.Object interface
func (m *Mutex) Equals(other types.Object) bool {
	otherMutex, ok := other.(*Mutex)
	if !ok {
		return false
	}
	return m == otherMutex
}

// Lock acquires the mutex
func (m *Mutex) Lock() {
	m.mu.Lock()
}

// Unlock releases the mutex
func (m *Mutex) Unlock() {
	m.mu.Unlock()
}

// TryLock attempts to acquire the mutex without blocking
func (m *Mutex) TryLock() bool {
	return m.mu.TryLock()
}

// RWMutex represents a reader/writer mutual exclusion lock
type RWMutex struct {
	mu sync.RWMutex
}

// NewRWMutex creates a new read-write mutex
func NewRWMutex() *RWMutex {
	return &RWMutex{}
}

// TypeCode implements types.Object interface
func (m *RWMutex) TypeCode() uint8 {
	return types.TypeRWMutex
}

// TypeName implements types.Object interface
func (m *RWMutex) TypeName() string {
	return "rwMutex"
}

// ToStr implements types.Object interface
func (m *RWMutex) ToStr() string {
	return "[rwMutex]"
}

// Equals implements types.Object interface
func (m *RWMutex) Equals(other types.Object) bool {
	otherRWMutex, ok := other.(*RWMutex)
	if !ok {
		return false
	}
	return m == otherRWMutex
}

// RLock acquires a read lock
func (m *RWMutex) RLock() {
	m.mu.RLock()
}

// RUnlock releases a read lock
func (m *RWMutex) RUnlock() {
	m.mu.RUnlock()
}

// Lock acquires a write lock
func (m *RWMutex) Lock() {
	m.mu.Lock()
}

// Unlock releases a write lock
func (m *RWMutex) Unlock() {
	m.mu.Unlock()
}

// TryRLock attempts to acquire a read lock without blocking
func (m *RWMutex) TryRLock() bool {
	return m.mu.TryRLock()
}

// TryLock attempts to acquire a write lock without blocking
func (m *RWMutex) TryLock() bool {
	return m.mu.TryLock()
}
