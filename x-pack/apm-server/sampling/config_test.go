// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

func TestNewProcessorConfigInvalid(t *testing.T) {
	var config sampling.Config
	assertInvalidConfigError := func(expectedError error) {
		t.Helper()
		agg, err := sampling.NewProcessor(config)
		require.Nil(t, agg)
		require.ErrorIs(t, err, expectedError)
		assert.ErrorContains(t, err, "invalid tail-sampling config: ")
	}
	assertInvalidConfigError(sampling.ErrBeatIDMissing)
	config.BeatID = "beat"

	assertInvalidConfigError(sampling.ErrBashProcessorMissing)
	config.BatchProcessor = struct{ model.BatchProcessor }{}

	assertInvalidConfigError(sampling.ErrFlushIntervalInvalid)
	config.FlushInterval = 1

	assertInvalidConfigError(sampling.ErrMaxDynamicServicesInvalid)
	config.MaxDynamicServices = 1

	assertInvalidConfigError(sampling.ErrPoliciesMissing)
	config.Policies = []sampling.Policy{{
		PolicyCriteria: sampling.PolicyCriteria{ServiceName: "foo"},
	}}
	assertInvalidConfigError(sampling.ErrDefaultPolicyMissing)
	config.Policies[0].PolicyCriteria = sampling.PolicyCriteria{}
	for _, invalid := range []float64{-1, 2.0} {
		config.Policies[0].SampleRate = invalid
		assertInvalidConfigError(sampling.ErrSampleRateInvalid)
	}
	config.Policies[0].SampleRate = 1.0

	for _, invalid := range []float64{-1, 0, 2.0} {
		config.IngestRateDecayFactor = invalid
		assertInvalidConfigError(sampling.ErrIngestRateDecayFactorInvalid)
	}
	config.IngestRateDecayFactor = 0.5

	config.CompressionLevel = 11
	assertInvalidConfigError(sampling.ErrCompressionLevelInvalid)
	config.CompressionLevel = 0

	assertInvalidConfigError(sampling.ErrElasticsearchMissing)
	var elasticsearchClient struct {
		elasticsearch.Client
	}
	config.Elasticsearch = elasticsearchClient

	assertInvalidConfigError(pubsub.ErrTypeMissing)
	assertInvalidConfigError(pubsub.ErrDatasetMissing)
	assertInvalidConfigError(pubsub.ErrNamespaceMissing)
	config.SampledTracesDataStream = sampling.DataStreamConfig{
		Type:      "traces",
		Dataset:   "sampled",
		Namespace: "testing",
	}

	assertInvalidConfigError(sampling.ErrDBMissing)
	config.DB = &badger.DB{}

	assertInvalidConfigError(sampling.ErrStorageDirMissing)
	config.StorageDir = "tbs"

	assertInvalidConfigError(sampling.ErrStorageGCIntervalInvalid)
	config.StorageGCInterval = 1

	assertInvalidConfigError(sampling.ErrTTLInvalid)
	config.TTL = 1
}
