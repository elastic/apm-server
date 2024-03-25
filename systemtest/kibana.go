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

package systemtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-server/systemtest/fleettest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

const (
	adminKibanaUser = adminElasticsearchUser
	adminKibanaPass = adminElasticsearchPass

	// agentPolicyDescription holds the description associated with agent
	// policies created by CreateAgentPolicy. This can be used in filters.
	agentPolicyDescription = "apm_systemtest"
)

var (
	// KibanaURL is the base URL for Kibana, including userinfo for
	// authenticating as the admin user.
	KibanaURL *url.URL

	// Fleet is a Fleet API client for use in tests.
	Fleet *fleettest.Client

	// IntegrationPackage holds the "apm" integration package details.
	//
	// IntegrationPackage is initialised by InitFleetPackage.
	IntegrationPackage *fleettest.Package
)

func init() {
	kibanaConfig := apmservertest.DefaultConfig().Kibana
	u, err := url.Parse(kibanaConfig.Host)
	if err != nil {
		log.Fatal(err)
	}
	u.User = url.UserPassword(adminKibanaUser, adminKibanaPass)
	KibanaURL = u
	Fleet = fleettest.NewClient(KibanaURL.String())
}

// InitFleet ensures Fleet is set up, destroys any existing agent policies previously
// created by the system tests and unenrolls the associated agents. After InitFleet
// returns successfully, the IntegrationPackage var will be initialised to the details
// of the installed APM integration package.
func InitFleet() error {
	if err := Fleet.Setup(); err != nil {
		log.Fatal(err)
	}
	agentPolicies, err := Fleet.AgentPolicies("ingest-agent-policies.description:" + agentPolicyDescription)
	if err != nil {
		return err
	}
	ids := make([]string, len(agentPolicies))
	for i, agentPolicy := range agentPolicies {
		ids[i] = agentPolicy.ID
	}
	if err := DestroyAgentPolicy(ids...); err != nil {
		return fmt.Errorf("failed to destroy agent policy: %w", err)
	}
	return InitFleetPackage()
}

// InitFleetPackage and sets IntegrationPackage to the details of the installed
// APM integration package. InitFleetPackage assumes that Fleet has been set up
// already.
func InitFleetPackage() error {
	packages, err := Fleet.ListPackages()
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		if pkg.Name != "apm" {
			continue
		}
		IntegrationPackage = &pkg
		return nil
	}
	return errors.New("'apm' integration package not installed")
}

// CreateAgentPolicy creates an Agent policy with the given name and namespace,
// creates an APM package policy with the provided config vars, and assigns it
// to the agent policy.
//
// The agent policy will be destroyed, and any assigned agents unenrolled, when
// the test completes.
//
// This should typically be used by tests instead of directly calling the
// fleettest.Client.CreateAgentPolicy method.
func CreateAgentPolicy(t testing.TB, name, namespace string, vars, config map[string]interface{}) (*fleettest.AgentPolicy, *fleettest.EnrollmentAPIKey) {
	agentPolicy, key, err := Fleet.CreateAgentPolicy(name, namespace, agentPolicyDescription)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := DestroyAgentPolicy(agentPolicy.ID)
		require.NoError(t, err)
	})

	packagePolicy := NewPackagePolicy(agentPolicy, vars, config)
	err = Fleet.CreatePackagePolicy(packagePolicy)
	require.NoError(t, err)

	return agentPolicy, key
}

// DestroyAgentPolicy deletes the agent policies with given IDs,
// and bulk unenrolls the agents assigned to them.
func DestroyAgentPolicy(id ...string) error {
	if len(id) == 0 {
		return nil
	}
	agents, err := Fleet.Agents()
	if err != nil {
		return err
	}
	agentsByPolicy := make(map[string][]fleettest.Agent)
	for _, agent := range agents {
		agentsByPolicy[agent.PolicyID] = append(agentsByPolicy[agent.PolicyID], agent)
	}
	for _, agentPolicyID := range id {
		if agents := agentsByPolicy[agentPolicyID]; len(agents) > 0 {
			agentIDs := make([]string, len(agents))
			for i, agent := range agents {
				agentIDs[i] = agent.ID
			}
			if err := Fleet.BulkUnenrollAgents(true, agentIDs...); err != nil {
				return err
			}
		}
		if err := Fleet.DeleteAgentPolicy(agentPolicyID); err != nil {
			return err
		}
	}
	return nil
}

