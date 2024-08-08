// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/elastic/apm-aggregation/aggregationpb"
)

const instrumentationName = "aggregators"

// Processor defines handling of the aggregated metrics post harvest.
// CombinedMetrics passed to the processor is pooled and it is released
// back to the pool after processor has returned. If the processor mutates
// the CombinedMetrics such that it can no longer access the pooled objects,
// then the Processor should release the objects back to the pool.
type Processor func(
	ctx context.Context,
	cmk CombinedMetricsKey,
	cm *aggregationpb.CombinedMetrics,
	aggregationIvl time.Duration,
) error

// config contains the required config for running the aggregator.
type config struct {
	DataDir                string
	Limits                 Limits
	Processor              Processor
	Partitions             uint16
	AggregationIntervals   []time.Duration
	HarvestDelay           time.Duration
	Lookback               time.Duration
	CombinedMetricsIDToKVs func([16]byte) []attribute.KeyValue
	InMemory               bool

	Meter           metric.Meter
	Tracer          trace.Tracer
	Logger          *zap.Logger
	OverflowLogging bool
}

// Option allows configuring aggregator based on functional options.
type Option func(config) config

// NewConfig creates a new aggregator config based on the passed options.
func newConfig(opts ...Option) (config, error) {
	cfg := defaultCfg()
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg, validateCfg(cfg)
}

// WithDataDir configures the data directory to be used by the database.
func WithDataDir(dataDir string) Option {
	return func(c config) config {
		c.DataDir = dataDir
		return c
	}
}

// WithLimits configures the limits to be used by the aggregator.
func WithLimits(limits Limits) Option {
	return func(c config) config {
		c.Limits = limits
		return c
	}
}

// WithProcessor configures the processor for handling of the aggregated
// metrics post harvest. Processor is called for each decoded combined
// metrics after they are harvested. CombinedMetrics passed to the
// processor is pooled and it is releasd back to the pool after processor
// has returned. If the processor mutates the CombinedMetrics such that it
// can no longer access the pooled objects, then the Processor should
// release the objects back to the pool.
func WithProcessor(processor Processor) Option {
	return func(c config) config {
		c.Processor = processor
		return c
	}
}

// WithPartitions configures the number of partitions for combined metrics
// written to pebble. Defaults to 1.
//
// Partition IDs are encoded in a way that all the partitions of a specific
// combined metric are listed before any other if compared using the bytes
// comparer.
func WithPartitions(n uint16) Option {
	return func(c config) config {
		c.Partitions = n
		return c
	}
}

// WithAggregationIntervals defines the intervals that aggregator will
// aggregate for.
func WithAggregationIntervals(aggIvls []time.Duration) Option {
	return func(c config) config {
		c.AggregationIntervals = aggIvls
		return c
	}
}

// WithHarvestDelay delays the harvest by the configured duration.
// This means that harvest for a specific processing time would be
// performed with the given delay.
//
// Without delay, a normal harvest schedule will harvest metrics
// aggregated for processing time, say `t0`, at time `t1`, where
// `t1 = t0 + aggregation_interval`. With delay of, say `d`, the
// harvester will harvest the metrics for `t0` at `t1 + d`. In
// addition to harvest the duration for which the metrics are
// aggregated by the AggregateBatch API will also be affected.
//
// The main purpose of the delay is to handle the latency of
// receiving the l1 aggregated metrics in l2 aggregation. Thus
// the value must be configured for the l2 aggregator and is
// not required for l1 aggregator. If used as such then the
// harvest delay has no effects on the duration for which the
// metrics are aggregated. This is because AggregateBatch API is
// not used by the l2 aggregator.
func WithHarvestDelay(delay time.Duration) Option {
	return func(c config) config {
		c.HarvestDelay = delay
		return c
	}
}

// WithLookback configures the maximum duration that the
// aggregator will use to query the database during harvest time
// in addition to the original period derived from aggregation
// interval i.e. the harvest interval for each aggregation interval
// will be defined as [end-Lookback-AggregationIvl, end).
//
// The main purpose of Lookback is to protect against data loss for
// multi level deployments of aggregators where AggregateCombinedMetrics
// is used to aggregate partial aggregates. In these cases, the
// Lookback configuration can protect against data loss due to
// delayed partial aggregates. Note that these delayed partial
// aggregates will only be aggregated with other delayed partial
// aggregates and thus we can have multiple aggregated metrics for
// the same CombinedMetricsKey{Interval, ProcessingTime, ID}.
func WithLookback(lookback time.Duration) Option {
	return func(c config) config {
		c.Lookback = lookback
		return c
	}
}

