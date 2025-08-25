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

package integrationservertest

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
	"github.com/elastic/apm-server/integrationservertest/internal/elasticsearch"
	"github.com/elastic/apm-server/integrationservertest/internal/gen"
	"github.com/elastic/apm-server/integrationservertest/internal/kibana"
	"github.com/elastic/apm-server/integrationservertest/internal/terraform"
)

const (
	// managedByDSL is the constant string used by Elasticsearch to specify that
	// an index is managed by Data Stream Lifecycle management.
	managedByDSL = "Data stream lifecycle"
	// managedByILM is the constant string used by Elasticsearch to specify that
	// an index is managed by Index Lifecycle Management.
	managedByILM = "Index Lifecycle Management"
)

const (
	targetQA = "qa"
	// we use 'pro' because it is the target passed by the Buildkite pipeline running
	// these tests.
	targetProd = "pro"
)

func RegionFrom(target string) string {
	switch target {
	case targetQA:
		return "aws-eu-west-1"
	case targetProd:
		return "gcp-us-west2"
	default:
		panic("target value is not accepted")
	}
}

func EndpointFrom(target string) string {
	switch target {
	case targetQA:
		return "https://public-api.qa.cld.elstc.co"
	case targetProd:
		return "https://api.elastic-cloud.com"
	default:
		panic("target value is not accepted")
	}
}

func DeploymentTemplateFrom(region string) string {
	switch region {
	case "aws-eu-west-1":
		return "aws-storage-optimized"
	case "gcp-us-west2":
		return "gcp-storage-optimized"
	default:
		panic("region value is not accepted")
	}
}

// terraformDirName returns the name of the Terraform files directory for this test.
func terraformDirName(t *testing.T) string {
	t.Helper()
	// Flatten the dir name in case of path separators
	return fmt.Sprintf("tf-%s", strings.ReplaceAll(t.Name(), "/", "_"))
}

// initTerraformRunner copies the static Terraform files to the Terraform directory
// for this test, then initializes the Terraform runner in that directory.
//
// Note: This function will remove all existing files from the test Terraform directory
// if it exists, before copying into it.
func initTerraformRunner(t *testing.T) *terraform.Runner {
	t.Helper()
	dirName := terraformDirName(t)
	err := os.RemoveAll(dirName)
	require.NoError(t, err)
	err = os.CopyFS(dirName, os.DirFS("infra/terraform"))
	require.NoError(t, err)

	tf, err := terraform.NewRunner(t, dirName)
	require.NoError(t, err)
	return tf
}

type deploymentInfo struct {
	DeploymentName string

	// ElasticsearchURL holds the Elasticsearch URL.
	ElasticsearchURL string

	// Username holds the Elasticsearch superuser username for basic auth.
	Username string

	// Password holds the Elasticsearch superuser password for basic auth.
	Password string

	// APMServerURL holds the APM Server URL.
	APMServerURL string

	// KibanaURL holds the Kibana URL.
	KibanaURL string
}

func deploymentName(t *testing.T) string {
	prefix := strings.Split(t.Name(), "_")[0]
	// We add random string to avoid having clashing alias in ECH.
	return fmt.Sprintf("%s_%s", prefix, rand.Text()[:10])
}

// createCluster runs terraform on the test terraform folder to spin up an
// Elastic Cloud Hosted (ECH) cluster for testing.
//
// It returns the deploymentID of the created cluster and a deploymentInfo
// object filled with cluster relevant information.
//
// It sets up a cleanup function to destroy resources if the test succeed.
// If the test fails, cleanupOnFailure determines whether the cleanup function
// will run.
func createCluster(
	t *testing.T,
	ctx context.Context,
	tf *terraform.Runner,
	target string,
	fromVersion ech.Version,
	enableIntegrations bool,
	cleanupOnFailure bool,
) deploymentInfo {
	t.Helper()

	deployName := deploymentName(t)
	t.Logf("creating deployment version %s", fromVersion)
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", RegionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", DeploymentTemplateFrom(RegionFrom(target)))
	ver := terraform.Var("stack_version", fromVersion.String())
	integrations := terraform.Var("integrations_server", strconv.FormatBool(enableIntegrations))
	name := terraform.Var("name", deployName)
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, ecDeploymentTpl, ver, integrations, name))

	t.Cleanup(func() {
		if !t.Failed() || cleanupOnFailure {
			t.Log("cleanup terraform resources")
			require.NoError(t, tf.Destroy(ctx, ecTarget, ecRegion, ecDeploymentTpl, name, ver))
		} else {
			t.Log("test failed and cleanup-on-failure is false, skipping cleanup")
		}
	})

	var deploymentID string
	require.NoError(t, tf.Output("deployment_id", &deploymentID))
	var apmID string
	require.NoError(t, tf.Output("apm_id", &apmID))
	info := deploymentInfo{DeploymentName: deployName}
	require.NoError(t, tf.Output("apm_url", &info.APMServerURL))
	require.NoError(t, tf.Output("es_url", &info.ElasticsearchURL))
	require.NoError(t, tf.Output("username", &info.Username))
	require.NoError(t, tf.Output("password", &info.Password))
	require.NoError(t, tf.Output("kb_url", &info.KibanaURL))

	standaloneOrManaged := "standalone"
	if enableIntegrations {
		standaloneOrManaged = "managed"
	}
	t.Logf("created deployment %s (%s) with %s APM (%s)", deployName, deploymentID, standaloneOrManaged, apmID)
	return info
}

// upgradeCluster applies the terraform configuration from the test terraform folder.
func upgradeCluster(
	t *testing.T,
	ctx context.Context,
	tf *terraform.Runner,
	deployName string,
	target string,
	toVersion ech.Version,
	enableIntegrations bool,
) {
	t.Helper()
	t.Logf("upgrade deployment to %s", toVersion)
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", RegionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", DeploymentTemplateFrom(RegionFrom(target)))
	ver := terraform.Var("stack_version", toVersion.String())
	integrations := terraform.Var("integrations_server", strconv.FormatBool(enableIntegrations))
	name := terraform.Var("name", deployName)
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, ecDeploymentTpl, ver, integrations, name))
}

// createESClient instantiate an HTTP API client with dedicated methods to query the Elasticsearch API.
func createESClient(t *testing.T, deployInfo deploymentInfo) *elasticsearch.Client {
	t.Helper()
	t.Log("create elasticsearch client")
	esc, err := elasticsearch.NewClient(deployInfo.ElasticsearchURL, deployInfo.Username, deployInfo.Password)
	require.NoError(t, err)
	return esc
}

// createKibanaClient instantiate an HTTP API client with dedicated methods to query the Kibana API.
func createKibanaClient(t *testing.T, deployInfo deploymentInfo) *kibana.Client {
	t.Helper()
	t.Log("create kibana client")
	kbc, err := kibana.NewClient(deployInfo.KibanaURL, deployInfo.Username, deployInfo.Password)
	require.NoError(t, err)
	return kbc
}

// createAPMGenerator instantiate a load generator for APM.
// This function will also create an Elasticsearch API key with full permissions to be used by the generator.
func createAPMGenerator(t *testing.T, ctx context.Context, esc *elasticsearch.Client, kbc *kibana.Client, deployInfo deploymentInfo) *gen.Generator {
	t.Helper()
	t.Log("create apm generator")
	apiKey, err := esc.CreateAPIKey(ctx, "apmgenerator", -1, map[string]types.RoleDescriptor{})
	require.NoError(t, err)
	logger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	g := gen.New(deployInfo.APMServerURL, apiKey, kbc, logger)
	return g
}
