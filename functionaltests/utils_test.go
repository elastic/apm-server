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

package functionaltests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

func formattedTestName(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "_")
}

// terraformDir returns the name of the Terraform files directory for this test.
func terraformDir(t *testing.T) string {
	t.Helper()
	// Flatten the dir name in case of path separators
	return fmt.Sprintf("tf-%s", formattedTestName(t))
}

// copyTerraforms copies the static Terraform files to the Terraform directory for this test.
// It will remove all existing files from the test Terraform directory if it exists, before copying into it.
func copyTerraforms(t *testing.T) {
	t.Helper()
	dirName := terraformDir(t)
	err := os.RemoveAll(dirName)
	require.NoError(t, err)
	err = os.CopyFS(terraformDir(t), os.DirFS("infra/terraform"))
	require.NoError(t, err)
}

type deploymentOutput struct {
	// ElasticsearchURL holds the Elasticsearch URL.
	ElasticsearchURL string

	// Username holds the Elasticsearch username for basic auth.
	Username string

	// Password holds the Elasticsearch password for basic auth.
	Password string

	// APMServerURL holds the APM Server URL.
	APMServerURL string

	// KibanaURL holds the Kibana URL.
	KibanaURL string
}

// createCluster runs terraform on the test terraform folder to spin up an Elastic Cloud Hosted cluster for testing.
// It returns the deploymentID of the created cluster and an esclient.Config object filled with cluster relevant
// information.
// It sets up a cleanup function to destroy resources if the test succeed, leveraging the cleanupOnFailure flag to
// skip this behavior if appropriate.
func createCluster(
	t *testing.T,
	ctx context.Context,
	tf *terraform.Runner,
	target string,
	fromVersion string,
	enableIntegrations bool,
) deploymentOutput {
	t.Helper()

	t.Logf("creating deployment version %s", fromVersion)
	// TODO: use a terraform var file for all vars that are not expected to change across upgrades
	// to simplify and clarify this code.
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", regionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", deploymentTemplateFrom(regionFrom(target)))
	version := terraform.Var("stack_version", fromVersion)
	integrations := terraform.Var("integrations_server", strconv.FormatBool(enableIntegrations))
	name := terraform.Var("name", formattedTestName(t))
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, ecDeploymentTpl, version, integrations, name))

	t.Cleanup(func() {
		if !t.Failed() || (t.Failed() && *cleanupOnFailure) {
			t.Log("cleanup terraform resources")
			require.NoError(t, tf.Destroy(ctx, ecTarget, ecRegion, ecDeploymentTpl, name, version))
		} else {
			t.Log("test failed and cleanup-on-failure is false, skipping cleanup")
		}
	})

	var deploymentID string
	require.NoError(t, tf.Output("deployment_id", &deploymentID))
	var apmID string
	require.NoError(t, tf.Output("apm_id", &apmID))
	var output deploymentOutput
	require.NoError(t, tf.Output("apm_url", &output.APMServerURL))
	require.NoError(t, tf.Output("es_url", &output.ElasticsearchURL))
	require.NoError(t, tf.Output("username", &output.Username))
	require.NoError(t, tf.Output("password", &output.Password))
	require.NoError(t, tf.Output("kb_url", &output.KibanaURL))

	standaloneOrManaged := "standalone"
	if enableIntegrations {
		standaloneOrManaged = "managed"
	}
	t.Logf("created %s deployment %s with APM (%s)", standaloneOrManaged, deploymentID, apmID)
	return output
}

// upgradeCluster applies the terraform configuration from the test terraform folder.
func upgradeCluster(t *testing.T, ctx context.Context, tf *terraform.Runner, target, toVersion string) {
	t.Helper()
	t.Logf("upgrade deployment to %s", toVersion)
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", regionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", deploymentTemplateFrom(regionFrom(target)))
	version := terraform.Var("stack_version", toVersion)
	name := terraform.Var("name", formattedTestName(t))
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, ecDeploymentTpl, name, version))
}

// createESClient instantiate an HTTP API client with dedicated methods to query the Elasticsearch API.
func createESClient(t *testing.T, deployOut deploymentOutput) *esclient.Client {
	t.Helper()
	t.Log("create elasticsearch client")
	esc, err := esclient.New(deployOut.ElasticsearchURL, deployOut.Username, deployOut.Password)
	require.NoError(t, err)
	return esc
}

// createKibanaClient instantiate an HTTP API client with dedicated methods to query the Kibana API.
// This function will also create an Elasticsearch API key with full permissions to be used by the HTTP client.
func createKibanaClient(t *testing.T, ctx context.Context, esc *esclient.Client, deployOut deploymentOutput) *kbclient.Client {
	t.Helper()
	t.Log("create kibana client")
	apiKey, err := esc.CreateAPIKey(ctx, "kbclient", -1, map[string]types.RoleDescriptor{})
	require.NoError(t, err)
	kbc, err := kbclient.New(deployOut.KibanaURL, apiKey, deployOut.Username, deployOut.Password)
	require.NoError(t, err)
	return kbc
}

// createAPMGenerator instantiate a load generator for APM.
// This function will also create an Elasticsearch API key with full permissions to be used by the generator.
func createAPMGenerator(t *testing.T, ctx context.Context, esc *esclient.Client, deployOut deploymentOutput) *gen.Generator {
	t.Helper()
	t.Log("create apm generator")
	apiKey, err := esc.CreateAPIKey(ctx, "apmgenerator", -1, map[string]types.RoleDescriptor{})
	require.NoError(t, err)
	g := gen.New(deployOut.APMServerURL, apiKey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	return g
}

// createRerouteIngestPipeline creates custom pipelines to reroute logs, metrics and traces to different
// data streams specified by namespace.
func createRerouteIngestPipeline(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) error {
	t.Helper()

	for _, pipeline := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		err := esc.CreateIngestPipeline(ctx, pipeline, []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{namespace},
				},
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// performManualRollovers rollover all logs, metrics and traces data streams to new indices.
func performManualRollovers(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) error {
	t.Helper()

	for _, ds := range allDataStreams(namespace) {
		err := esc.PerformManualRollover(ctx, ds)
		if err != nil {
			return err
		}
	}
	return nil
}
