// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage

import (
	"runtime"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/hashicorp/go-multierror"

	"github.com/elastic/apm-server/model"
)

// ShardedReadWriter provides sharded, locked, access to a Storage.
//
// ShardedReadWriter shards on trace ID.
type ShardedReadWriter struct {
	readWriters []lockedReadWriter
}

func newShardedReadWriter(storage *Storage) *ShardedReadWriter {
	s := &ShardedReadWriter{
		// Create as many ReadWriters as there are CPUs,
		// so we can ideally minimise lock contention.
		readWriters: make([]lockedReadWriter, runtime.NumCPU()),
	}
	for i := range s.readWriters {
		s.readWriters[i].rw = storage.NewReadWriter()
	}
	return s
}

// Close closes all sharded storage readWriters.
func (s *ShardedReadWriter) Close() {
	for i := range s.readWriters {
		s.readWriters[i].Close()
	}
}

// Flush flushes all sharded storage readWriters.
func (s *ShardedReadWriter) Flush() error {
	var result error
	for i := range s.readWriters {
		if err := s.readWriters[i].Flush(); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// ReadEvents calls Writer.ReadEvents, using a sharded, locked, Writer.
func (s *ShardedReadWriter) ReadEvents(traceID string, out *model.Batch) error {
	return s.getWriter(traceID).ReadEvents(traceID, out)
}

// WriteTransaction calls Writer.WriteTransaction, using a sharded, locked, Writer.
func (s *ShardedReadWriter) WriteTransaction(tx *model.Transaction) error {
	return s.getWriter(tx.TraceID).WriteTransaction(tx)
}

// WriteSpan calls Writer.WriteSpan, using a sharded, locked, Writer.
func (s *ShardedReadWriter) WriteSpan(span *model.Span) error {
	return s.getWriter(span.TraceID).WriteSpan(span)
}

// WriteTraceSampled calls Writer.WriteTraceSampled, using a sharded, locked, Writer.
func (s *ShardedReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	return s.getWriter(traceID).WriteTraceSampled(traceID, sampled)
}

// IsTraceSampled calls Writer.IsTraceSampled, using a sharded, locked, Writer.
func (s *ShardedReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.getWriter(traceID).IsTraceSampled(traceID)
}

// DeleteTransaction calls Writer.DeleteTransaction, using a sharded, locked, Writer.
func (s *ShardedReadWriter) DeleteTransaction(tx *model.Transaction) error {
	return s.getWriter(tx.TraceID).DeleteTransaction(tx)
}

// DeleteSpan calls Writer.DeleteSpan, using a sharded, locked, Writer.
func (s *ShardedReadWriter) DeleteSpan(span *model.Span) error {
	return s.getWriter(span.TraceID).DeleteSpan(span)
}

// getWriter returns an event storage writer for the given trace ID.
//
// This method is idempotent, which is necessary to avoid transaction
// conflicts and ensure all events are reported once a sampling decision
// has been recorded.
func (s *ShardedReadWriter) getWriter(traceID string) *lockedReadWriter {
	var h xxhash.Digest
	h.WriteString(traceID)
	return &s.readWriters[h.Sum64()%uint64(len(s.readWriters))]
}

type lockedReadWriter struct {
	mu sync.Mutex
	rw *ReadWriter
}

func (rw *lockedReadWriter) Close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.rw.Close()
}

func (rw *lockedReadWriter) Flush() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.Flush()
}

func (rw *lockedReadWriter) ReadEvents(traceID string, out *model.Batch) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.ReadEvents(traceID, out)
}

func (rw *lockedReadWriter) WriteTransaction(tx *model.Transaction) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.WriteTransaction(tx)
}

func (rw *lockedReadWriter) WriteSpan(s *model.Span) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.WriteSpan(s)
}

func (rw *lockedReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.WriteTraceSampled(traceID, sampled)
}

func (rw *lockedReadWriter) IsTraceSampled(traceID string) (bool, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.IsTraceSampled(traceID)
}

func (rw *lockedReadWriter) DeleteTransaction(tx *model.Transaction) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.DeleteTransaction(tx)
}

func (rw *lockedReadWriter) DeleteSpan(span *model.Span) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.rw.DeleteSpan(span)
}
