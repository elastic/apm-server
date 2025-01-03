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

// StorageManager encapsulates badger.DB.
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
	err := sm.reset()
	if err != nil {
		return nil, err
	}
	return sm, nil
}

// reset initializes db, storage, and rw.
func (s *StorageManager) reset() error {
	db, err := OpenBadger(s.storageDir, -1)
	if err != nil {
		return err
	}
	s.db = db
	s.storage = New(db, ProtobufCodec{})
	s.rw = s.storage.NewShardedReadWriter()
	return nil
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

func (s *StorageManager) runValueLogGC(discardRatio float64) error {
	return s.db.RunValueLogGC(discardRatio)
}

func (s *StorageManager) Close() error {
	s.rw.Close()
	return s.db.Close()
}

// Size returns the db size
func (s *StorageManager) Size() (lsm, vlog int64) {
	return s.db.Size()
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

// NewBypassReadWriter returns a ReadWriter directly reading and writing to the database,
// bypassing any wrapper e.g. ShardedReadWriter.
// This should be used for testing only, useful to check if data is actually persisted to the DB.
func (s *StorageManager) NewBypassReadWriter() *ReadWriter {
	return s.storage.NewReadWriter()
}
