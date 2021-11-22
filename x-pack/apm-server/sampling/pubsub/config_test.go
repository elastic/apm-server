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
		err    string
	}

	for _, test := range []test{{
		config: pubsub.Config{},
		err:    "Client unspecified",
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
		},
		err: "DataStream unspecified or invalid: Type unspecified",
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type: "type",
			},
		},
		err: "DataStream unspecified or invalid: Dataset unspecified",
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:    "type",
				Dataset: "dataset",
			},
		},
		err: "DataStream unspecified or invalid: Namespace unspecified",
	}, {
		config: pubsub.Config{
			Client: elasticsearchClient,
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
		},
		err: "BeatID unspecified",
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
		err: "SearchInterval unspecified or negative",
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
		err: "FlushInterval unspecified or negative",
	}} {
		pubsub, err := pubsub.New(test.config)
		require.Error(t, err)
		require.Nil(t, pubsub)
		assert.EqualError(t, err, "invalid pubsub config: "+test.err)
	}
}
