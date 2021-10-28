// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

// Config holds configuration for Processor.
type Config struct {
	// BeatID holds the unique ID of this apm-server.
	BeatID string

	// BatchProcessor holds the model.BatchProcessor, for asynchronously processing
	// tail-sampled trace events.
	BatchProcessor model.BatchProcessor

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

	// MaxDynamicServices holds the maximum number of dynamic services to track.
	//
	// Once MaxDynamicServices is reached, root transactions from a service that
	// does not have an explicit policy defined may be dropped.
	MaxDynamicServices int

	// Policies holds local tail-sampling policies. Policies are matched in the
	// order provided. Policies should therefore be ordered from most to least
	// specific.
	//
	// Policies must include at least one policy that matches all traces, to ensure
	// that dropping non-matching traces is intentional.
	Policies []Policy

	// IngestRateDecayFactor holds the ingest rate decay factor, used for calculating
	// the exponentially weighted moving average (EWMA) ingest rate for each trace
	// group.
	IngestRateDecayFactor float64
}

// RemoteSamplingConfig holds Processor configuration related to publishing and
// subscribing to remote sampling decisions.
type RemoteSamplingConfig struct {
	// CompressionLevel holds the gzip compression level to use when bulk
	// indexing sampled trace IDs.
	CompressionLevel int

	// Elasticsearch holds the Elasticsearch client to use for publishing
	// and subscribing to remote sampling decisions.
	Elasticsearch elasticsearch.Client

	// SampledTracesDataStream holds the identifiers for the Elasticsearch
	// data stream for storing and searching sampled trace IDs.
	SampledTracesDataStream DataStreamConfig
}

// DataStreamConfig holds configuration to identify a data stream.
type DataStreamConfig struct {
	// Type holds the data stream's type.
	Type string

	// Dataset holds the data stream's dataset.
	Dataset string

	// Namespace holds the data stream's namespace.
	Namespace string
}

// StorageConfig holds Processor configuration related to event storage.
type StorageConfig struct {
	// DB holds the badger database in which event storage will be maintained.
	//
	// DB will not be closed when the processor is closed.
	DB *badger.DB

	// StorageDir holds the directory in which event storage will be maintained.
	StorageDir string

	// StorageGCInterval holds the amount of time between storage garbage collections.
	StorageGCInterval time.Duration

	// TTL holds the amount of time before events and sampling decisions
	// are expired from local storage.
	TTL time.Duration
}

// Policy holds a tail-sampling policy: criteria for matching root transactions,
// and the sampling parameters to apply to their traces.
type Policy struct {
	PolicyCriteria

	// SampleRate holds the tail-based sample rate to use for traces that
	// match this policy.
	SampleRate float64
}

// PolicyCriteria holds the criteria for matching root transactions to a
// tail-sampling policy.
//
// All criteria are optional. If a field is empty, it will be excluded from
// the comparison. If none are specified, then the policy will match all
// transactions.
type PolicyCriteria struct {
	// ServiceName holds the service name for which this policy applies.
	//
	// If unspecified, transactions from differing services will be
	// grouped separately for sampling purposes. This can be used for
	// defining a default/catch-all policy.
	ServiceName string

	// ServiceEnvironment holds the service environment for which this
	// policy applies.
	//
	// If unspecified, transactions from differing environments (but still
	// from the same service *name*) will be grouped together for sampling
	// purposes.
	ServiceEnvironment string

	// TraceOutcome holds the root transaction outcome for which this
	// policy applies.
	//
	// If unspecified, root transactions with differing outcomes will be
	// grouped together for sampling purposes.
	TraceOutcome string

	// TraceName holds the root transaction name for which this policy
	// applies.
	//
	// If unspecified, root transactions with differing names (but still
	// from the same service) will be grouped together for sampling purposes,
	// similar to head-based sampling.
	TraceName string
}

// Validate validates the configuration.
func (config Config) Validate() error {
	if config.BeatID == "" {
		return errors.New("BeatID unspecified")
	}
	if config.BatchProcessor == nil {
		return errors.New("BatchProcessor unspecified")
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
	if config.MaxDynamicServices <= 0 {
		return errors.New("MaxDynamicServices unspecified or negative")
	}
	if len(config.Policies) == 0 {
		return errors.New("Policies unspecified")
	}
	var anyDefaultPolicy bool
	for i, policy := range config.Policies {
		if err := policy.validate(); err != nil {
			return errors.Wrapf(err, "Policy %d invalid", i)
		}
		if policy.PolicyCriteria == (PolicyCriteria{}) {
			anyDefaultPolicy = true
		}
	}
	if !anyDefaultPolicy {
		return errors.New("Policies does not contain a default (empty criteria) policy")
	}
	if config.IngestRateDecayFactor <= 0 || config.IngestRateDecayFactor > 1 {
		return errors.New("IngestRateDecayFactor unspecified or out of range (0,1]")
	}
	return nil
}

func (config RemoteSamplingConfig) validate() error {
	if config.CompressionLevel < -1 || config.CompressionLevel > 9 {
		return errors.New("CompressionLevel out of range [-1,9]")
	}
	if config.Elasticsearch == nil {
		return errors.New("Elasticsearch unspecified")
	}
	if err := config.SampledTracesDataStream.validate(); err != nil {
		return errors.New("SampledTracesDataStream unspecified or invalid")
	}
	return nil
}

func (config DataStreamConfig) validate() error {
	return pubsub.DataStreamConfig(config).Validate()
}

func (config StorageConfig) validate() error {
	if config.DB == nil {
		return errors.New("DB unspecified")
	}
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

func (p Policy) validate() error {
	// TODO(axw) allow sampling rate of 1.0 (100%), which would
	// cause the root transaction to be indexed, and a sampling
	// decision to be written to local storage, immediately.
	if p.SampleRate < 0 || p.SampleRate >= 1 {
		return errors.New("SampleRate unspecified or out of range [0,1)")
	}
	return nil
}
