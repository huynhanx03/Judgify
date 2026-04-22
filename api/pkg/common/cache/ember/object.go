package ember

import (
	"math/rand"
	"sync/atomic"
)

// ObjectType defines the type of data stored inside an Object.
type ObjectType uint8

const (
	TypeString ObjectType = iota
)

// Object represents a value stored in the Ember cache with its metadata.
// All timing fields use atomic operations for thread safety.
type Object struct {
	Value any
	Type  ObjectType
	Cost  int64

	// Atomic fields for concurrent access safety.
	lastAccess  atomic.Int64  // Unix nano timestamp for LRU
	lfuCounter  atomic.Uint32 // 8-bit log counter stored as uint32 for atomics
	lfuLastDecr atomic.Int64  // Unix nano timestamp for LFU decay
}

// NewObject creates a new Object with the given type and value.
// now is the current time in unix nanoseconds.
func NewObject(typ ObjectType, value any, cost int64, now int64) *Object {
	o := &Object{
		Value: value,
		Type:  typ,
		Cost:  cost,
	}
	o.lastAccess.Store(now)
	o.lfuLastDecr.Store(now)
	o.lfuCounter.Store(uint32(LFUInitVal))
	return o
}

// Touch updates the access metadata for LRU and LFU tracking.
// Thread-safe via atomic operations. Minor imprecision in LFU counter
// under high contention is acceptable (same trade-off as Redis).
func (o *Object) Touch(now int64) {
	o.lastAccess.Store(now)

	lastDecr := o.lfuLastDecr.Load()
	diff := now - lastDecr
	counter := uint8(o.lfuCounter.Load())

	// Decay the LFU counter based on elapsed time
	if diff > LFUDecayInterval.Nanoseconds() {
		decay := uint8(diff / LFUDecayInterval.Nanoseconds())
		if decay > counter {
			counter = 0
		} else {
			counter -= decay
		}
		o.lfuLastDecr.Store(now)
	}

	// Logarithmic increment: p = 1/((counter - base) * factor + 1)
	if counter < 255 {
		p := 1.0 / (float64(counter-LFUInitVal)*float64(LFULogFactor) + 1.0)
		if counter < LFUInitVal {
			p = 1.0
		}
		if rand.Float64() < p {
			counter++
		}
	}
	o.lfuCounter.Store(uint32(counter))
}

// LastAccess returns the last access timestamp (unix nano).
func (o *Object) LastAccess() int64 {
	return o.lastAccess.Load()
}

// LFUCounter returns the current LFU counter value.
func (o *Object) LFUCounter() uint8 {
	return uint8(o.lfuCounter.Load())
}

// TypeName returns the string representation of the object's type.
func (o *Object) TypeName() string {
	switch o.Type {
	case TypeString:
		return "string"
	default:
		return "unknown"
	}
}
