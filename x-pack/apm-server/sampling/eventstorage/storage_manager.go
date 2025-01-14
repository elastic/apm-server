// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	// subscriberPositionFile holds the file name used for persisting
	// the subscriber position across server restarts.
	subscriberPositionFile = "subscriber_position.json"
)

var (
	errDropAndRecreateInProgress = errors.New("db drop and recreate in progress")
)

// StorageManager encapsulates pebble.DB.
// It is to provide file system access, simplify synchronization and enable underlying db swaps.
// It assumes exclusive access to pebble DB at storageDir.
type StorageManager struct {
	storageDir string
	logger     *logp.Logger

	db      *pebble.DB
	storage *Storage
	rw      *ShardedReadWriter

	// mu guards db, storage, and rw swaps.
	mu sync.RWMutex
	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// runCh acts as a mutex to ensure only 1 Run is actively running per StorageManager.
	// as it is possible that 2 separate Run are created by 2 TBS processors during a hot reload.
	runCh chan struct{}
}

// NewStorageManager returns a new StorageManager with pebble DB at storageDir.
func NewStorageManager(storageDir string) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir: storageDir,
		runCh:      make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
	}
	err := sm.reset()
	if err != nil {
		return nil, err
	}
	return sm, nil
}

// reset initializes db, storage, and rw.
func (s *StorageManager) reset() error {
	db, err := OpenPebble(s.storageDir)
	if err != nil {
		return err
	}
	s.db = db
	s.storage = New(s, ProtobufCodec{})
	s.rw = s.storage.NewShardedReadWriter()
	return nil
}

// Close closes StorageManager's underlying ShardedReadWriter and badger DB
func (s *StorageManager) Close() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.rw.Close()
	return s.db.Close()
}

// Size returns the db size
func (s *StorageManager) Size() (lsm, vlog int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return 0, 0
	//return s.db.Size()
}

func (s *StorageManager) NewIndexedBatch() *pebble.Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.NewIndexedBatch()
}

// Run has the same lifecycle as the TBS processor as opposed to StorageManager to facilitate EA hot reload.
func (s *StorageManager) Run(stopping <-chan struct{}, gcInterval time.Duration, ttl time.Duration, storageLimit uint64, storageLimitThreshold float64) error {
	return nil
}

func (s *StorageManager) ReadSubscriberPosition() ([]byte, error) {
	s.subscriberPosMu.Lock()
	defer s.subscriberPosMu.Unlock()
	return os.ReadFile(filepath.Join(s.storageDir, subscriberPositionFile))
}

func (s *StorageManager) WriteSubscriberPosition(data []byte) error {
	s.subscriberPosMu.Lock()
	defer s.subscriberPosMu.Unlock()
	return os.WriteFile(filepath.Join(s.storageDir, subscriberPositionFile), data, 0644)
}

func (s *StorageManager) NewReadWriter() *ManagedReadWriter {
	return &ManagedReadWriter{
		sm: s,
	}
}

// ManagedReadWriter is a read writer that is transparent to badger DB changes done by StorageManager.
// It is a wrapper of the ShardedReadWriter under StorageManager.
type ManagedReadWriter struct {
	sm *StorageManager
}

func (s *ManagedReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	s.sm.mu.RLock()
	defer s.sm.mu.RUnlock()
	return s.sm.rw.ReadTraceEvents(traceID, out)
}

func (s *ManagedReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	ok := s.sm.mu.TryRLock()
	if !ok {
		return errDropAndRecreateInProgress
	}
	defer s.sm.mu.RUnlock()
	return s.sm.rw.WriteTraceEvent(traceID, id, event, opts)
}

func (s *ManagedReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	ok := s.sm.mu.TryRLock()
	if !ok {
		return errDropAndRecreateInProgress
	}
	defer s.sm.mu.RUnlock()
	return s.sm.rw.WriteTraceSampled(traceID, sampled, opts)
}

func (s *ManagedReadWriter) IsTraceSampled(traceID string) (bool, error) {
	s.sm.mu.RLock()
	defer s.sm.mu.RUnlock()
	return s.sm.rw.IsTraceSampled(traceID)
}

func (s *ManagedReadWriter) DeleteTraceEvent(traceID, id string) error {
	s.sm.mu.RLock()
	defer s.sm.mu.RUnlock()
	return s.sm.rw.DeleteTraceEvent(traceID, id)
}

func (s *ManagedReadWriter) Flush() error {
	s.sm.mu.RLock()
	defer s.sm.mu.RUnlock()
	return s.sm.rw.Flush()
}

// NewBypassReadWriter returns a ReadWriter directly reading and writing to the database,
// bypassing any wrapper e.g. ShardedReadWriter.
// This should be used for testing only, useful to check if data is actually persisted to the DB.
func (s *StorageManager) NewBypassReadWriter() *ReadWriter {
	return s.storage.NewReadWriter()
}
