// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package modelprocessor_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"

	"go.elastic.co/apm"
	"go.elastic.co/apm/apmtest"
	agent "go.elastic.co/apm/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCounter(t *testing.T) {
	counter := modelprocessor.NewServiceCounter()
	services := []model.Service{
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-node", Version: "1.2"},
		{Name: "apm-agent-node", Version: "1.2"},
		{Name: "apm-agent-java", Version: "1.1"},
	}

	batch := model.Batch{}
	for _, service := range services {
		batch = append(batch, model.APMEvent{Service: service})
	}
	err := counter.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metrics := gatherMetrics(counter)[1:]

	assert.Equal(t, []agent.Metrics{
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-go"},
				{Key: "service.version", Value: "1.3"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 3,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-java"},
				{Key: "service.version", Value: "1.1"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 1,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-node"},
				{Key: "service.version", Value: "1.2"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 2,
				},
			},
		},
	}, metrics)
}

func TestServiceReset(t *testing.T) {
	counter := modelprocessor.NewServiceCounter()
	services := []model.Service{
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-go", Version: "1.3"},
		{Name: "apm-agent-node", Version: "1.2"},
		{Name: "apm-agent-node", Version: "1.2"},
		{Name: "apm-agent-java", Version: "1.1"},
	}

	batch := model.Batch{}
	for _, service := range services {
		batch = append(batch, model.APMEvent{Service: service})
	}
	err := counter.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metrics := gatherMetrics(counter)[1:]

	assert.Equal(t, []agent.Metrics{
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-go"},
				{Key: "service.version", Value: "1.3"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 3,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-java"},
				{Key: "service.version", Value: "1.1"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 1,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-node"},
				{Key: "service.version", Value: "1.2"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 2,
				},
			},
		},
	}, metrics)

	services = []model.Service{
		{Name: "apm-agent-node", Version: "1.2"},
		{Name: "apm-agent-java", Version: "1.1"},
		{Name: "apm-agent-java", Version: "1.1"},
		{Name: "apm-agent-ruby", Version: "1.0"},
		{Name: "apm-agent-ruby", Version: "1.0"},
		{Name: "apm-agent-ruby", Version: "1.0"},
	}

	batch = model.Batch{}
	for _, service := range services {
		batch = append(batch, model.APMEvent{Service: service})
	}
	err = counter.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metrics = gatherMetrics(counter)[1:]

	assert.Equal(t, []agent.Metrics{
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-java"},
				{Key: "service.version", Value: "1.1"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 2,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-node"},
				{Key: "service.version", Value: "1.2"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 1,
				},
			},
		},
		{
			Labels: agent.StringMap{
				{Key: "service.name", Value: "apm-agent-ruby"},
				{Key: "service.version", Value: "1.0"},
			},
			Samples: map[string]agent.Metric{
				"processed_events": {
					Value: 3,
				},
			},
		},
	}, metrics)
}

func TestServiceLimit(t *testing.T) {
	counter := modelprocessor.NewServiceCounter()
	batch := make(model.Batch, 20000)
	for i := range batch {
		service := model.Service{Name: fmt.Sprintf("service%d", i)}
		batch[i] = model.APMEvent{Service: service}
	}

	originalBufSize := os.Getenv("ELASTIC_APM_METRICS_BUFFER_SIZE")
	defer func() {
		os.Setenv("ELASTIC_APM_METRICS_BUFFER_SIZE", originalBufSize)
	}()
	os.Setenv("ELASTIC_APM_METRICS_BUFFER_SIZE", "10MB")

	err := counter.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metrics := gatherMetrics(counter)[1:]
	// Limit is 10000 named services, and the "unknown" service for
	// overflow.
	assert.Equal(t, len(metrics), 10001)
}

func gatherMetrics(g apm.MetricsGatherer) []agent.Metrics {
	tracer := apmtest.NewRecordingTracer()
	defer tracer.Close()
	tracer.RegisterMetricsGatherer(g)
	tracer.SendMetrics(nil)
	metrics := tracer.Payloads().Metrics
	for i := range metrics {
		metrics[i].Timestamp = agent.Time{}
	}
	return metrics
}
