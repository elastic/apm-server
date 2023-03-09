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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/transport"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestFleetIntegration(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	apmIntegration := newAPMIntegration(t, nil)
	tx := apmIntegration.Tracer.StartTransaction("name", "type")
	tx.Duration = time.Second
	tx.End()
	apmIntegration.Tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectDocs(t, "traces-*", nil)
	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		"@timestamp", "timestamp.us",
		"trace.id", "transaction.id",
	)
}

func TestFleetIntegrationMonitoring(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	apmIntegration := newAPMIntegration(t, nil)

	const N = 15
	for i := 0; i < N; i++ {
		tx := apmIntegration.Tracer.StartTransaction("name", "type")
		tx.Duration = time.Second
		tx.End()
	}
	apmIntegration.Tracer.Flush(nil)
	systemtest.Elasticsearch.ExpectMinDocs(t, N, "traces-*", nil)

	var metrics struct {
		Libbeat map[string]interface{}
		Output  map[string]interface{}
	}
	apmIntegration.getBeatsMonitoringStats(t, &metrics)
	// Remove the output.write.bytes key since there isn't a way to assert the
	// writtenBytes at this layer.
	if o := metrics.Libbeat["output"].(map[string]interface{}); len(o) > 0 {
		if w := o["write"].(map[string]interface{}); len(w) > 0 {
			if w["bytes"] != nil {
				delete(w, "bytes")
			}
		}
	}
	assert.Equal(t, map[string]interface{}{
		"output": map[string]interface{}{
			"events": map[string]interface{}{
				"acked":   float64(N),
				"active":  0.0,
				"batches": 1.0,
				"failed":  0.0,
				"toomany": 0.0,
				"total":   float64(N),
			},
			"type":  "elasticsearch",
			"write": map[string]interface{}{},
		},
		"pipeline": map[string]interface{}{
			"events": map[string]interface{}{
				"total": float64(N),
			},
		},
	}, metrics.Libbeat)
	if es := metrics.Output["elasticsearch"].(map[string]interface{}); len(es) > 0 {
		if br := es["bulk_requests"].(map[string]interface{}); len(br) > 0 {
			assert.Greater(t, br["available"], float64(10))
			delete(br, "available")
		}
	}
	assert.Equal(t, map[string]interface{}{
		"elasticsearch": map[string]interface{}{
			"bulk_requests": map[string]interface{}{
				"completed": 1.0,
			},
			"indexers": map[string]interface{}{
				"active":    float64(1),
				"created":   0.0,
				"destroyed": 0.0,
			},
		},
	}, metrics.Output)
}

func TestFleetIntegrationAnonymousAuth(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	apmIntegration := newAPMIntegration(t, map[string]interface{}{
		"secret_token": "abc123",
		// RUM and anonymous auth are enabled by default.
		"anonymous_allow_service": []interface{}{"allowed_service"},
		"anonymous_allow_agent":   []interface{}{"allowed_agent"},
	})

	makePayload := func(service, agent string) io.Reader {
		const body = `{"metadata":{"service":{"name":%q,"agent":{"name":%q,"version":"5.5.0"}}}}
{"transaction":{"trace_id":"611f4fa950f04631aaaaaaaaaaaaaaaa","id":"611f4fa950f04631","type":"page-load","duration":643,"span_count":{"started":0}}}`
		return strings.NewReader(fmt.Sprintf(body, service, agent))
	}

	test := func(service, agent string, statusCode int) {
		req, _ := http.NewRequest("POST", apmIntegration.URL+"/intake/v2/rum/events", makePayload(service, agent))
		req.Header.Set("Content-Type", "application/x-ndjson")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		require.Equal(t, statusCode, resp.StatusCode, string(respBody))
	}
	test("allowed_service", "allowed_agent", http.StatusAccepted)
	test("allowed_service", "denied_agent", http.StatusForbidden)
	test("denied_service", "allowed_agent", http.StatusForbidden)
}

func TestFleetPackageNonMultiple(t *testing.T) {
	agentPolicy, _ := systemtest.CreateAgentPolicy(t, "apm_systemtest", "default", nil, nil)

	// Attempting to add the "apm" integration to the agent policy twice should fail.
	packagePolicy := systemtest.NewPackagePolicy(agentPolicy, nil, nil)
	packagePolicy.Name = "apm-2"
	err := systemtest.Fleet.CreatePackagePolicy(packagePolicy)
	require.Error(t, err)
	assert.EqualError(t, err, "Unable to create integration policy. Integration 'apm' already exists on this agent policy.")
}

