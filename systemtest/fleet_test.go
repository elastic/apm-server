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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"testing"

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
	systemtest.CleanupElasticsearch(t)

	fleet := fleettest.NewClient(systemtest.KibanaURL.String())
	require.NoError(t, fleet.Setup())
	cleanupFleet(t, fleet)
	defer cleanupFleet(t, fleet)

	agentPolicy, enrollmentAPIKey, err := fleet.CreateAgentPolicy("apm_systemtest", "default", "Agent policy for APM Server system tests")
	require.NoError(t, err)

	// Add the "apm" integration to the agent policy.
	apmPackage := getAPMIntegrationPackage(t, fleet)
	packagePolicy := fleettest.NewPackagePolicy(apmPackage, "apm", "default", agentPolicy.ID)
	packagePolicy.Package.Name = apmPackage.Name
	packagePolicy.Package.Version = apmPackage.Version
	packagePolicy.Package.Title = apmPackage.Title
	initAPMIntegrationPackagePolicyInputs(t, packagePolicy, apmPackage)

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

func TestFleetPackageNonMultiple(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	fleet := fleettest.NewClient(systemtest.KibanaURL.String())
	require.NoError(t, fleet.Setup())
	cleanupFleet(t, fleet)
	defer cleanupFleet(t, fleet)

	agentPolicy, _, err := fleet.CreateAgentPolicy("apm_systemtest", "default", "Agent policy for APM Server system tests")
	require.NoError(t, err)

	apmPackage := getAPMIntegrationPackage(t, fleet)
	packagePolicy := fleettest.NewPackagePolicy(apmPackage, "apm", "default", agentPolicy.ID)
	initAPMIntegrationPackagePolicyInputs(t, packagePolicy, apmPackage)

	err = fleet.CreatePackagePolicy(packagePolicy)
	require.NoError(t, err)

	// Attempting to add the "apm" integration to the agent policy twice should fail.
	packagePolicy.Name = "apm-2"
	err = fleet.CreatePackagePolicy(packagePolicy)
	require.Error(t, err)
	assert.EqualError(t, err, "Unable to create package policy. Package 'apm' already exists on this agent policy.")
}

func initAPMIntegrationPackagePolicyInputs(t *testing.T, packagePolicy *fleettest.PackagePolicy, apmPackage *fleettest.Package) {
	assert.Len(t, apmPackage.PolicyTemplates, 1)
	assert.Len(t, apmPackage.PolicyTemplates[0].Inputs, 1)
	for _, input := range apmPackage.PolicyTemplates[0].Inputs {
		vars := make(map[string]interface{})
		for _, inputVar := range input.Vars {
			varMap := map[string]interface{}{"type": inputVar.Type}
			switch inputVar.Name {
			case "host":
				varMap["value"] = ":8200"
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

func getAPMIntegrationPackage(t *testing.T, fleet *fleettest.Client) *fleettest.Package {
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
		// BUG(axw) the Fleet API is returning 404 when deleting agent policies
		// in some circumstances: https://github.com/elastic/kibana/issues/90544
		err = fleet.DeleteAgentPolicy(p.ID)
		var fleetError *fleettest.Error
		if errors.As(err, &fleetError) {
			assert.Equal(t, http.StatusNotFound, fleetError.StatusCode)
		}
	}
}
