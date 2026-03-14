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

func TestRangeIterator(t *testing.T) {
	// Test range iterator
	ri := NewRangeIterator(0, 5, 1)

	if ri.TypeName() != "range" {
		t.Errorf("Expected type name 'range', got '%s'", ri.TypeName())
	}

	// Test iteration
	count := 0
	for ri.HasNext() {
		ri.Next()
		count++
	}
	if count != 5 {
		t.Errorf("Expected 5 iterations, got %d", count)
	}

	// Test ToStr
	str := ri.ToStr()
	if str == "" {
		t.Error("Expected non-empty string for ToStr")
	}
}

func TestSeq(t *testing.T) {
	// Test NewSeq
	seq := NewSeq()
	if seq.Len() != 0 {
		t.Errorf("Expected length 0, got %d", seq.Len())
	}

	// Test NewSeqWithCapacity
	seq = NewSeqWithCapacity(10)
	if seq.Len() != 0 {
		t.Errorf("Expected length 0, got %d", seq.Len())
	}

	// Test NewSeqWithElements
	seq = NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})
	if seq.Len() != 3 {
		t.Errorf("Expected length 3, got %d", seq.Len())
	}

	// Test TypeCode
	if seq.TypeCode() != types.TypeSeq {
		t.Errorf("Expected type code %d, got %d", types.TypeSeq, seq.TypeCode())
	}

	// Test TypeName
	if seq.TypeName() != "seq" {
		t.Errorf("Expected type name 'seq', got '%s'", seq.TypeName())
	}

	// Test ToStr
	str := seq.ToStr()
	if str == "" {
		t.Error("Expected non-empty string")
	}

	// Test Get
	val := seq.Get(0)
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}

	// Test Set
	seq.Set(0, types.Int(10))
	val = seq.Get(0)
	if !val.Equals(types.Int(10)) {
		t.Errorf("Expected 10, got %v", val)
	}

	// Test Append
	seq.Append(types.Int(4))
	if seq.Len() != 4 {
		t.Errorf("Expected length 4, got %d", seq.Len())
	}

	// Test AppendMany
	seq.AppendMany(types.Int(5), types.Int(6))
	if seq.Len() != 6 {
		t.Errorf("Expected length 6, got %d", seq.Len())
	}

	// Test Pop
	popVal := seq.Pop()
	if !popVal.Equals(types.Int(6)) {
		t.Errorf("Expected 6, got %v", popVal)
	}

	// Test Clear
	seq.Clear()
	if seq.Len() != 0 {
		t.Errorf("Expected length 0, got %d", seq.Len())
	}
}

func TestSeqAdvanced(t *testing.T) {
	seq := NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3), types.Int(4), types.Int(5)})

	// Test Resize
	seq.Resize(3)
	if seq.Len() != 3 {
		t.Errorf("Expected length 3, got %d", seq.Len())
	}

	// Test Fill - creates a new seq
	seq = NewSeq()
	seq.Fill(types.Int(0), 3)
	if seq.Len() != 3 {
		t.Errorf("Expected length 3 after fill, got %d", seq.Len())
	}

	// Test Range
	seq = NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3), types.Int(4), types.Int(5)})
	rangeSeq := seq.Range(1, 3)
	if rangeSeq.Len() != 2 {
		t.Errorf("Expected range length 2, got %d", rangeSeq.Len())
	}

	// Test ForEach
	sum := 0
	seq.ForEach(func(v types.Object, i int) {
		if val, ok := v.(types.Int); ok {
			sum += int(val)
		}
	})
	if sum != 15 {
		t.Errorf("Expected sum 15, got %d", sum)
	}

	// Test Reverse
	seq.Reverse()
	if !seq.Get(0).Equals(types.Int(5)) {
		t.Errorf("Expected 5 after reverse, got %v", seq.Get(0))
	}

	// Test Elements
	elements := seq.Elements()
	if len(elements) != 5 {
		t.Errorf("Expected 5 elements, got %d", len(elements))
	}
}

func TestSeqSearch(t *testing.T) {
	seq := NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3), types.Int(2)})

	// Test Includes
	if !seq.Includes(types.Int(2)) {
		t.Error("Expected seq to include 2")
	}
	if seq.Includes(types.Int(10)) {
		t.Error("Expected seq to not include 10")
	}

	// Test IndexOf
	idx := seq.IndexOf(types.Int(2))
	if idx != 1 {
		t.Errorf("Expected index 1, got %d", idx)
	}

	// Test Find
	found := seq.Find(func(v types.Object, i int) bool {
		return v.Equals(types.Int(3))
	})
	if found == nil {
		t.Error("Expected to find 3")
	}

	// Test FindIndex
	findIdx := seq.FindIndex(func(v types.Object, i int) bool {
		return v.Equals(types.Int(3))
	})
	if findIdx != 2 {
		t.Errorf("Expected find index 2, got %d", findIdx)
	}

	// Test Join
	joined := seq.Join(",")
	if joined == "" {
		t.Error("Expected non-empty joined string")
	}
}

