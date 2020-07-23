// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package publish

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/elastic/beats/v7/libbeat/beat"
)

// waitPublishedAcker is a beat.ACKer which keeps track of the number of
// events published. waitPublishedAcker provides an interruptible Wait method
// that blocks until all events published at the time the client is closed are
// acknowledged.
type waitPublishedAcker struct {
	active int64 // atomic

	mu     sync.RWMutex
	closed bool
	done   chan struct{}
}

func newWaitPublishedAcker() *waitPublishedAcker {
	return &waitPublishedAcker{done: make(chan struct{})}
}

// AddEvent is called when an event has been published or dropped by the client,
// and increments a counter for published events.
func (w *waitPublishedAcker) AddEvent(event beat.Event, published bool) {
	if !published {
		return
	}
	atomic.AddInt64(&w.active, 1)
}

// ACKEvents is called when published events have been acknowledged.
func (w *waitPublishedAcker) ACKEvents(n int) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if atomic.AddInt64(&w.active, int64(-n)) == 0 && w.closed {
		close(w.done)
	}
}

// Close closes w, unblocking Wait when all previously published events have
// been acknowledged.
func (w *waitPublishedAcker) Close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

// Wait waits for w to be closed and all previously published events to be
// acknowledged.
func (w *waitPublishedAcker) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.done:
		return nil
	}
}
