// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func BenchmarkShardedWriteTransactionUncontended(b *testing.B) {
	db := newBadgerDB(b, badgerOptions)
	store := eventstorage.New(db, eventstorage.ProtobufCodec{})
	sharded := store.NewShardedReadWriter()
	defer sharded.Close()
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}

	b.RunParallel(func(pb *testing.PB) {
		traceID := uuid.Must(uuid.NewV4()).String()
		transaction := &modelpb.APMEvent{
			Transaction: &modelpb.Transaction{Id: traceID},
		}
		for pb.Next() {
			if err := sharded.WriteTraceEvent(traceID, traceID, transaction, wOpts); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkShardedWriteTransactionContended(b *testing.B) {
	db := newBadgerDB(b, badgerOptions)
	store := eventstorage.New(db, eventstorage.ProtobufCodec{})
	sharded := store.NewShardedReadWriter()
	defer sharded.Close()
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}

	// Use a single trace ID, causing all events to go through
	// the same sharded writer, contending for a single lock.
	traceID := uuid.Must(uuid.NewV4()).String()

	b.RunParallel(func(pb *testing.PB) {
		transactionID := uuid.Must(uuid.NewV4()).String()
		transaction := &modelpb.APMEvent{
			Transaction: &modelpb.Transaction{Id: transactionID},
		}
		for pb.Next() {
			if err := sharded.WriteTraceEvent(traceID, transactionID, transaction, wOpts); err != nil {
				b.Fatal(err)
			}
		}
	})
}
