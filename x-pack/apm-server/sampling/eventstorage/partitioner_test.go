// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartitioner(t *testing.T) {
	p := NewPartitioner(2, 0) // partition id 0, 1, 2

	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, 0, pid)
	})
	assert.Equal(t, []int{0, 2}, slices.Collect(p.ActiveIDs()))

	// 0 -> 1
	newCurrent, newInactive := p.Rotate()

	assert.Equal(t, 1, newCurrent)
	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, 1, pid)
	})
	assert.Equal(t, 2, newInactive)
	assert.Equal(t, []int{1, 0}, slices.Collect(p.ActiveIDs()))

	// 1 -> 2
	newCurrent, newInactive = p.Rotate()

	assert.Equal(t, 2, newCurrent)
	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, 2, pid)
	})
	assert.Equal(t, 0, newInactive)
	assert.Equal(t, []int{2, 1}, slices.Collect(p.ActiveIDs()))

	// 2 -> 0
	newCurrent, newInactive = p.Rotate()

	assert.Equal(t, 0, newCurrent)
	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, 0, pid)
	})
	assert.Equal(t, 1, newInactive)
	assert.Equal(t, []int{0, 2}, slices.Collect(p.ActiveIDs()))
}

func TestPartitionerCurrentID(t *testing.T) {
	p := NewPartitioner(2, 1)

	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, 1, pid)
	})
	assert.Equal(t, []int{1, 0}, slices.Collect(p.ActiveIDs()))
}

func TestPartitioner_ActiveIDs_Concurrent(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	p := NewPartitioner(2, 0)
	rotate := make(chan struct{})
	go func() {
		<-rotate
		newCurrent, newInactive := p.Rotate()
		assert.Equal(t, 1, newCurrent)
		assert.Equal(t, 2, newInactive)
		wg.Done()
	}()
	var activeIDs []int
	for pid := range p.ActiveIDs() {
		if rotate != nil {
			rotate <- struct{}{} // blocks
		}
		rotate = nil
		activeIDs = append(activeIDs, pid)
	}
	assert.Equal(t, []int{0, 2}, activeIDs)
	wg.Wait()
}

func TestPartitioner_CurrentIDFunc_Concurrent(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	p := NewPartitioner(2, 0)
	rotate := make(chan struct{})
	go func() {
		<-rotate
		newCurrent, newInactive := p.Rotate()
		assert.Equal(t, 1, newCurrent)
		assert.Equal(t, 2, newInactive)
		wg.Done()
	}()
	p.CurrentIDFunc(func(pid int) {
		rotate <- struct{}{} // blocks
		assert.Equal(t, 0, pid)
	})
	wg.Wait()
}
