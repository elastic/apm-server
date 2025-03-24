// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   = 's'
	entryMetaTraceUnsampled = 'u'
	entryMetaTraceEvent     = 'e'

	// Initial transaction size
	// len(txnKey) + 10
	baseTransactionSize = 10 + 11
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
	NewTransaction(update bool) *badger.Txn
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
	s.pendingSize.Add(baseTransactionSize)
	return &ReadWriter{
		s:           s,
		txn:         nil, // lazy init to avoid deadlock in storage manager
		pendingSize: baseTransactionSize,
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
	s   *Storage
	txn *badger.Txn

	// readKeyBuf is a reusable buffer for keys used in read operations.
	// This must not be used in write operations, as keys are expected to
	// be unmodified until the end of a transaction.
	readKeyBuf    []byte
	pendingWrites int
	// pendingSize tracks the size of pending writes in the current ReadWriter
	pendingSize int64
}

func (rw *ReadWriter) lazyInit() {
	if rw.txn == nil {
		rw.txn = rw.s.db.NewTransaction(true)
	}
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *ReadWriter) Close() {
	if rw.txn != nil {
		rw.txn.Discard()
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
	err := rw.txn.Commit()
	rw.txn = rw.s.db.NewTransaction(true)
	rw.s.pendingSize.Add(-rw.pendingSize)
	rw.pendingWrites = 0
	rw.pendingSize = baseTransactionSize
	rw.s.pendingSize.Add(baseTransactionSize)
	if err != nil {
		return fmt.Errorf(flushErrFmt, err)
	}
	return nil
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *ReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	rw.lazyInit()

	key := []byte(traceID)
	var meta uint8 = entryMetaTraceUnsampled
	if sampled {
		meta = entryMetaTraceSampled
	}
	return rw.writeEntry(badger.NewEntry(key[:], nil).WithMeta(meta), opts)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
	rw.lazyInit()

	rw.readKeyBuf = append(rw.readKeyBuf[:0], traceID...)
	item, err := rw.txn.Get(rw.readKeyBuf)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return false, ErrNotFound
		}
		return false, err
	}
	return item.UserMeta() == entryMetaTraceSampled, nil
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
	return rw.writeEntry(badger.NewEntry(key, data).WithMeta(entryMetaTraceEvent), opts)
}

func (rw *ReadWriter) writeEntry(e *badger.Entry, opts WriterOpts) error {
	rw.pendingWrites++
	entrySize := estimateSize(e)
	// The badger database has an async size reconciliation, with a 1 minute
	// ticker that keeps the lsm and vlog sizes updated in an in-memory map.
	// It's OK to call call s.db.Size() on the hot path, since the memory
	// lookup is cheap.
	lsm, vlog := rw.s.db.Size()

	// there are multiple ReadWriters writing to the same storage so add
	// the entry size and consider the new value to avoid TOCTOU issues.
	pendingSize := rw.s.pendingSize.Add(entrySize)
	rw.pendingSize += entrySize

	if current := pendingSize + lsm + vlog; opts.StorageLimitInBytes != 0 && current >= opts.StorageLimitInBytes {
		// flush what we currently have and discard the current entry
		if err := rw.Flush(); err != nil {
			return err
		}
		return fmt.Errorf("%w (current: %d, limit: %d)", ErrLimitReached, current, opts.StorageLimitInBytes)
	}

	if rw.pendingWrites >= 200 {
		// Attempt to flush if there are 200 or more uncommitted writes.
		// This ensures calls to ReadTraceEvents are not slowed down;
		// ReadTraceEvents uses an iterator, which must sort all keys
		// of uncommitted writes.
		// The 200 value yielded a good balance between read and write speed:
		// https://github.com/elastic/apm-server/pull/8407#issuecomment-1162994643
		if err := rw.Flush(); err != nil {
			return err
		}

		// the current ReadWriter flushed the transaction and reset the pendingSize so add
		// the entrySize again.
		rw.pendingSize += entrySize
		rw.s.pendingSize.Add(entrySize)
	}

	err := rw.txn.SetEntry(e.WithTTL(opts.TTL))

	// If the transaction is already too big to accommodate the new entry, flush
	// the existing transaction and set the entry on a new one, otherwise,
	// returns early.
	if err != badger.ErrTxnTooBig {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	rw.pendingSize += entrySize
	rw.s.pendingSize.Add(entrySize)
	if err := rw.txn.SetEntry(e.WithTTL(opts.TTL)); err != nil {
		rw.pendingSize -= entrySize
		rw.s.pendingSize.Add(-entrySize)
		return err
	}
	return nil
}

func estimateSize(e *badger.Entry) int64 {
	// See badger WithValueThreshold option
	// An storage usage of an entry depends on its size
	//
	// if len(e.Value) < threshold {
	// 	return len(e.Key) + len(e.Value) + 2 // Meta, UserMeta
	// }
	// return len(e.Key) + 12 + 2 // 12 for ValuePointer, 2 for metas.
	//
	// Make a good estimate by reserving more space
	estimate := len(e.Key) + len(e.Value) + 12 + 2
	// Extra bytes for the version in key.
	return int64(estimate) + 10
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

	err := rw.txn.Delete(key)
	// If the transaction is already too big to accommodate the new entry, flush
	// the existing transaction and set the entry on a new one, otherwise,
	// returns early.
	if err != badger.ErrTxnTooBig {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}

	return rw.txn.Delete(key)
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *ReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	rw.lazyInit()

	opts := badger.DefaultIteratorOptions
	rw.readKeyBuf = append(append(rw.readKeyBuf[:0], traceID...), ':')
	opts.Prefix = rw.readKeyBuf

	// 1st pass: check whether there exist keys matching the prefix.
	// Do not prefetch values so that the check is done in-memory.
	// This is to optimize for cases when it is a miss.
	opts.PrefetchValues = false
	iter := rw.txn.NewIterator(opts)
	iter.Rewind()
	if !iter.Valid() {
		iter.Close()
		return nil
	}
	iter.Close()

	// 2nd pass: this is only done when there exist keys matching the prefix.
	// Fetch the events with PrefetchValues for performance.
	// This is to optimize for cases when it is a hit.
	opts.PrefetchValues = true
	iter = rw.txn.NewIterator(opts)
	defer iter.Close()
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		if item.IsDeletedOrExpired() {
			continue
		}
		switch item.UserMeta() {
		case entryMetaTraceEvent:
			event := &modelpb.APMEvent{}
			if err := item.Value(func(data []byte) error {
				if err := rw.s.codec.DecodeEvent(data, event); err != nil {
					return fmt.Errorf("codec failed to decode event: %w", err)
				}
				return nil
			}); err != nil {
				return err
			}
			*out = append(*out, event)
		default:
			// Unknown entry meta: ignore.
			continue
		}
	}
	return nil
}
