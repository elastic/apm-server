// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/elastic/apm-server/model"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   = 's'
	entryMetaTraceUnsampled = 'u'
	entryMetaTransaction    = 'T'
	entryMetaSpan           = 'S'
)

// ErrNotFound is returned by by the Storage.IsTraceSampled method,
// for non-existing trace IDs.
var ErrNotFound = errors.New("key not found")

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db    *badger.DB
	codec Codec
	ttl   time.Duration
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeSpan([]byte, *model.Span) error
	DecodeTransaction([]byte, *model.Transaction) error
	EncodeSpan(*model.Span) ([]byte, error)
	EncodeTransaction(*model.Transaction) ([]byte, error)
}

// New returns a new Storage using db and codec.
//
// Storage entries expire after ttl.
func New(db *badger.DB, codec Codec, ttl time.Duration) *Storage {
	return &Storage{db: db, codec: codec, ttl: ttl}
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
		s:   s,
		txn: s.db.NewTransaction(true),
	}
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
	txn           *badger.Txn
	pendingWrites int

	// readKeyBuf is a reusable buffer for keys used in read operations.
	// This must not be used in write operations, as keys are expected to
	// be unmodified until the end of a transaction.
	readKeyBuf []byte
}

// Close closes the writer. Any writes that have not been flushed may be lost.
//
// This must be called when the writer is no longer needed, in order to reclaim
// resources.
func (rw *ReadWriter) Close() {
	rw.txn.Discard()
}

// Flush waits for preceding writes to be committed to storage.
//
// Flush must be called to ensure writes are committed to storage.
// If Flush is not called before the writer is closed, then writes
// may be lost.
func (rw *ReadWriter) Flush() error {
	err := rw.txn.Commit()
	rw.txn = rw.s.db.NewTransaction(true)
	rw.pendingWrites = 0
	return err
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *ReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	key := []byte(traceID)
	var meta uint8 = entryMetaTraceUnsampled
	if sampled {
		meta = entryMetaTraceSampled
	}
	entry := badger.NewEntry(key[:], nil).WithMeta(meta)
	return rw.writeEntry(entry.WithTTL(rw.s.ttl))
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *ReadWriter) IsTraceSampled(traceID string) (bool, error) {
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

// WriteTransaction writes tx to storage.
//
// WriteTransaction may return before the write is committed to storage.
// Call Flush to ensure the write is committed.
func (rw *ReadWriter) WriteTransaction(tx *model.Transaction) error {
	key := append(append([]byte(tx.TraceID), ':'), tx.ID...)
	data, err := rw.s.codec.EncodeTransaction(tx)
	if err != nil {
		return err
	}
	return rw.writeEvent(key[:], data, entryMetaTransaction)
}

// WriteSpan writes span to storage.
//
// WriteSpan may return before the write is committed to storage.
// Call Flush to ensure the write is committed.
func (rw *ReadWriter) WriteSpan(span *model.Span) error {
	key := append(append([]byte(span.TraceID), ':'), span.ID...)
	data, err := rw.s.codec.EncodeSpan(span)
	if err != nil {
		return err
	}
	return rw.writeEvent(key[:], data, entryMetaSpan)
}

func (rw *ReadWriter) writeEvent(key, value []byte, meta byte) error {
	return rw.writeEntry(badger.NewEntry(key, value).WithMeta(meta).WithTTL(rw.s.ttl))
}

func (rw *ReadWriter) writeEntry(e *badger.Entry) error {
	rw.pendingWrites++
	err := rw.txn.SetEntry(e)
	if err != badger.ErrTxnTooBig {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	return rw.txn.SetEntry(e)
}

// DeleteTransaction deletes the transaction from storage.
func (rw *ReadWriter) DeleteTransaction(tx *model.Transaction) error {
	key := append(append([]byte(tx.TraceID), ':'), tx.ID...)
	return rw.txn.Delete(key)
}

// DeleteSpan deletes the span from storage.
func (rw *ReadWriter) DeleteSpan(span *model.Span) error {
	key := append(append([]byte(span.TraceID), ':'), span.ID...)
	return rw.txn.Delete(key)
}

// ReadEvents reads events with the given trace ID from storage into a batch.
//
// ReadEvents may implicitly commit the current transaction when the number
// of pending writes exceeds a threshold. This is due to how Badger internally
// iterates over uncommitted writes, where it will sort keys for each new
// iterator.
func (rw *ReadWriter) ReadEvents(traceID string, out *model.Batch) error {
	opts := badger.DefaultIteratorOptions
	rw.readKeyBuf = append(append(rw.readKeyBuf[:0], traceID...), ':')
	opts.Prefix = rw.readKeyBuf

	// NewIterator slows down with uncommitted writes, as it must sort
	// all keys lexicographically. If there are a significant number of
	// writes pending, flush first.
	if rw.pendingWrites > 100 {
		if err := rw.Flush(); err != nil {
			return err
		}
	}

	iter := rw.txn.NewIterator(opts)
	defer iter.Close()
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		if item.IsDeletedOrExpired() {
			continue
		}
		switch item.UserMeta() {
		case entryMetaTransaction:
			var event model.Transaction
			if err := item.Value(func(data []byte) error {
				return rw.s.codec.DecodeTransaction(data, &event)
			}); err != nil {
				return err
			}
			out.Transactions = append(out.Transactions, &event)
		case entryMetaSpan:
			var event model.Span
			if err := item.Value(func(data []byte) error {
				return rw.s.codec.DecodeSpan(data, &event)
			}); err != nil {
				return err
			}
			out.Spans = append(out.Spans, &event)
		default:
			// Unknown entry meta: ignore.
			continue
		}
	}
	return nil
}
