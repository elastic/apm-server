// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub

import (
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/go-elasticsearch/v7"
)

// Config holds configuration for Pubsub.
type Config struct {
	// Client holds an Elasticsearch client, for indexing and searching for
	// trace ID observations.
	Client *elasticsearch.Client

	// Index holds the index name.
	Index string

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

// Validate validates the configuration.
func (config Config) Validate() error {
	if config.Client == nil {
		return errors.New("Client unspecified")
	}
	if config.Index == "" {
		return errors.New("Index unspecified")
	}
	if config.BeatID == "" {
		return errors.New("BeatID unspecified")
	}
	if config.SearchInterval <= 0 {
		return errors.New("SearchInterval unspecified or negative")
	}
	if config.FlushInterval <= 0 {
		return errors.New("FlushInterval unspecified or negative")
	}
	return nil
}
