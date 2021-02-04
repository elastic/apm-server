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
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/fleettest"
)

func TestFleetIntegration(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	fleet := fleettest.NewClient(systemtest.KibanaURL.String())
	require.NoError(t, fleet.Setup())
	cleanupFleet(t, fleet)
	defer cleanupFleet(t, fleet)

	agentPolicy, enrollmentAPIKey, err := fleet.CreateAgentPolicy("apm_systemtest", "default", "Agent policy for APM Server system tests")
	require.NoError(t, err)

	// Find the "apm" package to install.
	var apmPackage *fleettest.Package
	packages, err := fleet.ListPackages()
	require.NoError(t, err)
	for _, pkg := range packages {
		if pkg.Name == "apm" {
			apmPackage = &pkg
			break
		}
	}
	require.NotNil(t, apmPackage)

	// Add the "apm" integration to the agent policy.
	packagePolicy := fleettest.PackagePolicy{
		Name:          "apm",
		Namespace:     "default",
		Enabled:       true,
		AgentPolicyID: agentPolicy.ID,
	}
	packagePolicy.Package.Name = apmPackage.Name
	packagePolicy.Package.Version = apmPackage.Version
	packagePolicy.Package.Title = apmPackage.Title
	packagePolicy.Inputs = []fleettest.PackagePolicyInput{{
		Type:    "apm",
		Enabled: true,
		Streams: []interface{}{},
		Vars: map[string]interface{}{
			"enable_rum": map[string]interface{}{
				"type":  "bool",
				"value": false,
			},
			"host": map[string]interface{}{
				"type":  "string",
				"value": ":8200",
			},
		},
	}}
	err = fleet.CreatePackagePolicy(packagePolicy)
	require.NoError(t, err)

	agent, err := systemtest.NewUnstartedElasticAgentContainer()
	require.NoError(t, err)
	agent.FleetEnrollmentToken = enrollmentAPIKey.APIKey
	defer agent.Close()

	defer func() {
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
	}()

	// Build apm-server, and bind-mount it into the elastic-agent container's "install"
	// directory. This bypasses downloading the artifact.
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	apmServerArtifactName := fmt.Sprintf("apm-server-%s-linux-%s", agent.StackVersion, arch)

	// Bind-mount the apm-server binary and apm-server.yml into the container's
	// "install" directory. This causes elastic-agent to skip installing the
	// artifact.
	apmServerBinary, err := apmservertest.BuildServerBinary("linux")
	require.NoError(t, err)
	agent.BindMountInstall[apmServerBinary] = path.Join(apmServerArtifactName, "apm-server")
	apmServerConfigFile, err := filepath.Abs("../apm-server.yml")
	require.NoError(t, err)
	agent.BindMountInstall[apmServerConfigFile] = path.Join(apmServerArtifactName, "apm-server.yml")

	// Start elastic-agent with port 8200 exposed, and wait for the server to service
	// healthcheck requests to port 8200.
	agent.ExposedPorts = []string{"8200"}
	waitFor := wait.ForHTTP("/")
	waitFor.Port = "8200/tcp"
	agent.WaitingFor = waitFor
	err = agent.Start()
	require.NoError(t, err)

	// Elastic Agent has started apm-server. Connect to apm-server and send some data,
	// and make sure it gets indexed into a data stream.
	require.Len(t, agent.Addrs, 1)
	transport, err := transport.NewHTTPTransport()
	require.NoError(t, err)
	transport.SetServerURL(&url.URL{Scheme: "http", Host: agent.Addrs[0]})
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{Transport: transport})
	require.NoError(t, err)
	defer tracer.Close()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)

	systemtest.Elasticsearch.ExpectDocs(t, "traces-*", nil)
}

func cleanupFleet(t testing.TB, fleet *fleettest.Client) {
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
		require.NoError(t, fleet.DeleteAgentPolicy(p.ID))
	}
}
