// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/go-elasticsearch/v7"
)

// Config holds configuration for Processor.
type Config struct {
	// BeatID holds the unique ID of this apm-server.
	BeatID string

	// Reporter holds the publish.Reporter, for publishing tail-sampled trace events.
	Reporter publish.Reporter

	LocalSamplingConfig
	RemoteSamplingConfig
	StorageConfig
}

// LocalSamplingConfig holds Processor configuration related to local reservoir sampling.
type LocalSamplingConfig struct {
	// FlushInterval holds the local sampling interval.
	//
	// This controls how long it takes for servers to become aware of each other's
	// sampled trace IDs, and so should be in the order of tens of seconds, or low
	// minutes. In order not to lose sampled trace events, FlushInterval should be
	// no greater than half of the TTL.
	FlushInterval time.Duration

	// MaxTraceGroups holds the maximum number of trace groups to track.
	//
	// Once MaxTraceGroups is reached, any root transaction forming a new trace
	// group will dropped.
	MaxTraceGroups int

	// DefaultSampleRate is the default sample rate to assign to new trace groups.
	DefaultSampleRate float64

	// IngestRateDecayFactor holds the ingest rate decay factor, used for calculating
	// the exponentially weighted moving average (EWMA) ingest rate for each trace
	// group.
	IngestRateDecayFactor float64
}

// RemoteSamplingConfig holds Processor configuration related to publishing and
// subscribing to remote sampling decisions.
type RemoteSamplingConfig struct {
	// Elasticsearch holds the Elasticsearch client to use for publishing
	// and subscribing to remote sampling decisions.
	Elasticsearch *elasticsearch.Client

	// SampledTracesIndex holds the name of the Elasticsearch index for
	// storing and searching sampled trace IDs.
	SampledTracesIndex string
}

// StorageConfig holds Processor configuration related to event storage.
type StorageConfig struct {
	// StorageDir holds the directory in which event storage will be maintained.
	StorageDir string

	// StorageGCInterval holds the amount of time between storage garbage collections.
	StorageGCInterval time.Duration

	// TTL holds the amount of time before events and sampling decisions
	// are expired from local storage.
	TTL time.Duration
}

// Validate validates the configuration.
func (config Config) Validate() error {
	if config.BeatID == "" {
		return errors.New("BeatID unspecified")
	}
	if config.Reporter == nil {
		return errors.New("Reporter unspecified")
	}
	if err := config.LocalSamplingConfig.validate(); err != nil {
		return errors.Wrap(err, "invalid local sampling config")
	}
	if err := config.RemoteSamplingConfig.validate(); err != nil {
		return errors.Wrap(err, "invalid remote sampling config")
	}
	if err := config.StorageConfig.validate(); err != nil {
		return errors.Wrap(err, "invalid storage config")
	}
	return nil
}

func (config LocalSamplingConfig) validate() error {
	if config.FlushInterval <= 0 {
		return errors.New("FlushInterval unspecified or negative")
	}
	if config.MaxTraceGroups <= 0 {
		return errors.New("MaxTraceGroups unspecified or negative")
	}
	if config.DefaultSampleRate < 0 || config.DefaultSampleRate >= 1 {
		// TODO(axw) allow sampling rate of 1.0 (100%), which would
		// cause the root transaction to be indexed, and a sampling
		// decision to be written to local storage, immediately.
		return errors.New("DefaultSampleRate unspecified or out of range [0,1)")
	}
	if config.IngestRateDecayFactor <= 0 || config.IngestRateDecayFactor > 1 {
		return errors.New("IngestRateDecayFactor unspecified or out of range (0,1]")
	}
	return nil
}

func (config RemoteSamplingConfig) validate() error {
	if config.Elasticsearch == nil {
		return errors.New("Elasticsearch unspecified")
	}
	if config.SampledTracesIndex == "" {
		return errors.New("SampledTracesIndex unspecified")
	}
	return nil
}

func (config StorageConfig) validate() error {
	if config.StorageDir == "" {
		return errors.New("StorageDir unspecified")
	}
	if config.StorageGCInterval <= 0 {
		return errors.New("StorageGCInterval unspecified or negative")
	}
	if config.TTL <= 0 {
		return errors.New("TTL unspecified or negative")
	}
	return nil
}
