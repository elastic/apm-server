// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package baseaggregator

import "github.com/pkg/errors"

// Space models a backing array for aggregators to store their metrics. The backing
// array pre-allocates the required number of entries to reduce GC overhead.
//
// Requires protection using mutexes for concurrent usage by the caller.
// TODO: Update all aggregators to use Space.
type Space[k any] struct {
	index int
	space []k
}

// NewSpace creates a new space given the max number of elements.
func NewSpace[k any](limit int) *Space[k] {
	return &Space[k]{
		space: make([]k, limit),
	}
}

// Next returns the next entry from the space or error if no more entries can be
// returned.
func (s *Space[k]) Next() (*k, error) {
	if s.index == len(s.space) {
		return nil, errors.New("all entries are used")
	}
	e := &s.space[s.index]
	s.index++
	return e, nil
}

// Reset resets the space to be reused. Note that reset doesn't reset the fields
// for the struct and they must be updated or reset by the caller.
func (s *Space[k]) Reset() {
	s.index = 0
}
