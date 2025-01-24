// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func TestPartitioner(t *testing.T) {
	p := eventstorage.NewPartitioner(2) // partition id 0, 1, 2

	cur := p.Current()
	assert.Equal(t, 0, cur)

	inactive := p.Inactive()
	assert.True(t, inactive.Valid())
	assert.Equal(t, 1, inactive.ID())

	active := p.Actives()
	assert.True(t, active.Valid())
	assert.Equal(t, 0, active.ID())
	active = active.Prev()
	assert.True(t, active.Valid())
	assert.Equal(t, 2, active.ID())
	active = active.Prev()
	assert.False(t, active.Valid())

	p.Rotate()

	cur = p.Current()
	assert.Equal(t, 1, cur)

	inactive = p.Inactive()
	assert.True(t, inactive.Valid())
	assert.Equal(t, 2, inactive.ID())

	active = p.Actives()
	assert.True(t, active.Valid())
	assert.Equal(t, 1, active.ID())
	active = active.Prev()
	assert.True(t, active.Valid())
	assert.Equal(t, 0, active.ID())
	active = active.Prev()
	assert.False(t, active.Valid())

	p.Rotate()

	cur = p.Current()
	assert.Equal(t, 2, cur)

	inactive = p.Inactive()
	assert.True(t, inactive.Valid())
	assert.Equal(t, 0, inactive.ID())

	active = p.Actives()
	assert.True(t, active.Valid())
	assert.Equal(t, 2, active.ID())
	active = active.Prev()
	assert.True(t, active.Valid())
	assert.Equal(t, 1, active.ID())
	active = active.Prev()
	assert.False(t, active.Valid())

	p.Rotate()

	cur = p.Current()
	assert.Equal(t, 0, cur)

	inactive = p.Inactive()
	assert.True(t, inactive.Valid())
	assert.Equal(t, 1, inactive.ID())

	active = p.Actives()
	assert.True(t, active.Valid())
	assert.Equal(t, 0, active.ID())
	active = active.Prev()
	assert.True(t, active.Valid())
	assert.Equal(t, 2, active.ID())
	active = active.Prev()
	assert.False(t, active.Valid())
}
