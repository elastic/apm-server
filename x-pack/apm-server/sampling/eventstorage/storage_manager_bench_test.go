// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/logp"
)

func BenchmarkStorageManager_Size(b *testing.B) {
	stopping := make(chan struct{})
	defer close(stopping)
	sm := newStorageManagerLogger(b, logp.NewNopLogger())
	go sm.Run(stopping, time.Second)
	rw := newUnlimitedReadWriter(sm)
	for i := 0; i < 1000; i++ {
		traceID := uuid.Must(uuid.NewV4()).String()
		txnID := uuid.Must(uuid.NewV4()).String()
		txn := makeTransaction(txnID, traceID)
		err := rw.WriteTraceEvent(traceID, txnID, txn)
		require.NoError(b, err)
		err = rw.WriteTraceSampled(traceID, true)
		require.NoError(b, err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sm.Size()
	}
	b.StopTimer()
}
