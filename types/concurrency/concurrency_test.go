package concurrency

import (
	"testing"

	"github.com/topxeq/nxlang/types"
)

func TestMutexNew(t *testing.T) {
	m := NewMutex()
	if m == nil {
		t.Error("Expected non-nil Mutex")
	}
}

func TestMutexTypeCode(t *testing.T) {
	m := NewMutex()
	if m.TypeCode() != types.TypeMutex {
		t.Errorf("Expected TypeMutex %d, got %d", types.TypeMutex, m.TypeCode())
	}
}

func TestMutexTypeName(t *testing.T) {
	m := NewMutex()
	if m.TypeName() != "mutex" {
		t.Errorf("Expected 'mutex', got %q", m.TypeName())
	}
}

func TestMutexToStr(t *testing.T) {
	m := NewMutex()
	if m.ToStr() != "[mutex]" {
		t.Errorf("Expected '[mutex]', got %q", m.ToStr())
	}
}

func TestMutexEquals(t *testing.T) {
	m1 := NewMutex()
	m2 := NewMutex()

	if !m1.Equals(m1) {
		t.Error("Expected mutex to equal itself")
	}
	if m1.Equals(m2) {
		t.Error("Expected different mutexes to not be equal")
	}
	if m1.Equals(types.Int(1)) {
		t.Error("Expected mutex to not equal Int")
	}
}

func TestMutexLockUnlock(t *testing.T) {
	m := NewMutex()
	m.Lock()
	m.Unlock()
}

func TestMutexTryLock(t *testing.T) {
	m := NewMutex()
	locked := m.TryLock()
	if !locked {
		t.Error("Expected TryLock to succeed")
	}
	m.Unlock()
}

func TestRWMutexNew(t *testing.T) {
	rw := NewRWMutex()
	if rw == nil {
		t.Error("Expected non-nil RWMutex")
	}
}

func TestRWMutexTypeCode(t *testing.T) {
	rw := NewRWMutex()
	if rw.TypeCode() != types.TypeRWMutex {
		t.Errorf("Expected TypeRWMutex %d, got %d", types.TypeRWMutex, rw.TypeCode())
	}
}

func TestRWMutexTypeName(t *testing.T) {
	rw := NewRWMutex()
	if rw.TypeName() != "rwMutex" {
		t.Errorf("Expected 'rwMutex', got %q", rw.TypeName())
	}
}

func TestRWMutexToStr(t *testing.T) {
	rw := NewRWMutex()
	if rw.ToStr() != "[rwMutex]" {
		t.Errorf("Expected '[rwMutex]', got %q", rw.ToStr())
	}
}

func TestRWMutexEquals(t *testing.T) {
	rw1 := NewRWMutex()
	rw2 := NewRWMutex()

	if !rw1.Equals(rw1) {
		t.Error("Expected rwMutex to equal itself")
	}
	if rw1.Equals(rw2) {
		t.Error("Expected different rwMutexes to not be equal")
	}
	if rw1.Equals(types.Int(1)) {
		t.Error("Expected rwMutex to not equal Int")
	}
}

func TestRWMutexRLockRUnlock(t *testing.T) {
	rw := NewRWMutex()
	rw.RLock()
	rw.RUnlock()
}

func TestRWMutexLockUnlock(t *testing.T) {
	rw := NewRWMutex()
	rw.Lock()
	rw.Unlock()
}

func TestRWMutexTryRLock(t *testing.T) {
	rw := NewRWMutex()
	locked := rw.TryRLock()
	if !locked {
		t.Error("Expected TryRLock to succeed")
	}
	rw.RUnlock()
}

func TestRWMutexTryLock(t *testing.T) {
	rw := NewRWMutex()
	locked := rw.TryLock()
	if !locked {
		t.Error("Expected TryLock to succeed")
	}
	rw.Unlock()
}
