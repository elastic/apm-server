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
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/fleettest"
)

func TestFleetIntegration(t *testing.T) {
	apmIntegration := initAPMIntegration(t, nil)
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

func TestFleetPackageNonMultiple(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	cleanupFleet(t, systemtest.Fleet)
	defer cleanupFleet(t, systemtest.Fleet)

	agentPolicy, _, err := systemtest.Fleet.CreateAgentPolicy(
		"apm_systemtest", "default", "Agent policy for APM Server system tests",
	)
	require.NoError(t, err)

	apmPackage := getAPMIntegrationPackage(t, systemtest.Fleet)
	packagePolicy := fleettest.NewPackagePolicy(apmPackage, "apm", "default", agentPolicy.ID)
	initAPMIntegrationPackagePolicyInputs(t, packagePolicy, apmPackage, nil)

	err = systemtest.Fleet.CreatePackagePolicy(packagePolicy)
	require.NoError(t, err)

	// Attempting to add the "apm" integration to the agent policy twice should fail.
	packagePolicy.Name = "apm-2"
	err = systemtest.Fleet.CreatePackagePolicy(packagePolicy)
	require.Error(t, err)
	assert.EqualError(t, err, "Unable to create package policy. Package 'apm' already exists on this agent policy.")
}

func initAPMIntegration(t testing.TB, vars map[string]interface{}) apmIntegration {
	systemtest.CleanupElasticsearch(t)
	cleanupFleet(t, systemtest.Fleet)
	t.Cleanup(func() { cleanupFleet(t, systemtest.Fleet) })

	agentPolicy, enrollmentAPIKey, err := systemtest.Fleet.CreateAgentPolicy(
		"apm_systemtest", "default", "Agent policy for APM Server system tests",
	)
	require.NoError(t, err)

	// Add the "apm" integration to the agent policy.
	apmPackage := getAPMIntegrationPackage(t, systemtest.Fleet)
	packagePolicy := fleettest.NewPackagePolicy(apmPackage, "apm", "default", agentPolicy.ID)
	packagePolicy.Package.Name = apmPackage.Name
	packagePolicy.Package.Version = apmPackage.Version
	packagePolicy.Package.Title = apmPackage.Title
	initAPMIntegrationPackagePolicyInputs(t, packagePolicy, apmPackage, vars)

	err = systemtest.Fleet.CreatePackagePolicy(packagePolicy)
	require.NoError(t, err)

	// Enroll an elastic-agent to run the APM integration.
	agent, err := systemtest.NewUnstartedElasticAgentContainer()
	require.NoError(t, err)
	agent.FleetEnrollmentToken = enrollmentAPIKey.APIKey
	t.Cleanup(func() { agent.Close() })
	t.Cleanup(func() {
		// Log the elastic-agent container output if the test fails.
		if !t.Failed() {
			return
		}
		if logs, err := agent.Logs(context.Background()); err == nil {
			defer logs.Close()
			if out, err := ioutil.ReadAll(logs); err == nil {
				t.Logf("elastic-agent logs: %s", out)
			}
		}
	})

	// Start elastic-agent with port 8200 exposed, and wait for the server to service
	// healthcheck requests to port 8200.
	agent.ExposedPorts = []string{"8200"}
	agent.WaitingFor = wait.ForHTTP("/").WithPort("8200/tcp").WithStartupTimeout(5 * time.Minute)
	err = agent.Start()
	require.NoError(t, err)
	serverURL := &url.URL{Scheme: "http", Host: agent.Addrs["8200"]}

	// Create a Tracer which sends to the APM Server running under Elastic Agent.
	httpTransport, err := transport.NewHTTPTransport()
	require.NoError(t, err)
	origTransport := httpTransport.Client.Transport
	if secretToken, ok := vars["secret_token"].(string); ok {
		httpTransport.SetSecretToken(secretToken)
	}
	httpTransport.Client.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		r.Header.Set("X-Real-Ip", "10.11.12.13")
		return origTransport.RoundTrip(r)
	})
	httpTransport.SetServerURL(serverURL)
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

type apmIntegration struct {
	Agent *systemtest.ElasticAgentContainer

	// Tracer holds an apm.Tracer that may be used to send events
	// to the server.
	Tracer *apm.Tracer

	// URL holds the APM Server URL.
	URL string
}

func initAPMIntegrationPackagePolicyInputs(
	t testing.TB, packagePolicy *fleettest.PackagePolicy, apmPackage *fleettest.Package, varValues map[string]interface{},
) {
	assert.Len(t, apmPackage.PolicyTemplates, 1)
	assert.Len(t, apmPackage.PolicyTemplates[0].Inputs, 1)
	for _, input := range apmPackage.PolicyTemplates[0].Inputs {
		vars := make(map[string]interface{})
		for _, inputVar := range input.Vars {
			value, ok := varValues[inputVar.Name]
			if !ok {
				switch inputVar.Name {
				case "host":
					value = ":8200"
				default:
					value = inputVarDefault(inputVar)
				}
			}
			varMap := map[string]interface{}{"type": inputVar.Type}
			if value != nil {
				varMap["value"] = value
			}
			vars[inputVar.Name] = varMap
		}
		packagePolicy.Inputs = append(packagePolicy.Inputs, fleettest.PackagePolicyInput{
			Type:    input.Type,
			Enabled: true,
			Streams: []interface{}{},
			Vars:    vars,
		})
	}
}

func inputVarDefault(inputVar fleettest.PackagePolicyTemplateInputVar) interface{} {
	if inputVar.Default != nil {
		return inputVar.Default
	}
	if inputVar.Multi {
		return []interface{}{}
	}
	return nil
}

func cleanupFleet(t testing.TB, fleet *fleettest.Client) {
	cleanupFleetPolicies(t, fleet)
	apmPackage := getAPMIntegrationPackage(t, fleet)
	if apmPackage.Status == "installed" {
		err := fleet.DeletePackage(apmPackage.Name, apmPackage.Version)
		require.NoError(t, err)
	}
}

func getAPMIntegrationPackage(t testing.TB, fleet *fleettest.Client) *fleettest.Package {
	var apmPackage *fleettest.Package
	packages, err := fleet.ListPackages()
	require.NoError(t, err)
	for _, pkg := range packages {
		if pkg.Name != "apm" {
			continue
		}
		// ListPackages does not return all package details,
		// so we call Package to get them.
		apmPackage, err = fleet.Package(pkg.Name, pkg.Version)
		require.NoError(t, err)
		return apmPackage
	}
	t.Fatal("could not find package 'apm'")
	panic("unreachable")
}

func cleanupFleetPolicies(t testing.TB, fleet *fleettest.Client) {
	apmAgentPolicies, err := fleet.AgentPolicies("ingest-agent-policies.name:apm_systemtest")
	require.NoError(t, err)
	if len(apmAgentPolicies) == 0 {
		return
	}

	agents, err := fleet.Agents()
	require.NoError(t, err)
	agentsByPolicy := make(map[string][]fleettest.Agent)
	for _, agent := range agents {
		agentsByPolicy[agent.PolicyID] = append(agentsByPolicy[agent.PolicyID], agent)
	}

	for _, p := range apmAgentPolicies {
		if agents := agentsByPolicy[p.ID]; len(agents) > 0 {
			agentIDs := make([]string, len(agents))
			for i, agent := range agents {
				agentIDs[i] = agent.ID
			}
			require.NoError(t, fleet.BulkUnenrollAgents(true, agentIDs...))
		}
		err := fleet.DeleteAgentPolicy(p.ID)
		require.NoError(t, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
