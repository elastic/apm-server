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
	"maps"
	"slices"
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

// testStepsRunner consists of composable testStep(s) that is run
// in sequence.
type testStepsRunner struct {
	// Target is the target environment for the Elastic Cloud deployment.
	Target string

	// DataStreamNamespace is the namespace for the APM data streams
	// that is being tested. Defaults to "default".
	//
	// NOTE: Only applicable for stack versions >= 8.0 or 7.x with
	// integrations enabled.
	DataStreamNamespace string

	// Steps are the user defined test steps to be run.
	Steps []testStep
}

// Run runs the test steps in sequence, passing the result from the current step
// into the next step etc.
func (r testStepsRunner) Run(t *testing.T) {
	if r.Target == "" {
		r.Target = targetProd
	}
	if r.DataStreamNamespace == "" {
		r.DataStreamNamespace = "default"
	}

	if len(r.Steps) == 0 {
		t.Fatal("at least one step must be specified in test run")
	}
	if _, ok := r.Steps[0].(createStep); !ok {
		t.Fatal("first step of test run must be createStep")
	}

	start := time.Now()
	ctx := context.Background()

	env := testStepEnv{target: r.Target, dsNamespace: r.DataStreamNamespace}
	currentRes := testStepResult{}
	for _, step := range r.Steps {
		currentRes = step.Step(t, ctx, &env, currentRes)
		t.Logf("time elapsed: %s", time.Since(start))
	}
}

// testStepResult contains the results of running the step.
type testStepResult struct {
	// DSDocCount is the data streams document counts that is the
	// result of this step.
	//
	// Note: Only applicable for stack versions >= 8.0.
	DSDocCount esclient.DataStreamsDocCount

	// IndicesDocCount is the indices document counts that is the
	// result of this step.
	//
	// Note: Only applicable for stack versions < 8.0.
	IndicesDocCount esclient.IndicesDocCount
}

// testStepEnv is the environment of the step that is run.
type testStepEnv struct {
	target       string
	dsNamespace  string
	versions     []ecclient.StackVersion
	integrations bool
	tf           *terraform.Runner
	gen          *gen.Generator
	kbc          *kbclient.Client
	esc          *esclient.Client
}

func (env *testStepEnv) currentVersion() ecclient.StackVersion {
	if len(env.versions) == 0 {
		panic("test step env current version not found")
	}
	return env.versions[len(env.versions)-1]
}

type testStep interface {
	Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult
}

// apmDeploymentMode is the deployment mode of APM in the cluster.
// This is used instead of bool to avoid having to use bool pointer
// (since the default is true).
type apmDeploymentMode uint8

const (
	apmDefault apmDeploymentMode = iota
	apmManaged
	apmStandalone
)

func (mode apmDeploymentMode) enableIntegrations() bool {
	return mode == apmDefault || mode == apmManaged
}

// createStep initializes the Terraform runner and deploys an Elastic Cloud
// Hosted (ECH) cluster with the provided stack version. It also creates the
// necessary clients and set them into testStepEnv.
//
// The output of this step is the initial document counts in ES.
//
// Note: This step should always be the first step of any test runs, since it
// initializes all the necessary dependencies for subsequent steps.
type createStep struct {
	DeployVersion     ecclient.StackVersion
	APMDeploymentMode apmDeploymentMode
	CleanupOnFailure  bool
}

func (c createStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, _ testStepResult) testStepResult {
	integrations := c.APMDeploymentMode.enableIntegrations()
	if c.DeployVersion.Major < 8 && integrations {
		t.Fatal("create step cannot enable integrations for versions < 8.0")
	}

	t.Logf("------ cluster setup %s ------", c.DeployVersion)
	e.tf = initTerraformRunner(t)
	deployInfo := createCluster(t, ctx, e.tf, e.target, c.DeployVersion, integrations, c.CleanupOnFailure)
	e.esc = createESClient(t, deployInfo)
	e.kbc = createKibanaClient(t, deployInfo)
	e.gen = createAPMGenerator(t, ctx, e.esc, e.kbc, deployInfo)
	// Update the latest environment version to the new one.
	e.versions = append(e.versions, c.DeployVersion)
	e.integrations = integrations

	if e.currentVersion().Major < 8 {
		return testStepResult{IndicesDocCount: getDocCountPerIndexV7(t, ctx, e.esc)}
	}
	return testStepResult{DSDocCount: getDocCountPerDS(t, ctx, e.esc)}
}

// ingestStep performs ingestion to the APM Server deployed on ECH. After
// ingestion, it checks if the document counts difference between current
// and previous is expected, and if the data streams are in an expected
// state.
//
// The output of this step is the data streams document counts after ingestion.
//
// NOTE: Only works for versions >= 8.0.
type ingestStep struct {
	// CheckDataStream is used to check all data streams have the same expected values.
	// For checking each data stream individually, use CheckIndividualDataStream.
	CheckDataStream asserts.CheckDataStreamsWant

	// CheckIndividualDataStream is used to check the data streams individually
	// instead of as a whole using CheckDataStream.
	// The data stream names can contain '%s' to indicate namespace.
	CheckIndividualDataStream map[string]asserts.CheckDataStreamIndividualWant

	// IgnoreDataStreams are the data streams to be ignored in assertions.
	// The data stream names can contain '%s' to indicate namespace.
	IgnoreDataStreams []string
}

