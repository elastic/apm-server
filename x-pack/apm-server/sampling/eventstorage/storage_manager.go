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

// StorageManager encapsulates badger.DB.
// It is to provide file system access, simplify synchronization and enable underlying db swaps.
// It assumes exclusive access to badger DB at storageDir.
type StorageManager struct {
	storageDir string
	logger     *logp.Logger

<<<<<<< HEAD
	db      *badger.DB
	storage *Storage
	rw      *ShardedReadWriter
=======
	eventDB         *pebble.DB
	decisionDB      *pebble.DB
	eventStorage    *Storage
	decisionStorage *Storage

	partitioner *Partitioner

	codec Codec
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))

	// mu guards db, storage, and rw swaps.
	mu sync.RWMutex
	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

<<<<<<< HEAD
=======
	// cachedDBSize is a cached result of db size.
	cachedDBSize atomic.Uint64

>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
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
		// add buffer to avoid edge case storageLimitInBytes-lsm-vlog < buffer, when writes are still always rejected
		buffer := int64(baseTransactionSize * len(s.rw.readWriters))
		if uint64(lsm+vlog+buffer) >= storageLimitInBytes {
			now := time.Now()
			if firstExceeded.IsZero() {
				firstExceeded = now
				s.logger.Warnf(
					"badger db size (%d+%d=%d) has exceeded storage limit (%d*%.1f=%d); db will be dropped and recreated if problem persists for `sampling.tail.ttl` (%s)",
					lsm, vlog, lsm+vlog, storageLimitInBytes, storageLimitThreshold, int64(float64(storageLimitInBytes)*storageLimitThreshold), ttl.String())
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

<<<<<<< HEAD
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
=======
type partitionerMeta struct {
	ID int `json:"id"`
}

// loadPartitionID loads the last saved partition ID from database,
// such that partitioner resumes from where it left off before an apm-server restart.
func (sm *StorageManager) loadPartitionID() (int, error) {
	item, closer, err := sm.decisionDB.Get([]byte(partitionerMetaKey))
	if errors.Is(err, pebble.ErrNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	defer closer.Close()
	var pid partitionerMeta
	err = json.Unmarshal(item, &pid)
	return pid.ID, err
}

// savePartitionID saves the partition ID to database to be loaded by loadPartitionID later.
func (sm *StorageManager) savePartitionID(pid int) error {
	b, err := json.Marshal(partitionerMeta{ID: pid})
	if err != nil {
		return fmt.Errorf("error marshaling partition ID: %w", err)
	}
	return sm.decisionDB.Set([]byte(partitionerMetaKey), b, pebble.NoSync)
}

func (sm *StorageManager) Size() (lsm, vlog int64) {
	// This is reporting lsm and vlog for legacy reasons.
	// vlog is always 0 because pebble does not have a vlog.
	// Keeping this legacy structure such that the metrics are comparable across versions,
	// and we don't need to update the tooling, e.g. kibana dashboards.
	//
	// TODO(carsonip): Update this to report a more helpful size to monitoring,
	// maybe broken down into event DB vs decision DB, and LSM tree vs WAL vs misc.
	// Also remember to update
	// - x-pack/apm-server/sampling/processor.go:CollectMonitoring
	// - systemtest/benchtest/expvar/metrics.go
	return int64(sm.dbSize()), 0
}

// dbSize returns the disk usage of databases in bytes.
func (sm *StorageManager) dbSize() uint64 {
	// pebble DiskSpaceUsage overhead is not high, but it adds up when performed per-event.
	return sm.cachedDBSize.Load()
}

func (sm *StorageManager) updateDiskUsage() {
	sm.cachedDBSize.Store(sm.eventDB.Metrics().DiskSpaceUsage() + sm.decisionDB.Metrics().DiskSpaceUsage())
}

// runDiskUsageLoop runs a loop that updates cached disk usage regularly.
func (sm *StorageManager) runDiskUsageLoop(stopping <-chan struct{}) error {
	ticker := time.NewTicker(diskUsageFetchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopping:
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
			return nil
		}
<<<<<<< HEAD
		filename := filepath.Base(path)
		if filename == subscriberPositionFile {
=======
	}
}

func (sm *StorageManager) Flush() error {
	return errors.Join(
		wrapNonNilErr("event db flush error: %w", sm.eventDB.Flush()),
		wrapNonNilErr("decision db flush error: %w", sm.decisionDB.Flush()),
	)
}

func (sm *StorageManager) Close() error {
	return sm.close()
}

func (sm *StorageManager) close() error {
	if sm.metricRegistration != nil {
		if err := sm.metricRegistration.Unregister(); err != nil {
			sm.logger.With(logp.Error(err)).Error("failed to unregister metric")
		}
	}
	return errors.Join(
		wrapNonNilErr("event db flush error: %w", sm.eventDB.Flush()),
		wrapNonNilErr("decision db flush error: %w", sm.decisionDB.Flush()),
		wrapNonNilErr("event db close error: %w", sm.eventDB.Close()),
		wrapNonNilErr("decision db close error: %w", sm.decisionDB.Close()),
	)
}

// Reload flushes out pending disk writes to disk by reloading the database.
// For testing only.
// Read writers created prior to Reload cannot be used and will need to be recreated via NewUnlimitedReadWriter.
func (sm *StorageManager) Reload() error {
	if err := sm.close(); err != nil {
		return err
	}
	return sm.reset()
}

// Run has the same lifecycle as the TBS processor as opposed to StorageManager to facilitate EA hot reload.
func (sm *StorageManager) Run(stopping <-chan struct{}, ttl time.Duration) error {
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
	g.Go(func() error {
		return sm.runDiskUsageLoop(stopping)
	})

	return g.Wait()
}

// runTTLGCLoop runs the TTL GC loop.
// The loop triggers a rotation on partitions at an interval based on ttl and partitionsPerTTL.
func (sm *StorageManager) runTTLGCLoop(stopping <-chan struct{}, ttl time.Duration) error {
	ttlGCInterval := ttl / partitionsPerTTL
	ticker := time.NewTicker(ttlGCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopping:
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
			return nil
		}
<<<<<<< HEAD
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
=======
	}
}

// RotatePartitions rotates the partitions to clean up TTL-expired entries.
func (sm *StorageManager) RotatePartitions() error {
	newCurrentPID, newInactivePID := sm.partitioner.Rotate()

	if err := sm.savePartitionID(newCurrentPID); err != nil {
		return err
	}

	// No lock is needed here as the only writer to sm.partitioner is exactly this function.
	lbPrefix := byte(newInactivePID)

	lb := []byte{lbPrefix}
	ub := []byte{lbPrefix + 1} // Do not use % here as ub MUST BE greater than lb

	return errors.Join(
		wrapNonNilErr("event db delete range error: %w", sm.eventDB.DeleteRange(lb, ub, pebble.NoSync)),
		wrapNonNilErr("decision db delete range error: %w", sm.decisionDB.DeleteRange(lb, ub, pebble.NoSync)),
		wrapNonNilErr("event db compact error: %w", sm.eventDB.Compact(lb, ub, false)),
		wrapNonNilErr("decision db compact error: %w", sm.decisionDB.Compact(lb, ub, false)),
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

// NewUnlimitedReadWriter returns a read writer with no storage limit.
// For testing only.
func (sm *StorageManager) NewUnlimitedReadWriter() StorageLimitReadWriter {
	return sm.NewReadWriter(0)
}

// NewReadWriter returns a read writer with storage limit.
func (sm *StorageManager) NewReadWriter(storageLimit uint64) StorageLimitReadWriter {
	splitRW := SplitReadWriter{
		eventRW:    sm.eventStorage.NewReadWriter(),
		decisionRW: sm.decisionStorage.NewReadWriter(),
	}

	dbStorageLimit := func() uint64 {
		return storageLimit
	}
	if storageLimit == 0 {
		sm.logger.Infof("setting database storage limit to unlimited")
	} else {
		sm.logger.Infof("setting database storage limit to %.1fgb", float64(storageLimit))
	}

	// To limit db size to storage_limit
	dbStorageLimitChecker := NewStorageLimitCheckerFunc(sm.dbSize, dbStorageLimit)
	dbStorageLimitRW := NewStorageLimitReadWriter("database storage limit", dbStorageLimitChecker, splitRW)

	return dbStorageLimitRW
>>>>>>> dcb08ac9 (TBS: make storage_limit follow processor lifecycle; update TBS processor config (#15488))
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
