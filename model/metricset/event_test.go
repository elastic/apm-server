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

package metricset

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
)

func TestTransform(t *testing.T) {
	timestamp := time.Now()
	metadata := metadata.Metadata{
		Service: metadata.Service{Name: "myservice"},
	}

	const (
		trType = "request"
		trName = "GET /"

		spType    = "db"
		spSubtype = "sql"
	)

	tests := []struct {
		Metricset *Metricset
		Output    []common.MapStr
		Msg       string
	}{
		{
			Metricset: nil,
			Output:    nil,
			Msg:       "Nil metric",
		},
		{
			Metricset: &Metricset{Timestamp: timestamp, Metadata: metadata},
			Output: []common.MapStr{
				{
					"processor": common.MapStr{"event": "metric", "name": "metric"},
					"service": common.MapStr{
						"name": "myservice",
					},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Metricset: &Metricset{
				Metadata:  metadata,
				Labels:    common.MapStr{"a.b": "a.b.value"},
				Timestamp: timestamp,
				Samples: []Sample{
					{
						Name:  "a.counter",
						Value: 612,
					},
					{
						Name:  "some.gauge",
						Value: 9.16,
					},
				},
				Span:        Span{Type: spType, Subtype: spSubtype},
				Transaction: Transaction{Type: trType, Name: trName},
			},
			Output: []common.MapStr{
				{
					"processor":   common.MapStr{"event": "metric", "name": "metric"},
					"service":     common.MapStr{"name": "myservice"},
					"transaction": common.MapStr{"name": trName, "type": trType},
					"span":        common.MapStr{"type": spType, "subtype": spSubtype},
					"labels":      common.MapStr{"a.b": "a.b.value"},

					"a":    common.MapStr{"counter": float64(612)},
					"some": common.MapStr{"gauge": float64(9.16)},
				},
			},
			Msg: "Payload with valid metric.",
		},
	}

	tctx := &transform.Context{}
	for idx, test := range tests {
		outputEvents := test.Metricset.Transform(context.Background(), tctx)

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
			assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
		}
	}
}