// WithMeter defines a custom meter which will be used for collecting
// telemetry. Defaults to the meter provided by global provider.
func WithMeter(meter metric.Meter) Option {
	return func(c config) config {
		c.Meter = meter
		return c
	}
}

// WithTracer defines a custom tracer which will be used for collecting
// traces. Defaults to the tracer provided by global provider.
func WithTracer(tracer trace.Tracer) Option {
	return func(c config) config {
		c.Tracer = tracer
		return c
	}
}

// WithCombinedMetricsIDToKVs defines a function that converts a combined
// metrics ID to zero or more attribute.KeyValue for telemetry.
func WithCombinedMetricsIDToKVs(f func([16]byte) []attribute.KeyValue) Option {
	return func(c config) config {
		c.CombinedMetricsIDToKVs = f
		return c
	}
}

// WithLogger defines a custom logger to be used by aggregator.
func WithLogger(logger *zap.Logger) Option {
	return func(c config) config {
		c.Logger = logger
		return c
	}
}

// WithOverflowLogging enables warning logs at harvest time, when overflows have occurred.
//
// Logging of overflows is disabled by default, as most callers are expected to rely on
// metrics to surface cardinality issues. Support for logging exists for historical reasons.
func WithOverflowLogging(enabled bool) Option {
	return func(c config) config {
		c.OverflowLogging = enabled
		return c
	}
}

// WithInMemory defines whether aggregator uses in-memory file system.
func WithInMemory(enabled bool) Option {
	return func(c config) config {
		c.InMemory = enabled
		return c
	}
}

func defaultCfg() config {
	return config{
		DataDir:                "/tmp",
		Processor:              stdoutProcessor,
		Partitions:             1,
		AggregationIntervals:   []time.Duration{time.Minute},
		Meter:                  otel.Meter(instrumentationName),
		Tracer:                 otel.Tracer(instrumentationName),
		CombinedMetricsIDToKVs: func(_ [16]byte) []attribute.KeyValue { return nil },
		Logger:                 zap.Must(zap.NewDevelopment()),
	}
}

func validateCfg(cfg config) error {
	if cfg.DataDir == "" {
		return errors.New("data directory is required")
	}
	if cfg.Processor == nil {
		return errors.New("processor is required")
	}
	if cfg.Partitions == 0 {
		return errors.New("partitions must be greater than zero")
	}
	if len(cfg.AggregationIntervals) == 0 {
		return errors.New("at least one aggregation interval is required")
	}
	if !sort.SliceIsSorted(cfg.AggregationIntervals, func(i, j int) bool {
		return cfg.AggregationIntervals[i] < cfg.AggregationIntervals[j]
	}) {
		return errors.New("aggregation intervals must be in ascending order")
	}
	lowest := cfg.AggregationIntervals[0]
	highest := cfg.AggregationIntervals[len(cfg.AggregationIntervals)-1]
	for i := 1; i < len(cfg.AggregationIntervals); i++ {
		ivl := cfg.AggregationIntervals[i]
		if ivl%lowest != 0 {
			return errors.New("aggregation intervals must be a factor of lowest interval")
		}
	}
	// For encoding/decoding the processing time for combined metrics we only
	// consider seconds granularity making 1 sec the lowest possible
	// aggregation interval. We also encode interval as 2 unsigned bytes making
	// 65535 (~18 hours) the highest possible aggregation interval.
	if lowest < time.Second {
		return errors.New("aggregation interval less than one second is not supported")
	}
	if highest > 18*time.Hour {
		return errors.New("aggregation interval greater than 18 hours is not supported")
	}
	return nil
}

func stdoutProcessor(
	ctx context.Context,
	cmk CombinedMetricsKey,
	_ *aggregationpb.CombinedMetrics,
	_ time.Duration,
) error {
	fmt.Printf("Recevied combined metrics with key: %+v\n", cmk)
	return nil
}
