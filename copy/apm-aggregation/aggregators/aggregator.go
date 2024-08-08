// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package aggregators holds the logic for doing the actual aggregation.
package aggregators

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/telemetry"
	"github.com/elastic/apm-data/model/modelpb"
)

const (
	dbCommitThresholdBytes = 10 * 1024 * 1024 // commit every 10MB
	aggregationIvlKey      = "aggregation_interval"
	aggregationTypeKey     = "aggregation_type"
)

var (
	// ErrAggregatorClosed means that aggregator was closed when the
	// method was called and thus cannot be processed further.
	ErrAggregatorClosed = errors.New("aggregator is closed")
)

// Aggregator represents a LSM based aggregator instance to generate
// aggregated metrics. The metrics aggregated by the aggregator are
// harvested based on the aggregation interval and processed by the
// defined processor. The aggregated metrics are timestamped based
// on when the aggregator is created and the harvest loop. All the
// events collected between call to New and Run are collected in the
// same processing time bucket and thereafter the processing time
// bucket is advanced in factors of aggregation interval.
type Aggregator struct {
	db           *pebble.DB
	writeOptions *pebble.WriteOptions
	cfg          config

	mu             sync.Mutex
	processingTime time.Time
	batch          *pebble.Batch
	cachedEvents   cachedEventsMap

	closed     chan struct{}
	runStopped chan struct{}

	metrics *telemetry.Metrics
}

// New returns a new aggregator instance.
//
// Close must be called when the the aggregator is no longer needed.
func New(opts ...Option) (*Aggregator, error) {
	cfg, err := newConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregation config: %w", err)
	}

	pebbleOpts := &pebble.Options{
		Merger: &pebble.Merger{
			Name: "combined_metrics_merger",
			Merge: func(_, value []byte) (pebble.ValueMerger, error) {
				merger := combinedMetricsMerger{
					limits:      cfg.Limits,
					constraints: newConstraints(cfg.Limits),
				}
				pb := aggregationpb.CombinedMetrics{}
				if err := pb.UnmarshalVT(value); err != nil {
					return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
				}
				merger.merge(&pb)
				return &merger, nil
			},
		},
	}
	writeOptions := pebble.Sync
	if cfg.InMemory {
		pebbleOpts.FS = vfs.NewMem()
		pebbleOpts.DisableWAL = true
		writeOptions = pebble.NoSync
	}
	pb, err := pebble.Open(cfg.DataDir, pebbleOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create pebble db: %w", err)
	}

	metrics, err := telemetry.NewMetrics(
		func() *pebble.Metrics { return pb.Metrics() },
		telemetry.WithMeter(cfg.Meter),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	return &Aggregator{
		db:             pb,
		writeOptions:   writeOptions,
		cfg:            cfg,
		processingTime: time.Now().Truncate(cfg.AggregationIntervals[0]),
		closed:         make(chan struct{}),
		metrics:        metrics,
	}, nil
}

// AggregateBatch aggregates all events in the batch. This function will return
// an error if the aggregator's Run loop has errored or has been explicitly stopped.
// However, it doesn't require aggregator to be running to perform aggregation.
func (a *Aggregator) AggregateBatch(
	ctx context.Context,
	id [16]byte,
	b *modelpb.Batch,
) error {
	cmIDAttrs := a.cfg.CombinedMetricsIDToKVs(id)

	a.mu.Lock()
	defer a.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.closed:
		return ErrAggregatorClosed
	default:
	}

	var (
		errs                    []error
		successBytes, failBytes int64
	)
	cmk := CombinedMetricsKey{ID: id}
	for _, ivl := range a.cfg.AggregationIntervals {
		cmk.ProcessingTime = a.processingTime.Truncate(ivl)
		cmk.Interval = ivl
		for _, e := range *b {
			bytesIn, err := a.aggregateAPMEvent(ctx, cmk, e)
			if err != nil {
				errs = append(errs, err)
				failBytes += int64(bytesIn)
			} else {
				successBytes += int64(bytesIn)
			}
		}
		a.cachedEvents.add(ivl, id, float64(len(*b)))
	}

	var err error
	if len(errs) > 0 {
		a.metrics.BytesProcessed.Add(context.Background(), failBytes, metric.WithAttributeSet(
			attribute.NewSet(append(cmIDAttrs, telemetry.WithFailure())...),
		))
		err = fmt.Errorf("failed batch aggregation:\n%w", errors.Join(errs...))
	}
	a.metrics.BytesProcessed.Add(context.Background(), successBytes, metric.WithAttributeSet(
		attribute.NewSet(append(cmIDAttrs, telemetry.WithSuccess())...),
	))
	return err
}

