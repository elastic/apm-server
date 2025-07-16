// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package pubsub_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func TestConfigInvalid(t *testing.T) {
	type test struct {
		config pubsub.Config
		err    string
	}

	for _, test := range []test{{
		config: pubsub.Config{},
		err:    "client unspecified",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
		},
		err: "DataStream unspecified or invalid: type unspecified",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
			DataStream: pubsub.DataStreamConfig{
				Type: "type",
			},
		},
		err: "DataStream unspecified or invalid: dataset unspecified",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
			DataStream: pubsub.DataStreamConfig{
				Type:    "type",
				Dataset: "dataset",
			},
		},
		err: "DataStream unspecified or invalid: namespace unspecified",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
		},
		err: "ServerID unspecified",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
			ServerID: "server_id",
		},
		err: "SearchInterval unspecified or negative",
	}, {
		config: pubsub.Config{
			Client: &elastictransport.Client{},
			DataStream: pubsub.DataStreamConfig{
				Type:      "type",
				Dataset:   "dataset",
				Namespace: "namespace",
			},
			ServerID:       "server_id",
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
