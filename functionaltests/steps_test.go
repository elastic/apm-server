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
	namespace    string
	startVersion ecclient.StackVersion
	steps        []testStep
}

func (r testStepsRunner) Run(t *testing.T) {
	if r.namespace == "" {
		r.namespace = "default"
	}

	start := time.Now()
	ctx := context.Background()
	tf := initTerraformRunner(t)

	t.Logf("------ cluster setup %s ------", r.startVersion.String())
	esCfg := createCluster(t, ctx, tf, *target, r.startVersion.String())
	esc := createESClient(t, esCfg)
	kbc := createKibanaClient(t, ctx, esc, esCfg)
	g := createAPMGenerator(t, ctx, esc, esCfg)
	t.Logf("time elapsed: %s", time.Since(start))

	env := testStepEnv{
		namespace: r.namespace,
		version:   r.startVersion,
		tf:        tf,
		gen:       g,
		kbc:       kbc,
		esc:       esc,
	}

	docCounts, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)

	currentRes := testStepResult{DocCounts: docCounts}
	for _, step := range r.steps {
		currentRes = step.Step(t, ctx, &env, currentRes)
		t.Logf("time elapsed: %s", time.Since(start))
	}
}

type testStepResult struct {
	DocCounts esclient.DataStreamsDocCount
}

type testStepEnv struct {
	namespace string
	version   ecclient.StackVersion
	tf        *terraform.Runner
	gen       *gen.Generator
	kbc       *kbclient.Client
	esc       *esclient.Client
}

type testStep interface {
	Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult
}

type ingestionStep struct {
	checkDataStream checkDataStreamWant
}

var _ testStep = ingestionStep{}

func (i ingestionStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ ingestion ------")
	err := e.gen.RunBlockingWait(ctx, e.kbc, e.version.String())
	require.NoError(t, err)

	t.Log("------ ingestion check ------")
	t.Log("check number of documents after ingestion")
	docCounts, err := getDocsCountPerDS(t, ctx, e.esc)
	require.NoError(t, err)
	assertDocCount(t, docCounts, previousRes.DocCounts,
		expectedIngestForASingleRun(e.namespace),
		aggregationDataStreams(e.namespace))

	t.Log("check data streams after ingestion")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, i.checkDataStream, dss)

	return testStepResult{DocCounts: docCounts}
}

type upgradeStep struct {
	newVersion      ecclient.StackVersion
	checkDataStream checkDataStreamWant
}

var _ testStep = upgradeStep{}

func (u upgradeStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Logf("------ upgrade from %s to %s ------", e.version.String(), u.newVersion.String())
	upgradeCluster(t, ctx, e.tf, *target, u.newVersion.String())
	e.version = u.newVersion

	t.Log("------ upgrade check ------")
	t.Log("check number of documents across upgrade")
	docCounts, err := getDocsCountPerDS(t, ctx, e.esc)
	require.NoError(t, err)
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	assertDocCount(t, docCounts, previousRes.DocCounts,
		emptyIngestForASingleRun(e.namespace),
		aggregationDataStreams(e.namespace))

	t.Log("check data streams after upgrade")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, u.checkDataStream, dss)

	return testStepResult{DocCounts: docCounts}
}

type checkErrorLogsStep struct {
	apmErrorLogsIgnored []types.Query
}

var _ testStep = checkErrorLogsStep{}

func (c checkErrorLogsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := e.esc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = e.esc.GetAPMErrorLogs(ctx, c.apmErrorLogsIgnored)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)

	return previousRes
}
