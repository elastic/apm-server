// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	// subscriberPositionFile holds the file name used for persisting
	// the subscriber position across server restarts.
	subscriberPositionFile = "subscriber_position.json"
)

// StorageManager is an abstraction of badger.DB.
// It is to provide file system access, simplify synchronization and enable underlying db swaps.
// It assumes exclusive access to badger DB at storageDir.
type StorageManager struct {
	storageDir string
	logger     *logp.Logger

	db      *badger.DB
	storage *Storage
	rw      *ShardedReadWriter

	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// gcLoopCh acts as a mutex to ensure only 1 gc loop is running per StorageManager.
	// as it is possible that 2 separate RunGCLoop are created by 2 TBS processors during a hot reload.
	gcLoopCh chan struct{}
}

// NewStorageManager returns a new StorageManager with badger DB at storageDir.
func NewStorageManager(storageDir string) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir: storageDir,
		gcLoopCh:   make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
	}
	err := sm.Reset()
	if err != nil {
		return nil, err
	}
	return sm, nil
}

// RunGCLoop runs a loop that calls badger DB RunValueLogGC every gcInterval.
// The loop stops when it receives from stopping.
func (s *StorageManager) RunGCLoop(stopping <-chan struct{}, gcInterval time.Duration) error {
	select {
	case <-stopping:
		return nil
	case s.gcLoopCh <- struct{}{}:
	}
	defer func() {
		<-s.gcLoopCh
	}()
	// This goroutine is responsible for periodically garbage
	// collecting the Badger value log, using the recommended
	// discard ratio of 0.5.
	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopping:
			return nil
		case <-ticker.C:
			const discardRatio = 0.5
			var err error
			for err == nil {
				// Keep garbage collecting until there are no more rewrites,
				// or garbage collection fails.
				err = s.runValueLogGC(discardRatio)
			}
			if err != nil && err != badger.ErrNoRewrite {
				return err
			}
		}
	}
}

func (s *StorageManager) Close() error {
	s.rw.Close()
	return s.db.Close()
}

// Reset initializes db, storage, and rw.
func (s *StorageManager) Reset() error {
	db, err := OpenBadger(s.storageDir, -1)
	if err != nil {
		return err
	}
	s.db = db
	s.storage = New(db, ProtobufCodec{})
	s.rw = s.storage.NewShardedReadWriter()
	return nil
}

// Size returns the db size
//
// Caller should either be main Run loop or should be holding RLock already
func (s *StorageManager) Size() (lsm, vlog int64) {
	return s.db.Size()
}

func (s *StorageManager) runValueLogGC(discardRatio float64) error {
	return s.db.RunValueLogGC(discardRatio)
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

// ManagedReadWriter is a read writer that is transparent to underlying badger DB swaps done by StorageManager
type ManagedReadWriter struct {
	sm *StorageManager
}

func (s *ManagedReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.sm.rw.ReadTraceEvents(traceID, out)
}

func (s *ManagedReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	return s.sm.rw.WriteTraceEvent(traceID, id, event, opts)
}

func (s *ManagedReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	return s.sm.rw.WriteTraceSampled(traceID, sampled, opts)
}

func (s *ManagedReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.sm.rw.IsTraceSampled(traceID)
}

func (s *ManagedReadWriter) DeleteTraceEvent(traceID, id string) error {
	return s.sm.rw.DeleteTraceEvent(traceID, id)
}

func (s *ManagedReadWriter) Flush() error {
	return s.sm.rw.Flush()
}

func (s *ManagedReadWriter) Close() {
	// No-op as managed read writer should never be closed
}
