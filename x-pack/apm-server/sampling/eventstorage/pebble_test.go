// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
)

func TestEventComparer(t *testing.T) {
	err := pebble.CheckComparer(eventComparer(), [][]byte{
		[]byte("12:"),
		[]byte("123:"),
		[]byte("foo1:"),
		[]byte("foo12:"),
		[]byte("foo2:"),
	}, [][]byte{
		[]byte("12"),
		[]byte("123"),
		[]byte("bar1"),
		[]byte("bar12"),
		[]byte("bar2"),
	})
	assert.NoError(t, err)
}