func (i ingestStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major < 8 {
		t.Fatal("ingest step should only be used for versions >= 8.0")
	}

	t.Logf("------ ingest in %s------", e.currentVersion())
	err := e.gen.RunBlockingWait(ctx, e.currentVersion(), e.integrations)
	require.NoError(t, err)

	t.Logf("------ ingest check in %s ------", e.currentVersion())
	t.Log("check number of documents after ingestion")
	ignoreDS := formatAll(i.IgnoreDataStreams, e.dsNamespace)
	dsDocCount := getDocCountPerDS(t, ctx, e.esc, ignoreDS...)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		expectedDataStreamsIngest(e.dsNamespace))

	t.Log("check data streams after ingestion")
	dataStreams := getAPMDataStreams(t, ctx, e.esc, ignoreDS...)
	if i.CheckIndividualDataStream != nil {
		expected := formatAllMap(i.CheckIndividualDataStream, e.dsNamespace)
		asserts.CheckDataStreamsIndividually(t, expected, dataStreams)
	} else {
		asserts.CheckDataStreams(t, i.CheckDataStream, dataStreams)
	}

	return testStepResult{DSDocCount: dsDocCount}
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

// upgradeStep upgrades the ECH deployment from its current version to the new
// version. It also adds the new version into testStepEnv. After upgrade, it
// checks that the document counts did not change across upgrade.
//
// The output of this step is the data streams document counts after upgrade.
//
// NOTE: Only works from versions >= 8.0.
type upgradeStep struct {
	// NewVersion is the version to upgrade into.
	NewVersion ecclient.StackVersion

	// CheckDataStream is used to check all data streams have the same expected values.
	// For checking each data stream individually, use CheckIndividualDataStream.
	CheckDataStream asserts.CheckDataStreamsWant

	// CheckIndividualDataStream is used to check the data streams individually
	// instead of as a whole using CheckDataStream.
	// The data stream names can contain '%s' to indicate namespace.
	CheckIndividualDataStream map[string]asserts.CheckDataStreamIndividualWant

	// IgnoreDataStreams are the data streams to be ignored in assertions.
	// The data stream names can contain '%s' to indicate namespace.
	IgnoreDataStreams []string
}

func (u upgradeStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major < 8 {
		t.Fatal("upgrade step should only be used from versions >= 8.0")
	}

	t.Logf("------ upgrade %s to %s ------", e.currentVersion(), u.NewVersion)
	upgradeCluster(t, ctx, e.tf, e.target, u.NewVersion, e.integrations)
	// Update the environment version to the new one.
	e.versions = append(e.versions, u.NewVersion)

	t.Logf("------ upgrade check in %s ------", e.currentVersion())
	t.Log("check number of documents across upgrade")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	ignoreDS := formatAll(u.IgnoreDataStreams, e.dsNamespace)
	dsDocCount := getDocCountPerDS(t, ctx, e.esc, ignoreDS...)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		emptyDataStreamsIngest(e.dsNamespace))

	t.Log("check data streams after upgrade")
	dataStreams := getAPMDataStreams(t, ctx, e.esc, ignoreDS...)
	if u.CheckIndividualDataStream != nil {
		expected := formatAllMap(u.CheckIndividualDataStream, e.dsNamespace)
		asserts.CheckDataStreamsIndividually(t, expected, dataStreams)
	} else {
		asserts.CheckDataStreams(t, u.CheckDataStream, dataStreams)
	}

	return testStepResult{DSDocCount: dsDocCount}
}

// checkErrorLogsStep checks if there are any unexpected error logs from both
// Elasticsearch and APM Server. The provided APMErrorLogsIgnored is used to
// ignore some APM logs from being included in the assertion.
//
// The output of this step is the previous step's result.
type checkErrorLogsStep struct {
	// ESErrorLogsIgnored are the error logs query from Elasticsearch that are
	// to be ignored.
	ESErrorLogsIgnored esErrorLogs

	// APMErrorLogsIgnored are the error logs query from APM Server that are
	// to be ignored.
	APMErrorLogsIgnored apmErrorLogs
}

func (c checkErrorLogsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := e.esc.GetESErrorLogs(ctx, c.ESErrorLogsIgnored.ToQueries()...)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = e.esc.GetAPMErrorLogs(ctx, c.APMErrorLogsIgnored.ToQueries()...)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)

	return previousRes
}

// createReroutePipelineStep creates custom ingest pipelines to reroute logs,
// metrics and traces to different data streams specified by namespace.
type createReroutePipelineStep struct {
	DataStreamNamespace string
}

func (c createReroutePipelineStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("create reroute ingest pipelines")
	for _, pipeline := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		err := e.esc.CreateIngestPipeline(ctx, pipeline, []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{c.DataStreamNamespace},
				},
			},
		})
		require.NoError(t, err)
	}
	e.dsNamespace = c.DataStreamNamespace
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

func allDataStreams(namespace string) []string {
	return slices.Collect(maps.Keys(expectedDataStreamsIngest(namespace)))
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
