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

<<<<<<< HEAD
func TestDropAndRecreate_filesRecreated(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := NewStorageManager(tempDir)
	require.NoError(t, err)
	defer sm.Close()

	oldModTime := badgerModTime(tempDir)

	err = sm.dropAndRecreate()
=======
func newStorageManagerNoCleanup(tb testing.TB, path string, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm, err := eventstorage.NewStorageManager(path, opts...)
	if err != nil {
		tb.Fatal(err)
	}
	return sm
}

func TestStorageManager_samplingDecisionTTL(t *testing.T) {
	sm := newStorageManager(t)
	rw := sm.NewUnlimitedReadWriter()
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
	rw := sm.NewUnlimitedReadWriter()
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
	sm := newStorageManagerNoCleanup(t, tmpDir)

	// 0 -> 1
	assert.NoError(t, sm.RotatePartitions())

	// write to partition 1
	err := sm.NewUnlimitedReadWriter().WriteTraceSampled(traceID, true)
	assert.NoError(t, err)

	assert.NoError(t, sm.Close())

	// it should read directly from partition 1 on startup instead of 0
	sm = newStorageManagerNoCleanup(t, tmpDir)
	defer sm.Close()
	sampled, err := sm.NewUnlimitedReadWriter().IsTraceSampled(traceID)
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
	assert.NoError(t, err)

	newModTime := badgerModTime(tempDir)

	assert.Greater(t, newModTime, oldModTime)
}

<<<<<<< HEAD
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
=======
func TestStorageManager_DiskUsage(t *testing.T) {
	stopping := make(chan struct{})
	defer close(stopping)
	sm := newStorageManager(t)
	go sm.Run(stopping, time.Second)

	lsm, vlog := sm.Size()
	oldSize := lsm + vlog

	err := sm.NewUnlimitedReadWriter().WriteTraceSampled("foo", true)
	require.NoError(t, err)
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))

			err = sm.dropAndRecreate()
			assert.NoError(t, err)

<<<<<<< HEAD
			data, err := sm.ReadSubscriberPosition()
			if exists {
				assert.Equal(t, "{}", string(data))
			} else {
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
=======
	assert.Eventually(t, func() bool {
		lsm, vlog := sm.Size()
		newSize := lsm + vlog
		return newSize > oldSize
	}, 10*time.Second, 100*time.Millisecond)

	lsm, vlog = sm.Size()
	oldSize = lsm + vlog

	err = sm.NewUnlimitedReadWriter().WriteTraceEvent("foo", "bar", makeTransaction("bar", "foo"))
	require.NoError(t, err)

	err = sm.Flush()
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		lsm, vlog := sm.Size()
		newSize := lsm + vlog
		return newSize > oldSize
	}, 10*time.Second, 100*time.Millisecond)
}

func TestStorageManager_Run(t *testing.T) {
	done := make(chan struct{})
	stopping := make(chan struct{})
	sm := newStorageManager(t)
	go func() {
		assert.NoError(t, sm.Run(stopping, time.Second))
		close(done)
	}()
	close(stopping)
	<-done
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
}

func TestStorageManager_StorageLimit(t *testing.T) {
	done := make(chan struct{})
	stopping := make(chan struct{})
	sm := newStorageManager(t)
	go func() {
		assert.NoError(t, sm.Run(stopping, time.Second))
		close(done)
	}()
	require.NoError(t, sm.Flush())
	lsm, _ := sm.Size()
	assert.Greater(t, lsm, int64(1))

	traceID := uuid.Must(uuid.NewV4()).String()
	txnID := uuid.Must(uuid.NewV4()).String()
	txn := makeTransaction(txnID, traceID)

	small := sm.NewReadWriter(1)
	assert.ErrorIs(t, small.WriteTraceEvent(traceID, txnID, txn), eventstorage.ErrLimitReached)

	big := sm.NewReadWriter(10 << 10)
	assert.NoError(t, big.WriteTraceEvent(traceID, txnID, txn))

	close(stopping)
	<-done
}
