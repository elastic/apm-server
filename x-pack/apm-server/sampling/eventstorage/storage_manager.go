// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	// subscriberPositionFile holds the file name used for persisting
	// the subscriber position across server restarts.
	subscriberPositionFile = "subscriber_position.json"

	// partitionsPerTTL holds the number of partitions that events in 1 TTL should be stored over.
	// Increasing partitionsPerTTL increases read amplification, but decreases storage overhead,
	// as TTL GC can be performed sooner.
	partitionsPerTTL = 1
)

type wrappedDB struct {
	partitioner *Partitioner
	db          *pebble.DB
}

func (w *wrappedDB) Get(key []byte) ([]byte, io.Closer, error) {
	return w.db.Get(key)
}

func (w *wrappedDB) Set(key, value []byte, opts *pebble.WriteOptions) error {
	return w.db.Set(key, value, opts)
}

func (w *wrappedDB) Delete(key []byte, opts *pebble.WriteOptions) error {
	return w.db.Delete(key, opts)
}

func (w *wrappedDB) NewIter(o *pebble.IterOptions) (*pebble.Iterator, error) {
	return w.db.NewIter(o)
}

func (w *wrappedDB) ReadPartitions() PartitionIterator {
	return w.partitioner.Actives()
}

func (w *wrappedDB) WritePartition() PartitionIterator {
	return w.partitioner.Current()
}

type StorageManagerOptions func(*StorageManager)

func WithCodec(codec Codec) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.codec = codec
	}
}

// StorageManager encapsulates pebble.DB.
// It assumes exclusive access to pebble DB at storageDir.
type StorageManager struct {
	storageDir string
	logger     *logp.Logger

	eventDB         *pebble.DB
	decisionDB      *pebble.DB
	eventStorage    *Storage
	decisionStorage *Storage

	partitioner *Partitioner // FIXME: load the correct partition ID on restart

	codec Codec

	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// runCh acts as a mutex to ensure only 1 Run is actively running per StorageManager.
	// as it is possible that 2 separate Run are created by 2 TBS processors during a hot reload.
	runCh chan struct{}
}

// NewStorageManager returns a new StorageManager with pebble DB at storageDir.
func NewStorageManager(storageDir string, opts ...StorageManagerOptions) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir:  storageDir,
		runCh:       make(chan struct{}, 1),
		logger:      logp.NewLogger(logs.Sampling),
		codec:       ProtobufCodec{},
		partitioner: NewPartitioner(partitionsPerTTL + 1),
	}
	for _, opt := range opts {
		opt(sm)
	}
	err := sm.reset()
	if err != nil {
		return nil, err
	}
	return sm, nil
}

// reset initializes db and storage.
func (sm *StorageManager) reset() error {
	eventDB, err := OpenEventPebble(sm.storageDir)
	if err != nil {
		return err
	}
	sm.eventDB = eventDB
	sm.eventStorage = New(&wrappedDB{partitioner: sm.partitioner, db: sm.eventDB}, sm.codec)

	decisionDB, err := OpenDecisionPebble(sm.storageDir)
	if err != nil {
		return err
	}
	sm.decisionDB = decisionDB
	sm.decisionStorage = New(&wrappedDB{partitioner: sm.partitioner, db: sm.decisionDB}, sm.codec)

	return nil
}

func (sm *StorageManager) Size() (lsm, vlog int64) {
	// FIXME: separate WAL usage?
	return int64(sm.eventDB.Metrics().DiskSpaceUsage() + sm.decisionDB.Metrics().DiskSpaceUsage()), 0
}

func (sm *StorageManager) Close() error {
	return sm.close()
}

func (sm *StorageManager) close() error {
	return errors.Join(sm.eventDB.Close(), sm.decisionDB.Close())
}

// Reload flushes out pending disk writes to disk by reloading the database.
// It does not flush uncommitted writes.
// For testing only.
func (sm *StorageManager) Reload() error {
	if err := sm.close(); err != nil {
		return err
	}
	return sm.reset()
}

