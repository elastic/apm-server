// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func newStorageManager(tb testing.TB, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm := newStorageManagerNoCleanup(tb, tb.TempDir(), opts...)
	tb.Cleanup(func() { sm.Close() })
	return sm
}

func newStorageManagerNoCleanup(tb testing.TB, path string, opts ...eventstorage.StorageManagerOptions) *eventstorage.StorageManager {
	sm, err := eventstorage.NewStorageManager(path, opts...)
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
	sm := newStorageManagerNoCleanup(t, tmpDir)

	// 0 -> 1
	assert.NoError(t, sm.RotatePartitions())

	// write to partition 1
	err := newUnlimitedReadWriter(sm).WriteTraceSampled(traceID, true)
	assert.NoError(t, err)

	assert.NoError(t, sm.Close())

	// it should read directly from partition 1 on startup instead of 0
	sm = newStorageManagerNoCleanup(t, tmpDir)
	defer sm.Close()
	sampled, err := newUnlimitedReadWriter(sm).IsTraceSampled(traceID)
	assert.NoError(t, err)
	assert.True(t, sampled)
}

func TestStorageManager_DiskUsage(t *testing.T) {
	stopping := make(chan struct{})
	defer close(stopping)
	sm := newStorageManager(t)
	go sm.Run(stopping, time.Second)

	lsm, vlog := sm.Size()
	oldSize := lsm + vlog

	err := newUnlimitedReadWriter(sm).WriteTraceSampled("foo", true)
	require.NoError(t, err)

	err = sm.Flush()
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		lsm, vlog := sm.Size()
		newSize := lsm + vlog
		return newSize > oldSize
	}, 10*time.Second, 100*time.Millisecond)

	lsm, vlog = sm.Size()
	oldSize = lsm + vlog

	err = newUnlimitedReadWriter(sm).WriteTraceEvent("foo", "bar", makeTransaction("bar", "foo"))
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

	small := sm.NewReadWriter(1, 0)
	assert.ErrorIs(t, small.WriteTraceEvent(traceID, txnID, txn), eventstorage.ErrLimitReached)

	big := sm.NewReadWriter(10<<10, 0)
	assert.NoError(t, big.WriteTraceEvent(traceID, txnID, txn))

	close(stopping)
	<-done
}

func TestStorageManager_DiskThreshold(t *testing.T) {
	for _, tc := range []struct {
		name               string
		storageLimit       uint64
		diskUsage          eventstorage.DiskUsage
		getDiskUsageErr    error
		diskUsageThreshold float64
		wantErr            error
	}{
		{
			name: "under threshold",
			diskUsage: eventstorage.DiskUsage{
				UsedBytes:  1,
				TotalBytes: 10,
			},
			diskUsageThreshold: 0.9,
			wantErr:            nil,
		},
		{
			name: "above threshold",
			diskUsage: eventstorage.DiskUsage{
				UsedBytes:  9,
				TotalBytes: 10,
			},
			diskUsageThreshold: 0.8,
			wantErr:            eventstorage.ErrLimitReached,
		},
		{
			name: "unlimited 1",
			diskUsage: eventstorage.DiskUsage{
				UsedBytes:  9,
				TotalBytes: 10,
			},
			diskUsageThreshold: 1,
			wantErr:            nil,
		},
		{
			name: "unlimited 0",
			diskUsage: eventstorage.DiskUsage{
				UsedBytes:  9,
				TotalBytes: 10,
			},
			diskUsageThreshold: 0,
			wantErr:            nil,
		},
		{
			name:               "err with non-0 storage limit",
			getDiskUsageErr:    errors.New("boom"),
			diskUsageThreshold: 0.8,
			storageLimit:       1,
			wantErr:            eventstorage.ErrLimitReached,
		},
		{
			name:               "err with 0 storage limit non-0 disk usage threshold",
			getDiskUsageErr:    errors.New("boom"),
			diskUsageThreshold: 0.8,
			storageLimit:       0,
			wantErr:            nil, // overwrite storage limit to a fallback default
		},
		{
			name:               "err with 0 storage limit 0 disk usage threshold",
			getDiskUsageErr:    errors.New("boom"),
			diskUsageThreshold: 0,
			storageLimit:       0,
			wantErr:            nil, // should not fallback, unlimited in both storage limit and disk usage threshold
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockGetDiskUsage := func() (eventstorage.DiskUsage, error) {
				return tc.diskUsage, tc.getDiskUsageErr
			}
			done := make(chan struct{})
			stopping := make(chan struct{})
			sm := newStorageManager(t, eventstorage.WithGetDiskUsage(mockGetDiskUsage))
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

			rw := sm.NewReadWriter(tc.storageLimit, tc.diskUsageThreshold)
			err := rw.WriteTraceEvent(traceID, txnID, txn)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, eventstorage.ErrLimitReached)
			} else {
				assert.NoError(t, err)
			}

			close(stopping)
			<-done
		})
	}
}
