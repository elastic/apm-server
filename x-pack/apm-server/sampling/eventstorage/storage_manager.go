// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"go.opentelemetry.io/otel/metric"
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

	// dbStorageLimitFallback is the default fallback storage limit in bytes
	// that applies when disk usage threshold cannot be enforced due to an error.
	dbStorageLimitFallback = 3 << 30

	gb = float64(1 << 30)
)

type StorageManagerOptions func(*StorageManager)

func WithCodec(codec Codec) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.codec = codec
	}
}

func WithMeterProvider(mp metric.MeterProvider) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.meterProvider = mp
	}
}

// WithGetDBSize configures getDBSize function used by StorageManager.
// For testing only.
func WithGetDBSize(getDBSize func() uint64) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.getDBSize = getDBSize
	}
}

// WithGetDiskUsage configures getDiskUsage function used by StorageManager.
// For testing only.
func WithGetDiskUsage(getDiskUsage func() (DiskUsage, error)) StorageManagerOptions {
	return func(sm *StorageManager) {
		sm.getDiskUsage = getDiskUsage
	}
}

// DiskUsage is the struct returned by getDiskUsage.
type DiskUsage struct {
	UsedBytes, TotalBytes uint64
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

	codec Codec

	// subscriberPosMu protects the subscriber file from concurrent RW.
	subscriberPosMu sync.Mutex

	// getDBSize returns the total size of databases in bytes.
	getDBSize func() uint64
	// cachedDBSize is a cached result of db size.
	cachedDBSize atomic.Uint64

	// getDiskUsage returns the disk / filesystem usage statistics of storageDir.
	getDiskUsage func() (DiskUsage, error)
	// getDiskUsageFailed indicates if getDiskUsage calls ever failed.
	getDiskUsageFailed atomic.Bool
	// cachedDiskStat is disk usage statistics about the disk only, not related to the databases.
	cachedDiskStat struct {
		used, total atomic.Uint64
	}

	// runCh acts as a mutex to ensure only 1 Run is actively running per StorageManager.
	// as it is possible that 2 separate Run are created by 2 TBS processors during a hot reload.
	runCh chan struct{}

	// meterProvider is the OTel meter provider
	meterProvider metric.MeterProvider

	metricRegistration metric.Registration
}

// NewStorageManager returns a new StorageManager with pebble DB at storageDir.
func NewStorageManager(storageDir string, opts ...StorageManagerOptions) (*StorageManager, error) {
	sm := &StorageManager{
		storageDir: storageDir,
		runCh:      make(chan struct{}, 1),
		logger:     logp.NewLogger(logs.Sampling),
		codec:      ProtobufCodec{},
		getDiskUsage: func() (DiskUsage, error) {
			usage, err := vfs.Default.GetDiskUsage(storageDir)
			return DiskUsage{
				UsedBytes:  usage.UsedBytes,
				TotalBytes: usage.TotalBytes,
			}, err
		},
	}
	sm.getDBSize = func() uint64 {
		return sm.eventDB.Metrics().DiskSpaceUsage() + sm.decisionDB.Metrics().DiskSpaceUsage()
	}
	for _, opt := range opts {
		opt(sm)
	}

	if sm.meterProvider != nil {
		meter := sm.meterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage")
		lsmSizeGauge, _ := meter.Int64ObservableGauge("apm-server.sampling.tail.storage.lsm_size")
		valueLogSizeGauge, _ := meter.Int64ObservableGauge("apm-server.sampling.tail.storage.value_log_size")

		sm.metricRegistration, _ = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
			lsmSize, valueLogSize := sm.Size()
			o.ObserveInt64(lsmSizeGauge, lsmSize)
			o.ObserveInt64(valueLogSizeGauge, valueLogSize)
			return nil
		}, lsmSizeGauge, valueLogSizeGauge)
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
	sm.cachedDBSize.Store(sm.getDBSize())

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

// NewReadWriter returns a read writer configured with storage limit and disk usage threshold.
func (sm *StorageManager) NewReadWriter(storageLimit uint64, diskUsageThreshold float64) RW {
	var rw RW = SplitReadWriter{
		eventRW:    sm.eventStorage.NewReadWriter(),
		decisionRW: sm.decisionStorage.NewReadWriter(),
	}

	// If db storage limit is set, only enforce db storage limit.
	if storageLimit > 0 {
		// dbStorageLimit returns max size of db in bytes.
		// If size of db exceeds dbStorageLimit, writes should be rejected.
		dbStorageLimit := func() uint64 {
			return storageLimit
		}
		sm.logger.Infof("setting database storage limit to %0.1fgb", float64(storageLimit)/gb)
		dbStorageLimitChecker := NewStorageLimitCheckerFunc(sm.dbSize, dbStorageLimit)
		rw = NewStorageLimitReadWriter("database storage limit", dbStorageLimitChecker, rw)
		return rw
	}

	// DB storage limit is unlimited, enforce disk usage threshold if possible.
	// Load whether getDiskUsage failed, as it was called during StorageManager initialization.
	if sm.getDiskUsageFailed.Load() {
		// Limit db size to fallback storage limit as getDiskUsage returned an error
		dbStorageLimit := func() uint64 {
			return dbStorageLimitFallback
		}
		sm.logger.Warnf("overriding database storage limit to fallback default of %0.1fgb as get disk usage failed", float64(dbStorageLimitFallback)/gb)
		dbStorageLimitChecker := NewStorageLimitCheckerFunc(sm.dbSize, dbStorageLimit)
		rw = NewStorageLimitReadWriter("database storage limit", dbStorageLimitChecker, rw)
		return rw
	}

	// diskThreshold returns max used disk space in bytes, not in percentage.
	// If size of used disk space exceeds diskThreshold, writes should be rejected.
	diskThreshold := func() uint64 {
		return uint64(float64(sm.cachedDiskStat.total.Load()) * diskUsageThreshold)
	}
	// the total disk space could change in runtime, but it is still useful to print it out in logs.
	sm.logger.Infof("setting disk usage threshold to %.2f of total disk space of %0.1fgb", diskUsageThreshold, float64(sm.cachedDiskStat.total.Load())/gb)
	diskThresholdChecker := NewStorageLimitCheckerFunc(sm.diskUsed, diskThreshold)
	rw = NewStorageLimitReadWriter(
		fmt.Sprintf("disk usage threshold %.2f", diskUsageThreshold),
		diskThresholdChecker,
		rw,
	)
	return rw
}

// wrapNonNilErr only wraps an error with format if the error is not nil.
func wrapNonNilErr(format string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format, err)
}
