// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func TestNewProcessorConfigInvalid(t *testing.T) {
	var config sampling.Config
	assertInvalidConfigError := func(expectedError string) {
		t.Helper()
		agg, err := sampling.NewProcessor(sampling.ProcessorParams{
			Config: config,
			Logger: logptest.NewTestingLogger(t, ""),
		})
		require.Error(t, err)
		require.Nil(t, agg)
		assert.EqualError(t, err, "invalid tail-sampling config: "+expectedError)
	}

	assertInvalidConfigError("BatchProcessor unspecified")
	config.BatchProcessor = struct{ modelpb.BatchProcessor }{}

	assertInvalidConfigError("invalid local sampling config: FlushInterval unspecified or negative")
	config.FlushInterval = 1

	assertInvalidConfigError("invalid local sampling config: MaxDynamicServices unspecified or negative")
	config.MaxDynamicServices = 1

	assertInvalidConfigError("invalid local sampling config: Policies unspecified")
	config.Policies = []sampling.Policy{{
		PolicyCriteria: sampling.PolicyCriteria{ServiceName: "foo"},
	}}
	assertInvalidConfigError("invalid local sampling config: Policies does not contain a default (empty criteria) policy")
	config.Policies[0].PolicyCriteria = sampling.PolicyCriteria{}
	for _, invalid := range []float64{-1, 2.0} {
		config.Policies[0].SampleRate = invalid
		assertInvalidConfigError("invalid local sampling config: Policy 0 invalid: SampleRate unspecified or out of range [0,1]")
	}
	config.Policies[0].SampleRate = 1.0

	for _, invalid := range []float64{-1, 0, 2.0} {
		config.IngestRateDecayFactor = invalid
		assertInvalidConfigError("invalid local sampling config: IngestRateDecayFactor unspecified or out of range (0,1]")
	}
	config.IngestRateDecayFactor = 0.5

	config.CompressionLevel = 11
	assertInvalidConfigError("invalid remote sampling config: CompressionLevel out of range [-1,9]")
	config.CompressionLevel = 0

	assertInvalidConfigError("invalid remote sampling config: Elasticsearch unspecified")
	config.Elasticsearch = &elastictransport.Client{}

	assertInvalidConfigError("invalid remote sampling config: SampledTracesDataStream unspecified or invalid")
	config.SampledTracesDataStream = sampling.DataStreamConfig{
		Type:      "traces",
		Dataset:   "sampled",
		Namespace: "testing",
	}

	assertInvalidConfigError("invalid remote sampling config: UUID unspecified")
	config.UUID = "server"

	assertInvalidConfigError("invalid storage config: DB unspecified")
	config.DB = &eventstorage.StorageManager{}

	assertInvalidConfigError("invalid storage config: Storage unspecified")
	config.Storage = &eventstorage.SplitReadWriter{}

	assertInvalidConfigError("invalid storage config: TTL unspecified or negative")
	config.TTL = 1
}
