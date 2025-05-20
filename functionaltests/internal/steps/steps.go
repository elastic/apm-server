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

package steps

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

// Runner consists of composable Step(s) that is run
// in sequence.
type Runner struct {
	// DataStreamNamespace is the namespace for the APM data streams
	// that is being tested. Defaults to "default".
	//
	// NOTE: Only applicable for stack Versions >= 8.0 or 7.x with
	// integrations enabled.
	DataStreamNamespace string

	// CloudEnvironment is the target EC environment to run the
	// tests in.
	CloudEnvironment string

	// CloudRegion is the target EC region to run the tests in.
	CloudRegion string

	// CloudDeploymentTemplate is the deployment template to be
	// used for the EC deployment.
	CloudDeploymentTemplate string

	// CleanupOnFailure determines whether to clean up the test resources
	// if the test fails.
	CleanupOnFailure bool

	// Steps are the user defined test steps to be run.
	Steps []Step
}

// Run runs the test steps in sequence, passing the result from the current step
// into the next step etc.
func (r Runner) Run(t *testing.T) {
	if r.DataStreamNamespace == "" {
		r.DataStreamNamespace = "default"
	}

	if r.CloudEnvironment == "" || r.CloudRegion == "" || r.CloudDeploymentTemplate == "" {
		t.Fatal("cloud environment, region, deployment template required")
	}

	if len(r.Steps) == 0 {
		t.Fatal("at least one step must be specified in test run")
	}
	if _, ok := r.Steps[0].(CreateStep); !ok {
		t.Fatal("first step of test run must be CreateStep")
	}

	start := time.Now()
	ctx := context.Background()

	env := Env{
		dsNamespace:        r.DataStreamNamespace,
		target:             r.CloudEnvironment,
		region:             r.CloudRegion,
		deploymentTemplate: r.CloudDeploymentTemplate,
		cleanupOnFailure:   r.CleanupOnFailure,
	}

	currentRes := Result{}
	for _, step := range r.Steps {
		currentRes = step.Step(t, ctx, &env, currentRes)
		t.Logf("time elapsed: %s", time.Since(start))
	}
}

// Result contains the results of running the step.
type Result struct {
	// DSDocCount is the data streams document counts that is the
	// result of this step.
	//
	// Note: Only applicable for stack Versions >= 8.0.
	DSDocCount esclient.DataStreamsDocCount

	// IndicesDocCount is the indices document counts that is the
	// result of this step.
	//
	// Note: Only applicable for stack Versions < 8.0.
	IndicesDocCount esclient.IndicesDocCount
}

// Env is the environment of the step that is run.
type Env struct {
	// Defined in runner, cannot be changed midway.
	dsNamespace        string
	target             string
	region             string
	deploymentTemplate string
	cleanupOnFailure   bool
	// Initialized after CreateStep.
	Versions     []ecclient.StackVersion
	Integrations bool
	TF           *terraform.Runner
	Generator    *gen.Generator
	KibanaClient *kbclient.Client
	ESClient     *esclient.Client
}

func (env *Env) currentVersion() ecclient.StackVersion {
	if len(env.Versions) == 0 {
		panic("test step env current version not found")
	}
	return env.Versions[len(env.Versions)-1]
}

type Step interface {
	Step(t *testing.T, ctx context.Context, e *Env, previousRes Result) Result
}

// APMDeploymentMode is the deployment mode of APM in the cluster.
// This is used instead of bool to avoid having to use bool pointer
// (since the default is true).
type APMDeploymentMode uint8

const (
	APMDefault APMDeploymentMode = iota
	APMManaged
	APMStandalone
)

func (mode APMDeploymentMode) enableIntegrations() bool {
	return mode == APMDefault || mode == APMManaged
}

