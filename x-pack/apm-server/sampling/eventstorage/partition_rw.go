// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"

	"github.com/elastic/apm-data/model/modelpb"
)

// PartitionReadWriter provides a means of reading events from storage, and batched
// writing of events to storage.
type PartitionReadWriter struct {
	s *Storage
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *PartitionReadWriter) Close() {}

// Flush waits for preceding writes to be committed to storage.
//
// Flush must be called to ensure writes are committed to storage.
// If Flush is not called before the writer is closed, then writes
// may be lost.
func (rw *PartitionReadWriter) Flush() error {
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *PartitionReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	pid := rw.s.db.WritePartition().ID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceSampled(traceID, sampled, opts)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *PartitionReadWriter) IsTraceSampled(traceID string) (bool, error) {
	// FIXME: this needs to be fast, as it is in the hot path
	// It should minimize disk IO on miss due to
	// 1. (pubsub) remote sampling decision
	// 2. (hot path) sampling decision not made yet
	var errs []error
	for it := rw.s.db.ReadPartitions(); it.Valid(); it = it.Prev() {
		sampled, err := NewPrefixReadWriter(rw.s.db, byte(it.ID()), rw.s.codec).IsTraceSampled(traceID)
		if err == nil {
			return sampled, nil
		} else if err != ErrNotFound {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return false, errors.Join(errs...)
	}
	return false, ErrNotFound
}

// WriteTraceEvent writes a trace event to storage.
//
// WriteTraceEvent may return before the write is committed to storage.
func (rw *PartitionReadWriter) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	pid := rw.s.db.WritePartition().ID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceEvent(traceID, id, event, opts)
}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *PartitionReadWriter) DeleteTraceEvent(traceID, id string) error {
	// FIXME: use range delete
	var errs []error
	for it := rw.s.db.ReadPartitions(); it.Valid(); it = it.Prev() {
		err := NewPrefixReadWriter(rw.s.db, byte(it.ID()), rw.s.codec).DeleteTraceEvent(traceID, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *PartitionReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	var errs []error
	for it := rw.s.db.ReadPartitions(); it.Valid(); it = it.Prev() {
		err := NewPrefixReadWriter(rw.s.db, byte(it.ID()), rw.s.codec).ReadTraceEvents(traceID, out)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
