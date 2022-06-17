// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

const (
	defaultStorageLimit = 3 * 1024 * 1024 * 1024 // 3GB storage limit.
	defaultThreshold    = 0.90                   // Allow 90% of the quota to be used.
	defaultPeriod       = 100 * time.Millisecond
)

var zeroLimiter = NewMockLimiter(0, 0, 1)

// Abstracts away the calls to badger.DB.Size(), which return the size of
// LSM and vlog.
type badgerSizer interface {
	Size() (lsm int64, vlog int64)
}

// LimiterCfg configuration options.
type LimiterCfg struct {
	// The instance to the badger database.
	DB badgerSizer

	// Set the limit in bytes
	Limit int64

	// The threshold from which to start returning true on l.reached()
	Threshold float64
}

// Limiter wraps a badger database with a storage size limit, allowing callers
// to explicitly limit writes to the badger database by checking if the current
// size is equal or above the configured size limit.
type Limiter struct {
	// Instance to the *badger.DB.
	db badgerSizer

	// limit represents the storage limit in Bytes.
	limit int64

	// local cache of the last updated value for LSM and vlog files.
	lsm, vlog int64
}

// NewLimiter returns an instance of Limiter or an error when the badger
// database is nil. The configured limit will multiply the config's limit
// by the threshold. This means that the hard will be set to 90% of the
// set limit.
// If the configured limit is zero or negative, 3GB will be used.
// If the configured threshold is zero or negative, 0.9 will be used (90%).
func NewLimiter(cfg LimiterCfg) (*Limiter, error) {
	if cfg.DB == nil {
		return nil, errors.New("eventstorage: badger.DB cannot be nil")
	}
	if cfg.Limit <= 0 {
		cfg.Limit = defaultStorageLimit
	}
	if cfg.Threshold <= 0 || cfg.Threshold < 1.0 {
		cfg.Threshold = defaultThreshold
	}
	return &Limiter{
		db:    cfg.DB,
		limit: int64(float64(cfg.Limit) * cfg.Threshold),
	}, nil
}

// Size returns the stored size of the lsm and vlog.
func (l *Limiter) Size() (lsm, vlog int64) {
	return atomic.LoadInt64(&l.lsm), atomic.LoadInt64(&l.vlog)
}

// Reached returns ture when the configured threshold has reached or exceeded.
func (l *Limiter) Reached() bool {
	lsm, vlog := l.Size()
	current := lsm + vlog
	return current >= l.limit
}

// PollSize checks the badger database LSM and Vlog size and updates its local
// value.
func (l *Limiter) PollSize(ctx context.Context, period time.Duration) {
	if period < defaultPeriod {
		period = defaultPeriod
	}
	t := time.NewTicker(period)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			lsm, vlog := l.db.Size()
			atomic.StoreInt64(&l.lsm, lsm)
			atomic.StoreInt64(&l.vlog, vlog)
		}
	}
}

type mockSizer struct {
	lsm, vlog int64
}

func (s mockSizer) Size() (int64, int64) { return s.lsm, s.vlog }

// NewMockLimiter is used to create a limiter with a fake badger.DB adapter
// which always returns the size it's been created with.
func NewMockLimiter(lsm, vlog, limit int64) *Limiter {
	return &Limiter{
		db:    mockSizer{lsm: lsm, vlog: vlog},
		limit: limit,
	}
}
