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
