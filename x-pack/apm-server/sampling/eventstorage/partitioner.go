// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import "sync/atomic"

type Partitioner struct {
	total   int // length of the ring
	current atomic.Int32
}

func NewPartitioner(actives int) *Partitioner {
	return &Partitioner{total: actives + 1} // actives + 1 inactive
}

func (p *Partitioner) Rotate() {
	p.current.Store(int32((int(p.current.Load()) + 1) % p.total))
}

func (p *Partitioner) Actives() PartitionIterator {
	return PartitionIterator{
		id:        int(p.current.Load()),
		remaining: p.total - 1 - 1,
		total:     p.total,
	}
}
func (p *Partitioner) Current() PartitionIterator {
	return PartitionIterator{
		id:        int(p.current.Load()),
		remaining: 0,
		total:     p.total,
	}
}

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
