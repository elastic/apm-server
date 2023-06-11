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
		defer agent.Close()
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
