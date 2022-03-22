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
	"testing"
	"time"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"

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
	ds := model.DataStream{
		Type:      "metrics",
		Dataset:   "apm_server",
		Namespace: "testing",
	}

	batch := model.Batch{}
	for _, service := range services {
		batch = append(batch, model.APMEvent{Timestamp: time.Now(), DataStream: ds, Service: service})
	}
	err := counter.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	tracer := apmtest.NewRecordingTracer()
	defer tracer.Close()
	tracer.RegisterMetricsGatherer(counter)
	tracer.SendMetrics(nil)
	metrics := tracer.Payloads().Metrics[1:]
	for i := range metrics {
		metrics[i].Timestamp = agent.Time{}
	}

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
