// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	prefixSamplingDecision string = "!"
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
	NewIndexedBatch(opts ...pebble.BatchOption) *pebble.Batch
	Size() (lsm, vlog int64)
	Close() error
}

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db db
	// pendingSize tracks the total size of pending writes across ReadWriters
	pendingSize *atomic.Int64
	codec       Codec

	sampled sync.Map
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db and codec.
func New(db db, codec Codec) *Storage {
	return &Storage{db: db, pendingSize: &atomic.Int64{}, codec: codec}
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
		//txn:         nil, // lazy init to avoid deadlock in storage manager
		//pendingSize: baseTransactionSize,
	}
}

// WriterOpts provides configuration options for writes to storage
type WriterOpts struct {
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
	s     *Storage
	batch *pebble.Batch

	// readKeyBuf is a reusable buffer for keys used in read operations.
	// This must not be used in write operations, as keys are expected to
	// be unmodified until the end of a transaction.
	readKeyBuf    []byte
	pendingWrites int
	// pendingSize tracks the size of pending writes in the current ReadWriter
	pendingSize int64
}

func (rw *ReadWriter) lazyInit() {
	if rw.batch == nil {
		rw.batch = rw.s.db.NewIndexedBatch(
			pebble.WithInitialSizeBytes(initialPebbleBatchSize),
			pebble.WithMaxRetainedSizeBytes(maxRetainedPebbleBatchSize),
		)
	}
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *ReadWriter) Close() {
	if rw.batch != nil {
		rw.batch.Close()
	}
}

// Flush waits for preceding writes to be committed to storage.
//
// Flush must be called to ensure writes are committed to storage.
// If Flush is not called before the writer is closed, then writes
// may be lost.
func (rw *ReadWriter) Flush() error {
	rw.lazyInit()

	const flushErrFmt = "failed to flush pending writes: %w"
	err := rw.batch.Commit(pebble.NoSync)
	rw.batch.Close()
	rw.batch = nil
	rw.lazyInit() // FIXME: this shouldn't be needed
	//rw.s.pendingSize.Add(-rw.pendingSize)
	rw.pendingWrites = 0
	//rw.pendingSize = baseTransactionSize
	//rw.s.pendingSize.Add(baseTransactionSize)
	if err != nil {
		return fmt.Errorf(flushErrFmt, err)
	}
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *ReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	rw.s.sampled.Store(traceID, sampled)
	return nil
	//rw.lazyInit()
	//
	//key := []byte(prefixSamplingDecision + traceID)
	//meta := entryMetaTraceUnsampled
	//if sampled {
	//	meta = entryMetaTraceSampled
	//}
	//return rw.batch.Set(key, []byte{meta}, pebble.NoSync)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
	if sampled, ok := rw.s.sampled.Load(traceID); !ok {
		return false, ErrNotFound
	} else {
		return sampled.(bool), nil
	}
	//rw.lazyInit()
	//
	//// FIXME: this needs to be fast, as it is in the hot path
	//// It should minimize disk IO on miss due to
	//// 1. (pubsub) remote sampling decision
	//// 2. (hot path) sampling decision not made yet
	//item, closer, err := rw.batch.Get([]byte(prefixSamplingDecision + traceID))
	//if err == pebble.ErrNotFound {
	//	return false, ErrNotFound
	//}
	//defer closer.Close()
	//return item[0] == entryMetaTraceSampled, nil
}

// WriteTraceEvent writes a trace event to storage.
//
// WriteTraceEvent may return before the write is committed to storage.
// Call Flush to ensure the write is committed.
func (rw *ReadWriter) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	rw.lazyInit()

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
	// FIXME: possibly change key structure, because the append is going to be expensive
	if err := rw.batch.Set(key, data, pebble.NoSync); err != nil {
		return err
	}

	if rw.batch.Len() >= dbCommitThresholdBytes {
		if err := rw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *ReadWriter) DeleteTraceEvent(traceID, id string) error {
	rw.lazyInit()

	var buf bytes.Buffer
	buf.Grow(len(traceID) + 1 + len(id))
	buf.WriteString(traceID)
	buf.WriteByte(':')
	buf.WriteString(id)
	key := buf.Bytes()

	err := rw.batch.Delete(key, pebble.NoSync)
	if err != nil {
		return err
	}
	//if rw.batch.Len() > flushThreshold {
	//	if err := rw.Flush(); err != nil {
	//		return err
	//	}
	//}
	return nil
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *ReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	rw.lazyInit()

	iter, err := rw.batch.NewIter(&pebble.IterOptions{
		LowerBound: append([]byte(traceID), ':'),
		UpperBound: append([]byte(traceID), ';'), // This is a hack to stop before next ID
	})
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
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
