// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResizeReservoir(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	res := newWeightedRandomSample(rng, 2)
	res.Sample(1, "a")
	res.Sample(2, "b")
	assert.Len(t, res.Values(), 2)
	res.Resize(1)
	assert.Len(t, res.Values(), 1)
}

func TestResetReservoir(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	res := newWeightedRandomSample(rng, 2)
	res.Sample(1, "a")
	res.Sample(2, "b")
	res.Reset()
	assert.Len(t, res.Values(), 0)
}
