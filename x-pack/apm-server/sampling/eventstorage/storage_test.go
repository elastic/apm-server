// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func TestWriteEvents(t *testing.T) {
	// Run two tests:
	//  - 1 transaction and 1 span
	//  - 1 transaction and 100 spans
	//
	// The latter test will cause ReadTraceEvents to implicitly call flush.
	t.Run("no_flush", func(t *testing.T) {
		testWriteEvents(t, 1)
	})
	t.Run("implicit_flush", func(t *testing.T) {
		testWriteEvents(t, 100)
	})
}

func testWriteEvents(t *testing.T, numSpans int) {
	db := newBadgerDB(t, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})
	readWriter := store.NewShardedReadWriter()
	defer readWriter.Close()

	beforeWrite := time.Now()
	traceID := uuid.Must(uuid.NewV4()).String()
	transactionID := uuid.Must(uuid.NewV4()).String()
	transaction := model.APMEvent{
		Transaction: &model.Transaction{ID: transactionID},
	}
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}
	assert.NoError(t, readWriter.WriteTraceEvent(traceID, transactionID, &transaction, wOpts))

	var spanEvents []model.APMEvent
	for i := 0; i < numSpans; i++ {
		spanID := uuid.Must(uuid.NewV4()).String()
		span := model.APMEvent{
			Span: &model.Span{ID: spanID},
		}
		assert.NoError(t, readWriter.WriteTraceEvent(traceID, spanID, &span, wOpts))
		spanEvents = append(spanEvents, span)
	}
	afterWrite := time.Now()

	// We can read our writes without flushing.
	var batch model.Batch
	assert.NoError(t, readWriter.ReadTraceEvents(traceID, &batch))
	assert.ElementsMatch(t, append(spanEvents, transaction), batch)

	// Flush in order for the writes to be visible to other readers.
	assert.NoError(t, readWriter.Flush(wOpts.StorageLimitInBytes))

	var recorded []model.APMEvent
	assert.NoError(t, db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.IteratorOptions{
			Prefix: []byte(traceID),
		})
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			expiresAt := item.ExpiresAt()
			expiryTime := time.Unix(int64(expiresAt), 0)

			// The expiry time should be somewhere between when we
			// started and finished writing + the TTL. The expiry time
			// is recorded as seconds since the Unix epoch, hence the
			// truncation.
			lowerBound := beforeWrite.Add(wOpts.TTL).Truncate(time.Second)
			upperBound := afterWrite.Add(wOpts.TTL).Truncate(time.Second)
			assert.Condition(t, func() bool {
				return !lowerBound.After(expiryTime)
			}, "expiry time %s is before %s", expiryTime, lowerBound)
			assert.Condition(t, func() bool {
				return !expiryTime.After(upperBound)
			}, "expiry time %s is after %s", expiryTime, upperBound)

			var event model.APMEvent
			require.Equal(t, "e", string(item.UserMeta()))
			assert.NoError(t, item.Value(func(data []byte) error {
				return json.Unmarshal(data, &event)
			}))
			recorded = append(recorded, event)
		}
		return nil
	}))
	assert.ElementsMatch(t, batch, recorded)
}

func TestWriteTraceSampled(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})
	readWriter := store.NewShardedReadWriter()
	defer readWriter.Close()
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}

	before := time.Now()
	assert.NoError(t, readWriter.WriteTraceSampled("sampled_trace_id", true, wOpts))
	assert.NoError(t, readWriter.WriteTraceSampled("unsampled_trace_id", false, wOpts))

	// We can read our writes without flushing.
	isSampled, err := readWriter.IsTraceSampled("sampled_trace_id")
	assert.NoError(t, err)
	assert.True(t, isSampled)

	// Flush in order for the writes to be visible to other readers.
	assert.NoError(t, readWriter.Flush(wOpts.StorageLimitInBytes))

	sampled := make(map[string]bool)
	assert.NoError(t, db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.IteratorOptions{})
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			expiresAt := item.ExpiresAt()
			expiryTime := time.Unix(int64(expiresAt), 0)
			assert.Condition(t, func() bool {
				return !before.After(expiryTime) && !expiryTime.After(before.Add(wOpts.TTL))
			})

			key := string(item.Key())
			switch meta := item.UserMeta(); meta {
			case 's':
				sampled[key] = true
			case 'u':
				sampled[key] = false
			default:
				t.Fatalf("invalid meta %q", meta)
			}
			assert.Zero(t, item.ValueSize())
		}
		return nil
	}))
	assert.Equal(t, map[string]bool{
		"sampled_trace_id":   true,
		"unsampled_trace_id": false,
	}, sampled)
}

