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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestAPMServerMonitoring(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
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
	assert.True(t, gjson.GetBytes(doc.Metrics, "apm-server").Exists())
}

func TestMonitoring(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = newFastMonitoringConfig()
	err := srv.Start()
	require.NoError(t, err)

	const N = 15
	tracer := srv.Tracer()
	for i := 0; i < N; i++ {
		tx := tracer.StartTransaction("name", "type")
		tx.Duration = time.Second
		tx.End()
	}
	tracer.Flush(nil)
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, N, "traces-*", nil)

	var metrics struct {
		Libbeat json.RawMessage
		Output  json.RawMessage
	}

	assert.Eventually(t, func() bool {
		metrics.Libbeat = nil
		metrics.Output = nil
		getBeatsMonitoringStats(t, srv, &metrics)
		acked := gjson.GetBytes(metrics.Libbeat, "output.events.acked")
		return acked.Int() == N
	}, 10*time.Second, 10*time.Millisecond)

	// Assert the presence of output.write.bytes, and that it is non-zero;
	// the exact value may change over time, and that is not relevant to this test.
	outputWriteBytes := gjson.GetBytes(metrics.Libbeat, "output.write.bytes")
	assert.NotZero(t, outputWriteBytes.Int())

	// Assert there was a non-zero number of batches written. There's no
	// guarantee that a single batch is written.
	batches := gjson.GetBytes(metrics.Libbeat, "output.events.batches")
	assert.NotZero(t, batches.Int())

	assert.Equal(t, int64(0), gjson.GetBytes(metrics.Libbeat, "output.events.active").Int())
	assert.Equal(t, int64(0), gjson.GetBytes(metrics.Libbeat, "output.events.failed").Int())
	assert.Equal(t, int64(0), gjson.GetBytes(metrics.Libbeat, "output.events.toomany").Int())
	assert.Equal(t, int64(N), gjson.GetBytes(metrics.Libbeat, "output.events.total").Int())
	assert.Equal(t, int64(N), gjson.GetBytes(metrics.Libbeat, "pipeline.events.total").Int())
	assert.Equal(t, "elasticsearch", gjson.GetBytes(metrics.Libbeat, "output.type").Str)

	bulkRequestsAvailable := gjson.GetBytes(metrics.Output, "elasticsearch.bulk_requests.available")
	assert.Greater(t, bulkRequestsAvailable.Int(), int64(10))
	assert.Equal(t, batches.Int(), gjson.GetBytes(metrics.Output, "elasticsearch.bulk_requests.completed").Int())
	assert.Equal(t, int64(1), gjson.GetBytes(metrics.Output, "elasticsearch.indexers.active").Int())
	assert.Zero(t, gjson.GetBytes(metrics.Output, "elasticsearch.indexers.created").Int())
	assert.Zero(t, gjson.GetBytes(metrics.Output, "elasticsearch.indexers.destroyed").Int())
}

func TestAPMServerMonitoringBuiltinUser(t *testing.T) {
	// This test is about ensuring the "apm_system" built-in user
	// has sufficient privileges to index monitoring data.
	const username = "apm_system"
	const password = "changeme"
	systemtest.ChangeUserPassword(t, username, password)

	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = &apmservertest.MonitoringConfig{
		Enabled:     true,
		StatePeriod: time.Duration(time.Second),
		Elasticsearch: &apmservertest.ElasticsearchOutputConfig{
			Enabled:  true,
			Username: username,
			Password: password,
		},
	}
	require.NoError(t, srv.Start())

	getBeatsMonitoringState(t, srv, nil)
}

func getBeatsMonitoringState(t testing.TB, srv *apmservertest.Server, out interface{}) *beatsMonitoringDoc {
	return getBeatsMonitoring(t, srv, "beats_state", out)
}

func getBeatsMonitoringStats(t testing.TB, srv *apmservertest.Server, out interface{}) *beatsMonitoringDoc {
	return getBeatsMonitoring(t, srv, "beats_stats", out)
}

func getBeatsMonitoring(t testing.TB, srv *apmservertest.Server, type_ string, out interface{}) *beatsMonitoringDoc {
	var result espoll.SearchResult
	req := systemtest.Elasticsearch.NewSearchRequest(".monitoring-beats-*").WithQuery(
		espoll.TermQuery{Field: type_ + ".beat.uuid", Value: srv.BeatUUID},
	).WithSort("timestamp:desc")
	if _, err := req.Do(context.Background(), &result, espoll.WithCondition(result.Hits.MinHitsCondition(1))); err != nil {
		t.Error(err)
	}

	var doc beatsMonitoringDoc
	doc.RawSource = []byte(result.Hits.Hits[0].RawSource)
	err := json.Unmarshal(doc.RawSource, &doc)
	require.NoError(t, err)
	if out != nil {
		switch doc.Type {
		case "beats_state":
			assert.NoError(t, json.Unmarshal(doc.State, out))
		case "beats_stats":
			assert.NoError(t, json.Unmarshal(doc.Metrics, out))
		}
	}
	return &doc
}

type beatsMonitoringDoc struct {
	RawSource  []byte    `json:"-"`
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
	BeatsState `json:"beats_state,omitempty"`
	BeatsStats `json:"beats_stats,omitempty"`
}

type BeatsState struct {
	State json.RawMessage `json:"state"`
}

type BeatsStats struct {
	Metrics json.RawMessage `json:"metrics"`
}

func newFastMonitoringConfig() *apmservertest.MonitoringConfig {
	return &apmservertest.MonitoringConfig{
		Enabled:       true,
		MetricsPeriod: 100 * time.Millisecond,
		StatePeriod:   100 * time.Millisecond,
	}
}