// AggregateCombinedMetrics aggregates partial metrics into a bigger aggregate.
// This function will return an error if the aggregator's Run loop has errored
// or has been explicitly stopped. However, it doesn't require aggregator to be
// running to perform aggregation.
func (a *Aggregator) AggregateCombinedMetrics(
	ctx context.Context,
	cmk CombinedMetricsKey,
	cm *aggregationpb.CombinedMetrics,
) error {
	cmIDAttrs := a.cfg.CombinedMetricsIDToKVs(cmk.ID)
	traceAttrs := append(cmIDAttrs,
		attribute.String(aggregationIvlKey, formatDuration(cmk.Interval)),
		attribute.String("processing_time", cmk.ProcessingTime.String()),
	)
	ctx, span := a.cfg.Tracer.Start(ctx, "AggregateCombinedMetrics", trace.WithAttributes(traceAttrs...))
	defer span.End()

	a.mu.Lock()
	defer a.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.closed:
		return ErrAggregatorClosed
	default:
	}

	if cmk.ProcessingTime.Before(a.processingTime.Add(-a.cfg.Lookback)) {
		a.metrics.EventsProcessed.Add(
			context.Background(), cm.EventsTotal,
			metric.WithAttributeSet(attribute.NewSet(
				append(a.cfg.CombinedMetricsIDToKVs(cmk.ID),
					attribute.String(aggregationIvlKey, formatDuration(cmk.Interval)),
					telemetry.WithFailure(),
				)...,
			)),
		)
		a.cfg.Logger.Warn(
			"received expired combined metrics, dropping silently",
			zap.Time("received_processing_time", cmk.ProcessingTime),
			zap.Time("current_processing_time", a.processingTime),
		)
		return nil
	}

	var attrSetOpt metric.MeasurementOption
	bytesIn, err := a.aggregate(ctx, cmk, cm)
	if err != nil {
		attrSetOpt = metric.WithAttributeSet(
			attribute.NewSet(append(cmIDAttrs, telemetry.WithFailure())...),
		)
	} else {
		attrSetOpt = metric.WithAttributeSet(
			attribute.NewSet(append(cmIDAttrs, telemetry.WithSuccess())...),
		)
	}

	span.SetAttributes(attribute.Int("bytes_ingested", bytesIn))
	a.cachedEvents.add(cmk.Interval, cmk.ID, cm.EventsTotal)
	a.metrics.BytesProcessed.Add(context.Background(), int64(bytesIn), attrSetOpt)
	return err
}

// Run harvests the aggregated results periodically. For an aggregator,
// Run must be called at-most once.
// - Running more than once will return an error
// - Running after aggregator is stopped will return ErrAggregatorClosed.
func (a *Aggregator) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.runStopped != nil {
		a.mu.Unlock()
		return errors.New("aggregator is already running")
	}
	a.runStopped = make(chan struct{})
	a.mu.Unlock()
	defer close(a.runStopped)

	to := a.processingTime.Add(a.cfg.AggregationIntervals[0])
	timer := time.NewTimer(time.Until(to.Add(a.cfg.HarvestDelay)))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.closed:
			return ErrAggregatorClosed
		case <-timer.C:
		}

		a.mu.Lock()
		batch := a.batch
		a.batch = nil
		a.processingTime = to
		cachedEventsStats := a.cachedEvents.loadAndDelete(to)
		a.mu.Unlock()

		if err := a.commitAndHarvest(ctx, batch, to, cachedEventsStats); err != nil {
			a.cfg.Logger.Warn("failed to commit and harvest metrics", zap.Error(err))
		}
		to = to.Add(a.cfg.AggregationIntervals[0])
		timer.Reset(time.Until(to.Add(a.cfg.HarvestDelay)))
	}
}

