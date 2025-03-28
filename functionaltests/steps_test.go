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

type testStepsRunner struct {
	DataStreamNamespace string
	Steps               []testStep
}

func (r testStepsRunner) Run(t *testing.T) {
	if r.DataStreamNamespace == "" {
		r.DataStreamNamespace = "default"
	}

	start := time.Now()
	ctx := context.Background()

	env := testStepEnv{dsNamespace: r.DataStreamNamespace}
	currentRes := testStepResult{}
	for _, step := range r.Steps {
		currentRes = step.Step(t, ctx, &env, currentRes)
		t.Logf("time elapsed: %s", time.Since(start))
	}
}

type testStepResult struct {
	DSDocCount esclient.DataStreamsDocCount
}

type testStepEnv struct {
	dsNamespace  string
	version      ecclient.StackVersion
	integrations bool
	tf           *terraform.Runner
	gen          *gen.Generator
	kbc          *kbclient.Client
	esc          *esclient.Client
}

type testStep interface {
	Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult
}

type createStep struct {
	DeployVersion      ecclient.StackVersion
	EnableIntegrations bool
}

func (c createStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, _ testStepResult) testStepResult {
	t.Log("------ cluster setup ------")
	e.tf = initTerraformRunner(t)
	deployInfo := createCluster(t, ctx, e.tf, *target, c.DeployVersion, c.EnableIntegrations)
	e.esc = createESClient(t, deployInfo)
	e.kbc = createKibanaClient(t, ctx, e.esc, deployInfo)
	e.gen = createAPMGenerator(t, ctx, e.esc, e.kbc, deployInfo)

	docCount := getDocCountPerDS(t, ctx, e.esc)
	return testStepResult{DSDocCount: docCount}
}

var _ testStep = ingestionStep{}

type ingestionStep struct {
	CheckDataStream asserts.CheckDataStreamsWant
}

var _ testStep = ingestionStep{}

func (i ingestionStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ ingestion ------")
	err := e.gen.RunBlockingWait(ctx, e.version, e.integrations)
	require.NoError(t, err)

	t.Log("------ ingestion check ------")
	t.Log("check number of documents after ingestion")
	docCount := getDocCountPerDS(t, ctx, e.esc)
	asserts.CheckDocCount(t, docCount, previousRes.DSDocCount,
		expectedIngestForASingleRun(e.dsNamespace),
		aggregationDataStreams(e.dsNamespace))

	t.Log("check data streams after ingestion")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	asserts.CheckDataStreams(t, i.CheckDataStream, dss)

	return testStepResult{DSDocCount: docCount}
}

type upgradeStep struct {
	NewVersion      ecclient.StackVersion
	CheckDataStream asserts.CheckDataStreamsWant
}

var _ testStep = upgradeStep{}

func (u upgradeStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ upgrade ------")
	upgradeCluster(t, ctx, e.tf, *target, u.NewVersion, e.integrations)
	e.version = u.NewVersion

	t.Log("------ upgrade check ------")
	t.Log("check number of documents across upgrade")
	docCount := getDocCountPerDS(t, ctx, e.esc)
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	asserts.CheckDocCount(t, docCount, previousRes.DSDocCount,
		emptyIngestForASingleRun(e.dsNamespace),
		aggregationDataStreams(e.dsNamespace))

	t.Log("check data streams after upgrade")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	asserts.CheckDataStreams(t, u.CheckDataStream, dss)

	return testStepResult{DSDocCount: docCount}
}

type checkErrorLogsStep struct {
	APMErrorLogsIgnored []types.Query
}

var _ testStep = checkErrorLogsStep{}

func (c checkErrorLogsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := e.esc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = e.esc.GetAPMErrorLogs(ctx, c.APMErrorLogsIgnored)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)

	return previousRes
}

type stepFunc func(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult

type customStep struct {
	Func stepFunc
}

var _ testStep = customStep{}

func (c customStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	return c.Func(t, ctx, e, previousRes)
}