// newAPMIntegration creates a new agent policy and assigns the APM integration
// with the provided config vars.
func newAPMIntegration(t testing.TB, vars map[string]interface{}) apmIntegration {
	return newAPMIntegrationConfig(t, vars, nil)
}

func newAPMIntegrationConfig(t testing.TB, vars, config map[string]interface{}) apmIntegration {
	policyName := fmt.Sprintf("apm_systemtest_%d", atomic.AddInt64(&apmIntegrationCounter, 1))
	_, enrollmentAPIKey := systemtest.CreateAgentPolicy(t, policyName, "default", vars, config)

	// Enroll an elastic-agent to run the APM integration.
	var output bytes.Buffer
	agent, err := systemtest.NewUnstartedElasticAgentContainer(systemtest.ContainerConfig{})
	require.NoError(t, err)
	agent.Stdout = &output
	agent.Stderr = &output
	agent.FleetEnrollmentToken = enrollmentAPIKey.APIKey
	t.Cleanup(func() {
		// Log the elastic-agent container output if the test fails.
		if !t.Failed() {
			return
		}
		t.Logf("elastic-agent logs: %s", output.String())
		if log, err := agent.APMServerLog(); err == nil {
			t.Log("apm-server logs:")
			io.Copy(os.Stdout, log)
			log.Close()
		}
		agent.Close()
	})

	// Start elastic-agent with port 8200 exposed, and wait for the server to service
	// healthcheck requests to port 8200.
	agent.ExposedPorts = []string{"8200"}
	agent.WaitingFor = wait.ForHTTP("/").WithPort("8200/tcp").WithStartupTimeout(5 * time.Minute)
	err = agent.Start()
	require.NoError(t, err)
	serverURL := &url.URL{Scheme: "http", Host: agent.Addrs["8200"]}

	var secretToken string
	if token, ok := vars["secret_token"].(string); ok {
		secretToken = token
	}
	// Create a Tracer which sends to the APM Server running under Elastic Agent.
	httpTransport, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{
		SecretToken: secretToken,
		ServerURLs:  []*url.URL{serverURL},
	})
	require.NoError(t, err)
	origTransport := httpTransport.Client.Transport
	httpTransport.Client.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		r.Header.Set("X-Real-Ip", "10.11.12.13")
		return origTransport.RoundTrip(r)
	})
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		Transport: apmservertest.NewFilteringTransport(
			httpTransport,
			apmservertest.DefaultMetadataFilter{},
		),
	})
	require.NoError(t, err)
	t.Cleanup(tracer.Close)
	return apmIntegration{
		Agent:  agent,
		Tracer: tracer,
		URL:    serverURL.String(),
	}
}

var apmIntegrationCounter int64

type apmIntegration struct {
	Agent *systemtest.ElasticAgentContainer

	// Tracer holds an apm.Tracer that may be used to send events
	// to the server.
	Tracer *apm.Tracer

	// URL holds the APM Server URL.
	URL string
}

func (a *apmIntegration) getBeatsMonitoringState(t testing.TB, out interface{}) *beatsMonitoringDoc {
	return a.getBeatsMonitoring(t, "beats_state", out)
}

func (a *apmIntegration) getBeatsMonitoringStats(t testing.TB, out interface{}) *beatsMonitoringDoc {
	return a.getBeatsMonitoring(t, "beats_stats", out)
}

func (a *apmIntegration) getBeatsMonitoring(t testing.TB, type_ string, out interface{}) *beatsMonitoringDoc {
	// We create all agent policies with metrics enabled, which causes apm-server
	// to be started with the libbeat HTTP introspection server started.
	const socket = "/usr/share/elastic-agent/state/data/tmp/apm-default.sock"

	var path string
	switch type_ {
	case "beats_state":
		path = "/state"
	case "beats_stats":
		path = "/stats"
	}
	stdout, stderr, err := a.Agent.Exec(context.Background(),
		"curl", "--unix-socket", socket,
		"http://localhost"+path,
	)
	require.NoError(t, err, string(stderr))

	var doc beatsMonitoringDoc
	doc.RawSource = stdout
	doc.Timestamp = time.Now()
	doc.Type = type_
	switch doc.Type {
	case "beats_state":
		err = json.Unmarshal(doc.RawSource, &doc.State)
		require.NoError(t, err)
	case "beats_stats":
		err = json.Unmarshal(doc.RawSource, &doc.Metrics)
		require.NoError(t, err)
	}
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
