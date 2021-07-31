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
		input:  model.APMEvent{Transaction: &model.Transaction{}},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Span: &model.Span{}},
		output: model.DataStream{Type: "traces", Dataset: "apm", Namespace: "custom"},
	}, {
		input:  model.APMEvent{Error: &model.Error{}},
		output: model.DataStream{Type: "logs", Dataset: "apm.error", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Service: model.Service{Name: "service-name"},
			Metricset: &model.Metricset{
				Transaction: model.MetricsetTransaction{Name: "foo"},
			},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.internal", Namespace: "custom"},
	}, {
		input: model.APMEvent{
			Service:   model.Service{Name: "service-name"},
			Metricset: &model.Metricset{},
		},
		output: model.DataStream{Type: "metrics", Dataset: "apm.app.service_name", Namespace: "custom"},
	}, {
		input:  model.APMEvent{ProfileSample: &model.ProfileSample{}},
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
