// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"iter"
	"sync"
)

const (
	// maxTotalPartitions is the maximum number of total partitions.
	// It is used for a sanity check specific to how we use it as a byte prefix in database keys.
	// It MUST be less than 256 to be contained in a byte.
	// It has additional (arbitrary) limitations:
	// - MUST be less than reservedKeyPrefix to avoid accidentally overwriting reserved keys down the line.
	// - MUST be less than traceIDSeparator to avoid being misinterpreted as the separator during pebble internal key comparisons
	maxTotalPartitions = int(min(reservedKeyPrefix, traceIDSeparator)) - 1
)

// Partitioner is a partitioned ring with `total` number of partitions.
// 1 of them is inactive while all the others are active.
// `current` points at the rightmost active partition.
//
// Example for total=4:
// (A: active, I: inactive, ^ points at the current active entry)
// A-I-A-A
// ^......
// current
type Partitioner struct {
	total     int // length of the ring. constant
	current   int // current active partition id, protected by currentMu
	currentMu sync.RWMutex
}

// NewPartitioner returns a partitioner with `actives` number of active partitions.
func NewPartitioner(actives, currentID int) *Partitioner {
	total := actives + 1 // actives + 1 inactive
	if total >= maxTotalPartitions {
		panic("too many partitions")
	}
	return &Partitioner{total: total, current: currentID}
}

// Rotate rotates partitions to the right by 1 position and
// returns the ID of the new current active entry.
//
// Example for total=4:
// (A: active, I: inactive, ^ points at the current active entry)
// A-I-A-A
// ^......
//
// After Rotate:
// A-A-I-A
// ..^....
func (p *Partitioner) Rotate() (newCurrent, newInactive int) {
	p.currentMu.Lock()
	defer p.currentMu.Unlock()
	p.current = (p.current + 1) % p.total
	return p.current, (p.current + 1) % p.total
}

// ActiveIDs returns an iterator containing all active partitions.
// It contains total - 1 partitions.
//
// There is no guarantee that the returned IDs are up to date after the function returns.
func (p *Partitioner) ActiveIDs() iter.Seq[int] {
	p.currentMu.RLock()
	defer p.currentMu.RUnlock()
	current := p.current
	total := p.total
	return func(yield func(int) bool) {
		// all variables in iterator func are local vars
		for i := 0; i < total-1; i++ {
			if !yield((current + total - i) % total) {
				return
			}
		}
	}
}

// CurrentID returns the ID of the current partition (rightmost active).
//
// There is no guarantee that the returned ID is up to date after the function returns.
func (p *Partitioner) CurrentID() int {
	p.currentMu.RLock()
	defer p.currentMu.RUnlock()
	return p.current
}
