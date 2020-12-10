// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
)

func TestNewProcessorConfigInvalid(t *testing.T) {
	var config sampling.Config
	assertInvalidConfigError := func(expectedError string) {
		t.Helper()
		agg, err := sampling.NewProcessor(config)
		require.Error(t, err)
		require.Nil(t, agg)
		assert.EqualError(t, err, "invalid tail-sampling config: "+expectedError)
	}
	assertInvalidConfigError("BeatID unspecified")
	config.BeatID = "beat"

	assertInvalidConfigError("Reporter unspecified")
	config.Reporter = func(ctx context.Context, req publish.PendingReq) error { return nil }

	assertInvalidConfigError("invalid local sampling config: FlushInterval unspecified or negative")
	config.FlushInterval = 1

	assertInvalidConfigError("invalid local sampling config: MaxDynamicServices unspecified or negative")
	config.MaxDynamicServices = 1

	assertInvalidConfigError("invalid local sampling config: Policies unspecified")
	config.Policies = []sampling.Policy{{}}
	for _, invalid := range []float64{-1, 1.0, 2.0} {
		config.Policies[0].SampleRate = invalid
		assertInvalidConfigError("invalid local sampling config: Policy 0 invalid: SampleRate unspecified or out of range [0,1)")
	}
	config.Policies[0].SampleRate = 0.5

	for _, invalid := range []float64{-1, 0, 2.0} {
		config.IngestRateDecayFactor = invalid
		assertInvalidConfigError("invalid local sampling config: IngestRateDecayFactor unspecified or out of range (0,1]")
	}
	config.IngestRateDecayFactor = 0.5

	assertInvalidConfigError("invalid remote sampling config: Elasticsearch unspecified")
	var elasticsearchClient struct {
		elasticsearch.Client
	}
	config.Elasticsearch = elasticsearchClient

	assertInvalidConfigError("invalid remote sampling config: SampledTracesIndex unspecified")
	config.SampledTracesIndex = "sampled-traces"

	assertInvalidConfigError("invalid storage config: StorageDir unspecified")
	config.StorageDir = "tbs"

	assertInvalidConfigError("invalid storage config: StorageGCInterval unspecified or negative")
	config.StorageGCInterval = 1

	assertInvalidConfigError("invalid storage config: TTL unspecified or negative")
	config.TTL = 1
}
