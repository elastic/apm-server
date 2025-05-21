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
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

var (
	// cleanupOnFailure determines whether the created resources should be cleaned up on test failure.
	cleanupOnFailure = flag.Bool(
		"cleanup-on-failure",
		true,
		"Whether to run cleanup even if the test failed.",
	)

	// target is the Elastic Cloud environment to target with these test.
	// We use 'pro' for production as that is the key used to retrieve EC_API_KEY from secret storage.
	target = flag.String(
		"target",
		"pro",
		"The target environment where to run tests againts. Valid values are: qa, pro.",
	)
)

const (
	// managedByDSL is the constant string used by Elasticsearch to specify that an Index is managed by Data Stream Lifecycle management.
	managedByDSL = "Data stream lifecycle"
	// managedByILM is the constant string used by Elasticsearch to specify that an Index is managed by Index Lifecycle Management.
	managedByILM = "Index Lifecycle Management"
)

var (
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates ecclient.StackVersionInfos
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots ecclient.StackVersionInfos
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions ecclient.StackVersionInfos
)

// getLatestVersionOrSkip retrieves the latest non-snapshot version for the version prefix.
// If the version is not found, the test is skipped via t.Skip.
func getLatestVersionOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := fetchedVersions.LatestFor(prefix)
	if !ok {
		t.Skipf("version for '%s' not found in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}
	return version
}

// getLatestBCOrSkip retrieves the latest build-candidate version for the version prefix.
// If the version is not found, the test is skipped via t.Skip.
func getLatestBCOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	candidate, ok := fetchedCandidates.LatestFor(prefix)
	if !ok {
		t.Skipf("BC for '%s' not found in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}

	// Check that the BC version is actually latest, otherwise skip test.
	versionInfo := getLatestVersionOrSkip(t, prefix)
	if versionInfo.Version.Major != candidate.Version.Major {
		t.Skipf("BC for '%s' is invalid in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}
	if versionInfo.Version.Minor > candidate.Version.Minor {
		t.Skipf("BC for '%s' is less than latest normal version in EC region %s, skipping test",
			prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}

	return candidate
}

// getLatestSnapshot retrieves the latest snapshot version for the version prefix.
func getLatestSnapshot(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "snapshot for '%s' found in EC region %s", prefix, regionFrom(*target))
	return version
}

// expectedDataStreamsIngest represent the expected number of ingested document
// after a single run of ingest.
//
// NOTE: The aggregation data streams have negative counts, because they are
// expected to appear but the document counts should not be asserted.
func expectedDataStreamsIngest(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     15013,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 1437,
		fmt.Sprintf("metrics-apm.internal-%s", namespace):           1351,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 364,
		// Ignore aggregation data streams.
		fmt.Sprintf("metrics-apm.service_destination.1m-%s", namespace): -1,
		fmt.Sprintf("metrics-apm.service_transaction.1m-%s", namespace): -1,
		fmt.Sprintf("metrics-apm.service_summary.1m-%s", namespace):     -1,
		fmt.Sprintf("metrics-apm.transaction.1m-%s", namespace):         -1,
	}
}

// emptyDataStreamsIngest represent an empty ingestion.
// It is useful for asserting that the document count did not change after an operation.
//
// NOTE: The aggregation data streams have negative counts, because they
// are expected to appear but the document counts should not be asserted.
func emptyDataStreamsIngest(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     0,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 0,
		fmt.Sprintf("metrics-apm.internal-%s", namespace):           0,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 0,
		// Ignore aggregation data streams.
		fmt.Sprintf("metrics-apm.service_destination.1m-%s", namespace): -1,
		fmt.Sprintf("metrics-apm.service_transaction.1m-%s", namespace): -1,
		fmt.Sprintf("metrics-apm.service_summary.1m-%s", namespace):     -1,
		fmt.Sprintf("metrics-apm.transaction.1m-%s", namespace):         -1,
	}
}

func allDataStreams(namespace string) []string {
	return slices.Collect(maps.Keys(expectedDataStreamsIngest(namespace)))
}

const (
	targetQA = "qa"
	// we use 'pro' because is the target passed by the Buildkite pipeline running
	// these tests.
	targetProd = "pro"
)

// regionFrom returns the appropriate region to run test
// against based on specified target.
// https://www.elastic.co/guide/en/cloud/current/ec-regions-templates-instances.html
func regionFrom(target string) string {
	switch target {
	case targetQA:
		return "aws-eu-west-1"
	case targetProd:
		return "gcp-us-west2"
	default:
		panic("target value is not accepted")
	}
}

func endpointFrom(target string) string {
	switch target {
	case targetQA:
		return "https://public-api.qa.cld.elstc.co"
	case targetProd:
		return "https://api.elastic-cloud.com"
	default:
		panic("target value is not accepted")
	}
}

func deploymentTemplateFrom(region string) string {
	switch region {
	case "aws-eu-west-1":
		return "aws-storage-optimized"
	case "gcp-us-west2":
		return "gcp-storage-optimized"
	default:
		panic("region value is not accepted")
	}
}

func formattedTestName(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "_")
}

// terraformDir returns the name of the Terraform files directory for this test.
func terraformDir(t *testing.T) string {
	t.Helper()
	// Flatten the dir name in case of path separators
	return fmt.Sprintf("tf-%s", formattedTestName(t))
}

// initTerraformRunner copies the static Terraform files to the Terraform directory for this test,
// then initializes the Terraform runner in that directory.
//
// Note: This function will remove all existing files from the test Terraform directory if it exists,
// before copying into it.
func initTerraformRunner(t *testing.T) *terraform.Runner {
	t.Helper()
	dirName := terraformDir(t)
	err := os.RemoveAll(dirName)
	require.NoError(t, err)
	err = os.CopyFS(terraformDir(t), os.DirFS("infra/terraform"))
	require.NoError(t, err)

	tf, err := terraform.New(t, dirName)
	require.NoError(t, err)
	return tf
}

type deploymentInfo struct {
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
	fromVersion ecclient.StackVersion,
	enableIntegrations bool,
) deploymentInfo {
	t.Helper()

	t.Logf("creating deployment version %s", fromVersion)
	// TODO: use a terraform var file for all vars that are not expected to change across upgrades
	// to simplify and clarify this code.
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", regionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", deploymentTemplateFrom(regionFrom(target)))
	version := terraform.Var("stack_version", fromVersion.String())
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
	var info deploymentInfo
	require.NoError(t, tf.Output("apm_url", &info.APMServerURL))
	require.NoError(t, tf.Output("es_url", &info.ElasticsearchURL))
	require.NoError(t, tf.Output("username", &info.Username))
	require.NoError(t, tf.Output("password", &info.Password))
	require.NoError(t, tf.Output("kb_url", &info.KibanaURL))

	standaloneOrManaged := "standalone"
	if enableIntegrations {
		standaloneOrManaged = "managed"
	}
	t.Logf("created deployment %s with %s APM (%s)", deploymentID, standaloneOrManaged, apmID)
	return info
}

