// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	//prefixSamplingDecision string = "!"
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   byte = 's'
	entryMetaTraceUnsampled byte = 'u'
	entryMetaTraceEvent     byte = 'e'
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
	//Size() (lsm, vlog int64)
	Get(key []byte) ([]byte, io.Closer, error)
	Set(key, value []byte, opts *pebble.WriteOptions) error
	Delete(key []byte, opts *pebble.WriteOptions) error
	NewIter(o *pebble.IterOptions) (*pebble.Iterator, error)
}

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db         db
	decisionDB db
	// pendingSize tracks the total size of pending writes across ReadWriters
	pendingSize *atomic.Int64
	codec       Codec
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db, decisionDB and codec.
func New(db db, decisionDB db, codec Codec) *Storage {
	return &Storage{
		db:          db,
		decisionDB:  decisionDB,
		pendingSize: &atomic.Int64{},
		codec:       codec,
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
	//s.pendingSize.Add(baseTransactionSize)
	return &ReadWriter{
		s: s,
		//pendingSize: baseTransactionSize,
	}
}

// WriterOpts provides configuration options for writes to storage
type WriterOpts struct {
	TimeNow             func() time.Time
	TTL                 time.Duration
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
	s             *Storage
	batch         *pebble.Batch
	decisionBatch *pebble.Batch

	// readKeyBuf is a reusable buffer for keys used in read operations.
	// This must not be used in write operations, as keys are expected to
	// be unmodified until the end of a transaction.
	readKeyBuf    []byte
	pendingWrites int
	// pendingSize tracks the size of pending writes in the current ReadWriter
	pendingSize int64
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
	var now time.Time
	if opts.TimeNow != nil {
		now = opts.TimeNow()
	} else {
		now = time.Now()
	}

	return TTLReadWriter{truncatedTime: now.Truncate(opts.TTL), db: rw.s.decisionDB}.WriteTraceSampled(traceID, sampled, opts)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
	// FIXME: this needs to be fast, as it is in the hot path
	// It should minimize disk IO on miss due to
	// 1. (pubsub) remote sampling decision
	// 2. (hot path) sampling decision not made yet
	now := time.Now()
	ttl := time.Minute
	sampled, err := TTLReadWriter{truncatedTime: now.Truncate(ttl), db: rw.s.decisionDB}.IsTraceSampled(traceID)
	if err == nil {
		return sampled, nil
	}

	sampled, err = TTLReadWriter{truncatedTime: now.Add(-ttl).Truncate(ttl), db: rw.s.decisionDB}.IsTraceSampled(traceID)
	if err == nil {
		return sampled, nil
	}

	return false, err
}

// WriteTraceEvent writes a trace event to storage.
//
// WriteTraceEvent may return before the write is committed to storage.
func (rw *ReadWriter) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	data, err := rw.s.codec.EncodeEvent(event)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.Grow(len(traceID) + 1 + len(id))
	buf.WriteString(traceID)
	buf.WriteByte(':')
	buf.WriteString(id)
	key := buf.Bytes()
	return rw.writeEntry(key, data)
}

func (rw *ReadWriter) writeEntry(key, data []byte) error {
	rw.pendingWrites++

	if err := rw.s.db.Set(key, data, pebble.NoSync); err != nil {
		return err
	}
	return nil
}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *ReadWriter) DeleteTraceEvent(traceID, id string) error {
	// FIXME: use range delete
	var buf bytes.Buffer
	buf.Grow(len(traceID) + 1 + len(id))
	buf.WriteString(traceID)
	buf.WriteByte(':')
	buf.WriteString(id)
	key := buf.Bytes()

	err := rw.s.db.Delete(key, pebble.NoSync)
	if err != nil {
		return err
	}
	return nil
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *ReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	iter, err := rw.s.db.NewIter(&pebble.IterOptions{
		LowerBound: append([]byte(traceID), ':'),
		UpperBound: append([]byte(traceID), ';'), // This is a hack to stop before next ID
	})
	if err != nil {
		return err
	}
	defer iter.Close()
	// SeekPrefixGE uses the prefix bloom filter, so that a miss will be much faster
	if valid := iter.SeekPrefixGE(append([]byte(traceID), ':')); !valid {
		return nil
	}
	for ; iter.Valid(); iter.Next() {
		event := &modelpb.APMEvent{}
		data, err := iter.ValueAndErr()
		if err != nil {
			return err
		}
		if err := rw.s.codec.DecodeEvent(data, event); err != nil {
			return fmt.Errorf("codec failed to decode event: %w", err)
		}
		*out = append(*out, event)
	}
	return nil
}

func timePrefix(t time.Time) string {
	// FIXME: use TTL
	// FIXME: convert int to bytes
	return fmt.Sprintf("%d", t.Unix())
}
