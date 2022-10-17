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

package systemtest_test

import (
	"testing"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestIntakeLog(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)
	systemtest.SendBackendEventsPayload(t, srv.URL, `../testdata/intake-v2/logs.ndjson`)

	tests := []struct {
		Name            string
		Message         string
		ExpectedMinDocs int
		DynamicFields   []string
	}{
		{
			Name:            "without_timestamp",
			Message:         "test log message without timestamp",
			ExpectedMinDocs: 1,
			DynamicFields:   []string{"@timestamp"},
		},
		{
			Name:            "with_timestamp",
			Message:         "test log message with timestamp",
			ExpectedMinDocs: 1,
		},
		{
			Name:            "with_timestamp_as_str",
			Message:         "test log message with string timestamp",
			ExpectedMinDocs: 1,
		},
		{
			Name:            "with_faas",
			Message:         "test log message with faas",
			ExpectedMinDocs: 1,
		},
		{
			Name:            "with_flat_ecs_fields",
			Message:         "test log message with ecs fields",
			ExpectedMinDocs: 1,
		},
		{
			Name:            "with_nested_ecs_fields",
			Message:         "test log message with nested ecs fields",
			ExpectedMinDocs: 1,
		},
		{
			Name:            "with_nested_ecs_fields_overrides_flat_fields",
			Message:         "test log message with override of flat ecs fields by nested ecs fields",
			ExpectedMinDocs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			result := systemtest.Elasticsearch.ExpectMinDocs(t, test.ExpectedMinDocs, "logs-apm.app-*", estest.BoolQuery{
				Filter: []interface{}{
					estest.TermQuery{Field: "processor.event", Value: "log"},
					estest.MatchPhraseQuery{Field: "message", Value: test.Message},
				},
			})
			systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits, test.DynamicFields...)
		})
	}
}