// upgradeCluster applies the terraform configuration from the test terraform folder.
func upgradeCluster(
	t *testing.T,
	ctx context.Context,
	tf *terraform.Runner,
	target string,
	toVersion ecclient.StackVersion,
	enableIntegrations bool,
) {
	t.Helper()
	t.Logf("upgrade deployment to %s", toVersion)
	ecTarget := terraform.Var("ec_target", target)
	ecRegion := terraform.Var("ec_region", regionFrom(target))
	ecDeploymentTpl := terraform.Var("ec_deployment_template", deploymentTemplateFrom(regionFrom(target)))
	version := terraform.Var("stack_version", toVersion.String())
	integrations := terraform.Var("integrations_server", strconv.FormatBool(enableIntegrations))
	name := terraform.Var("name", formattedTestName(t))
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, ecDeploymentTpl, version, integrations, name))
}

// createESClient instantiate an HTTP API client with dedicated methods to query the Elasticsearch API.
func createESClient(t *testing.T, deployInfo deploymentInfo) *esclient.Client {
	t.Helper()
	t.Log("create elasticsearch client")
	esc, err := esclient.New(deployInfo.ElasticsearchURL, deployInfo.Username, deployInfo.Password)
	require.NoError(t, err)
	return esc
}

// createKibanaClient instantiate an HTTP API client with dedicated methods to query the Kibana API.
func createKibanaClient(t *testing.T, deployInfo deploymentInfo) *kbclient.Client {
	t.Helper()
	t.Log("create kibana client")
	kbc, err := kbclient.New(deployInfo.KibanaURL, deployInfo.Username, deployInfo.Password)
	require.NoError(t, err)
	return kbc
}

