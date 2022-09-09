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

	t.Run("without_timestamp", func(t *testing.T) {
		result := systemtest.Elasticsearch.ExpectMinDocs(t, 1, "logs-apm.app-*", estest.BoolQuery{
			Filter: []interface{}{
				estest.TermQuery{Field: "processor.event", Value: "log"},
				estest.MatchPhraseQuery{Field: "message", Value: "test log message without timestamp"},
			},
		})
		systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits, "@timestamp")
	})

	t.Run("with_timestamp", func(t *testing.T) {
		result := systemtest.Elasticsearch.ExpectMinDocs(t, 1, "logs-apm.app-*", estest.BoolQuery{
			Filter: []interface{}{
				estest.TermQuery{Field: "processor.event", Value: "log"},
				estest.MatchPhraseQuery{Field: "message", Value: "test log message with timestamp"},
			},
		})
		systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
	})

	t.Run("with_faas", func(t *testing.T) {
		result := systemtest.Elasticsearch.ExpectMinDocs(t, 1, "logs-apm.app-*", estest.BoolQuery{
			Filter: []interface{}{
				estest.TermQuery{Field: "processor.event", Value: "log"},
				estest.MatchPhraseQuery{Field: "message", Value: "test log message with faas"},
			},
		})
		systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
	})
}
