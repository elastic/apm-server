// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartitioner(t *testing.T) {
	p := NewPartitioner(2, 0) // partition id 0, 1, 2

	assert.Equal(t, 0, p.currentID())
	assert.Equal(t, 1, p.inactiveID())
	assert.Equal(t, []int{0, 2}, slices.Collect(p.activeIDs()))

	// 0 -> 1
	newCurrent := p.Rotate()

	assert.Equal(t, 1, newCurrent)
	assert.Equal(t, 1, p.currentID())
	assert.Equal(t, 2, p.inactiveID())
	assert.Equal(t, []int{1, 0}, slices.Collect(p.activeIDs()))

	// 1 -> 2
	newCurrent = p.Rotate()

	assert.Equal(t, 2, newCurrent)
	assert.Equal(t, 2, p.currentID())
	assert.Equal(t, 0, p.inactiveID())
	assert.Equal(t, []int{2, 1}, slices.Collect(p.activeIDs()))

	// 2 -> 0
	newCurrent = p.Rotate()

	assert.Equal(t, 0, newCurrent)
	assert.Equal(t, 0, p.currentID())
	assert.Equal(t, 1, p.inactiveID())
	assert.Equal(t, []int{0, 2}, slices.Collect(p.activeIDs()))
}

func TestPartitionerCurrentID(t *testing.T) {
	p := NewPartitioner(2, 1)

	assert.Equal(t, 1, p.currentID())
	assert.Equal(t, 2, p.inactiveID())
	assert.Equal(t, []int{1, 0}, slices.Collect(p.activeIDs()))
}