// createAPMGenerator instantiate a load generator for APM.
// This function will also create an Elasticsearch API key with full permissions to be used by the generator.
func createAPMGenerator(t *testing.T, ctx context.Context, esc *esclient.Client, kbc *kbclient.Client, deployInfo deploymentInfo) *gen.Generator {
	t.Helper()
	t.Log("create apm generator")
	apiKey, err := esc.CreateAPIKey(ctx, "apmgenerator", -1, map[string]types.RoleDescriptor{})
	require.NoError(t, err)
	logger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	g := gen.New(deployInfo.APMServerURL, apiKey, kbc, logger)
	return g
}

func sliceToSet[T comparable](s []T) map[T]bool {
	m := make(map[T]bool)
	for _, ele := range s {
		m[ele] = true
	}
	return m
}

// getAPMDataStreams get all APM related data streams.
func getAPMDataStreams(t *testing.T, ctx context.Context, esc *esclient.Client, ignoreDS ...string) []types.DataStream {
	t.Helper()
	dataStreams, err := esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)

	ignore := sliceToSet(ignoreDS)
	return slices.DeleteFunc(dataStreams, func(ds types.DataStream) bool {
		return ignore[ds.Name]
	})
}

// getDocCountPerDS retrieves document count per data stream for versions >= 8.0.
func getDocCountPerDS(t *testing.T, ctx context.Context, esc *esclient.Client, ignoreDS ...string) esclient.DataStreamsDocCount {
	t.Helper()
	count, err := esc.APMDSDocCount(ctx)
	require.NoError(t, err)

	ignore := sliceToSet(ignoreDS)
	maps.DeleteFunc(count, func(ds string, _ int) bool {
		return ignore[ds]
	})
	return count
}

// getDocCountPerDS retrieves document count per data stream for versions < 8.0.
func getDocCountPerDSV7(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) esclient.DataStreamsDocCount {
	t.Helper()
	count, err := esc.APMDSDocCountV7(ctx, namespace)
	require.NoError(t, err)
	return count
}

// getDocCountPerIndexV7 retrieves document count per index for versions < 8.0.
func getDocCountPerIndexV7(t *testing.T, ctx context.Context, esc *esclient.Client) esclient.IndicesDocCount {
	t.Helper()
	count, err := esc.APMIdxDocCountV7(ctx)
	require.NoError(t, err)
	return count
}

// createRerouteIngestPipeline creates custom pipelines to reroute logs, metrics and traces to different
// data streams specified by namespace.
func createRerouteIngestPipeline(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) {
	t.Helper()
	for _, pipeline := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		err := esc.CreateIngestPipeline(ctx, pipeline, []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{namespace},
				},
			},
		})
		require.NoError(t, err)
	}
}

// performManualRollovers rollover all logs, metrics and traces data streams to new indices.
func performManualRollovers(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) {
	t.Helper()

	for _, ds := range allDataStreams(namespace) {
		err := esc.PerformManualRollover(ctx, ds)
		require.NoError(t, err)
	}
}
