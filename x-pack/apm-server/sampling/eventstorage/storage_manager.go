// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v2"
	"go.opentelemetry.io/otel/metric"
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

var (
	errDropAndRecreateInProgress = errors.New("db drop and recreate in progress")
)

type StorageManagerOptions func(*StorageManager)

func WithMeterProvider(mp metric.MeterProvider) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.meterProvider = mp
	}
}

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

	meterProvider metric.MeterProvider
}

// NewStorageManager returns a new StorageManager with badger DB at storageDir.
func NewStorageManager(storageDir string, opts ...StorageManagerOptions) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir: storageDir,
		runCh:      make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
	}

	for _, opt := range opts {
		opt(sm)
	}

	if err := sm.reset(); err != nil {
		return nil, fmt.Errorf("storage manager reset error: %w", err)
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
func (s *StorageManager) Run(stopping <-chan struct{}, gcInterval time.Duration, ttl time.Duration, storageLimit uint64, storageLimitThreshold float64) error {
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
		return s.runDropLoop(stopping, ttl, storageLimit, storageLimitThreshold)
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
// This is a mitigation for issue https://github.com/elastic/apm-server/issues/14923
func (s *StorageManager) runDropLoop(stopping <-chan struct{}, ttl time.Duration, storageLimitInBytes uint64, storageLimitThreshold float64) error {
	if storageLimitInBytes == 0 {
		return nil
	}

	var firstExceeded time.Time
	checkAndFix := func() error {
		lsm, vlog := s.Size()
		s.mu.RLock() // s.storage requires mutex RLock
		pending := s.storage.pendingSize.Load()
		s.mu.RUnlock()
		total := lsm + vlog + pending
		actualLimit := int64(float64(storageLimitInBytes) * storageLimitThreshold)
		if total >= actualLimit {
			now := time.Now()
			if firstExceeded.IsZero() {
				firstExceeded = now
				s.logger.Warnf(
					"badger db size (lsm+vlog+pending) (%d+%d+%d=%d) has exceeded storage limit (%d*%.1f=%d); db will be dropped and recreated if problem persists for `sampling.tail.ttl` (%s)",
					lsm, vlog, pending, total, storageLimitInBytes, storageLimitThreshold, actualLimit, ttl.String())
			}
			if now.Sub(firstExceeded) >= ttl {
				s.logger.Warnf("badger db size has exceeded storage limit for over `sampling.tail.ttl` (%s), please consider increasing `sampling.tail.storage_limit`; dropping and recreating badger db to recover", ttl.String())
				err := s.dropAndRecreate()
				if err != nil {
					s.logger.With(logp.Error(err)).Error("error dropping and recreating badger db to recover storage space")
				} else {
					s.logger.Info("badger db dropped and recreated")
				}
				firstExceeded = time.Time{}
			}
		} else {
			firstExceeded = time.Time{}
		}
		return nil
	}

	timer := time.NewTicker(time.Minute) // Eval db size every minute as badger reports them with 1m lag
	defer timer.Stop()
	for {
		if err := checkAndFix(); err != nil {
			return err
		}

		select {
		case <-stopping:
			return nil
		case <-timer.C:
			continue
		}
	}
}

// dropAndRecreate deletes the underlying badger DB files at the file system level, and replaces it with a new badger DB.
func (s *StorageManager) dropAndRecreate() (retErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	defer func() {
		// In any case (errors or not), reset StorageManager while lock is held
		err := s.reset()
		if err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("error reopening badger db: %w", err))
		}
	}()

	// Intentionally not flush rw, as storage is full.
	s.rw.Close()
	err := s.db.Close()
	if err != nil {
		return fmt.Errorf("error closing badger db: %w", err)
	}

	err = s.deleteBadgerFiles()
	if err != nil {
		return fmt.Errorf("error deleting badger db files: %w", err)
	}

	return nil
}

func (s *StorageManager) deleteBadgerFiles() error {
	// Although removing the files in place can be slower, it is less error-prone than rename-and-delete.
	// Delete every file except subscriber position file
	var (
		rootVisited         bool
		sstFiles, vlogFiles int
		otherFilenames      []string
	)
	err := filepath.WalkDir(s.storageDir, func(path string, d fs.DirEntry, _ error) error {
		if !rootVisited {
			rootVisited = true
			return nil
		}
		filename := filepath.Base(path)
		if filename == subscriberPositionFile {
			return nil
		}
		switch ext := filepath.Ext(filename); ext {
		case ".sst":
			sstFiles++
		case ".vlog":
			vlogFiles++
		default:
			otherFilenames = append(otherFilenames, filename)
		}
		return os.RemoveAll(path)
	})
	s.logger.Infof("deleted badger files: %d SST files, %d VLOG files, %d other files: [%s]",
		sstFiles, vlogFiles, len(otherFilenames), strings.Join(otherFilenames, ", "))
	return err
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
