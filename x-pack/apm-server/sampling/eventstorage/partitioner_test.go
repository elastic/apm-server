// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func iterToSlice[T any](it iter.Seq[T]) (s []T) {
	for i := range it {
		s = append(s, i)
	}
	return
}

func TestPartitioner(t *testing.T) {
	p := eventstorage.NewPartitioner(2, 0) // partition id 0, 1, 2

	assert.Equal(t, 0, p.Current())
	assert.Equal(t, 1, p.Inactive())
	assert.Equal(t, []int{0, 2}, iterToSlice(p.Actives()))

	// 0 -> 1
	p.Rotate()

	assert.Equal(t, 1, p.Current())
	assert.Equal(t, 2, p.Inactive())
	assert.Equal(t, []int{1, 0}, iterToSlice(p.Actives()))

	// 1 -> 2
	p.Rotate()

	assert.Equal(t, 2, p.Current())
	assert.Equal(t, 0, p.Inactive())
	assert.Equal(t, []int{2, 1}, iterToSlice(p.Actives()))

	// 2 -> 0
	p.Rotate()

	assert.Equal(t, 0, p.Current())
	assert.Equal(t, 1, p.Inactive())
	assert.Equal(t, []int{0, 2}, iterToSlice(p.Actives()))
}

func TestPartitionerCurrentID(t *testing.T) {
	p := eventstorage.NewPartitioner(2, 1)

	assert.Equal(t, 1, p.Current())
	assert.Equal(t, 2, p.Inactive())
	assert.Equal(t, []int{1, 0}, iterToSlice(p.Actives()))
}
