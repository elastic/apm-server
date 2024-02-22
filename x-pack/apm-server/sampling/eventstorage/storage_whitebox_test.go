// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
)

func newReadWriter(tb testing.TB) *ReadWriter {
	tempdir := tb.TempDir()
	opts := badger.DefaultOptions("").WithLogger(nil)
	opts = opts.WithInMemory(false)
	opts = opts.WithDir(tempdir).WithValueDir(tempdir)

	db, err := badger.Open(opts)
	if err != nil {
		panic(err)
	}
	tb.Cleanup(func() { db.Close() })

	store := New(db, ProtobufCodec{})
	readWriter := store.NewReadWriter()
	tb.Cleanup(func() { readWriter.Close() })

	return readWriter
}

func TestDeleteTraceEvent_ErrTxnTooBig(t *testing.T) {
	readWriter := newReadWriter(t)

	traceID, transactionID := writeEvent(t, readWriter)
	assert.True(t, eventExists(t, readWriter, traceID, transactionID))

	fillTxnUntilTxnTooBig(readWriter.txn)

	err := readWriter.DeleteTraceEvent(traceID, transactionID)
	assert.NoError(t, err)

	assert.False(t, eventExists(t, readWriter, traceID, transactionID))
}

func TestWriteTraceEvent_ErrTxnTooBig(t *testing.T) {
	readWriter := newReadWriter(t)

	fillTxnUntilTxnTooBig(readWriter.txn)

	traceID, transactionID := writeEvent(t, readWriter)
	assert.True(t, eventExists(t, readWriter, traceID, transactionID))
}

func writeEvent(t *testing.T, readWriter *ReadWriter) (traceID, transactionID string) {
	traceID = uuid.Must(uuid.NewV4()).String()
	transactionID = uuid.Must(uuid.NewV4()).String()
	transaction := modelpb.APMEvent{Transaction: &modelpb.Transaction{Id: transactionID}}
	err := readWriter.WriteTraceEvent(traceID, transactionID, &transaction, WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	})
	assert.NoError(t, err)
	return
}

func eventExists(t *testing.T, readWriter *ReadWriter, traceID, transactionID string) (ok bool) {
	var batch modelpb.Batch
	err := readWriter.ReadTraceEvents(traceID, &batch)
	require.NoError(t, err)
	for _, e := range batch {
		if e.GetTransaction().GetId() == transactionID {
			ok = true
		}
	}
	return
}

func fillTxnUntilTxnTooBig(txn *badger.Txn) {
	var err error
	for {
		if err == badger.ErrTxnTooBig {
			break
		}
		entry := badger.NewEntry([]byte{0}, []byte{})
		err = txn.SetEntry(entry)
	}
}
