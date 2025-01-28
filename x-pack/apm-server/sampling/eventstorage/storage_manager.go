// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"golang.org/x/sync/errgroup"

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
)

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

	partitioner *Partitioner

	storageLimit atomic.Uint64

	codec Codec

	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// cachedDiskUsage is a cached result of DiskUsage
	cachedDiskUsage atomic.Uint64

	// runCh acts as a mutex to ensure only 1 Run is actively running per StorageManager.
	// as it is possible that 2 separate Run are created by 2 TBS processors during a hot reload.
	runCh chan struct{}
}

// NewStorageManager returns a new StorageManager with pebble DB at storageDir.
func NewStorageManager(storageDir string, opts ...StorageManagerOptions) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir: storageDir,
		runCh:      make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
		codec:      ProtobufCodec{},
	}
	for _, opt := range opts {
		opt(sm)
	}

	if err := sm.reset(); err != nil {
		return nil, fmt.Errorf("storage manager reset error: %w", err)
	}

	return sm, nil
}

// reset initializes db and storage.
func (sm *StorageManager) reset() error {
	eventDB, err := OpenEventPebble(sm.storageDir)
	if err != nil {
		return fmt.Errorf("open event db error: %w", err)
	}
	sm.eventDB = eventDB

	decisionDB, err := OpenDecisionPebble(sm.storageDir)
	if err != nil {
		return fmt.Errorf("open decision db error: %w", err)
	}
	sm.decisionDB = decisionDB

	// Only recreate partitioner on initial create
	if sm.partitioner == nil {
		var currentPID int
		if currentPID, err = sm.loadPartitionID(); err != nil {
			sm.logger.With(logp.Error(err)).Warn("failed to load partition ID, using 0 instead")
		}
		// We need to keep an extra partition as buffer to respect the TTL,
		// as the moving window needs to cover at least TTL at all times,
		// where the moving window is defined as:
		// all active partitions excluding current partition + duration since the start of current partition
		activePartitions := partitionsPerTTL + 1
		sm.partitioner = NewPartitioner(activePartitions, currentPID)
	}

	sm.eventStorage = New(sm.eventDB, sm.partitioner, sm.codec)
	sm.decisionStorage = New(sm.decisionDB, sm.partitioner, sm.codec)

	sm.updateDiskUsage()

	return nil
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
	var pid struct {
		ID int `json:"id"`
	}
	err = json.Unmarshal(item, &pid)
	return pid.ID, err
}

// savePartitionID saves the partition ID to database to be loaded by loadPartitionID later.
func (sm *StorageManager) savePartitionID(pid int) error {
	return sm.decisionDB.Set([]byte(partitionerMetaKey), []byte(fmt.Sprintf(`{"id":%d}`, pid)), pebble.NoSync)
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
	return int64(sm.DiskUsage()), 0
}

// DiskUsage returns the disk usage of databases in bytes.
func (sm *StorageManager) DiskUsage() uint64 {
	// pebble DiskSpaceUsage overhead is not high, but it adds up when performed per-event.
	return sm.cachedDiskUsage.Load()
}

func (sm *StorageManager) updateDiskUsage() {
	sm.cachedDiskUsage.Store(sm.eventDB.Metrics().DiskSpaceUsage() + sm.decisionDB.Metrics().DiskSpaceUsage())
}

// runDiskUsageLoop runs a loop that updates cached disk usage regularly.
func (sm *StorageManager) runDiskUsageLoop(stopping <-chan struct{}) error {
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

func (sm *StorageManager) StorageLimit() uint64 {
	return sm.storageLimit.Load()
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
	return errors.Join(
		wrapNonNilErr("event db flush error: %w", sm.eventDB.Flush()),
		wrapNonNilErr("decision db flush error: %w", sm.decisionDB.Flush()),
		wrapNonNilErr("event db close error: %w", sm.eventDB.Close()),
		wrapNonNilErr("decision db close error: %w", sm.decisionDB.Close()),
	)
}

// Reload flushes out pending disk writes to disk by reloading the database.
// For testing only.
func (sm *StorageManager) Reload() error {
	if err := sm.close(); err != nil {
		return err
	}
	return sm.reset()
}

// Run has the same lifecycle as the TBS processor as opposed to StorageManager to facilitate EA hot reload.
func (sm *StorageManager) Run(stopping <-chan struct{}, ttl time.Duration, storageLimit uint64) error {
	select {
	case <-stopping:
		return nil
	case sm.runCh <- struct{}{}:
	}
	defer func() {
		<-sm.runCh
	}()

	sm.storageLimit.Store(storageLimit)

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
			return nil
		case <-ticker.C:
			sm.logger.Info("running TTL GC to clear expired entries and reclaim disk space")
			if err := sm.RotatePartitions(); err != nil {
				sm.logger.With(logp.Error(err)).Error("failed to rotate partition")
			}
			sm.logger.Info("finished running TTL GC")
		}
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

func (sm *StorageManager) NewReadWriter() StorageLimitReadWriter {
	return NewStorageLimitReadWriter(sm, SplitReadWriter{
		eventRW:    sm.eventStorage.NewReadWriter(),
		decisionRW: sm.decisionStorage.NewReadWriter(),
	})
}

// wrapNonNilErr only wraps an error with format if the error is not nil.
func wrapNonNilErr(format string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format, err)
}
