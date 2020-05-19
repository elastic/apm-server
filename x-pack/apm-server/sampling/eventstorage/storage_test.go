// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func TestWriteEvents(t *testing.T) {
	// Run two tests:
	//  - 1 transaction and 1 span
	//  - 1 transaction and 100 spans
	//
	// The latter test will cause ReadEvents to implicitly call flush.
	t.Run("no_flush", func(t *testing.T) {
		testWriteEvents(t, 1)
	})
	t.Run("implicit_flush", func(t *testing.T) {
		testWriteEvents(t, 100)
	})
}

func testWriteEvents(t *testing.T, numSpans int) {
	db := newBadgerDB(t, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)
	readWriter := store.NewShardedReadWriter()
	defer readWriter.Close()

	before := time.Now()

	traceUUID := uuid.Must(uuid.NewV4())
	transactionUUID := uuid.Must(uuid.NewV4())
	transaction := &model.Transaction{
		TraceID: traceUUID.String(),
		ID:      transactionUUID.String(),
	}
	assert.NoError(t, readWriter.WriteTransaction(transaction))

	var spans []*model.Span
	for i := 0; i < numSpans; i++ {
		spanUUID := uuid.Must(uuid.NewV4())
		span := &model.Span{
			TraceID: traceUUID.String(),
			ID:      spanUUID.String(),
		}
		assert.NoError(t, readWriter.WriteSpan(span))
		spans = append(spans, span)
	}

	// We can read our writes without flushing.
	var batch model.Batch
	assert.NoError(t, readWriter.ReadEvents(traceUUID.String(), &batch))
	assert.ElementsMatch(t, []*model.Transaction{transaction}, batch.Transactions)
	assert.ElementsMatch(t, spans, batch.Spans)

	// Flush in order for the writes to be visible to other readers.
	assert.NoError(t, readWriter.Flush())

	var recorded []interface{}
	assert.NoError(t, db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.IteratorOptions{
			Prefix: []byte(traceUUID.String()),
		})
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			expiresAt := item.ExpiresAt()
			expiryTime := time.Unix(int64(expiresAt), 0)
			assert.Condition(t, func() bool {
				return !before.After(expiryTime) && !expiryTime.After(before.Add(ttl))
			})

			var value interface{}
			switch meta := item.UserMeta(); meta {
			case 't':
				value = &model.Transaction{}
			case 's':
				value = &model.Span{}
			default:
				t.Fatalf("invalid meta %q", meta)
			}
			assert.NoError(t, item.Value(func(data []byte) error {
				return json.Unmarshal(data, value)
			}))
			recorded = append(recorded, value)
		}
		return nil
	}))
	assert.ElementsMatch(t, batch.Transformables(), recorded)
}

func TestWriteTraceSampled(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)
	readWriter := store.NewShardedReadWriter()
	defer readWriter.Close()

	before := time.Now()
	assert.NoError(t, readWriter.WriteTraceSampled("sampled_trace_id", true))
	assert.NoError(t, readWriter.WriteTraceSampled("unsampled_trace_id", false))

	// We can read our writes without flushing.
	isSampled, err := readWriter.IsTraceSampled("sampled_trace_id")
	assert.NoError(t, err)
	assert.True(t, isSampled)

	// Flush in order for the writes to be visible to other readers.
	assert.NoError(t, readWriter.Flush())

	sampled := make(map[string]bool)
	assert.NoError(t, db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.IteratorOptions{})
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			expiresAt := item.ExpiresAt()
			expiryTime := time.Unix(int64(expiresAt), 0)
			assert.Condition(t, func() bool {
				return !before.After(expiryTime) && !expiryTime.After(before.Add(ttl))
			})

			key := string(item.Key())
			switch meta := item.UserMeta(); meta {
			case 'S':
				sampled[key] = true
			case 'U':
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

func TestReadEvents(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)

	traceID := [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		key := append(traceID[:], ":12345678"...)
		value := []byte(`{"name":"transaction"}`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('t')); err != nil {
			return err
		}

		key = append(traceID[:], ":87654321"...)
		value = []byte(`{"name":"span"}`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('s')); err != nil {
			return err
		}

		// Write an entry with the trace ID as a prefix, but with no
		// proceeding colon, causing it to be ignored.
		key = append(traceID[:], "nocolon"...)
		value = []byte(`not-json`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('s')); err != nil {
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
	assert.NoError(t, reader.ReadEvents(string(traceID[:]), &events))
	assert.Equal(t, []*model.Transaction{{Name: "transaction"}}, events.Transactions)
	assert.Equal(t, []*model.Span{{Name: "span"}}, events.Spans)
}

func TestReadEventsDecodeError(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)

	traceID := [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		key := append(traceID[:], ":12345678"...)
		value := []byte(`wat`)
		if err := txn.SetEntry(badger.NewEntry(key, value).WithMeta('t')); err != nil {
			return err
		}
		return nil
	}))

	reader := store.NewShardedReadWriter()
	defer reader.Close()

	var events model.Batch
	err := reader.ReadEvents(string(traceID[:]), &events)
	assert.Error(t, err)
}

func TestIsTraceSampled(t *testing.T) {
	db := newBadgerDB(t, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)

	require.NoError(t, db.Update(func(txn *badger.Txn) error {
		if err := txn.SetEntry(badger.NewEntry([]byte("sampled_trace_id"), nil).WithMeta('S')); err != nil {
			return err
		}
		if err := txn.SetEntry(badger.NewEntry([]byte("unsampled_trace_id"), nil).WithMeta('U')); err != nil {
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