// Close commits and closes any buffered writes, stops any running harvester,
// performs a final harvest, and closes the underlying database.
//
// No further writes may be performed after Close is called, and no further
// harvests will be performed once Close returns.
func (a *Aggregator) Close(ctx context.Context) error {
	ctx, span := a.cfg.Tracer.Start(ctx, "Aggregator.Close")
	defer span.End()

	a.mu.Lock()
	defer a.mu.Unlock()

	select {
	case <-a.closed:
	default:
		a.cfg.Logger.Info("stopping aggregator")
		close(a.closed)
	}
	if a.runStopped != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for run to complete: %w", ctx.Err())
		case <-a.runStopped:
		}
	}

	if a.db != nil {
		a.cfg.Logger.Info("running final aggregation")
		if a.batch != nil {
			if err := a.batch.Commit(a.writeOptions); err != nil {
				span.RecordError(err)
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			if err := a.batch.Close(); err != nil {
				span.RecordError(err)
				return fmt.Errorf("failed to close batch: %w", err)
			}
			a.batch = nil
		}
		var errs []error
		for _, ivl := range a.cfg.AggregationIntervals {
			// At any particular time there will be 1 harvest candidate for
			// each aggregation interval. We will align the end time and
			// process each of these.
			//
			// TODO (lahsivjar): It is possible to harvest the same
			// time multiple times, not an issue but can be optimized.
			to := a.processingTime.Truncate(ivl).Add(ivl)
			if err := a.harvest(ctx, to, a.cachedEvents.loadAndDelete(to)); err != nil {
				span.RecordError(err)
				errs = append(errs, fmt.Errorf(
					"failed to harvest metrics for interval %s: %w", formatDuration(ivl), err),
				)
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("failed while running final harvest: %w", errors.Join(errs...))
		}
		if err := a.db.Close(); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to close pebble: %w", err)
		}
		// All future operations are invalid after db is closed
		a.db = nil
	}
	if err := a.metrics.CleanUp(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to cleanup instrumentation: %w", err)
	}
	return nil
}

func (a *Aggregator) aggregateAPMEvent(
	ctx context.Context,
	cmk CombinedMetricsKey,
	e *modelpb.APMEvent,
) (int, error) {
	var totalBytesIn int
	aggregateFunc := func(k CombinedMetricsKey, m *aggregationpb.CombinedMetrics) error {
		bytesIn, err := a.aggregate(ctx, k, m)
		totalBytesIn += bytesIn
		return err
	}
	err := eventToCombinedMetrics(e, cmk, a.cfg.Partitions, aggregateFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate combined metrics: %w", err)
	}
	return totalBytesIn, nil
}

// aggregate aggregates combined metrics for a given key and returns
// number of bytes ingested along with the error, if any.
func (a *Aggregator) aggregate(
	ctx context.Context,
	cmk CombinedMetricsKey,
	cm *aggregationpb.CombinedMetrics,
) (int, error) {
	if a.batch == nil {
		// Batch is backed by a sync pool. After each commit we will release the batch
		// back to the pool by calling Batch#Close and subsequently acquire a new batch.
		a.batch = a.db.NewBatch()
	}

	op := a.batch.MergeDeferred(cmk.SizeBinary(), cm.SizeVT())
	if err := cmk.MarshalBinaryToSizedBuffer(op.Key); err != nil {
		return 0, fmt.Errorf("failed to marshal combined metrics key: %w", err)
	}
	if _, err := cm.MarshalToSizedBufferVT(op.Value); err != nil {
		return 0, fmt.Errorf("failed to marshal combined metrics: %w", err)
	}
	if err := op.Finish(); err != nil {
		return 0, fmt.Errorf("failed to finalize merge operation: %w", err)
	}

	bytesIn := cm.SizeVT()
	if a.batch.Len() >= dbCommitThresholdBytes {
		if err := a.batch.Commit(a.writeOptions); err != nil {
			return bytesIn, fmt.Errorf("failed to commit pebble batch: %w", err)
		}
		if err := a.batch.Close(); err != nil {
			return bytesIn, fmt.Errorf("failed to close pebble batch: %w", err)
		}
		a.batch = nil
	}
	return bytesIn, nil
}

