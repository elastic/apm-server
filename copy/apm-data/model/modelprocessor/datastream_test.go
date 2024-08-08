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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestSetDataStream(t *testing.T) {
	tests := []struct {
		input  *modelpb.APMEvent
		output *modelpb.DataStream
	}{{
		input:  &modelpb.APMEvent{},
		output: &modelpb.DataStream{Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Transaction: &modelpb.Transaction{Type: "type"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Span: &modelpb.Span{Type: "type"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Span:       &modelpb.Span{Type: "type"},
			DataStream: &modelpb.DataStream{Dataset: "dataset", Namespace: "namespace"},
		},
		output: &modelpb.DataStream{Type: "traces", Dataset: "dataset", Namespace: "namespace"},
	}, {
		input:  &modelpb.APMEvent{Transaction: &modelpb.Transaction{Type: "type"}, Agent: &modelpb.Agent{Name: "js-base"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Span: &modelpb.Span{Type: "type"}, Agent: &modelpb.Agent{Name: "js-base"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Transaction: &modelpb.Transaction{Type: "type"}, Agent: &modelpb.Agent{Name: "rum-js"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Span: &modelpb.Span{Type: "type"}, Agent: &modelpb.Agent{Name: "rum-js"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Transaction: &modelpb.Transaction{Type: "type"}, Agent: &modelpb.Agent{Name: "iOS/swift"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Span: &modelpb.Span{Type: "type"}, Agent: &modelpb.Agent{Name: "iOS/swift"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Span:       &modelpb.Span{Type: "type"},
			Agent:      &modelpb.Agent{Name: "iOS/swift"},
			DataStream: &modelpb.DataStream{Dataset: "dataset", Namespace: "namespace"},
		},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm.rum", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Transaction: &modelpb.Transaction{Type: "type"}, Agent: &modelpb.Agent{Name: "go"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Span: &modelpb.Span{Type: "type"}, Agent: &modelpb.Agent{Name: "go"}},
		output: &modelpb.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Error: &modelpb.Error{}},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{Error: &modelpb.Error{}, DataStream: &modelpb.DataStream{
			Dataset:   "dataset",
			Namespace: "namespace",
		}},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "namespace"},
	}, {
		input:  &modelpb.APMEvent{Log: &modelpb.Log{}},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.app.unknown", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Event:   &modelpb.Event{Kind: "event"},
			Service: &modelpb.Service{Name: "service-name"},
		},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input:  &modelpb.APMEvent{Error: &modelpb.Error{}, Agent: &modelpb.Agent{Name: "iOS/swift"}},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Log:     &modelpb.Log{},
			Agent:   &modelpb.Agent{Name: "iOS/swift"},
			Service: &modelpb.Service{Name: "service-name"},
		},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Log:        &modelpb.Log{},
			Agent:      &modelpb.Agent{Name: "iOS/swift"},
			Service:    &modelpb.Service{Name: "service-name"},
			DataStream: &modelpb.DataStream{Dataset: "dataset", Namespace: "namespace"},
		},
		output: &modelpb.DataStream{Type: "logs", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:       &modelpb.Agent{Name: "rum-js"},
			Service:     &modelpb.Service{Name: "service-name"},
			Metricset:   &modelpb.Metricset{},
			Transaction: &modelpb.Transaction{Name: "foo"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:   &modelpb.Agent{Name: "rum-js"},
			Service: &modelpb.Service{Name: "service-name"},
			Metricset: &modelpb.Metricset{
				Samples: []*modelpb.MetricsetSample{
					{Name: "system.memory.total"}, // known agent metric
				},
			},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:   &modelpb.Agent{Name: "rum-js"},
			Service: &modelpb.Service{Name: "service-name"},
			Metricset: &modelpb.Metricset{
				Samples: []*modelpb.MetricsetSample{
					{Name: "system.memory.total"}, // known agent metric
					{Name: "custom_metric"},       // custom metric
				},
			},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:   &modelpb.Agent{Name: "rum-js"},
			Service: &modelpb.Service{Name: "service-name"},
			Metricset: &modelpb.Metricset{
				Samples: []*modelpb.MetricsetSample{
					{Name: "system.memory.total"}, // known agent metric
					{Name: "custom_metric"},       // custom metric
				},
			},
			DataStream: &modelpb.DataStream{Dataset: "dataset", Namespace: "namespace"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Service:     &modelpb.Service{Name: "service-name"},
			Metricset:   &modelpb.Metricset{},
			Transaction: &modelpb.Transaction{Name: "foo"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Service:     &modelpb.Service{Name: "service-name"},
			Metricset:   &modelpb.Metricset{},
			Transaction: &modelpb.Transaction{Name: "foo"},
			DataStream:  &modelpb.DataStream{Dataset: "dataset", Namespace: "namespace"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "dataset", Namespace: "namespace"},
	}, {
		input: &modelpb.APMEvent{
			Service:     &modelpb.Service{Name: "service-name"},
			Metricset:   &modelpb.Metricset{Name: "transaction", Interval: "1m"},
			Transaction: &modelpb.Transaction{Name: "foo"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.transaction.1m", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Service:     &modelpb.Service{Name: "service-name"},
			Metricset:   &modelpb.Metricset{Name: "transaction", Interval: "60m"},
			Transaction: &modelpb.Transaction{Name: "foo"},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.transaction.60m", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Metricset: &modelpb.Metricset{
				Name: "agent_config",
				Samples: []*modelpb.MetricsetSample{
					{Name: "agent_config_applied", Value: 1},
				},
			},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:   &modelpb.Agent{Name: "otel"},
			Service: &modelpb.Service{Name: "service-name"},
			Metricset: &modelpb.Metricset{
				Samples: []*modelpb.MetricsetSample{
					{Name: "system.memory.total"},
				},
			},
			Labels: map[string]*modelpb.LabelValue{
				"otel_remapped": &modelpb.LabelValue{Value: "true"}, // otel translated hostmetrics
			},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input: &modelpb.APMEvent{
			Agent:   &modelpb.Agent{Name: "otel"},
			Service: &modelpb.Service{Name: "service-name"},
			Metricset: &modelpb.Metricset{
				Samples: []*modelpb.MetricsetSample{
					{Name: "system.memory.total"},
				},
			},
			Labels: map[string]*modelpb.LabelValue{
				"event.provider": &modelpb.LabelValue{Value: "kernel"},
			},
		},
		output: &modelpb.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			batch := modelpb.Batch{test.input}
			processor := modelprocessor.SetDataStream{Namespace: "custom"}
			err := processor.ProcessBatch(context.Background(), &batch)
			assert.NoError(t, err)
			assert.Equal(t, test.output, batch[0].DataStream)
		})
	}

}

func TestSetDataStreamInternalMetricsetTypeUnit(t *testing.T) {
	batch := modelpb.Batch{{
		Agent:   &modelpb.Agent{Name: "rum-js"},
		Service: &modelpb.Service{Name: "service-name"},
		Metricset: &modelpb.Metricset{
			Samples: []*modelpb.MetricsetSample{
				{Name: "system.memory.total", Type: modelpb.MetricType_METRIC_TYPE_GAUGE, Unit: "byte"},
				{Name: "system.process.memory.size", Type: modelpb.MetricType_METRIC_TYPE_GAUGE, Unit: "byte"},
			},
		},
	}}

	processor := modelprocessor.SetDataStream{Namespace: "custom"}
	err := processor.ProcessBatch(context.Background(), &batch)
	assert.NoError(t, err)
	for _, sample := range batch[0].Metricset.Samples {
		assert.Zero(t, sample.Type)
		assert.Zero(t, sample.Unit)
	}
}

func TestSetDataStreamServiceName(t *testing.T) {
	processor := modelprocessor.SetDataStream{Namespace: "custom"}
	batch := modelpb.Batch{{
		Service: &modelpb.Service{Name: "UPPER-CASE"},
		Metricset: &modelpb.Metricset{
			Samples: []*modelpb.MetricsetSample{{
				Name: "foo",
				Type: modelpb.MetricType_METRIC_TYPE_COUNTER,
			}},
		},
	}, {
		Service: &modelpb.Service{Name: "\\/*?\"<>| ,#:"},
		Metricset: &modelpb.Metricset{
			Samples: []*modelpb.MetricsetSample{{
				Name: "foo",
				Type: modelpb.MetricType_METRIC_TYPE_COUNTER,
			}},
		},
	}}

	err := processor.ProcessBatch(context.Background(), &batch)
	assert.NoError(t, err)
	assert.Equal(t, "apm.app.upper_case", batch[0].DataStream.Dataset)
	assert.Equal(t, "apm.app.____________", batch[1].DataStream.Dataset)
}
