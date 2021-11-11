// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling

import (
	"container/heap"
	"math"
	"math/rand"
)

// weightedRandomSample provides a weighted, random, reservoir sampling
// implementing Algorithm A-Res (Algorithm A with Reservoir).
//
// See https://en.wikipedia.org/wiki/Reservoir_sampling#Algorithm_A-Res
type weightedRandomSample struct {
	rng *rand.Rand
	itemheap
}

// newWeightedRandomSample constructs a new weighted random sampler,
// with the given random number generator and reservoir size.
func newWeightedRandomSample(rng *rand.Rand, reservoirSize int) *weightedRandomSample {
	return &weightedRandomSample{
		rng: rng,
		itemheap: itemheap{
			keys:   make([]float64, 0, reservoirSize),
			values: make([]string, 0, reservoirSize),
		},
	}
}

// Sample records a trace ID with a random probability, proportional to
// the given weight in the range [0, math.MaxFloat64].
func (s *weightedRandomSample) Sample(weight float64, traceID string) bool {
	k := math.Pow(s.rng.Float64(), 1/weight)
	if len(s.values) < cap(s.values) {
		heap.Push(&s.itemheap, item{key: k, value: traceID})
		return true
	}
	if k > s.keys[0] {
		s.keys[0] = k
		s.values[0] = traceID
		heap.Fix(&s.itemheap, 0)
		return true
	}
	return false
}

// Reset clears the current values, retaining the underlying storage space.
func (s *weightedRandomSample) Reset() {
	s.keys = s.keys[:0]
	s.values = s.values[:0]
}

// Size returns the reservoir capacity.
func (s *weightedRandomSample) Size() int {
	return cap(s.keys)
}

// Resize resizes the reservoir capacity to n.
//
// Resize is not guaranteed to retain the items with the greatest weight,
// due to randomisation.
func (s *weightedRandomSample) Resize(n int) {
	if n > cap(s.keys) {
		// Increase capacity by copying into a new slice.
		keys := make([]float64, len(s.keys), n)
		values := make([]string, len(s.values), n)
		copy(keys, s.keys)
		copy(values, s.values)
		s.keys = keys
		s.values = values
	} else if cap(s.keys) > n {
		for len(s.keys) > n {
			heap.Pop(&s.itemheap)
		}
		s.keys = s.keys[0:len(s.keys):n]
		s.values = s.values[0:len(s.values):n]
	}
}

// Pop removes the trace ID with the lowest weight.
// Pop panics when called on an empty reservoir.
func (s *weightedRandomSample) Pop() string {
	item := heap.Pop(&s.itemheap).(item)
	return item.value
}

// Values returns a copy of at most n of the currently sampled trace IDs.
func (s *weightedRandomSample) Values() []string {
	values := make([]string, len(s.values))
	copy(values, s.values)
	return values
}

type itemheap struct {
	keys   []float64
	values []string
}

type item struct {
	key   float64
	value string
}

func (h itemheap) Len() int           { return len(h.keys) }
func (h itemheap) Less(i, j int) bool { return h.keys[i] < h.keys[j] }
func (h itemheap) Swap(i, j int) {
	h.keys[i], h.keys[j] = h.keys[j], h.keys[i]
	h.values[i], h.values[j] = h.values[j], h.values[i]
}

func (h *itemheap) Push(x interface{}) {
	item := x.(item)
	h.keys = append(h.keys, item.key)
	h.values = append(h.values, item.value)
}

func (h *itemheap) Pop() interface{} {
	n := len(h.keys)
	item := item{
		key:   h.keys[n-1],
		value: h.values[n-1],
	}
	h.keys = h.keys[:n-1]
	h.values = h.values[:n-1]
	return item
}
