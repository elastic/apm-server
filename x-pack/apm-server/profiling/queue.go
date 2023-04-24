// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"context"
	"sync"
	"time"

	lru "github.com/hnlq715/golang-lru/simplelru"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

type QueueConfig struct {
	Size,
	CacheSize int
	FlushPeriod,
	FlushTimeout time.Duration

	// CacheItemExpiration is the expiration time for LRU entries.
	// This solves the following issue:
	// Items (leaf frames or executables) that are regularly seen would never
	// be queued again because they stay in the LRU. That means they will not
	// be symbolized again, in case the symbolization failed.
	CacheItemExpiration time.Duration
}

func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Size:                128,
		CacheSize:           1024,
		FlushPeriod:         10 * time.Second,
		FlushTimeout:        10 * time.Second,
		CacheItemExpiration: time.Hour * 1,
	}
}

type SymQueue[V any] struct {
	// `known` and `data` needs to be synchronized
	mutex sync.RWMutex
	known *lru.LRU
	data  []V

	dataC  chan []V
	doneC  chan bool
	ticker *time.Ticker
	config QueueConfig
}

// FlushCallback is used to get a callback when a cache entry is evicted
type FlushCallback[V any] func(context.Context, []V)

func (q *SymQueue[V]) Add(datum V) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if _, ok := q.known.Get(datum); ok {
		return
	}

	q.known.Add(datum, libpf.Void{})
	q.ticker.Reset(q.config.FlushPeriod)
	q.data = append(q.data, datum)

	if len(q.data) >= q.config.Size {
		q.flushData()
	}
}

func (q *SymQueue[V]) flushData() {
	select {
	case q.dataC <- q.data:
		// data successfully stored in channel
		break
	default:
		// log.Warnf("%d queued entries have been dropped", len(q.data))

		// clean up the cache
		for i := 0; i < len(q.data); i++ {
			q.known.Remove(q.data[i])
		}
	}

	q.data = make([]V, 0, q.config.Size)
}

// Close stops the internal timer and the go routine.
func (q *SymQueue[V]) Close() error {
	q.ticker.Stop()

	q.dataC <- nil // signal Go routine to stop
	<-q.doneC      // wait until Go routine stopped

	return nil
}

func NewQueue[V any](config QueueConfig, onFlush FlushCallback[V]) *SymQueue[V] {
	q := &SymQueue[V]{}
	q.config = config
	q.known, _ = lru.NewLRUWithExpire(config.CacheSize, config.CacheItemExpiration, nil)
	q.data = make([]V, 0, config.Size)
	q.ticker = time.NewTicker(config.FlushPeriod)
	q.dataC = make(chan []V, 1)
	q.doneC = make(chan bool)

	go func() {
		for {
			select {
			case <-q.ticker.C:
				q.mutex.Lock()
				if len(q.data) > 0 {
					q.flushData()
				}
				q.mutex.Unlock()
			case data := <-q.dataC:
				if data == nil {
					q.doneC <- true
					return
				}
				ctx, cancel := context.WithTimeout(context.TODO(), q.config.FlushTimeout)
				onFlush(ctx, data)
				cancel()
			}
		}
	}()

	return q
}
