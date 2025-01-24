package eventstorage

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
)

func TestEventComparer(t *testing.T) {
	err := pebble.CheckComparer(eventComparer(), [][]byte{
		nil,
		[]byte("12!"),
		[]byte("123!"),
		[]byte("foo1!"),
		[]byte("foo12!"),
		[]byte("foo2!"),
	}, [][]byte{
		nil,
		[]byte("12"),
		[]byte("123"),
		[]byte("bar1"),
		[]byte("bar12"),
		[]byte("bar2"),
	})
	assert.NoError(t, err)
}