// CreateStep initializes the Terraform runner and deploys an Elastic Cloud
// Hosted (ECH) cluster with the provided stack version. It also creates the
// necessary clients and set them into Env.
//
// The output of this step is the initial document counts in ES.
//
// Note: This step should always be the first step of any test runs, since it
// initializes all the necessary dependencies for subsequent steps.
type CreateStep struct {
	DeployVersion     ecclient.StackVersion
	APMDeploymentMode APMDeploymentMode
}

func (c CreateStep) Step(t *testing.T, ctx context.Context, e *Env, _ Result) Result {
	integrations := c.APMDeploymentMode.enableIntegrations()
	if c.DeployVersion.Major < 8 && integrations {
		t.Fatal("create step cannot enable integrations for versions < 8.0")
	}

	t.Logf("------ cluster setup %s ------", c.DeployVersion)
	e.TF = initTerraformRunner(t)
	deployInfo := createCluster(t, ctx, e.TF, e.target, e.region, e.deploymentTemplate, c.DeployVersion, integrations, e.cleanupOnFailure)
	e.ESClient = createESClient(t, deployInfo)
	e.KibanaClient = createKibanaClient(t, deployInfo)
	e.Generator = createAPMGenerator(t, ctx, e.ESClient, e.KibanaClient, deployInfo)
	// Update the latest environment version to the new one.
	e.Versions = append(e.Versions, c.DeployVersion)
	e.Integrations = integrations

	if e.currentVersion().Major < 8 {
		return Result{IndicesDocCount: getDocCountPerIndexV7(t, ctx, e.ESClient)}
	}
	return Result{DSDocCount: getDocCountPerDS(t, ctx, e.ESClient)}
}

// IngestStep performs ingestion to the APM Server deployed on ECH. After
// ingestion, it checks if the document counts difference between current
// and previous is expected, and if the data streams are in an expected
// state.
//
// The output of this step is the data streams document counts after ingestion.
//
// NOTE: Only works for Versions >= 8.0.
type IngestStep struct {
	CheckDataStream asserts.CheckDataStreamsWant
	// IgnoreDataStreams are the data streams to be ignored in assertions.
	// The data stream names can contain '%s' to indicate namespace.
	IgnoreDataStreams []string
	// CheckIndividualDataStream is used to check the data streams individually
	// instead of as a whole using CheckDataStream.
	// The data stream names can contain '%s' to indicate namespace.
	CheckIndividualDataStream map[string]asserts.CheckDataStreamIndividualWant
}

func (i IngestStep) Step(t *testing.T, ctx context.Context, e *Env, previousRes Result) Result {
	if e.currentVersion().Major < 8 {
		t.Fatal("ingest step should only be used for versions >= 8.0")
	}

	t.Logf("------ ingest in %s------", e.currentVersion())
	err := e.Generator.RunBlockingWait(ctx, e.currentVersion(), e.Integrations)
	require.NoError(t, err)

	t.Logf("------ ingest check in %s ------", e.currentVersion())
	t.Log("check number of documents after ingestion")
	ignoreDS := formatAll(i.IgnoreDataStreams, e.dsNamespace)
	dsDocCount := getDocCountPerDS(t, ctx, e.ESClient, ignoreDS...)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		expectedDataStreamsIngest(e.dsNamespace))

	t.Log("check data streams after ingestion")
	dataStreams := getAPMDataStreams(t, ctx, e.ESClient, ignoreDS...)
	if i.CheckIndividualDataStream != nil {
		expected := formatAllMap(i.CheckIndividualDataStream, e.dsNamespace)
		asserts.CheckDataStreamsIndividually(t, expected, dataStreams)
	} else {
		asserts.CheckDataStreams(t, i.CheckDataStream, dataStreams)
	}

	return Result{DSDocCount: dsDocCount}
}

func formatAll(formats []string, s string) []string {
	res := make([]string, 0, len(formats))
	for _, format := range formats {
		res = append(res, fmt.Sprintf(format, s))
	}
	return res
}

