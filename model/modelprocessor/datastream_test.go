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

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetDataStream(t *testing.T) {
	tests := []struct {
		input  model.APMEvent
		output model.DataStream
	}{{
		input:  model.APMEvent{},
		output: model.DataStream{Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.TransactionProcessor},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.SpanProcessor},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.TransactionProcessor, Agent: model.Agent{Name: "js-base"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.SpanProcessor, Agent: model.Agent{Name: "js-base"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.TransactionProcessor, Agent: model.Agent{Name: "rum-js"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.SpanProcessor, Agent: model.Agent{Name: "rum-js"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.TransactionProcessor, Agent: model.Agent{Name: "iOS/swift"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.SpanProcessor, Agent: model.Agent{Name: "iOS/swift"}},
		output: model.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.TransactionProcessor, Agent: model.Agent{Name: "go"}},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.SpanProcessor, Agent: model.Agent{Name: "go"}},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.ErrorProcessor},
		output: model.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.LogProcessor},
		output: model.DataStream{Type: "logs", Dataset: "apm.app", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.ErrorProcessor, Agent: model.Agent{Name: "iOS/swift"}},
		output: model.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Processor: model.LogProcessor, Agent: model.Agent{Name: "iOS/swift"}},
		output: model.DataStream{Type: "logs", Dataset: "apm.app", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Agent:       model.Agent{Name: "rum-js"},
			Processor:   model.MetricsetProcessor,
			Service:     model.Service{Name: "service-name"},
			Metricset:   &model.Metricset{},
			Transaction: &model.Transaction{Name: "foo"},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Agent:     model.Agent{Name: "rum-js"},
			Processor: model.MetricsetProcessor,
			Service:   model.Service{Name: "service-name"},
			Metricset: &model.Metricset{},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Agent:         model.Agent{Name: "rum-js"},
			Processor:     model.ProfileProcessor,
			ProfileSample: &model.ProfileSample{},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.profiling", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Processor:   model.MetricsetProcessor,
			Service:     model.Service{Name: "service-name"},
			Metricset:   &model.Metricset{},
			Transaction: &model.Transaction{Name: "foo"},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "agent_config",
				Samples: map[string]model.MetricsetSample{
					"agent_config_applied": {Value: 1},
				},
			},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Processor: model.MetricsetProcessor,
			Service:   model.Service{Name: "service-name"},
			Metricset: &model.Metricset{},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Processor:     model.ProfileProcessor,
			ProfileSample: &model.ProfileSample{},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.profiling", Namespace: "custom"},
	}}

	for _, test := range tests {
		batch := model.Batch{test.input}
		processor := modelprocessor.SetDataStream{Namespace: "custom"}
		err := processor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)
		assert.Equal(t, test.output, batch[0].DataStream)
	}

}
