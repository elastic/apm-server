// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"

	"github.com/elastic/apm-data/model/modelpb"
)

// PartitionReadWriter reads from and writes to storage across partitions.
type PartitionReadWriter struct {
	s *Storage
}

// Flush waits for preceding writes to be committed to storage.
func (rw *PartitionReadWriter) Flush() error {
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *PartitionReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	pid := rw.s.db.WritePartition().ID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceSampled(traceID, sampled)
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
func (rw *PartitionReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	pid := rw.s.db.WritePartition().ID()
	return NewPrefixReadWriter(rw.s.db, byte(pid), rw.s.codec).WriteTraceEvent(traceID, id, event)
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