func TestReadTraceEvents(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})

	traceID := [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		key := append(traceID[:], ":12345678"...)
		value := []byte(`{"transaction":{"name":"transaction"}}`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('e')); err != nil {
			return err
		}

		key = append(traceID[:], ":87654321"...)
		value = []byte(`{"span":{"name":"span"}}`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('e')); err != nil {
			return err
		}

		// Write an entry with the trace ID as a prefix, but with no
		// proceeding colon, causing it to be ignored.
		key = append(traceID[:], "nocolon"...)
		value = []byte(`not-json`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('e')); err != nil {
			return err
		}

		// Write an entry with an unknown meta value. It will be ignored.
		key = append(traceID[:], ":11111111"...)
		value = []byte(`not-json`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('?')); err != nil {
			return err
		}
		return nil
	}))

	reader := store.NewShardedReadWriter()
	defer reader.Close()

	var events model.Batch
	assert.NoError(t, reader.ReadTraceEvents(string(traceID[:]), &events))
	assert.Equal(t, model.Batch{
		{Transaction: &model.Transaction{Name: "transaction"}},
		{Span: &model.Span{Name: "span"}},
	}, events)
}

func TestReadTraceEventsDecodeError(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})

	traceID := [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		key := append(traceID[:], ":12345678"...)
		value := []byte(`wat`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('e')); err != nil {
			return err
		}
		return nil
	}))

	reader := store.NewShardedReadWriter()
	defer reader.Close()

	var events model.Batch
	err := reader.ReadTraceEvents(string(traceID[:]), &events)
	assert.Error(t, err)
}

func TestIsTraceSampled(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})

	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		if err := txn.SetEntry(badger.NewEntry([]byte("sampled_trace_id"), nil).WithMeta('s')); err != nil {
			return err
		}
		if err := txn.SetEntry(badger.NewEntry([]byte("unsampled_trace_id"), nil).WithMeta('u')); err != nil {
			return err
		}
		return nil
	}))

	reader := store.NewShardedReadWriter()
	defer reader.Close()

	sampled, err := reader.IsTraceSampled("sampled_trace_id")
	assert.NoError(t, err)
	assert.True(t, sampled)

	sampled, err = reader.IsTraceSampled("unsampled_trace_id")
	assert.NoError(t, err)
	assert.False(t, sampled)

	_, err = reader.IsTraceSampled("unknown_trace_id")
	assert.Equal(t, err, eventstorage.ErrNotFound)
}

func TestStorageLimit(t *testing.T) {
	tempdir := t.TempDir()
	opts := func() badger.Options {
		opts := badgerOptions()
		opts = opts.WithInMemory(false)
		opts = opts.WithDir(tempdir).WithValueDir(tempdir)
		return opts
	}

	// Open and close the database to create a non-empty value log file,
	// which will cause writes below to fail due to the storage limit being
	// exceeded. We would otherwise have to rely on Badger's one minute
	// timer to refresh the size.
	db := newBadgerDB(t, opts)
	db.Close()
	db = newBadgerDB(t, opts)
	lsm, vlog := db.Size()

	store := eventstorage.New(db, eventstorage.JSONCodec{})
	readWriter := store.NewReadWriter()
	defer readWriter.Close()

	traceID := uuid.Must(uuid.NewV4()).String()
	transactionID := uuid.Must(uuid.NewV4()).String()
	transaction := model.APMEvent{Transaction: &model.Transaction{ID: transactionID}}
	err := readWriter.WriteTraceEvent(traceID, transactionID, &transaction, eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 1, // ignored in the write, because there's no implicit flush
	})
	assert.NoError(t, err)
	err = readWriter.Flush(1)
	assert.EqualError(t, err, fmt.Sprintf(
		"failed to flush pending writes: configured storage limit reached (current: %d, limit: 1)", lsm+vlog,
	))
	assert.ErrorIs(t, err, eventstorage.ErrLimitReached)

	// Assert the stored write has been discarded.
	var batch model.Batch
	readWriter.ReadTraceEvents(traceID, &batch)
	assert.Equal(t, 0, len(batch))
}

func badgerOptions() badger.Options {
	return badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
}

type badgerOptionsFunc func() badger.Options

func newBadgerDB(tb testing.TB, badgerOptions badgerOptionsFunc) *badger.DB {
	db, err := badger.Open(badgerOptions())
	if err != nil {
		panic(err)
	}
	tb.Cleanup(func() { db.Close() })
	return db
}
