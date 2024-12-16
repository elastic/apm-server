// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstoragepebble

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   byte = 's'
	entryMetaTraceUnsampled byte = 'u'
	entryMetaTraceEvent     byte = 'e'
)

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db *pebble.DB
	// pendingSize tracks the total size of pending writes across ReadWriters
	pendingSize *atomic.Int64
	codec       Codec
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db and codec.
func New(db *pebble.DB, codec Codec) *Storage {
	return &Storage{db: db, pendingSize: &atomic.Int64{}, codec: codec}
}

// NewReadWriter returns a new ReadWriter for reading events from and
// writing events to storage.
//
// The returned ReadWriter must be closed when it is no longer needed.
func (s *Storage) NewReadWriter() *ReadWriter {
	//s.pendingSize.Add(baseTransactionSize)
	return &ReadWriter{
		s:     s,
		batch: s.db.NewIndexedBatch(),
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
	s *Storage

	// readKeyBuf is a reusable buffer for keys used in read operations.
	// This must not be used in write operations, as keys are expected to
	// be unmodified until the end of a transaction.
	readKeyBuf []byte

	mu    sync.Mutex
	batch *pebble.Batch
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *ReadWriter) Close() {
	//rw.txn.Discard()
}

// Flush waits for preceding writes to be committed to storage.
//
// Flush must be called to ensure writes are committed to storage.
// If Flush is not called before the writer is closed, then writes
// may be lost.
func (rw *ReadWriter) Flush() error {
	err := rw.batch.Commit(pebble.NoSync)
	rw.batch.Close()
	rw.batch = rw.s.db.NewIndexedBatch()
	const flushErrFmt = "failed to flush pending writes: %w"
	if err != nil {
		return fmt.Errorf(flushErrFmt, err)
	}
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *ReadWriter) WriteTraceSampled(traceID string, sampled bool, opts eventstorage.WriterOpts) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	key := []byte(traceID)
	meta := entryMetaTraceUnsampled
	if sampled {
		meta = entryMetaTraceSampled
	}
	return rw.batch.Set(key, []byte{meta}, pebble.NoSync)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	item, closer, err := rw.batch.Get([]byte(traceID))
	if err == pebble.ErrNotFound {
		return false, eventstorage.ErrNotFound
	}
	defer closer.Close()
	return item[0] == entryMetaTraceSampled, nil
}

// WriteTraceEvent writes a trace event to storage.
//
// WriteTraceEvent may return before the write is committed to storage.
// Call Flush to ensure the write is committed.
func (rw *ReadWriter) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent, opts eventstorage.WriterOpts) error {
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
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if err := rw.batch.Set(key, append([]byte{entryMetaTraceEvent}, data...), pebble.NoSync); err != nil {
		return err
	}

	if rw.batch.Len() > 2000 {
		if err := rw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

//
//func estimateSize(e *badger.Entry) int64 {
//	// See badger WithValueThreshold option
//	// An storage usage of an entry depends on its size
//	//
//	// if len(e.Value) < threshold {
//	// 	return len(e.Key) + len(e.Value) + 2 // Meta, UserMeta
//	// }
//	// return len(e.Key) + 12 + 2 // 12 for ValuePointer, 2 for metas.
//	//
//	// Make a good estimate by reserving more space
//	estimate := len(e.Key) + len(e.Value) + 12 + 2
//	// Extra bytes for the version in key.
//	return int64(estimate) + 10
//}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *ReadWriter) DeleteTraceEvent(traceID, id string) error {
	//var buf bytes.Buffer
	//buf.Grow(len(traceID) + 1 + len(id))
	//buf.WriteString(traceID)
	//buf.WriteByte(':')
	//buf.WriteString(id)
	//key := buf.Bytes()
	//
	//err := rw.txn.Delete(key)
	//// If the transaction is already too big to accommodate the new entry, flush
	//// the existing transaction and set the entry on a new one, otherwise,
	//// returns early.
	//if err != badger.ErrTxnTooBig {
	//	return err
	//}
	//if err := rw.Flush(); err != nil {
	//	return err
	//}
	//
	//return rw.txn.Delete(key)
	return nil
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *ReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	iter, err := rw.batch.NewIter(&pebble.IterOptions{
		LowerBound: append([]byte(traceID), ':'),
		UpperBound: append([]byte(traceID), ';'),
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
		if err := rw.s.codec.DecodeEvent(data[1:], event); err != nil {
			return fmt.Errorf("codec failed to decode event: %w", err)
		}
		*out = append(*out, event)
	}
	return nil
}