func formatAllMap[T any](m map[string]T, s string) map[string]T {
	res := make(map[string]T)
	for k, v := range m {
		res[fmt.Sprintf(k, s)] = v
	}
	return res
}

// UpgradeStep upgrades the ECH deployment from its current version to the new
// version. It also adds the new version into Env. After upgrade, it
// checks that the document counts did not change across upgrade.
//
// The output of this step is the data streams document counts after upgrade.
//
// NOTE: Only works from Versions >= 8.0.
type UpgradeStep struct {
	NewVersion      ecclient.StackVersion
	CheckDataStream asserts.CheckDataStreamsWant
	// IgnoreDataStreams are the data streams to be ignored in assertions.
	// The data stream names can contain '%s' to indicate namespace.
	IgnoreDataStreams []string
	// CheckIndividualDataStream is used to check the data streams individually
	// instead of as a whole using CheckDataStream.
	// The data stream names can contain '%s' to indicate namespace.
	CheckIndividualDataStream map[string]asserts.CheckDataStreamIndividualWant
}

func (u UpgradeStep) Step(t *testing.T, ctx context.Context, e *Env, previousRes Result) Result {
	if e.currentVersion().Major < 8 {
		t.Fatal("upgrade step should only be used from versions >= 8.0")
	}

	t.Logf("------ upgrade %s to %s ------", e.currentVersion(), u.NewVersion)
	upgradeCluster(t, ctx, e.TF, e.target, e.region, e.deploymentTemplate, u.NewVersion, e.Integrations)
	// Update the environment version to the new one.
	e.Versions = append(e.Versions, u.NewVersion)

	t.Logf("------ upgrade check in %s ------", e.currentVersion())
	t.Log("check number of documents across upgrade")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	ignoreDS := formatAll(u.IgnoreDataStreams, e.dsNamespace)
	dsDocCount := getDocCountPerDS(t, ctx, e.ESClient, ignoreDS...)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		emptyDataStreamsIngest(e.dsNamespace))

	t.Log("check data streams after upgrade")
	dataStreams := getAPMDataStreams(t, ctx, e.ESClient, ignoreDS...)
	if u.CheckIndividualDataStream != nil {
		expected := formatAllMap(u.CheckIndividualDataStream, e.dsNamespace)
		asserts.CheckDataStreamsIndividually(t, expected, dataStreams)
	} else {
		asserts.CheckDataStreams(t, u.CheckDataStream, dataStreams)
	}

	return Result{DSDocCount: dsDocCount}
}

// CheckErrorLogsStep checks if there are any unexpected error logs from both
// Elasticsearch and APM Server. The provided APMErrorLogsIgnored is used to
// ignore some APM logs from being included in the assertion.
//
// The output of this step is the previous step's result.
type CheckErrorLogsStep struct {
	ESErrorLogsIgnored  ESErrorLogs
	APMErrorLogsIgnored APMErrorLogs
}

func (c CheckErrorLogsStep) Step(t *testing.T, ctx context.Context, e *Env, previousRes Result) Result {
	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := e.ESClient.GetESErrorLogs(ctx, c.ESErrorLogsIgnored.ToQueries()...)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = e.ESClient.GetAPMErrorLogs(ctx, c.APMErrorLogsIgnored.ToQueries()...)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)

	return previousRes
}

// CreateReroutePipelinesStep creates ingest pipelines that reroute logs,
// metrics and traces to different data streams specified by namespace.
type CreateReroutePipelinesStep struct {
	RerouteNamespace string
}

func (c CreateReroutePipelinesStep) Step(t *testing.T, ctx context.Context, e *Env, previousRes Result) Result {
	t.Log("create reroute ingest pipelines")
	for _, pipeline := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		err := e.ESClient.CreateIngestPipeline(ctx, pipeline, []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{c.RerouteNamespace},
				},
			},
		})
		require.NoError(t, err)
	}

	e.dsNamespace = c.RerouteNamespace
	return previousRes
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
