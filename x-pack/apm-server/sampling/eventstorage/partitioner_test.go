package eventstorage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func TestPartitioner(t *testing.T) {
	p := eventstorage.NewPartitioner(2) // partition id 0, 1, 2

	cur := p.Current()
	assert.True(t, cur.Valid())
	assert.Equal(t, 0, cur.ID())

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
	assert.True(t, cur.Valid())
	assert.Equal(t, 1, cur.ID())

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
	assert.True(t, cur.Valid())
	assert.Equal(t, 2, cur.ID())

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
	assert.True(t, cur.Valid())
	assert.Equal(t, 0, cur.ID())

	active = p.Actives()
	assert.True(t, active.Valid())
	assert.Equal(t, 0, active.ID())
	active = active.Prev()
	assert.True(t, active.Valid())
	assert.Equal(t, 2, active.ID())
	active = active.Prev()
	assert.False(t, active.Valid())
}
