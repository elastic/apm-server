// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitioner(t *testing.T) {
	p := NewPartitioner(2, 0) // partition id 0, 1, 2

	assert.Equal(t, 0, p.CurrentID())
	assert.Equal(t, []int{0, 2}, slices.Collect(p.ActiveIDs()))

	// 0 -> 1
	newCurrent, newInactive := p.Rotate()

	assert.Equal(t, 1, newCurrent)
	assert.Equal(t, 1, p.CurrentID())
	assert.Equal(t, 2, newInactive)
	assert.Equal(t, []int{1, 0}, slices.Collect(p.ActiveIDs()))

	// 1 -> 2
	newCurrent, newInactive = p.Rotate()

	assert.Equal(t, 2, newCurrent)
	assert.Equal(t, 2, p.CurrentID())
	assert.Equal(t, 0, newInactive)
	assert.Equal(t, []int{2, 1}, slices.Collect(p.ActiveIDs()))

	// 2 -> 0
	newCurrent, newInactive = p.Rotate()

	assert.Equal(t, 0, newCurrent)
	assert.Equal(t, 0, p.CurrentID())
	assert.Equal(t, 1, newInactive)
	assert.Equal(t, []int{0, 2}, slices.Collect(p.ActiveIDs()))
}

func TestPartitionerCurrentID(t *testing.T) {
	p := NewPartitioner(2, 1)

	assert.Equal(t, 1, p.CurrentID())
	assert.Equal(t, []int{1, 0}, slices.Collect(p.ActiveIDs()))
}

func TestPartitioner_ActiveIDs_Concurrent(t *testing.T) {
	wg := sync.WaitGroup{}
	p := NewPartitioner(2, 0)
	require.Equal(t, []int{0, 2}, slices.Collect(p.ActiveIDs()))
	done := make(chan struct{})
	wg.Go(func() {
		newCurrent, newInactive := p.Rotate()
		assert.Equal(t, 1, newCurrent)
		assert.Equal(t, 2, newInactive)
		close(done)
	})
	wg.Go(func() {
		_ = slices.Collect(p.ActiveIDs()) // ensure no data race by calling concurrently
		<-done
		// check eventual state
		assert.Equal(t, []int{1, 0}, slices.Collect(p.ActiveIDs()))
	})
	wg.Wait()
}

func TestPartitioner_CurrentIDFunc_Concurrent(t *testing.T) {
	// Rotate() concurrently with CurrentID()
	// Ensure no data race, and end state is reached eventually.
	wg := sync.WaitGroup{}
	p := NewPartitioner(2, 0)
	require.Equal(t, 0, p.CurrentID())
	done := make(chan struct{})
	wg.Go(func() {
		newCurrent, newInactive := p.Rotate()
		assert.Equal(t, 1, newCurrent)
		assert.Equal(t, 2, newInactive)
		close(done)
	})
	wg.Go(func() {
		id := p.CurrentID() // ensure no data race, but return value can be either 0 or 1
		assert.GreaterOrEqual(t, id, 0)
		assert.LessOrEqual(t, id, 1)
		<-done
		// check eventual state
		assert.Equal(t, 1, p.CurrentID())
	})
	wg.Wait()
}
