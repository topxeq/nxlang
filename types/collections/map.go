package collections

import (
	"fmt"
	"sort"
	"strings"

	"github.com/topxeq/nxlang/types"
)

// Map represents an unordered key-value map
type Map struct {
	Entries map[string]types.Object
}

// NewMap creates a new empty map
func NewMap() *Map {
	return &Map{
		Entries: make(map[string]types.Object),
	}
}

// TypeCode implements types.Object interface
func (m *Map) TypeCode() uint8 {
	return types.TypeMap
}

// TypeName implements types.Object interface
func (m *Map) TypeName() string {
	return "map"
}

// ToStr implements types.Object interface
func (m *Map) ToStr() string {
	pairs := make([]string, 0, len(m.Entries))
	for key, value := range m.Entries {
		pairs = append(pairs, fmt.Sprintf("%q: %s", key, value.ToStr()))
	}
	return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
}

// Equals implements types.Object interface
func (m *Map) Equals(other types.Object) bool {
	otherMap, ok := other.(*Map)
	if !ok {
		return false
	}
	if len(m.Entries) != len(otherMap.Entries) {
		return false
	}
	for key, value := range m.Entries {
		otherValue, exists := otherMap.Entries[key]
		if !exists || !value.Equals(otherValue) {
			return false
		}
	}
	return true
}

// Get returns the value for the given key
func (m *Map) Get(key string) types.Object {
	value, exists := m.Entries[key]
	if !exists {
		return types.UndefinedValue
	}
	return value
}

// Set sets the value for the given key
func (m *Map) Set(key string, value types.Object) {
	m.Entries[key] = value
}

// Has checks if the map contains the given key
func (m *Map) Has(key string) bool {
	_, exists := m.Entries[key]
	return exists
}

// Delete removes the key from the map
func (m *Map) Delete(key string) types.Object {
	value, exists := m.Entries[key]
	if !exists {
		return types.UndefinedValue
	}
	delete(m.Entries, key)
	return value
}

// Len returns the number of entries in the map
func (m *Map) Len() int {
	return len(m.Entries)
}

// Keys returns all keys in the map as an array
func (m *Map) Keys() *Array {
	keys := make([]types.Object, 0, len(m.Entries))
	for key := range m.Entries {
		keys = append(keys, types.String(key))
	}
	return NewArrayWithElements(keys)
}

// Values returns all values in the map as an array
func (m *Map) Values() *Array {
	values := make([]types.Object, 0, len(m.Entries))
	for _, value := range m.Entries {
		values = append(values, value)
	}
	return NewArrayWithElements(values)
}

// Clear removes all entries from the map
func (m *Map) Clear() {
	m.Entries = make(map[string]types.Object)
}

// OrderedMap represents an ordered key-value map that preserves insertion order
type OrderedMap struct {
	Entries map[string]types.Object
	Order   []string
}

// NewOrderedMap creates a new empty ordered map
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		Entries: make(map[string]types.Object),
		Order:   []string{},
	}
}

// TypeCode implements types.Object interface
func (om *OrderedMap) TypeCode() uint8 {
	return types.TypeOrderedMap
}

// TypeName implements types.Object interface
func (om *OrderedMap) TypeName() string {
	return "orderedMap"
}

// ToStr implements types.Object interface
func (om *OrderedMap) ToStr() string {
	pairs := make([]string, 0, len(om.Order))
	for _, key := range om.Order {
		value := om.Entries[key]
		pairs = append(pairs, fmt.Sprintf("%q: %s", key, value.ToStr()))
	}
	return fmt.Sprintf("OrderedMap{%s}", strings.Join(pairs, ", "))
}

// Equals implements types.Object interface
func (om *OrderedMap) Equals(other types.Object) bool {
	otherOM, ok := other.(*OrderedMap)
	if !ok {
		return false
	}
	if len(om.Entries) != len(otherOM.Entries) {
		return false
	}
	if len(om.Order) != len(otherOM.Order) {
		return false
	}
	for i, key := range om.Order {
		if key != otherOM.Order[i] {
			return false
		}
		if !om.Entries[key].Equals(otherOM.Entries[key]) {
			return false
		}
	}
	return true
}

// Get returns the value for the given key
func (om *OrderedMap) Get(key string) types.Object {
	value, exists := om.Entries[key]
	if !exists {
		return types.UndefinedValue
	}
	return value
}

// Set sets the value for the given key
func (om *OrderedMap) Set(key string, value types.Object) {
	_, exists := om.Entries[key]
	if !exists {
		om.Order = append(om.Order, key)
	}
	om.Entries[key] = value
}

// Has checks if the map contains the given key
func (om *OrderedMap) Has(key string) bool {
	_, exists := om.Entries[key]
	return exists
}

// Delete removes the key from the map
func (om *OrderedMap) Delete(key string) types.Object {
	value, exists := om.Entries[key]
	if !exists {
		return types.UndefinedValue
	}
	delete(om.Entries, key)
	// Remove from order
	for i, k := range om.Order {
		if k == key {
			om.Order = append(om.Order[:i], om.Order[i+1:]...)
			break
		}
	}
	return value
}

// Len returns the number of entries in the map
func (om *OrderedMap) Len() int {
	return len(om.Entries)
}

// Keys returns all keys in insertion order as an array
func (om *OrderedMap) Keys() *Array {
	keys := make([]types.Object, len(om.Order))
	for i, key := range om.Order {
		keys[i] = types.String(key)
	}
	return NewArrayWithElements(keys)
}

// Values returns all values in insertion order as an array
func (om *OrderedMap) Values() *Array {
	values := make([]types.Object, len(om.Order))
	for i, key := range om.Order {
		values[i] = om.Entries[key]
	}
	return NewArrayWithElements(values)
}

// Sort sorts the map keys using the given comparator
func (om *OrderedMap) Sort(comparator func(a, b string) bool) {
	sort.Slice(om.Order, func(i, j int) bool {
		return comparator(om.Order[i], om.Order[j])
	})
}

// Clear removes all entries from the map
func (om *OrderedMap) Clear() {
	om.Entries = make(map[string]types.Object)
	om.Order = []string{}
}

// MoveTo moves the given key to the specified 0-based index
// Returns true if the key exists and index is valid, false otherwise
func (om *OrderedMap) MoveTo(key string, index int) bool {
	if !om.Has(key) {
		return false
	}
	if index < 0 || index >= len(om.Order) {
		return false
	}

	// Find current index
	currentIdx := -1
	for i, k := range om.Order {
		if k == key {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 || currentIdx == index {
		return false
	}

	// Remove from current position
	keyToMove := om.Order[currentIdx]
	om.Order = append(om.Order[:currentIdx], om.Order[currentIdx+1:]...)

	// Insert at target position
	if index >= len(om.Order) {
		om.Order = append(om.Order, keyToMove)
	} else {
		om.Order = append(om.Order[:index], append([]string{keyToMove}, om.Order[index:]...)...)
	}

	return true
}

// MoveToFirst moves the given key to the first position
func (om *OrderedMap) MoveToFirst(key string) bool {
	return om.MoveTo(key, 0)
}

// MoveToLast moves the given key to the last position
func (om *OrderedMap) MoveToLast(key string) bool {
	return om.MoveTo(key, len(om.Order)-1)
}
