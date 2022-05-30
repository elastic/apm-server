// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package pubsub

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/elasticsearch"
)

// Config holds configuration for Pubsub.
type Config struct {
	// Client holds an Elasticsearch client, for indexing and searching for
	// trace ID observations.
	Client elasticsearch.Client

	// CompressionLevel holds the gzip compression level to use when bulk indexing.
	// See model/modelindexer.Config.CompressionLevel for details.
	CompressionLevel int

	// DataStream holds the data stream.
	DataStream DataStreamConfig

	// BeatID holds the APM Server's unique ID, used for filtering out
	// local observations in the subscriber.
	BeatID string

	// SearchInterval holds the time between searches initiated by the subscriber.
	//
	// This controls how long it takes for servers to become aware of each other's
	// sampled trace IDs, and so should be in the order of tens of seconds, or low
	// minutes. In order not to lose sampled trace events, SearchInterval should be
	// no greater than half of the TTL for events in local storage.
	SearchInterval time.Duration

	// FlushInterval holds the amount of time to wait before flushing the bulk indexer.
	//
	// This adds some delay to how long it takes for other servers to become aware
	// of locally sampled trace IDs, and so should be in the order of seconds.
	FlushInterval time.Duration

	// Logger is used for logging publish and subscribe operations -- particularly
	// errors that occur asynchronously.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger
}

// DataStreamConfig holds data stream configuration for Pubsub.
type DataStreamConfig struct {
	// Type holds the data stream's type.
	Type string

	// Dataset holds the data stream's dataset.
	Dataset string

	// Namespace holds the data stream's namespace.
	Namespace string
}

var (
	ErrClientMissing         = errors.New("Client unspecified")
	ErrBeatIDMissing         = errors.New("BeatID unspecified")
	ErrSearchIntervalInvalid = errors.New("SearchInterval unspecified or negative")
	ErrFlushIntervalInvalid  = errors.New("FlushInterval unspecified or negative")
	ErrTypeMissing           = errors.New("Type unspecified")
	ErrDatasetMissing        = errors.New("Dataset unspecified")
	ErrNamespaceMissing      = errors.New("Namespace unspecified")
)

// Validate validates the configuration.
func (config Config) Validate() error {
	var result error

	if config.Client == nil {
		result = multierror.Append(result, ErrClientMissing)
	}
	if err := config.DataStream.Validate(); err != nil {
		result = multierror.Append(result, fmt.Errorf("DataStream unspecified or invalid: %w", err))
	}
	if config.BeatID == "" {
		result = multierror.Append(result, ErrBeatIDMissing)
	}
	if config.SearchInterval <= 0 {
		result = multierror.Append(result, ErrSearchIntervalInvalid)
	}
	if config.FlushInterval <= 0 {
		result = multierror.Append(result, ErrFlushIntervalInvalid)
	}
	return result
}

// Validate validates the configuration.
func (config DataStreamConfig) Validate() error {
	var result error

	if config.Type == "" {
		result = multierror.Append(result, ErrTypeMissing)
	}
	if config.Dataset == "" {
		result = multierror.Append(result, ErrDatasetMissing)
	}
	if config.Namespace == "" {
		result = multierror.Append(result, ErrNamespaceMissing)
	}
	return result
}

// String returns the data stream as a combined string.
func (config DataStreamConfig) String() string {
	return fmt.Sprintf("%s-%s-%s", config.Type, config.Dataset, config.Namespace)
}
