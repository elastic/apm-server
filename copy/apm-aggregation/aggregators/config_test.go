// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestNewConfig(t *testing.T) {
	defaultCfg := defaultCfg()
	customMeter := metric.NewMeterProvider().Meter("test")
	customTracer := trace.NewTracerProvider().Tracer("test")
	for _, tc := range []struct {
		name             string
		opts             []Option
		expected         func() config
		expectedErrorMsg string
	}{
		{
			name: "empty",
			opts: nil,
			expected: func() config {
				return defaultCfg
			},
		},
		{
			name: "with_data_dir",
			opts: []Option{
				WithDataDir("/test"),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.DataDir = "/test"
				return cfg
			},
		},
		{
			name: "with_limits",
			opts: []Option{
				WithLimits(Limits{
					MaxServices:                           10,
					MaxSpanGroups:                         10,
					MaxSpanGroupsPerService:               10,
					MaxTransactionGroups:                  10,
					MaxTransactionGroupsPerService:        10,
					MaxServiceTransactionGroups:           10,
					MaxServiceTransactionGroupsPerService: 10,
				}),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.Limits = Limits{
					MaxServices:                           10,
					MaxSpanGroups:                         10,
					MaxSpanGroupsPerService:               10,
					MaxTransactionGroups:                  10,
					MaxTransactionGroupsPerService:        10,
					MaxServiceTransactionGroups:           10,
					MaxServiceTransactionGroupsPerService: 10,
				}
				return cfg
			},
		},
		{
			name: "with_aggregation_intervals",
			opts: []Option{
				WithAggregationIntervals([]time.Duration{time.Minute, time.Hour}),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.AggregationIntervals = []time.Duration{time.Minute, time.Hour}
				return cfg
			},
		},
		{
			name: "with_harvest_delay",
			opts: []Option{
				WithHarvestDelay(time.Hour),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.HarvestDelay = time.Hour
				return cfg
			},
		},
		{
			name: "with_lookback",
			opts: []Option{
				WithLookback(time.Hour),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.Lookback = time.Hour
				return cfg
			},
		},
		{
			name: "with_meter",
			opts: []Option{
				WithMeter(customMeter),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.Meter = customMeter
				return cfg
			},
		},
		{
			name: "with_tracer",
			opts: []Option{
				WithTracer(customTracer),
			},
			expected: func() config {
				cfg := defaultCfg
				cfg.Tracer = customTracer
				return cfg
			},
		},
		{
			name: "with_empty_data_dir",
			opts: []Option{
				WithDataDir(""),
			},
			expectedErrorMsg: "data directory is required",
		},
		{
			name: "with_nil_processor",
			opts: []Option{
				WithProcessor(nil),
			},
			expectedErrorMsg: "processor is required",
		},
		{
			name: "with_no_aggregation_interval",
			opts: []Option{
				WithAggregationIntervals(nil),
			},
			expectedErrorMsg: "at least one aggregation interval is required",
		},
		{
			name: "with_unsorted_aggregation_intervals",
			opts: []Option{
				WithAggregationIntervals([]time.Duration{time.Hour, time.Minute}),
			},
			expectedErrorMsg: "aggregation intervals must be in ascending order",
		},
		{
			name: "with_invalid_aggregation_intervals",
			opts: []Option{
				WithAggregationIntervals([]time.Duration{10 * time.Second, 15 * time.Second}),
			},
			expectedErrorMsg: "aggregation intervals must be a factor of lowest interval",
		},
		{
			name: "with_out_of_lower_range_aggregation_interval",
			opts: []Option{
				WithAggregationIntervals([]time.Duration{time.Millisecond}),
			},
			expectedErrorMsg: "aggregation interval less than one second is not supported",
		},
		{
			name: "with_out_of_upper_range_aggregation_interval",
			opts: []Option{
				WithAggregationIntervals([]time.Duration{20 * time.Hour}),
			},
			expectedErrorMsg: "aggregation interval greater than 18 hours is not supported",
		},
	} {
		actual, err := newConfig(tc.opts...)

		if tc.expectedErrorMsg != "" {
			assert.EqualError(t, err, tc.expectedErrorMsg)
			continue
		}

		expected := tc.expected()
		assert.NoError(t, err)

		// New logger is created for every call
		assert.NotNil(t, actual.Logger)
		actual.Logger, expected.Logger = nil, nil

		// Function values are not comparable
		assert.NotNil(t, actual.CombinedMetricsIDToKVs)
		actual.CombinedMetricsIDToKVs, expected.CombinedMetricsIDToKVs = nil, nil
		assert.NotNil(t, actual.Processor)
		actual.Processor, expected.Processor = nil, nil

		assert.Equal(t, expected, actual)
	}
}
