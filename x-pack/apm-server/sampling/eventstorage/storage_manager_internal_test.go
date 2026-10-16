// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"runtime"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestStorageManager_loadPartitionID(t *testing.T) {
	tests := []struct {
		name      string
		saved     []byte // raw metadata value, nil to save nothing
		wantID    int
		wantError bool
	}{
		{name: "none saved", saved: nil, wantID: 0},
		{name: "first", saved: []byte(`{"id":0}`), wantID: 0},
		{name: "last in ring", saved: []byte(`{"id":2}`), wantID: partitionRingSize - 1},
		{name: "one past the ring", saved: []byte(`{"id":3}`), wantError: true},
		{name: "far past the ring", saved: []byte(`{"id":1000}`), wantError: true},
		{name: "negative", saved: []byte(`{"id":-1}`), wantError: true},
		{name: "invalid json", saved: []byte(`not json`), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			sm, err := NewStorageManager(tmpDir, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)
			if tt.saved != nil {
				require.NoError(t, sm.decisionDB.Set([]byte(partitionerMetaKey), tt.saved, pebble.Sync))
			}

			id, err := sm.loadPartitionID()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantID, id)
			require.NoError(t, sm.Close())
		})
	}
}

// TestStorageManager_invalidSavedPartitionID checks that a saved ID outside the
// ring does not stop storage from starting or panic on use: it falls back to
// partition 0, like when the saved ID cannot be loaded at all.
func TestStorageManager_invalidSavedPartitionID(t *testing.T) {
	for _, id := range []string{"3", "1000", "-1"} {
		t.Run(id, func(t *testing.T) {
			tmpDir := t.TempDir()
			sm, err := NewStorageManager(tmpDir, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)
			require.NoError(t, sm.decisionDB.Set(
				[]byte(partitionerMetaKey), []byte(`{"id":`+id+`}`), pebble.Sync,
			))
			require.NoError(t, sm.Close())

			sm, err = NewStorageManager(tmpDir, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)
			defer sm.Close()

			var current int
			sm.partitioner.CurrentIDFunc(func(pid int) { current = pid })
			assert.Equal(t, 0, current)

			// Writes go through CurrentIDFunc, which indexes by the current ID.
			rw := sm.NewReadWriter(0, 0)
			require.NoError(t, rw.WriteTraceSampled("foo", true))
			sampled, err := rw.IsTraceSampled("foo")
			require.NoError(t, err)
			assert.True(t, sampled)

			// Rotation persists a valid ID again.
			require.NoError(t, sm.RotatePartitions())
			loaded, err := sm.loadPartitionID()
			require.NoError(t, err)
			assert.Equal(t, 1, loaded)
		})
	}
}

// TestStorageManager_deleteInsideReadDuringRotation reproduces the remote
// decision path: ReadTraceEventsCallback calls DeleteTraceEvent for the events
// it has just read, while the TTL GC is rotating and waiting on the read.
func TestStorageManager_deleteInsideReadDuringRotation(t *testing.T) {
	sm, err := NewStorageManager(t.TempDir(), logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	defer sm.Close()
	rw := sm.NewReadWriter(0, 0)

	const traceID = "trace"
	ids := []string{"a", "b", "c"}
	for _, id := range ids {
		require.NoError(t, rw.WriteTraceEvent(traceID, id, &modelpb.APMEvent{
			Transaction: &modelpb.Transaction{Id: id},
			Trace:       &modelpb.Trace{Id: traceID},
		}))
	}

	rotated := make(chan error, 1)
	waiterRegistered := func() bool {
		for i := range sm.partitioner.waiters {
			if sm.partitioner.waiters[i].Load() > 0 {
				return true
			}
		}
		return false
	}

	readDone := make(chan error, 1)
	go func() {
		var scratch modelpb.Batch
		var rotating bool
		// A tiny memory limit makes every event its own callback.
		readDone <- rw.ReadTraceEventsCallback(traceID, 1, &scratch, func(batch modelpb.Batch) error {
			if !rotating {
				rotating = true
				go func() { rotated <- sm.RotatePartitions() }()
				// The read pins the oldest active partition, so the rotation
				// parks in WaitInactive until this callback chain returns.
				deadline := time.Now().Add(5 * time.Second)
				for !waiterRegistered() {
					if time.Now().After(deadline) {
						t.Error("rotation never reached WaitInactive")
						return nil
					}
					runtime.Gosched()
				}
			}
			for _, event := range batch {
				if err := rw.DeleteTraceEvent(event.Trace.Id, event.Transaction.Id); err != nil {
					return err
				}
			}
			return nil
		})
	}()

	select {
	case err := <-readDone:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: ReadTraceEventsCallback with nested DeleteTraceEvent did not finish")
	}
	select {
	case err := <-rotated:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("rotation did not finish after the read returned")
	}

	var remaining, scratch modelpb.Batch
	require.NoError(t, rw.ReadTraceEventsCallback(traceID, 1<<30, &scratch, func(batch modelpb.Batch) error {
		remaining = append(remaining, batch...)
		return nil
	}))
	assert.Empty(t, remaining, "events read for a remote decision should have been deleted")
}