// NewPackagePolicy returns a new fleettest.PackagePolicy with config vars, but does not create it.
//
// The returned package policy is suitable for passing to Fleet.CreatePackagePolicy.
func NewPackagePolicy(agentPolicy *fleettest.AgentPolicy, varValues, extraConfig map[string]interface{}) *fleettest.PackagePolicy {
	// Package policy names must be globally unique. We generate unique agent
	// policy names, so just append the package name to that.
	packagePolicyName := agentPolicy.Name + "-apm"
	packagePolicy := fleettest.NewPackagePolicy(IntegrationPackage, packagePolicyName, agentPolicy.Namespace, agentPolicy.ID)
	packagePolicy.Package.Name = IntegrationPackage.Name
	packagePolicy.Package.Version = IntegrationPackage.Version
	packagePolicy.Package.Title = IntegrationPackage.Title

	for _, input := range IntegrationPackage.PolicyTemplates[0].Inputs {
		vars := make(map[string]interface{})
		for _, inputVar := range input.Vars {
			value, ok := varValues[inputVar.Name]
			if !ok {
				value = inputVarDefault(inputVar)
			}
			varMap := map[string]interface{}{"type": inputVar.Type}
			if value != nil {
				if inputVar.Type == "yaml" {
					encoded, err := json.Marshal(value)
					if err != nil {
						panic(err)
					}
					value = string(encoded)
				}
				varMap["value"] = value
			}
			vars[inputVar.Name] = varMap
		}
		packagePolicy.Inputs = append(packagePolicy.Inputs, fleettest.PackagePolicyInput{
			Type:    input.Type,
			Enabled: true,
			Streams: []interface{}{},
			Vars:    vars,
			Config:  extraConfig,
		})
	}
	return packagePolicy
}

func inputVarDefault(inputVar fleettest.PackagePolicyTemplateInputVar) interface{} {
	if inputVar.Name == "host" {
		return ":8200"
	}
	if inputVar.Default != nil {
		defaultValue := inputVar.Default
		if inputVar.Type == "yaml" {
			var v interface{}
			if err := yaml.Unmarshal([]byte(defaultValue.(string)), &v); err != nil {
				panic(err)
			}
			defaultValue = v
		}
		return defaultValue
	}
	if inputVar.Multi {
		return []interface{}{}
	}
	return nil
}

// SourceMap holds information about a source map stored by Kibana.
type SourceMap struct {
	ID      string                 `json:"id"`
	Created time.Time              `json:"created"`
	Body    map[string]interface{} `json:"body"`
}

// CreateSourceMap creates or replaces a source map with the given service name
// and version, and bundle filepath. CreateSourceMap returns the ID of the stored
// source map, which may be passed to DeleteSourceMap for cleanup.
func CreateSourceMap(t testing.TB, sourcemap []byte, serviceName, serviceVersion, bundleFilepath string) string {
	t.Helper()

	var data bytes.Buffer
	mw := multipart.NewWriter(&data)
	require.NoError(t, mw.WriteField("service_name", serviceName))
	require.NoError(t, mw.WriteField("service_version", serviceVersion))
	require.NoError(t, mw.WriteField("bundle_filepath", bundleFilepath))

	sourcemapFileWriter, err := mw.CreateFormFile("sourcemap", "sourcemap.js.map")
	require.NoError(t, err)
	sourcemapFileWriter.Write(sourcemap)
	require.NoError(t, mw.Close())

	apiURL := *KibanaURL
	apiURL.Path += "/api/apm/sourcemaps"
	req, _ := http.NewRequest("POST", apiURL.String(), &data)
	req.Header.Add("Content-Type", mw.FormDataContentType())
	req.Header.Set("kbn-xsrf", "1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))

	var result struct {
		ID string `json:"id"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	cleanPath := bundleFilepath
	u, err := url.Parse(bundleFilepath)
	if err == nil {
		u.Fragment = ""
		u.RawQuery = ""
		u.Path = path.Clean(u.Path)
		cleanPath = u.String()
	}

	id := serviceName + "-" + serviceVersion + "-" + cleanPath
	estest.ExpectMinDocs(t, Elasticsearch, 1, ".apm-source-map", espoll.TermQuery{
		Field: "_id",
		Value: id,
	})

	t.Cleanup(func() {
		DeleteSourceMap(t, result.ID)
	})

	return result.ID
}

// DeleteSourceMap deletes a source map with the given ID.
func DeleteSourceMap(t testing.TB, id string) {
	t.Helper()

	url := *KibanaURL
	url.Path += "/api/apm/sourcemaps/" + id
	req, _ := http.NewRequest("DELETE", url.String(), nil)
	req.Header.Set("kbn-xsrf", "1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
}
