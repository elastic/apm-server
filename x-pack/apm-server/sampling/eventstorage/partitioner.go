// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import "sync/atomic"

const (
	// maxTotalPartitions is the maximum number of total partitions.
	// It is used for a sanity check specific to how we use it as a byte prefix in database keys.
	// It MUST be less than 256 to be contained in a byte.
	// It has an additional (arbitrary) limitation to be less than reservedKeyPrefix
	// to avoid accidentally overwriting reserved keys down the line.
	maxTotalPartitions = int(reservedKeyPrefix) - 1
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
	total   int // length of the ring
	current atomic.Int32
}

// NewPartitioner returns a partitioner with `actives` number of active partitions.
func NewPartitioner(actives int) *Partitioner {
	total := actives + 1 // actives + 1 inactive
	if total >= maxTotalPartitions {
		panic("too many partitions")
	}
	return &Partitioner{total: total}
}

// SetCurrentID sets the input partition ID as current partition.
func (p *Partitioner) SetCurrentID(current int) {
	p.current.Store(int32(current))
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
func (p *Partitioner) Rotate() int {
	newCurrent := (int(p.current.Load()) + 1) % p.total
	p.current.Store(int32(newCurrent))
	return newCurrent
}

// Actives returns a PartitionIterator containing all active partitions.
// It contains total - 1 partitions.
func (p *Partitioner) Actives() PartitionIterator {
	return PartitionIterator{
		id:        int(p.current.Load()),
		remaining: p.total - 1 - 1,
		total:     p.total,
	}
}

// Inactive returns a PartitionIterator pointing to the inactive partition.
// It contains only 1 partition.
func (p *Partitioner) Inactive() PartitionIterator {
	return PartitionIterator{
		id:        (int(p.current.Load()) + 1) % p.total,
		remaining: 0,
		total:     p.total,
	}
}

// Current returns the ID of the current partition (rightmost active).
func (p *Partitioner) Current() int {
	return int(p.current.Load())
}

// PartitionIterator is for iterating on partition results.
// In theory Partitioner could have returned a slice of partition IDs,
// but returning an iterator should avoid allocs.
//
// Example usage:
// for it := rw.s.db.ReadPartitions(); it.Valid(); it = it.Prev() {
// // do something with it.ID()
// }
type PartitionIterator struct {
	id        int
	total     int // length of the ring
	remaining int
}

func (it PartitionIterator) Prev() PartitionIterator {
	return PartitionIterator{
		id:        (it.id + it.total - 1) % it.total,
		remaining: it.remaining - 1,
		total:     it.total,
	}
}

func (it PartitionIterator) Valid() bool {
	return it.remaining >= 0
}

func (it PartitionIterator) ID() int {
	return it.id
}
