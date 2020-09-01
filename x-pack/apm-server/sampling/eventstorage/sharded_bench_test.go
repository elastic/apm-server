// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func BenchmarkShardedWriteTransactionUncontended(b *testing.B) {
	db := newBadgerDB(b, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)
	sharded := store.NewShardedReadWriter()
	defer sharded.Close()

	b.RunParallel(func(pb *testing.PB) {
		traceUUID := uuid.Must(uuid.NewV4())
		transaction := &model.Transaction{TraceID: traceUUID.String(), ID: traceUUID.String()}
		for pb.Next() {
			if err := sharded.WriteTransaction(transaction); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkShardedWriteTransactionContended(b *testing.B) {
	db := newBadgerDB(b, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)
	sharded := store.NewShardedReadWriter()
	defer sharded.Close()

	// Use a single trace ID, causing all events to go through
	// the same sharded writer, contending for a single lock.
	traceUUID := uuid.Must(uuid.NewV4())

	b.RunParallel(func(pb *testing.PB) {
		transactionUUID := uuid.Must(uuid.NewV4())
		transaction := &model.Transaction{TraceID: traceUUID.String(), ID: transactionUUID.String()}
		for pb.Next() {
			if err := sharded.WriteTransaction(transaction); err != nil {
				b.Fatal(err)
			}
		}
	})
}