// Run has the same lifecycle as the TBS processor as opposed to StorageManager to facilitate EA hot reload.
func (sm *StorageManager) Run(stopping <-chan struct{}, gcInterval time.Duration, ttl time.Duration, storageLimit uint64, storageLimitThreshold float64) error {
	select {
	case <-stopping:
		return nil
	case sm.runCh <- struct{}{}:
	}
	defer func() {
		<-sm.runCh
	}()

	g := errgroup.Group{}
	g.Go(func() error {
		return sm.runTTLGCLoop(stopping, ttl)
	})
	return g.Wait()
}

func (sm *StorageManager) runTTLGCLoop(stopping <-chan struct{}, ttl time.Duration) error {
	ticker := time.NewTicker(ttl / partitionsPerTTL)
	defer ticker.Stop()
	for {
		select {
		case <-stopping:
			return nil
		case <-ticker.C:
			if err := sm.IncrementPartition(); err != nil {
				sm.logger.With(logp.Error(err)).Error("failed to increment partition")
			}
		}
	}
}

func (sm *StorageManager) IncrementPartition() error {
	sm.partitioner.Rotate()
	// FIXME: potential race, wait for a bit before deleting?
	pidToDelete := sm.partitioner.Inactive().ID()
	lbPrefix := byte(pidToDelete)
	ubPrefix := lbPrefix + 1 // Do not use % here as it MUST BE greater than lb
	return errors.Join(
		sm.eventDB.DeleteRange([]byte{lbPrefix}, []byte{ubPrefix}, pebble.NoSync),
		sm.decisionDB.DeleteRange([]byte{lbPrefix}, []byte{ubPrefix}, pebble.NoSync),
		sm.eventDB.Compact([]byte{lbPrefix}, []byte{ubPrefix}, false),
		sm.decisionDB.Compact([]byte{lbPrefix}, []byte{ubPrefix}, false),
	)
}

func (sm *StorageManager) ReadSubscriberPosition() ([]byte, error) {
	sm.subscriberPosMu.Lock()
	defer sm.subscriberPosMu.Unlock()
	return os.ReadFile(filepath.Join(sm.storageDir, subscriberPositionFile))
}

func (sm *StorageManager) WriteSubscriberPosition(data []byte) error {
	sm.subscriberPosMu.Lock()
	defer sm.subscriberPosMu.Unlock()
	return os.WriteFile(filepath.Join(sm.storageDir, subscriberPositionFile), data, 0644)
}

func (sm *StorageManager) NewReadWriter() SplitReadWriter {
	return SplitReadWriter{
		eventRW:    sm.eventStorage.NewReadWriter(),
		decisionRW: sm.decisionStorage.NewReadWriter(),
	}
}

// NewBypassReadWriter returns a SplitReadWriter directly reading and writing to the database,
// bypassing any wrappers.
// This should be used for testing only, useful to check if data is actually persisted to the DB.
func (sm *StorageManager) NewBypassReadWriter() SplitReadWriter {
	return SplitReadWriter{
		eventRW:    sm.eventStorage.NewReadWriter(),
		decisionRW: sm.decisionStorage.NewReadWriter(),
	}
}

type SplitReadWriter struct {
	eventRW, decisionRW RW
}

func (s SplitReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.eventRW.ReadTraceEvents(traceID, out)
}

func (s SplitReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	return s.eventRW.WriteTraceEvent(traceID, id, event, opts)
}

func (s SplitReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	return s.decisionRW.WriteTraceSampled(traceID, sampled, opts)
}

func (s SplitReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.decisionRW.IsTraceSampled(traceID)
}

func (s SplitReadWriter) DeleteTraceEvent(traceID, id string) error {
	return s.eventRW.DeleteTraceEvent(traceID, id)
}

func (s SplitReadWriter) Flush() error {
	return nil
}

func (s SplitReadWriter) Close() error {
	return nil
}
