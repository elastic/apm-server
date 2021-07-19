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

func TestSetMetricsetName(t *testing.T) {
	tests := []struct {
		metricset model.Metricset
		name      string
	}{{
		metricset: model.Metricset{},
		name:      "",
	}, {
		metricset: model.Metricset{Name: "already_set"},
		name:      "already_set",
	}, {
		metricset: model.Metricset{Transaction: model.MetricsetTransaction{Type: "request"}},
		name:      "",
	}, {
		metricset: model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"transaction.breakdown.count": {},
			},
		},
		name: "app",
	}, {
		metricset: model.Metricset{
			Transaction: model.MetricsetTransaction{Type: "request"},
			Samples: map[string]model.MetricsetSample{
				"transaction.duration.count":  {},
				"transaction.breakdown.count": {},
			},
		},
		name: "transaction_breakdown",
	}, {
		metricset: model.Metricset{
			Transaction: model.MetricsetTransaction{Type: "request"},
			Samples: map[string]model.MetricsetSample{
				"span.self_time.count": {},
			},
		},
		name: "span_breakdown",
	}}

	for _, test := range tests {
		batch := model.Batch{{Metricset: &test.metricset}}
		processor := modelprocessor.SetMetricsetName{}
		err := processor.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)
		assert.Equal(t, test.name, batch[0].Metricset.Name)
	}

}
