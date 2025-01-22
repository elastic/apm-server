// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"io"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   byte = 's'
	entryMetaTraceUnsampled byte = 'u'
)

var (
	// ErrNotFound is returned by by the Storage.IsTraceSampled method,
	// for non-existing trace IDs.
	ErrNotFound = errors.New("key not found")

	// ErrLimitReached is returned by the ReadWriter.Flush method when
	// the configured StorageLimiter.Limit is true.
	ErrLimitReached = errors.New("configured storage limit reached")
)

type db interface {
	Get(key []byte) ([]byte, io.Closer, error)
	Set(key, value []byte, opts *pebble.WriteOptions) error
	Delete(key []byte, opts *pebble.WriteOptions) error
	NewIter(o *pebble.IterOptions) (*pebble.Iterator, error)
}

type partitionedDB interface {
	db
	PartitionID() int32
	PartitionCount() int32
}

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db    partitionedDB
	codec Codec
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db and codec.
func New(db partitionedDB, codec Codec) *Storage {
	return &Storage{
		db:    db,
		codec: codec,
	}
}

// NewShardedReadWriter returns a new ShardedReadWriter, for sharded
// reading and writing.
//
// The returned ShardedReadWriter must be closed when it is no longer
// needed.
func (s *Storage) NewShardedReadWriter() *ShardedReadWriter {
	return newShardedReadWriter(s)
}

// NewReadWriter returns a new ReadWriter for reading events from and
// writing events to storage.
//
// The returned ReadWriter must be closed when it is no longer needed.
func (s *Storage) NewReadWriter() *ReadWriter {
	return &ReadWriter{
		s: s,
	}
}

// WriterOpts provides configuration options for writes to storage
type WriterOpts struct {
	StorageLimitInBytes int64
}

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
