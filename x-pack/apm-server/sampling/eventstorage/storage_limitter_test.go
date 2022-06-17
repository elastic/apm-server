// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimiter(t *testing.T) {
	test := func(t *testing.T, lsmSize, vlogSize, limit int64) *Limiter {
		limiter, err := NewLimiter(LimiterCfg{
			DB:    mockSizer{lsm: lsmSize, vlog: vlogSize},
			Limit: limit,
		})
		require.NoError(t, err)
		lsm, vlog := limiter.Size()
		assert.Zero(t, lsm)
		assert.Zero(t, vlog)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go limiter.PollSize(ctx, defaultPeriod)

		time.Sleep(defaultPeriod * 2)
		lsm, vlog = limiter.Size()
		assert.Equal(t, lsmSize, lsm)
		assert.Equal(t, vlogSize, vlog)
		return limiter
	}
	t.Run("limit_reached", func(t *testing.T) {
		limiter := test(t, 1024, 1024*1024, 4096)
		assert.True(t, limiter.Reached())
	})
	t.Run("no_limit_reached", func(t *testing.T) {
		limiter := test(t, 1024, 1024*1024, 10*1024*1024)
		assert.False(t, limiter.Reached())
	})
}
