// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package pubsub_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

func TestConfigInvalid(t *testing.T) {
	var elasticsearchClient struct {
		elasticsearch.Client
	}

	type test struct {
		config pubsub.Config
		err    error
	}

	for _, test := range []test{{
		config: pubsub.Config{},
		err:    pubsub.ErrClientMissing,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
		},
		err: pubsub.ErrTypeMissing,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type: "type",
			},
		},
		err: pubsub.ErrDatasetMissing,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:    "type",
				Dataset: "dataset",
			},
		},
		err: pubsub.ErrNamespaceMissing,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
		},
		err: pubsub.ErrBeatIDMissing,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
			BeatID: "beat_id",
		},
		err: pubsub.ErrSearchIntervalInvalid,
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
			BeatID:         "beat_id",
			SearchInterval: time.Second,
		},
		err: pubsub.ErrFlushIntervalInvalid,
	}} {
		pubsub, err := pubsub.New(test.config)
		require.Nil(t, pubsub)
		assert.ErrorContains(t, err, "invalid pubsub config:")
		require.ErrorIs(t, err, test.err)
	}
}
