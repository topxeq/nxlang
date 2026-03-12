package collections

import (
	"testing"

	"github.com/topxeq/nxlang/types"
)

func TestArrayNew(t *testing.T) {
	arr := NewArray()
	if arr.Len() != 0 {
		t.Errorf("Expected length 0, got %d", arr.Len())
	}
}

func TestArrayNewWithElements(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	if arr.Len() != 3 {
		t.Errorf("Expected length 3, got %d", arr.Len())
	}
}

func TestArrayTypeCode(t *testing.T) {
	arr := NewArray()
	if arr.TypeCode() != types.TypeArray {
		t.Errorf("Expected type code %d, got %d", types.TypeArray, arr.TypeCode())
	}
}

func TestArrayTypeName(t *testing.T) {
	arr := NewArray()
	if arr.TypeName() != "array" {
		t.Errorf("Expected type name 'array', got '%s'", arr.TypeName())
	}
}

func TestArrayToStr(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2)})
	str := arr.ToStr()
	if str == "" {
		t.Error("Expected non-empty string")
	}
}

func TestArrayEquals(t *testing.T) {
	arr1 := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2)})
	arr2 := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2)})
	arr3 := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})

	if !arr1.Equals(arr2) {
		t.Error("Expected arr1 to equal arr2")
	}
	if arr1.Equals(arr3) {
		t.Error("Expected arr1 to not equal arr3")
	}
	if arr1.Equals(types.Int(1)) {
		t.Error("Expected arr1 to not equal Int(1)")
	}
}

func TestArrayGet(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	val := arr.Get(0)
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}

	val = arr.Get(2)
	if !val.Equals(types.Int(3)) {
		t.Errorf("Expected 3, got %v", val)
	}
}

func TestArraySet(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	arr.Set(0, types.Int(10))
	val := arr.Get(0)
	if !val.Equals(types.Int(10)) {
		t.Errorf("Expected 10, got %v", val)
	}
}

func TestArrayAppend(t *testing.T) {
	arr := NewArray()
	arr.Append(types.Int(1))
	arr.Append(types.Int(2))
	if arr.Len() != 2 {
		t.Errorf("Expected length 2, got %d", arr.Len())
	}
}

func TestArrayInsert(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(3)})
	arr.Insert(1, types.Int(2))
	if arr.Len() != 3 {
		t.Errorf("Expected length 3, got %d", arr.Len())
	}
	val := arr.Get(1)
	if !val.Equals(types.Int(2)) {
		t.Errorf("Expected 2, got %v", val)
	}
}

func TestArrayRemove(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	val := arr.Remove(1)
	if !val.Equals(types.Int(2)) {
		t.Errorf("Expected removed value 2, got %v", val)
	}
	if arr.Len() != 2 {
		t.Errorf("Expected length 2, got %d", arr.Len())
	}
}

func TestArrayClear(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	arr.Clear()
	if arr.Len() != 0 {
		t.Errorf("Expected length 0, got %d", arr.Len())
	}
}

func TestMapNew(t *testing.T) {
	m := NewMap()
	if m.Len() != 0 {
		t.Errorf("Expected length 0, got %d", m.Len())
	}
}

func TestMapTypeCode(t *testing.T) {
	m := NewMap()
	if m.TypeCode() != types.TypeMap {
		t.Errorf("Expected type code %d, got %d", types.TypeMap, m.TypeCode())
	}
}

func TestMapTypeName(t *testing.T) {
	m := NewMap()
	if m.TypeName() != "map" {
		t.Errorf("Expected type name 'map', got '%s'", m.TypeName())
	}
}

func TestMapToStr(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	str := m.ToStr()
	if str == "" {
		t.Error("Expected non-empty string")
	}
}

