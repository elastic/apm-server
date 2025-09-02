// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
<<<<<<< HEAD
)

func badgerModTime(dir string) time.Time {
	oldest := time.Now()
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		if (ext == ".vlog" || ext == ".sst") && info.ModTime().Before(oldest) {
			oldest = info.ModTime()
		}
		return nil
	})
	return oldest
}

func TestDropAndRecreate_filesRecreated(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := NewStorageManager(tempDir)
	require.NoError(t, err)
=======

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func newStorageManager(tb testing.TB, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm := newStorageManagerLogger(tb, logptest.NewTestingLogger(tb, ""), opts...)
	return sm
}

func newStorageManagerLogger(tb testing.TB, logger *logp.Logger, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm := newStorageManagerNoCleanup(tb, tb.TempDir(), logger, opts...)
	tb.Cleanup(func() { sm.Close() })
	return sm
}

func newStorageManagerNoCleanup(tb testing.TB, path string, logger *logp.Logger, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm, err := eventstorage.NewStorageManager(path, logger, opts...)
	if err != nil {
		tb.Fatal(err)
	}
	return sm
}

func newUnlimitedReadWriter(sm *eventstorage.StorageManager) eventstorage.RW {
	return sm.NewReadWriter(0, 0)
}

func TestStorageManager_samplingDecisionTTL(t *testing.T) {
	sm := newStorageManager(t)
	rw := newUnlimitedReadWriter(sm)
	traceID := uuid.Must(uuid.NewV4()).String()
	err := rw.WriteTraceSampled(traceID, true)
	assert.NoError(t, err)
	sampled, err := rw.IsTraceSampled(traceID)
	assert.NoError(t, err)
	assert.True(t, sampled)

	// after 1 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	sampled, err = rw.IsTraceSampled(traceID)
	assert.NoError(t, err)
	assert.True(t, sampled)

	// after 2 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	_, err = rw.IsTraceSampled(traceID)
	assert.ErrorIs(t, err, eventstorage.ErrNotFound)

	// after 3 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	_, err = rw.IsTraceSampled(traceID)
	assert.ErrorIs(t, err, eventstorage.ErrNotFound)
}

func TestStorageManager_eventTTL(t *testing.T) {
	sm := newStorageManager(t)
	rw := newUnlimitedReadWriter(sm)
	traceID := uuid.Must(uuid.NewV4()).String()
	txnID1 := uuid.Must(uuid.NewV4()).String()
	txn1 := makeTransaction(txnID1, traceID)
	err := rw.WriteTraceEvent(traceID, txnID1, txn1)
	assert.NoError(t, err)

	var out modelpb.Batch
	err = rw.ReadTraceEvents(traceID, &out)
	assert.NoError(t, err)
	assert.Len(t, out, 1)

	// after 1 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	txnID2 := uuid.Must(uuid.NewV4()).String()
	txn2 := makeTransaction(txnID2, traceID)
	err = rw.WriteTraceEvent(traceID, txnID2, txn2)
	assert.NoError(t, err)

	out = nil
	err = rw.ReadTraceEvents(traceID, &out)
	assert.NoError(t, err)
	assert.Equal(t, modelpb.Batch{txn2, txn1}, out)

	// after 2 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	out = nil
	err = rw.ReadTraceEvents(traceID, &out)
	assert.NoError(t, err)
	assert.Equal(t, modelpb.Batch{txn2}, out)

	// after 3 TTL
	err = sm.RotatePartitions()
	assert.NoError(t, err)

	out = nil
	err = rw.ReadTraceEvents(traceID, &out)
	assert.NoError(t, err)
	assert.Len(t, out, 0)
}

func TestStorageManager_partitionID(t *testing.T) {
	const traceID = "foo"
	tmpDir := t.TempDir()
	sm := newStorageManagerNoCleanup(t, tmpDir, logptest.NewTestingLogger(t, ""))

	// 0 -> 1
	assert.NoError(t, sm.RotatePartitions())

	// write to partition 1
	err := newUnlimitedReadWriter(sm).WriteTraceSampled(traceID, true)
	assert.NoError(t, err)

	assert.NoError(t, sm.Close())

	// it should read directly from partition 1 on startup instead of 0
	sm = newStorageManagerNoCleanup(t, tmpDir, logptest.NewTestingLogger(t, ""))
>>>>>>> d007a3d6 (test: use noop logger in benchmarks (#18448))
	defer sm.Close()

	oldModTime := badgerModTime(tempDir)

	err = sm.dropAndRecreate()
	assert.NoError(t, err)

	newModTime := badgerModTime(tempDir)

	assert.Greater(t, newModTime, oldModTime)
}

func TestDropAndRecreate_subscriberPositionFile(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("exists=%t", exists), func(t *testing.T) {
			tempDir := t.TempDir()
			sm, err := NewStorageManager(tempDir)
			require.NoError(t, err)
			defer sm.Close()

			if exists {
				err := sm.WriteSubscriberPosition([]byte("{}"))
				require.NoError(t, err)
			}

			err = sm.dropAndRecreate()
			assert.NoError(t, err)

			data, err := sm.ReadSubscriberPosition()
			if exists {
				assert.Equal(t, "{}", string(data))
			} else {
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
