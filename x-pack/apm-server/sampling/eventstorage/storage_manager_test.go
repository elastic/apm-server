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

	"github.com/elastic/apm-data/model/modelpb"
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

// TestDropAndRecreate_DeadlockWithInFlightWrite holds sm.mu in a write, queues
// dropAndRecreate on Lock, then lets the write call Size. Size must not take
// sm.mu again: Go's RWMutex blocks a new RLock while a writer is waiting, and
// that second lock used to deadlock the drop.
func TestDropAndRecreate_DeadlockWithInFlightWrite(t *testing.T) {
	sm, err := NewStorageManager(t.TempDir())
	require.NoError(t, err)

	rw := sm.NewReadWriter()
	traceID := "trace-1"
	event := &modelpb.APMEvent{Transaction: &modelpb.Transaction{Id: "txn-1"}}
	opts := WriterOpts{TTL: time.Minute}

	// Open the shard transaction first. The next write then reaches EncodeEvent
	// while holding sm.mu, and calls Size only after that.
	require.NoError(t, rw.WriteTraceEvent(traceID, "txn-1", event, opts))

	entered := make(chan struct{})
	release := make(chan struct{})
	sm.storage.codec = blockingCodec{
		Codec: ProtobufCodec{},
		block: func() {
			close(entered)
			<-release
		},
	}

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- rw.WriteTraceEvent(traceID, "txn-2", event, opts)
	}()
	select {
	case <-entered:
	case err := <-writeDone:
		t.Fatalf("write returned before blocking in encode: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for write to hold sm.mu")
	}

	dropDone := make(chan error, 1)
	go func() {
		dropDone <- sm.dropAndRecreate()
	}()
	require.Eventually(t, func() bool {
		// TryRLock fails once Lock has announced a waiting writer.
		if sm.mu.TryRLock() {
			sm.mu.RUnlock()
			return false
		}
		return true
	}, 5*time.Second, time.Millisecond)

	close(release)

	const deadlockTimeout = 2 * time.Second
	select {
	case err := <-dropDone:
		require.NoError(t, err)
	case <-time.After(deadlockTimeout):
		t.Fatal("deadlock: dropAndRecreate blocked in sm.mu.Lock while the write re-acquired sm.mu.RLock via Size")
	}
	select {
	case err := <-writeDone:
		require.NoError(t, err)
	case <-time.After(deadlockTimeout):
		t.Fatal("deadlock: write blocked re-acquiring sm.mu.RLock via Size")
	}
	require.NoError(t, sm.Close())
}

// blockingCodec runs block before encoding so a test can pause a write
// while it holds sm.mu and before it calls Size.
type blockingCodec struct {
	Codec
	block func()
}

func (c blockingCodec) EncodeEvent(event *modelpb.APMEvent) ([]byte, error) {
	if c.block != nil {
		c.block()
	}
	return c.Codec.EncodeEvent(event)
}
