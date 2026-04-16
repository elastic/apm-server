// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func newEventPebble(t *testing.T) *pebble.DB {
	db, err := eventstorage.OpenEventPebble(t.TempDir(), 8<<20, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func newDecisionPebble(t *testing.T) *pebble.DB {
	db, err := eventstorage.OpenDecisionPebble(t.TempDir(), 8<<20, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func TestPrefixReadWriter_WriteTraceEvent(t *testing.T) {
	codec := eventstorage.ProtobufCodec{}
	db := newEventPebble(t)
	traceID := "foo"
	txnID := "bar"
	txn := makeTransaction(txnID, traceID)
	rw := eventstorage.NewPrefixReadWriter(db, 1, codec)

	check := func() {
		err := rw.WriteTraceEvent(traceID, txnID, txn)
		assert.NoError(t, err)
		item, closer, err := db.Get(append([]byte{1}, []byte("foo:bar")...))
		assert.NoError(t, err)
		defer closer.Close()
		var actual modelpb.APMEvent
		err = codec.DecodeEvent(item, &actual)
		assert.NoError(t, err)
		assert.Equal(t, *txn, actual)
	}

	check()

	// Try writing to the same key again to simulate misbehaving agent / race condition
	check()
}

// readAllTraceEvents is a test helper that reads all events for a trace
// into a single batch using ReadTraceEventsCallback.
func readAllTraceEvents(rw eventstorage.RW, traceID string) (modelpb.Batch, error) {
	var scratch, out modelpb.Batch
	err := rw.ReadTraceEventsCallback(traceID, 1<<30, &scratch, func(batch modelpb.Batch) error {
		out = append(out, batch...)
		return nil
	})
	return out, err
}

// readAllTraceEventsInto appends all events for a trace into the given batch.
func readAllTraceEventsInto(rw eventstorage.RW, traceID string, out *modelpb.Batch) error {
	var scratch modelpb.Batch
	return rw.ReadTraceEventsCallback(traceID, 1<<30, &scratch, func(batch modelpb.Batch) error {
		*out = append(*out, batch...)
		return nil
	})
}

func TestPrefixReadWriter_ReadTraceEventsCallback(t *testing.T) {
	codec := eventstorage.ProtobufCodec{}
	db := newEventPebble(t)
	rw := eventstorage.NewPrefixReadWriter(db, 1, codec)

	traceID := "trace1"
	txnIDs := []string{"a", "b", "c", "d", "e"}
	for _, txnID := range txnIDs {
		txn := makeTransaction(txnID, traceID)
		err := rw.WriteTraceEvent(traceID, txnID, txn)
		require.NoError(t, err)
	}

	// Compute the encoded size of a single event for test thresholds.
	singleEvent := makeTransaction("a", traceID)
	encoded, err := codec.EncodeEvent(singleEvent)
	require.NoError(t, err)
	eventSize := len(encoded)

	// Write events for a different trace to ensure isolation
	otherTxn := makeTransaction("x", "trace2")
	err = rw.WriteTraceEvent("trace2", "x", otherTxn)
	require.NoError(t, err)

	var scratch modelpb.Batch

	t.Run("limit_larger_than_total", func(t *testing.T) {
		var callCount int
		var allEvents modelpb.Batch
		// Set limit high enough that all 5 events fit in one batch.
		err := rw.ReadTraceEventsCallback(traceID, eventSize*10, &scratch, func(batch modelpb.Batch) error {
			callCount++
			allEvents = append(allEvents, batch...)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
		assert.Len(t, allEvents, 5)
	})

	t.Run("limit_one_byte_flushes_per_event", func(t *testing.T) {
		// With a 1-byte limit, every event exceeds the limit immediately.
		var callCount int
		var pageSizes []int
		err := rw.ReadTraceEventsCallback(traceID, 1, &scratch, func(batch modelpb.Batch) error {
			callCount++
			pageSizes = append(pageSizes, len(batch))
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 5, callCount)
		assert.Equal(t, []int{1, 1, 1, 1, 1}, pageSizes)
	})

	t.Run("limit_splits_into_batches", func(t *testing.T) {
		// Set limit to fit exactly 2 events, so 5 events -> 3 batches (2+2+1).
		var callCount int
		var pageSizes []int
		err := rw.ReadTraceEventsCallback(traceID, eventSize*2, &scratch, func(batch modelpb.Batch) error {
			callCount++
			pageSizes = append(pageSizes, len(batch))
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
		assert.Equal(t, []int{2, 2, 1}, pageSizes)
	})

	t.Run("callback_error_stops_iteration", func(t *testing.T) {
		expectedErr := fmt.Errorf("stop")
		var callCount int
		err := rw.ReadTraceEventsCallback(traceID, eventSize*2, &scratch, func(batch modelpb.Batch) error {
			callCount++
			return expectedErr
		})
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, 1, callCount)
	})

	t.Run("no_events", func(t *testing.T) {
		var callCount int
		err := rw.ReadTraceEventsCallback("nonexistent", eventSize*10, &scratch, func(batch modelpb.Batch) error {
			callCount++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, callCount)
	})

	t.Run("trace_isolation", func(t *testing.T) {
		// Verify that events from similar trace IDs (trace1, trace12, trace2)
		// are not returned when reading trace1.
		out, err := readAllTraceEvents(rw, traceID)
		assert.NoError(t, err)
		assert.Len(t, out, 5) // only trace1 events, not trace2
	})
}

func TestPrefixReadWriter_DeleteTraceEvent(t *testing.T) {
	codec := eventstorage.ProtobufCodec{}
	db := newEventPebble(t)
	traceID := "foo"
	txnID := "bar"
	txn := makeTransaction(txnID, traceID)
	rw := eventstorage.NewPrefixReadWriter(db, 1, codec)
	err := rw.WriteTraceEvent(traceID, txnID, txn)
	require.NoError(t, err)

	key := append([]byte{1}, []byte("foo:bar")...)

	_, closer, err := db.Get(key)
	assert.NoError(t, err)
	err = closer.Close()
	assert.NoError(t, err)

	err = rw.DeleteTraceEvent(traceID, txnID)
	assert.NoError(t, err)

	_, _, err = db.Get(key)
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestPrefixReadWriter_WriteTraceSampled(t *testing.T) {
	for _, sampled := range []bool{true, false} {
		t.Run(fmt.Sprintf("sampled=%v", sampled), func(t *testing.T) {
			codec := eventstorage.ProtobufCodec{}
			db := newDecisionPebble(t)
			traceID := "foo"
			rw := eventstorage.NewPrefixReadWriter(db, 1, codec)

			check := func() {
				err := rw.WriteTraceSampled(traceID, sampled)
				assert.NoError(t, err)
				item, closer, err := db.Get(append([]byte{1}, []byte("foo")...))
				assert.NoError(t, err)
				defer closer.Close()
				assert.NoError(t, err)
				if sampled {
					assert.Equal(t, []byte{'s'}, item)
				} else {
					assert.Equal(t, []byte{'u'}, item)
				}
			}

			check()

			// Try writing to the same key again to simulate misbehaving agent / race condition
			check()
		})
	}
}

func TestPrefixReadWriter_IsTraceSampled(t *testing.T) {
	for _, tc := range []struct {
		sampled bool
		missing bool
	}{
		{
			sampled: true,
		},
		{
			sampled: false,
		},
		{
			missing: true,
		},
	} {
		t.Run(fmt.Sprintf("sampled=%v,missing=%v", tc.sampled, tc.missing), func(t *testing.T) {
			db := newDecisionPebble(t)
			rw := eventstorage.NewPrefixReadWriter(db, 1, nopCodec{})
			traceID := uuid.Must(uuid.NewV4()).String()
			if !tc.missing {
				err := rw.WriteTraceSampled(traceID, tc.sampled)
				require.NoError(t, err)
			}
			sampled, err := rw.IsTraceSampled(traceID)
			if tc.missing {
				assert.ErrorIs(t, err, eventstorage.ErrNotFound)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.sampled, sampled)
			}
		})
	}
}
