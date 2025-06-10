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
<<<<<<< HEAD
=======

	// partitionsPerTTL holds the number of partitions that events in 1 TTL should be stored over.
	// Increasing partitionsPerTTL increases read amplification, but decreases storage overhead,
	// as TTL GC can be performed sooner.
	//
	// For example, partitionPerTTL=1 means we need to keep 2 partitions active,
	// such that the last entry in the previous partition is also kept for a full TTL.
	// This means storage requirement is 2 * TTL, and it needs to read 2 keys per trace ID read.
	// If partitionPerTTL=2, storage requirement is 1.5 * TTL at the expense of 3 reads per trace ID read.
	partitionsPerTTL = 1

	// reservedKeyPrefix is the prefix of internal keys used by StorageManager
	reservedKeyPrefix byte = '~'

	// partitionerMetaKey is the key used to store partitioner metadata, e.g. last partition ID, in decision DB.
	partitionerMetaKey = string(reservedKeyPrefix) + "partitioner"

	// diskUsageFetchInterval is how often disk usage is fetched which is equivalent to how long disk usage is cached.
	diskUsageFetchInterval = 1 * time.Second

	// dbStorageLimitFallback is the default fallback storage limit in bytes
	// that applies when disk usage threshold cannot be enforced due to an error.
	dbStorageLimitFallback = 3 << 30

	// defaultValueLogSize default is 0 because pebble does not have a vlog.
	defaultValueLogSize = 0

	gb = float64(1 << 30)
>>>>>>> d147b7af (tbs: Update storage metrics to be reported synchronously in the existing `runDiskUsageLoop` method (#17154))
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
<<<<<<< HEAD
}

// NewStorageManager returns a new StorageManager with badger DB at storageDir.
func NewStorageManager(storageDir string) (*StorageManager, error) {
=======

	// meterProvider is the OTel meter provider
	meterProvider  metric.MeterProvider
	storageMetrics storageMetrics

	metricRegistration metric.Registration
}

type storageMetrics struct {
	lsmSizeGauge      metric.Int64Gauge
	valueLogSizeGauge metric.Int64Gauge
}

// NewStorageManager returns a new StorageManager with pebble DB at storageDir.
func NewStorageManager(storageDir string, logger *logp.Logger, opts ...StorageManagerOptions) (*StorageManager, error) {
>>>>>>> d147b7af (tbs: Update storage metrics to be reported synchronously in the existing `runDiskUsageLoop` method (#17154))
	sm := &StorageManager{
		storageDir: storageDir,
		runCh:      make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
	}
	err := sm.reset()
	if err != nil {
		return nil, err
	}
<<<<<<< HEAD
=======
	for _, opt := range opts {
		opt(sm)
	}

	if sm.meterProvider != nil {
		meter := sm.meterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage")

		sm.storageMetrics.lsmSizeGauge, _ = meter.Int64Gauge("apm-server.sampling.tail.storage.lsm_size")
		sm.storageMetrics.valueLogSizeGauge, _ = meter.Int64Gauge("apm-server.sampling.tail.storage.value_log_size")
	}

	if err := sm.reset(); err != nil {
		return nil, fmt.Errorf("storage manager reset error: %w", err)
	}

>>>>>>> d147b7af (tbs: Update storage metrics to be reported synchronously in the existing `runDiskUsageLoop` method (#17154))
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

<<<<<<< HEAD
func (s *StorageManager) NewTransaction(update bool) *badger.Txn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.NewTransaction(update)
=======
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
	return int64(sm.dbSize()), defaultValueLogSize
}

// dbSize returns the disk usage of databases in bytes.
func (sm *StorageManager) dbSize() uint64 {
	// pebble DiskSpaceUsage overhead is not high, but it adds up when performed per-event.
	return sm.cachedDBSize.Load()
}

func (sm *StorageManager) updateDiskUsage() {
	lsmSize := sm.getDBSize()
	sm.cachedDBSize.Store(lsmSize)

	if sm.storageMetrics.lsmSizeGauge != nil {
		sm.storageMetrics.lsmSizeGauge.Record(context.Background(), int64(lsmSize))
	}
	if sm.storageMetrics.valueLogSizeGauge != nil {
		sm.storageMetrics.valueLogSizeGauge.Record(context.Background(), int64(defaultValueLogSize))
	}

	if sm.getDiskUsageFailed.Load() {
		// Skip GetDiskUsage under the assumption that
		// it will always get the same error if GetDiskUsage ever returns one,
		// such that it does not keep logging GetDiskUsage errors.
		return
	}
	usage, err := sm.getDiskUsage()
	if err != nil {
		sm.logger.With(logp.Error(err)).Warn("failed to get disk usage")
		sm.getDiskUsageFailed.Store(true)
		sm.cachedDiskStat.used.Store(0)
		sm.cachedDiskStat.total.Store(0) // setting total to 0 to disable any running disk usage threshold checks
		return
	}
	sm.cachedDiskStat.used.Store(usage.UsedBytes)
	sm.cachedDiskStat.total.Store(usage.TotalBytes)
}

// diskUsed returns the actual used disk space in bytes.
// Not to be confused with dbSize which is specific to database.
func (sm *StorageManager) diskUsed() uint64 {
	return sm.cachedDiskStat.used.Load()
}

// runDiskUsageLoop runs a loop that updates cached disk usage regularly and reports usage.
func (sm *StorageManager) runDiskUsageLoop(stopping <-chan struct{}) error {
	// initial disk usage update so data is available immediately
	sm.updateDiskUsage()

	ticker := time.NewTicker(diskUsageFetchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopping:
			return nil
		case <-ticker.C:
			sm.updateDiskUsage()
		}
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
>>>>>>> d147b7af (tbs: Update storage metrics to be reported synchronously in the existing `runDiskUsageLoop` method (#17154))
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