func (a *Aggregator) commitAndHarvest(
	ctx context.Context,
	batch *pebble.Batch,
	to time.Time,
	cachedEventsStats map[time.Duration]map[[16]byte]float64,
) error {
	ctx, span := a.cfg.Tracer.Start(ctx, "commitAndHarvest")
	defer span.End()

	var errs []error
	if batch != nil {
		if err := batch.Commit(a.writeOptions); err != nil {
			span.RecordError(err)
			errs = append(errs, fmt.Errorf("failed to commit batch before harvest: %w", err))
		}
		if err := batch.Close(); err != nil {
			span.RecordError(err)
			errs = append(errs, fmt.Errorf("failed to close batch before harvest: %w", err))
		}
	}
	if err := a.harvest(ctx, to, cachedEventsStats); err != nil {
		span.RecordError(err)
		errs = append(errs, fmt.Errorf("failed to harvest aggregated metrics: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// harvest collects the mature metrics for all aggregation intervals and
// deletes the entries in db once the metrics are fully harvested. Harvest
// takes an end time denoting the exclusive upper bound for harvesting.
func (a *Aggregator) harvest(
	ctx context.Context,
	end time.Time,
	cachedEventsStats map[time.Duration]map[[16]byte]float64,
) error {
	snap := a.db.NewSnapshot()
	defer snap.Close()

	var errs []error
	for _, ivl := range a.cfg.AggregationIntervals {
		// Check if the given aggregation interval needs to be harvested now
		if end.Truncate(ivl).Equal(end) {
			start := end.Add(-ivl).Add(-a.cfg.Lookback)
			cmCount, err := a.harvestForInterval(
				ctx, snap, start, end, ivl, cachedEventsStats[ivl],
			)
			if err != nil {
				errs = append(errs, fmt.Errorf(
					"failed to harvest aggregated metrics for interval %s: %w",
					ivl, err,
				))
			}
			a.cfg.Logger.Debug(
				"Finished harvesting aggregated metrics",
				zap.Int("combined_metrics_successfully_harvested", cmCount),
				zap.Duration("aggregation_interval_ns", ivl),
				zap.Time("harvested_till(exclusive)", end),
				zap.Error(err),
			)
		}
	}
	return errors.Join(errs...)
}

// harvestForInterval harvests aggregated metrics for a given interval.
// Returns the number of combined metrics successfully harvested and an
// error. It is possible to have non nil error and greater than 0
// combined metrics if some of the combined metrics failed harvest.
func (a *Aggregator) harvestForInterval(
	ctx context.Context,
	snap *pebble.Snapshot,
	start, end time.Time,
	ivl time.Duration,
	cachedEventsStats map[[16]byte]float64,
) (int, error) {
	from := CombinedMetricsKey{
		Interval:       ivl,
		ProcessingTime: start,
	}
	to := CombinedMetricsKey{
		Interval:       ivl,
		ProcessingTime: end,
	}
	lb := make([]byte, CombinedMetricsKeyEncodedSize)
	ub := make([]byte, CombinedMetricsKeyEncodedSize)
	from.MarshalBinaryToSizedBuffer(lb)
	to.MarshalBinaryToSizedBuffer(ub)

	iter, err := snap.NewIter(&pebble.IterOptions{
		LowerBound: lb,
		UpperBound: ub,
		KeyTypes:   pebble.IterKeyTypePointsOnly,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create iter: %w", err)
	}
	defer iter.Close()

	var errs []error
	var cmCount int
	ivlAttr := attribute.String(aggregationIvlKey, formatDuration(ivl))
	for iter.First(); iter.Valid(); iter.Next() {
		var cmk CombinedMetricsKey
		if err := cmk.UnmarshalBinary(iter.Key()); err != nil {
			errs = append(errs, fmt.Errorf("failed to unmarshal key: %w", err))
			continue
		}
		harvestStats, err := a.processHarvest(ctx, cmk, iter.Value(), ivl)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cmCount++

		commonAttrsOpt := metric.WithAttributeSet(attribute.NewSet(
			append(a.cfg.CombinedMetricsIDToKVs(cmk.ID), ivlAttr)...,
		))

		// Report the estimated number of overflowed metrics per aggregation interval.
		// It is not meaningful to aggregate these across intervals or aggregators,
		// as the overflowed aggregation keys may be overlapping sets.
		recordMetricsOverflow := func(n uint64, aggregationType string) {
			if n == 0 {
				return
			}
			a.metrics.MetricsOverflowed.Add(context.Background(), int64(n), commonAttrsOpt,
				metric.WithAttributeSet(attribute.NewSet(
					attribute.String(aggregationTypeKey, aggregationType),
				)),
			)
		}
		recordMetricsOverflow(harvestStats.servicesOverflowed, "service")
		recordMetricsOverflow(harvestStats.transactionsOverflowed, "transaction")
		recordMetricsOverflow(harvestStats.serviceTransactionsOverflowed, "service_transaction")
		recordMetricsOverflow(harvestStats.spansOverflowed, "service_destination")

		// processingDelay is normalized by subtracting aggregation interval and
		// harvest delay, both of which are expected delays. Normalization helps
		// us to use the lower (higher resolution) range of the histogram for the
		// important values. The normalized processingDelay can be negative as a
		// result of premature harvest triggered by a stop of the aggregator. The
		// negative value is accepted as a good value and recorded in the lower
		// histogram buckets.
		processingDelay := time.Since(cmk.ProcessingTime).Seconds() -
			(ivl.Seconds() + a.cfg.HarvestDelay.Seconds())
		// queuedDelay is not explicitly normalized because we want to record the
		// full delay. For a healthy deployment, the queued delay would be
		// implicitly normalized due to the usage of youngest event timestamp.
		// Negative values are possible at edges due to delays in running the
		// harvest loop or time sync issues between agents and server.
		queuedDelay := time.Since(harvestStats.youngestEventTimestamp).Seconds()
		outcomeAttrOpt := metric.WithAttributeSet(attribute.NewSet(
			telemetry.WithSuccess()),
		)
		a.metrics.MinQueuedDelay.Record(context.Background(), queuedDelay, commonAttrsOpt, outcomeAttrOpt)
		a.metrics.ProcessingLatency.Record(context.Background(), processingDelay, commonAttrsOpt, outcomeAttrOpt)
		// Events harvested have been successfully processed, publish these
		// as success. Update the map to keep track of events failed.
		a.metrics.EventsProcessed.Add(context.Background(), harvestStats.eventsTotal, commonAttrsOpt, outcomeAttrOpt)
		cachedEventsStats[cmk.ID] -= harvestStats.eventsTotal
	}
	err = a.db.DeleteRange(lb, ub, a.writeOptions)
	if len(errs) > 0 {
		err = errors.Join(err, fmt.Errorf(
			"failed to process %d out of %d metrics:\n%w",
			len(errs), cmCount, errors.Join(errs...),
		))
	}

	// All remaining events in the cached events map should be failed events.
	// Record these events with a failure outcome.
	for cmID, eventsTotal := range cachedEventsStats {
		if eventsTotal == 0 {
			continue
		}
		if eventsTotal < 0 {
			fields := append([]zap.Field{
				zap.Duration("aggregation_interval_ns", ivl),
				zap.Float64("remaining_events", eventsTotal),
			}, otelKVsToZapFields(a.cfg.CombinedMetricsIDToKVs(cmID))...)
			a.cfg.Logger.Warn(
				"unexpectedly failed to harvest all collected events",
				fields...,
			)
			continue
		}
		attrSetOpt := metric.WithAttributeSet(
			attribute.NewSet(append(
				a.cfg.CombinedMetricsIDToKVs(cmID),
				ivlAttr,
				telemetry.WithFailure(),
			)...),
		)
		a.metrics.EventsProcessed.Add(context.Background(), eventsTotal, attrSetOpt)
	}
	return cmCount, err
}

func (a *Aggregator) processHarvest(
	ctx context.Context,
	cmk CombinedMetricsKey,
	cmb []byte,
	aggIvl time.Duration,
) (harvestStats, error) {
	cm := aggregationpb.CombinedMetrics{}
	if err := cm.UnmarshalVT(cmb); err != nil {
		return harvestStats{}, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	// Processor can mutate the CombinedMetrics, so we cannot rely on the
	// CombinedMetrics after Processor is called. Take a snapshot of the
	// fields we record if processing succeeds.
	hs := harvestStats{
		eventsTotal:            cm.EventsTotal,
		youngestEventTimestamp: modelpb.ToTime(cm.YoungestEventTimestamp),
		servicesOverflowed:     hllSketchEstimate(cm.OverflowServicesEstimator),
	}
	overflowLogger := nopLogger
	if a.cfg.OverflowLogging {
		fields := append([]zap.Field{
			zap.Duration("aggregation_interval_ns", aggIvl),
		}, otelKVsToZapFields(a.cfg.CombinedMetricsIDToKVs(cmk.ID))...)
		overflowLogger = a.cfg.Logger.WithLazy(fields...)
	}
	hs.addOverflows(&cm, a.cfg.Limits, overflowLogger)

	if err := a.cfg.Processor(ctx, cmk, &cm, aggIvl); err != nil {
		return harvestStats{}, fmt.Errorf("failed to process combined metrics ID %s: %w", cmk.ID, err)
	}
	return hs, nil
}

var (
	nopLogger = zap.NewNop()

	// TODO(carsonip): Update this log message when global labels implementation changes
	serviceGroupLimitReachedMessage = fmt.Sprintf(""+
		"Service limit reached, new metric documents will be grouped under a dedicated "+
		"overflow bucket identified by service name '%s'. "+
		"If you are sending global labels that are request-specific (e.g. client IP), it may cause "+
		"high cardinality and lead to exhaustion of services.",
		overflowBucketName,
	)

	transactionGroupLimitReachedMessage = "" +
		"Transaction group per service limit reached, " + transactionGroupLimitReachedSuffix
	overallTransactionGroupLimitReachedMessage = "" +
		"Overall transaction group limit reached, " + transactionGroupLimitReachedSuffix
	transactionGroupLimitReachedSuffix = fmt.Sprintf(""+
		"new metric documents will be grouped under a dedicated bucket identified by transaction name '%s'. "+
		"This is typically caused by ineffective transaction grouping, "+
		"e.g. by creating many unique transaction names. "+
		"If you are using an agent with 'use_path_as_transaction_name' enabled, it may cause "+
		"high cardinality. If your agent supports the 'transaction_name_groups' option, setting "+
		"that configuration option appropriately, may lead to better results.",
		overflowBucketName,
	)

	serviceTransactionGroupLimitReachedMessage = fmt.Sprintf(""+
		"Service transaction group per service limit reached, new metric documents will be grouped "+
		"under a dedicated bucket identified by transaction type '%s'.",
		overflowBucketName,
	)
	overallServiceTransactionGroupLimitReachedMessage = fmt.Sprintf(""+
		"Overall service transaction group limit reached, new metric documents will be grouped "+
		"under a dedicated bucket identified by transaction type '%s'.",
		overflowBucketName,
	)

	spanGroupLimitReachedMessage = fmt.Sprintf(""+
		"Span group per service limit reached, new metric documents will be grouped "+
		"under a dedicated bucket identified by service target name '%s'.",
		overflowBucketName,
	)
	overallSpanGroupLimitReachedMessage = fmt.Sprintf(""+
		"Overall span group limit reached, new metric documents will be grouped "+
		"under a dedicated bucket identified by service target name '%s'.",
		overflowBucketName,
	)
)

type harvestStats struct {
	eventsTotal            float64
	youngestEventTimestamp time.Time

	servicesOverflowed            uint64
	transactionsOverflowed        uint64
	serviceTransactionsOverflowed uint64
	spansOverflowed               uint64
}

func (hs *harvestStats) addOverflows(cm *aggregationpb.CombinedMetrics, limits Limits, logger *zap.Logger) {
	if hs.servicesOverflowed != 0 {
		logger.Warn(serviceGroupLimitReachedMessage, zap.Int("limit", limits.MaxServices))
	}

	// Flags to indicate the overall limit reached per aggregation type,
	// so that they are only logged once.
	var loggedOverallTransactionGroupLimitReached bool
	var loggedOverallServiceTransactionGroupLimitReached bool
	var loggedOverallSpanGroupLimitReached bool
	logLimitReached := func(
		n, limit int,
		serviceKey *aggregationpb.ServiceAggregationKey,
		perServiceMessage string,
		overallMessage string,
		loggedOverallMessage *bool,
	) {
		if serviceKey == nil {
			// serviceKey will be nil for the service overflow,
			// which is due to cardinality service keys, not
			// metric keys.
			return
		}
		if n >= limit {
			logger.Warn(
				perServiceMessage,
				zap.String("service_name", serviceKey.GetServiceName()),
				zap.Int("limit", limit),
			)
			return
		} else if !*loggedOverallMessage {
			logger.Warn(overallMessage, zap.Int("limit", limit))
			*loggedOverallMessage = true
		}
	}

	addOverflow := func(o *aggregationpb.Overflow, ksm *aggregationpb.KeyedServiceMetrics) {
		if o == nil {
			return
		}
		if overflowed := hllSketchEstimate(o.OverflowTransactionsEstimator); overflowed > 0 {
			hs.transactionsOverflowed += overflowed
			logLimitReached(
				len(ksm.GetMetrics().GetTransactionMetrics()),
				limits.MaxTransactionGroupsPerService,
				ksm.GetKey(),
				transactionGroupLimitReachedMessage,
				overallTransactionGroupLimitReachedMessage,
				&loggedOverallTransactionGroupLimitReached,
			)
		}
		if overflowed := hllSketchEstimate(o.OverflowServiceTransactionsEstimator); overflowed > 0 {
			hs.serviceTransactionsOverflowed += overflowed
			logLimitReached(
				len(ksm.GetMetrics().GetServiceTransactionMetrics()),
				limits.MaxServiceTransactionGroupsPerService,
				ksm.GetKey(),
				serviceTransactionGroupLimitReachedMessage,
				overallServiceTransactionGroupLimitReachedMessage,
				&loggedOverallServiceTransactionGroupLimitReached,
			)
		}
		if overflowed := hllSketchEstimate(o.OverflowSpansEstimator); overflowed > 0 {
			hs.spansOverflowed += overflowed
			logLimitReached(
				len(ksm.GetMetrics().GetSpanMetrics()),
				limits.MaxSpanGroupsPerService,
				ksm.GetKey(),
				spanGroupLimitReachedMessage,
				overallSpanGroupLimitReachedMessage,
				&loggedOverallSpanGroupLimitReached,
			)
		}
	}

	addOverflow(cm.OverflowServices, nil)
	for _, ksm := range cm.ServiceMetrics {
		addOverflow(ksm.GetMetrics().GetOverflowGroups(), ksm)
	}
}
