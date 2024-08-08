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

package modeljson

import (
	"testing"
	"time"

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestSpanToModelJSON(t *testing.T) {
	sync := true

	testCases := map[string]struct {
		proto    *modelpb.Span
		expected *modeljson.Span
	}{
		"empty": {
			proto:    &modelpb.Span{},
			expected: &modeljson.Span{},
		},
		"no pointers": {
			proto: &modelpb.Span{
				Kind:                "kind",
				Action:              "action",
				Subtype:             "subtype",
				Id:                  "id",
				Type:                "type",
				Name:                "name",
				RepresentativeCount: 8,
			},
			expected: &modeljson.Span{
				Kind:                "kind",
				Action:              "action",
				Subtype:             "subtype",
				ID:                  "id",
				Type:                "type",
				Name:                "name",
				RepresentativeCount: 8,
			},
		},
		"full": {
			proto: &modelpb.Span{
				Composite: &modelpb.Composite{
					CompressionStrategy: modelpb.CompressionStrategy_COMPRESSION_STRATEGY_EXACT_MATCH,
					Count:               1,
					Sum:                 2,
				},
				DestinationService: &modelpb.DestinationService{
					Type:     "destination_type",
					Name:     "destination_name",
					Resource: "destination_resource",
					ResponseTime: &modelpb.AggregatedDuration{
						Count: 3,
						Sum:   uint64(4 * time.Second),
					},
				},
				Db: &modelpb.DB{
					RowsAffected: uintPtr(5),
					Instance:     "db_instace",
					Statement:    "db_statement",
					Type:         "db_type",
					UserName:     "db_username",
					Link:         "db_link",
				},
				Sync:    &sync,
				Kind:    "kind",
				Action:  "action",
				Subtype: "subtype",
				Id:      "id",
				Type:    "type",
				Name:    "name",
				Links: []*modelpb.SpanLink{
					{
						TraceId: "trace_id",
						SpanId:  "id1",
					},
				},
				SelfTime: &modelpb.AggregatedDuration{
					Count: 6,
					Sum:   uint64(7 * time.Second),
				},
				RepresentativeCount: 8,
			},
			expected: &modeljson.Span{
				Composite: &modeljson.SpanComposite{
					CompressionStrategy: "exact_match",
					Count:               1,
					Sum: modeljson.SpanCompositeSum{
						US: 2000,
					},
				},
				Destination: &modeljson.SpanDestination{
					Service: modeljson.SpanDestinationService{
						Type:     "destination_type",
						Name:     "destination_name",
						Resource: "destination_resource",
						ResponseTime: modeljson.AggregatedDuration{
							Count: 3,
							Sum:   4 * time.Second,
						},
					},
				},
				DB: &modeljson.DB{
					RowsAffected: uintPtr(5),
					Instance:     "db_instace",
					Statement:    "db_statement",
					Type:         "db_type",
					User: modeljson.DBUser{
						Name: "db_username",
					},
					Link: "db_link",
				},
				Sync:    &sync,
				Kind:    "kind",
				Action:  "action",
				Subtype: "subtype",
				ID:      "id",
				Type:    "type",
				Name:    "name",
				Links: []modeljson.SpanLink{
					{
						Trace: modeljson.SpanLinkTrace{
							ID: "trace_id",
						},
						// TODO other fields missing
						Span: modeljson.SpanLinkSpan{
							ID: "id1",
						},
					},
				},
				SelfTime: modeljson.AggregatedDuration{
					Count: 6,
					Sum:   7 * time.Second,
				},
				RepresentativeCount: 8,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			out := modeljson.Span{
				Message:     &modeljson.Message{},
				Composite:   &modeljson.SpanComposite{},
				Destination: &modeljson.SpanDestination{},
				DB:          &modeljson.DB{},
			}
			SpanModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out)
			require.Empty(t, diff)
		})
	}
}
