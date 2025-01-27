// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func BenchmarkStorageManager_DiskUsage(b *testing.B) {
	sm := newStorageManager(b)
	rw := sm.NewReadWriter()
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
		_ = sm.DiskUsage()
	}
}