func TestMapEquals(t *testing.T) {
	m1 := NewMap()
	m1.Set("a", types.Int(1))
	m1.Set("b", types.Int(2))

	m2 := NewMap()
	m2.Set("a", types.Int(1))
	m2.Set("b", types.Int(2))

	m3 := NewMap()
	m3.Set("a", types.Int(1))

	if !m1.Equals(m2) {
		t.Error("Expected m1 to equal m2")
	}
	if m1.Equals(m3) {
		t.Error("Expected m1 to not equal m3")
	}
	if m1.Equals(types.Int(1)) {
		t.Error("Expected m1 to not equal Int(1)")
	}
}

func TestMapGet(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	m.Set("b", types.Int(2))

	val := m.Get("a")
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}
}

func TestMapSet(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	val := m.Get("a")
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}
}

func TestMapHas(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))

	if !m.Has("a") {
		t.Error("Expected m to have key 'a'")
	}
	if m.Has("b") {
		t.Error("Expected m to not have key 'b'")
	}
}

func TestMapDelete(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	deleted := m.Delete("a")
	if deleted == nil {
		t.Error("Expected delete to return true")
	}
	if m.Len() != 0 {
		t.Errorf("Expected length 0, got %d", m.Len())
	}
}

func TestMapKeys(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	m.Set("b", types.Int(2))

	keys := m.Keys()
	if keys.Len() != 2 {
		t.Errorf("Expected 2 keys, got %d", keys.Len())
	}
}

func TestMapValues(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	m.Set("b", types.Int(2))

	values := m.Values()
	if values.Len() != 2 {
		t.Errorf("Expected 2 values, got %d", values.Len())
	}
}

func TestMapClear(t *testing.T) {
	m := NewMap()
	m.Set("a", types.Int(1))
	m.Set("b", types.Int(2))

	m.Clear()
	if m.Len() != 0 {
		t.Errorf("Expected length 0, got %d", m.Len())
	}
}

func TestOrderedMapNew(t *testing.T) {
	om := NewOrderedMap()
	if om.Len() != 0 {
		t.Errorf("Expected length 0, got %d", om.Len())
	}
}

func TestOrderedMapSet(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	if om.Len() != 2 {
		t.Errorf("Expected length 2, got %d", om.Len())
	}
}

func TestOrderedMapGet(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	val := om.Get("a")
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}
}

func TestOrderedMapHas(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))

	if !om.Has("a") {
		t.Error("Expected om to have key 'a'")
	}
	if om.Has("b") {
		t.Error("Expected om to not have key 'b'")
	}
}

func TestOrderedMapDelete(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Delete("a")

	if om.Len() != 0 {
		t.Errorf("Expected length 0, got %d", om.Len())
	}
}

func TestOrderedMapKeys(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	keys := om.Keys()
	if keys.Len() != 2 {
		t.Errorf("Expected 2 keys, got %d", keys.Len())
	}
}

func TestOrderedMapValues(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	values := om.Values()
	if values.Len() != 2 {
		t.Errorf("Expected 2 values, got %d", values.Len())
	}
}

func TestOrderedMapTypeName(t *testing.T) {
	om := NewOrderedMap()
	if om.TypeName() != "orderedMap" {
		t.Errorf("Expected type name 'orderedMap', got '%s'", om.TypeName())
	}
}

func TestOrderedMapClear(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	om.Clear()
	if om.Len() != 0 {
		t.Errorf("Expected length 0, got %d", om.Len())
	}
}

func TestOrderedMapMoveTo(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))
	om.Set("c", types.Int(3))

	om.MoveTo("c", 0)

	keys := om.Keys()
	if keys.Len() != 3 {
		t.Errorf("Expected 3 keys, got %d", keys.Len())
	}
}

func TestOrderedMapMoveToFirst(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	om.MoveToFirst("b")

	keys := om.Keys()
	if keys.Len() != 2 {
		t.Errorf("Expected 2 keys, got %d", keys.Len())
	}
}

func TestOrderedMapMoveToLast(t *testing.T) {
	om := NewOrderedMap()
	om.Set("a", types.Int(1))
	om.Set("b", types.Int(2))

	om.MoveToLast("a")

	keys := om.Keys()
	if keys.Len() != 2 {
		t.Errorf("Expected 2 keys, got %d", keys.Len())
	}
}
