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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestAPMServerMonitoring(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Monitoring = &apmservertest.MonitoringConfig{
		Enabled:       true,
		MetricsPeriod: time.Duration(time.Second),
		StatePeriod:   time.Duration(time.Second),
	}
	err := srv.Start()
	require.NoError(t, err)

	var state struct {
		Output struct {
			Name string
		}
	}
	getBeatsMonitoringState(t, srv, &state)
	assert.Equal(t, "elasticsearch", state.Output.Name)

	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.Contains(t, doc.Metrics, "apm-server")
}

func getBeatsMonitoringState(t testing.TB, srv *apmservertest.Server, out interface{}) *beatsMonitoringDoc {
	return getBeatsMonitoring(t, srv, "beats_state", out)
}

func getBeatsMonitoringStats(t testing.TB, srv *apmservertest.Server, out interface{}) *beatsMonitoringDoc {
	return getBeatsMonitoring(t, srv, "beats_stats", out)
}

func getBeatsMonitoring(t testing.TB, srv *apmservertest.Server, type_ string, out interface{}) *beatsMonitoringDoc {
	var result estest.SearchResult
	_, err := systemtest.Elasticsearch.Search(".monitoring-beats-*").WithQuery(estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{
				Field: type_ + ".beat.uuid",
				Value: srv.BeatUUID,
			},
		},
	}).Do(context.Background(), &result,
		estest.WithTimeout(10*time.Second),
		estest.WithCondition(result.Hits.NonEmptyCondition()),
	)
	require.NoError(t, err)

	var doc beatsMonitoringDoc
	err = json.Unmarshal([]byte(result.Hits.Hits[0].RawSource), &doc)
	require.NoError(t, err)
	if out != nil {
		switch doc.Type {
		case "beats_state":
			assert.NoError(t, mapstructure.Decode(doc.State, out))
		case "beats_stats":
			assert.NoError(t, mapstructure.Decode(doc.Metrics, out))
		}
	}
	return &doc
}

type beatsMonitoringDoc struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
	BeatsState `json:"beats_state,omitempty"`
	BeatsStats `json:"beats_stats,omitempty"`
}

type BeatsState struct {
	State map[string]interface{} `json:"state"`
}

type BeatsStats struct {
	Metrics map[string]interface{} `json:"metrics"`
}
