// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"

	"github.com/elastic/apm-data/model/modelpb"
)

// ReadWriter provides a means of reading events from storage, and batched
// writing of events to storage.
//
// ReadWriter is not safe for concurrent access. All operations that involve
// a given trace ID should be performed with the same ReadWriter in order to
// avoid conflicts, e.g. by using consistent hashing to distribute to one of
// a set of ReadWriters, such as implemented by ShardedReadWriter.
type ReadWriter struct {
	s *Storage
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *ReadWriter) Close() {}

// Flush waits for preceding writes to be committed to storage.
//
// Flush must be called to ensure writes are committed to storage.
// If Flush is not called before the writer is closed, then writes
// may be lost.
func (rw *ReadWriter) Flush() error {
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *ReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	pid := rw.s.db.PartitionID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceSampled(traceID, sampled, opts)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
	currentPID := rw.s.db.PartitionID()
	prevPID := (currentPID + rw.s.db.PartitionCount() - 1) % rw.s.db.PartitionCount()
	// FIXME: this needs to be fast, as it is in the hot path
	// It should minimize disk IO on miss due to
	// 1. (pubsub) remote sampling decision
	// 2. (hot path) sampling decision not made yet
	sampled, err := NewPrefixReadWriter(rw.s.db, byte(currentPID), rw.s.codec).IsTraceSampled(traceID)
	if err == nil {
		return sampled, nil
	}

	sampled, err = NewPrefixReadWriter(rw.s.db, byte(prevPID), rw.s.codec).IsTraceSampled(traceID)
	if err == nil {
		return sampled, nil
	}

	return false, err
}

// WriteTraceEvent writes a trace event to storage.
//
// WriteTraceEvent may return before the write is committed to storage.
func (rw *ReadWriter) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	pid := rw.s.db.PartitionID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceEvent(traceID, id, event, opts)
}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *ReadWriter) DeleteTraceEvent(traceID, id string) error {
	// FIXME: use range delete
	currentPID := rw.s.db.PartitionID()
	prevPID := (currentPID + rw.s.db.PartitionCount() - 1) % rw.s.db.PartitionCount()
	return errors.Join(
		NewPrefixReadWriter(rw.s.db, byte(currentPID), rw.s.codec).DeleteTraceEvent(traceID, id),
		NewPrefixReadWriter(rw.s.db, byte(prevPID), rw.s.codec).DeleteTraceEvent(traceID, id),
	)
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *ReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	currentPID := rw.s.db.PartitionID()
	prevPID := (currentPID + rw.s.db.PartitionCount() - 1) % rw.s.db.PartitionCount()
	return errors.Join(
		NewPrefixReadWriter(rw.s.db, byte(currentPID), rw.s.codec).ReadTraceEvents(traceID, out),
		NewPrefixReadWriter(rw.s.db, byte(prevPID), rw.s.codec).ReadTraceEvents(traceID, out),
	)
}
