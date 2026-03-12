package collections

import (
	"github.com/topxeq/nxlang/types"
)

type RangeIterator struct {
	start   int
	end     int
	step    int
	current int
	done    bool
}

func NewRangeIterator(start, end, step int) *RangeIterator {
	return &RangeIterator{
		start:   start,
		end:     end,
		step:    step,
		current: start,
		done:    start >= end && step > 0 || start <= end && step < 0,
	}
}

func (ri *RangeIterator) TypeCode() uint8 {
	return 0x40 // New type code for range iterator
}

func (ri *RangeIterator) TypeName() string {
	return "range"
}

func (ri *RangeIterator) ToStr() string {
	return "range()"
}

func (ri *RangeIterator) Equals(other types.Object) bool {
	return false
}

func (ri *RangeIterator) Next() types.Object {
	if ri.done {
		return types.UndefinedValue
	}
	val := types.Int(ri.current)
	ri.current += ri.step
	if ri.step > 0 && ri.current >= ri.end {
		ri.done = true
	} else if ri.step < 0 && ri.current <= ri.end {
		ri.done = true
	}
	return val
}

func (ri *RangeIterator) HasNext() bool {
	return !ri.done
}