func TestSeqMapFilter(t *testing.T) {
	seq := NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})

	// Test Map
	mapped := seq.Map(func(v types.Object, i int) types.Object {
		if val, ok := v.(types.Int); ok {
			return types.Int(int(val) * 2)
		}
		return v
	})
	if !mapped.Get(0).Equals(types.Int(2)) {
		t.Errorf("Expected mapped first element to be 2, got %v", mapped.Get(0))
	}

	// Test Filter
	filtered := seq.Filter(func(v types.Object, i int) bool {
		if val, ok := v.(types.Int); ok {
			return int(val) > 1
		}
		return false
	})
	if filtered.Len() != 2 {
		t.Errorf("Expected filtered length 2, got %d", filtered.Len())
	}
}

func TestStack(t *testing.T) {
	// Test NewStack
	stack := NewStack()

	if stack.TypeCode() != types.TypeStack {
		t.Errorf("Expected type code %d, got %d", types.TypeStack, stack.TypeCode())
	}

	if stack.TypeName() != "stack" {
		t.Errorf("Expected type name 'stack', got '%s'", stack.TypeName())
	}

	// Test Push
	stack.Push(types.Int(1))
	stack.Push(types.Int(2))
	stack.Push(types.Int(3))

	if stack.Len() != 3 {
		t.Errorf("Expected length 3, got %d", stack.Len())
	}

	// Test Peek
	top := stack.Peek()
	if !top.Equals(types.Int(3)) {
		t.Errorf("Expected peek to return 3, got %v", top)
	}

	// Test Pop
	val := stack.Pop()
	if !val.Equals(types.Int(3)) {
		t.Errorf("Expected pop to return 3, got %v", val)
	}

	// Test IsEmpty
	if stack.IsEmpty() {
		t.Error("Expected stack to not be empty")
	}

	// Test Clear
	stack.Clear()
	if !stack.IsEmpty() {
		t.Error("Expected stack to be empty after clear")
	}

	// Test ToStr
	stack.Push(types.Int(1))
	str := stack.ToStr()
	if str == "" {
		t.Error("Expected non-empty string")
	}

	// Test Equals
	stack2 := NewStack()
	stack2.Push(types.Int(1))
	if !stack.Equals(stack2) {
		t.Error("Expected stacks to be equal")
	}
}

func TestQueue(t *testing.T) {
	// Test NewQueue
	queue := NewQueue()

	if queue.TypeCode() != types.TypeQueue {
		t.Errorf("Expected type code %d, got %d", types.TypeQueue, queue.TypeCode())
	}

	if queue.TypeName() != "queue" {
		t.Errorf("Expected type name 'queue', got '%s'", queue.TypeName())
	}

	// Test Enqueue
	queue.Enqueue(types.Int(1))
	queue.Enqueue(types.Int(2))
	queue.Enqueue(types.Int(3))

	if queue.Len() != 3 {
		t.Errorf("Expected length 3, got %d", queue.Len())
	}

	// Test Peek
	front := queue.Peek()
	if !front.Equals(types.Int(1)) {
		t.Errorf("Expected peek to return 1, got %v", front)
	}

	// Test Dequeue
	val := queue.Dequeue()
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected dequeue to return 1, got %v", val)
	}

	// Test IsEmpty
	if queue.IsEmpty() {
		t.Error("Expected queue to not be empty")
	}

	// Test Clear
	queue.Clear()
	if !queue.IsEmpty() {
		t.Error("Expected queue to be empty after clear")
	}

	// Test ToStr
	queue.Enqueue(types.Int(1))
	str := queue.ToStr()
	if str == "" {
		t.Error("Expected non-empty string")
	}

	// Test Equals
	queue2 := NewQueue()
	queue2.Enqueue(types.Int(1))
	if !queue.Equals(queue2) {
		t.Error("Expected queues to be equal")
	}
}

func TestArrayNegativeIndex(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})

	// Test negative index - behavior depends on implementation
	// Some implementations may not support negative indexing
	val := arr.Get(-1)
	// Just verify it returns something (could be undefined or an error)
	_ = val
}

func TestArrayOutOfBounds(t *testing.T) {
	arr := NewArrayWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})

	// Test out of bounds Get
	val := arr.Get(10)
	// Behavior depends on implementation - might return undefined or error
	_ = val
}

func TestSeqGetAuto(t *testing.T) {
	seq := NewSeqWithElements([]types.Object{types.Int(1), types.Int(2), types.Int(3)})

	// Test GetAuto with existing index
	val := seq.GetAuto(0)
	if !val.Equals(types.Int(1)) {
		t.Errorf("Expected 1, got %v", val)
	}

	// Test GetAuto with index beyond length (should auto-grow)
	val = seq.GetAuto(5)
	// Should return undefined after auto-growing
	if val == nil {
		t.Error("Expected some value (undefined)")
	}
}
