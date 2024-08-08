// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// cachedEventsMap holds a counts of cached events, keyed by interval and ID.
// Cached events are events that have been processed by Aggregate methods,
// but which haven't yet been harvested. Event counts are fractional because
// an event may be spread over multiple partitions.
//
// Access to the map is protected with a mutex. During harvest, an exclusive
// (write) lock is held. Concurrent aggregations may perform atomic updates
// to the map, and the harvester may assume that the map will not be modified
// while it is reading it.
type cachedEventsMap struct {
	// (interval, id) -> count
	m         sync.Map
	countPool sync.Pool
}

func (m *cachedEventsMap) loadAndDelete(end time.Time) map[time.Duration]map[[16]byte]float64 {
	loaded := make(map[time.Duration]map[[16]byte]float64)
	m.m.Range(func(k, v any) bool {
		key := k.(cachedEventsStatsKey)
		if !end.Truncate(key.interval).Equal(end) {
			return true
		}
		intervalMetrics, ok := loaded[key.interval]
		if !ok {
			intervalMetrics = make(map[[16]byte]float64)
			loaded[key.interval] = intervalMetrics
		}
		vscaled := *v.(*uint64)
		value := float64(vscaled / math.MaxUint16)
		intervalMetrics[key.id] = value
		m.m.Delete(k)
		m.countPool.Put(v)
		return true
	})
	return loaded
}

func (m *cachedEventsMap) add(interval time.Duration, id [16]byte, n float64) {
	// We use a pool for the value to minimise allocations, as it will
	// always escape to the heap through LoadOrStore.
	nscaled, ok := m.countPool.Get().(*uint64)
	if !ok {
		nscaled = new(uint64)
	}
	// Scale by the maximum number of partitions to get an integer value,
	// for simpler atomic operations.
	*nscaled = uint64(n * math.MaxUint16)
	key := cachedEventsStatsKey{interval: interval, id: id}
	old, loaded := m.m.Load(key)
	if !loaded {
		old, loaded = m.m.LoadOrStore(key, nscaled)
		if !loaded {
			// Stored a new value.
			return
		}
	}
	atomic.AddUint64(old.(*uint64), *nscaled)
	m.countPool.Put(nscaled)
}

type cachedEventsStatsKey struct {
	interval time.Duration
	id       [16]byte
}
