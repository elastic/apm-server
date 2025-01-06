// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"
	"golang.org/x/sync/errgroup"

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

	// mu guards db, storage, and rw swaps.
	mu sync.RWMutex
	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// runCh acts as a mutex to ensure only 1 Run is actively running per StorageManager.
	// as it is possible that 2 separate Run are created by 2 TBS processors during a hot reload.
	runCh chan struct{}
}

// NewStorageManager returns a new StorageManager with badger DB at storageDir.
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
	db, err := OpenBadger(s.storageDir, -1)
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
	return s.db.Size()
}

func (s *StorageManager) NewTransaction(update bool) *badger.Txn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.NewTransaction(update)
}

// Run has the same lifecycle as the TBS processor as opposed to StorageManager to facilitate EA hot reload.
func (s *StorageManager) Run(stopping <-chan struct{}, gcInterval time.Duration, ttl time.Duration, storageLimit uint64) error {
	select {
	case <-stopping:
		return nil
	case s.runCh <- struct{}{}:
	}
	defer func() {
		<-s.runCh
	}()

	g := errgroup.Group{}
	g.Go(func() error {
		return s.runGCLoop(stopping, gcInterval)
	})
	g.Go(func() error {
		return s.runDropLoop(stopping, ttl, storageLimit)
	})
	return g.Wait()
}

// runGCLoop runs a loop that calls badger DB RunValueLogGC every gcInterval.
func (s *StorageManager) runGCLoop(stopping <-chan struct{}, gcInterval time.Duration) error {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.RunValueLogGC(discardRatio)
}

// runDropLoop runs a loop that detects if storage limit has been exceeded for at least ttl.
// If so, it drops and recreates the underlying badger DB.
func (s *StorageManager) runDropLoop(stopping <-chan struct{}, ttl time.Duration, storageLimitInBytes uint64) error {
	if storageLimitInBytes == 0 {
		return nil
	}

	timer := time.NewTicker(min(time.Minute, ttl)) // Eval db size every minute as badger reports them with 1m lag, but use min to facilitate testing
	defer timer.Stop()
	var firstExceeded time.Time
	for {
		select {
		case <-stopping:
			return nil
		case <-timer.C:
			lsm, vlog := s.Size()
			if uint64(lsm+vlog) >= storageLimitInBytes { //FIXME: Add a bit of buffer? Is s.storage.pendingSize reliable enough?
				now := time.Now()
				if firstExceeded.IsZero() {
					firstExceeded = now
					s.logger.Warnf("badger db size has exceeded storage limit; db will be dropped and recreated if problem persists for `sampling.tail.ttl` (%s)", ttl.String())
				}
				if now.Sub(firstExceeded) >= ttl {
					s.logger.Warnf("badger db size has exceeded storage limit for over `sampling.tail.ttl` (%s), please consider increasing `sampling.tail.storage_limit`; dropping and recreating badger db to recover", ttl.String())
					err := s.dropAndRecreate()
					if err != nil {
						return fmt.Errorf("error dropping and recreating badger db to recover storage space: %w", err)
					}
					s.logger.Info("badger db dropped and recreated")
				}
			} else {
				firstExceeded = time.Time{}
			}
		}
	}
}

func getBackupPath(path string) string {
	return filepath.Join(filepath.Dir(path), filepath.Base(path)+".old")
}

// dropAndRecreate deletes the underlying badger DB at a file system level, and replaces it with a new badger DB.
func (s *StorageManager) dropAndRecreate() (retErr error) {
	backupPath := getBackupPath(s.storageDir)
	if err := os.RemoveAll(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error removing existing backup dir: %w", err)
	}

	defer func() {
		if retErr == nil {
			if err := os.RemoveAll(backupPath); err != nil {
				retErr = fmt.Errorf("error removing old badger db: %w", err)
			}
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Intentionally not flush rw, as storage is full.
	s.rw.Close()
	err := s.db.Close()
	if err != nil {
		return fmt.Errorf("error closing badger db: %w", err)
	}

	s.subscriberPosMu.Lock()
	defer s.subscriberPosMu.Unlock()

	err = os.Rename(s.storageDir, backupPath)
	if err != nil {
		return fmt.Errorf("error backing up existing badger db: %w", err)
	}

	// Since subscriber position file lives in the same tail sampling directory as badger DB,
	// Create tail sampling dir, move back subscriber position file, as it is not a part of the DB.

	// Use mode 0700 as hardcoded in badger: https://github.com/dgraph-io/badger/blob/c5b434a643bbea0c0075d9b7336b496403d0f399/db.go#L1778
	err = os.Mkdir(s.storageDir, 0700)
	if err != nil {
		return fmt.Errorf("error creating storage directory: %w", err)
	}
	err = os.Rename(filepath.Join(backupPath, subscriberPositionFile), filepath.Join(s.storageDir, subscriberPositionFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error copying subscriber position file: %w", err)
	}

	err = s.reset()
	if err != nil {
		return fmt.Errorf("error creating new badger db: %w", err)
	}
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
	s.sm.mu.RLock()
	defer s.sm.mu.RUnlock()
	return s.sm.rw.WriteTraceEvent(traceID, id, event, opts)
}

func (s *ManagedReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	s.sm.mu.RLock()
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
