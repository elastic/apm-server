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

package beater

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/elastic/beats/v7/libbeat/beat"
)

// waitPublishedAcker is a beat.ACKer which keeps track of the number of
// events published. waitPublishedAcker provides an interruptible Wait method
// that blocks until all clients are closed, and all published events at the
// time the clients are closed are acknowledged.
type waitPublishedAcker struct {
	active int64 // atomic

	mu    sync.Mutex
	empty *sync.Cond
}

// newWaitPublishedAcker returns a new waitPublishedAcker.
func newWaitPublishedAcker() *waitPublishedAcker {
	acker := &waitPublishedAcker{}
	acker.empty = sync.NewCond(&acker.mu)
	return acker
}

// AddEvent is called when an event has been published or dropped by the client,
// and increments a counter for published events.
func (w *waitPublishedAcker) AddEvent(event beat.Event, published bool) {
	if published {
		w.incref(1)
	}
}

// ACKEvents is called when published events have been acknowledged.
func (w *waitPublishedAcker) ACKEvents(n int) {
	w.decref(int64(n))
}

// Open must be called exactly once before any new pipeline client is opened,
// incrementing the acker's reference count.
func (w *waitPublishedAcker) Open() {
	w.incref(1)
}

// Close is called when a pipeline client is closed, and decrements the
// acker's reference count.
//
// This must be called at most once for each call to Open.
func (w *waitPublishedAcker) Close() {
	w.decref(1)
}

func (w *waitPublishedAcker) incref(n int64) {
	atomic.AddInt64(&w.active, 1)
}

func (w *waitPublishedAcker) decref(n int64) {
	if atomic.AddInt64(&w.active, int64(-n)) == 0 {
		w.empty.Broadcast()
	}
}

// Wait waits for w to be closed and all previously published events to be
// acknowledged.
func (w *waitPublishedAcker) Wait(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		w.empty.Broadcast()
	}()

	w.mu.Lock()
	defer w.mu.Unlock()
	for atomic.LoadInt64(&w.active) != 0 && ctx.Err() == nil {
		w.empty.Wait()
	}
	return ctx.Err()
}
