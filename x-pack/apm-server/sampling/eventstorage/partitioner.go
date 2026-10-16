// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"iter"
	"sync"
	"sync/atomic"
)

const (
	// maxTotalPartitions is the maximum number of total partitions.
	// It is used for a sanity check specific to how we use it as a byte prefix in database keys.
	// It MUST be less than 256 to be contained in a byte.
	// It has additional (arbitrary) limitations:
	// - MUST be less than reservedKeyPrefix to avoid accidentally overwriting reserved keys down the line.
	// - MUST be less than traceIDSeparator to avoid being misinterpreted as the separator during pebble internal key comparisons
	maxTotalPartitions = int(min(reservedKeyPrefix, traceIDSeparator)) - 1

	// partitionRingSize is the number of partitions in the ring: partitionsPerTTL+1
	// active ones plus the inactive one. It sizes the stack array used by ActiveIDs.
	partitionRingSize = partitionsPerTTL + 2
)

// Partitioner is a partitioned ring with `total` number of partitions.
// 1 of them is inactive while all the others are active.
// `current` points at the rightmost active partition.
//
// Example for total=3:
// (A: active, I: inactive, ^ points at the current active entry)
// A-I-A
// ^....
// current
//
// ActiveIDs and CurrentIDFunc pin the partitions they observe, then drop the
// read lock before running caller code. Rotate publishes a new current
// immediately. WaitInactive blocks until pins on the outgoing partition reach
// zero, which is the point at which that partition may be deleted.
type Partitioner struct {
	total int // length of the ring

	mu      sync.RWMutex
	current int
	// refs[id] counts the ActiveIDs and CurrentIDFunc calls currently using
	// partition id. Pins are taken while holding mu.RLock, so once Rotate has
	// taken mu.Lock no new call can pin the outgoing partition, and WaitInactive
	// only has to wait for the existing ones.
	refs []atomic.Int32

	// waitMu and wait let WaitInactive sleep until a release drops refs to zero.
	// waiters[id] counts the WaitInactive calls on partition id, so a release
	// only takes waitMu when someone waits for the partition it just emptied.
	waitMu  sync.Mutex
	wait    *sync.Cond
	waiters []atomic.Int32
}

// NewPartitioner returns a partitioner with `actives` number of active partitions.
func NewPartitioner(actives, currentID int) *Partitioner {
	total := actives + 1 // actives + 1 inactive
	if total > partitionRingSize || total >= maxTotalPartitions {
		panic("too many partitions")
	}
	p := &Partitioner{
		total:   total,
		refs:    make([]atomic.Int32, total),
		waiters: make([]atomic.Int32, total),
		current: currentID,
	}
	p.wait = sync.NewCond(&p.waitMu)
	return p
}

// Rotate advances the ring by one partition and returns the new current
// partition and the partition that just became inactive.
//
// Rotate does not wait for in-flight readers or writers. Persist newCurrent,
// then call WaitInactive(newInactive), and only then delete that partition.
// Persisting first means a crash while WaitInactive is blocked restarts on the
// partition that is already accepting writes.
func (p *Partitioner) Rotate() (newCurrent, newInactive int) {
	p.mu.Lock()
	p.current = (p.current + 1) % p.total
	newCurrent = p.current
	newInactive = (p.current + 1) % p.total
	p.mu.Unlock()
	return newCurrent, newInactive
}

// WaitInactive blocks until every ActiveIDs or CurrentIDFunc call that pinned
// id has finished. id is the inactive partition returned by Rotate.
func (p *Partitioner) WaitInactive(id int) {
	// Counted before checking refs: a release that drops refs[id] to zero after
	// our check then sees the waiter and broadcasts, so the wake-up is not missed.
	p.waiters[id].Add(1)
	defer p.waiters[id].Add(-1)

	p.waitMu.Lock()
	defer p.waitMu.Unlock()
	for p.refs[id].Load() > 0 {
		p.wait.Wait()
	}
}

// ActiveIDs returns an iterator over the active partitions, total-1 of them,
// from current backwards. The yielded IDs are a snapshot: the read lock is not
// held across yield, and the IDs are stale if used after the loop.
//
// Each yielded partition stays readable until the loop returns, including on
// break. Calling WaitInactive from inside the loop deadlocks, because the loop
// holds the pin that WaitInactive waits for.
func (p *Partitioner) ActiveIDs() iter.Seq[int] {
	return func(yield func(int) bool) {
		var ids [partitionRingSize - 1]int
		n := p.total - 1
		p.mu.RLock()
		for i := range n {
			id := (p.current + p.total - i) % p.total
			ids[i] = id
			// Pin while holding the read lock, so Rotate cannot run between
			// choosing the ID and pinning it.
			p.refs[id].Add(1)
		}
		p.mu.RUnlock()

		defer p.release(ids, n)
		for i := range n {
			if !yield(ids[i]) {
				return
			}
		}
	}
}

// CurrentIDFunc calls f with the current partition (rightmost active).
//
// The read lock is not held across f. The partition passed to f stays readable
// until f returns, even if a later Rotate moves current on. The ID is stale if
// used after f returns.
func (p *Partitioner) CurrentIDFunc(f func(int)) {
	p.mu.RLock()
	current := p.current
	p.refs[current].Add(1)
	p.mu.RUnlock()

	defer p.releaseOne(current)
	f(current)
}

func (p *Partitioner) release(ids [partitionRingSize - 1]int, n int) {
	for i := range n {
		p.releaseOne(ids[i])
	}
}

func (p *Partitioner) releaseOne(id int) {
	if p.refs[id].Add(-1) != 0 || p.waiters[id].Load() == 0 {
		return
	}
	// Broadcast under waitMu. A waiter holds waitMu from its refs check until it
	// is registered in Wait, so the broadcast cannot land in between and be lost.
	p.waitMu.Lock()
	p.wait.Broadcast()
	p.waitMu.Unlock()
}
