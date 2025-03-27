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
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

type additionalFunc func(t *testing.T, ctx context.Context, esc *esclient.Client, kbc *kbclient.Client) error

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
	if mode == apmDefault || mode == apmManaged {
		return true
	}
	return false
}

// singleUpgradeTestCase is a basic functional test case that performs a
// cluster upgrade between 2 specified versions.
//
// The cluster is created, some data is ingested and the first
// check is run to ensure it's in a known state.
// Then an upgrade is triggered and once completed a second check
// is run, to confirm the state did not drift after upgrade.
// A new ingestion is performed and a final check is run, to
// verify that ingestion works after upgrade and brings the cluster
// to a know state.
type singleUpgradeTestCase struct {
	fromVersion ecclient.StackVersion
	toVersion   ecclient.StackVersion
	// apmDeployMode determines whether to deploy APM in
	// managed mode (default) as opposed to standalone
	apmDeployMode apmDeploymentMode

	dataStreamNamespace          string
	setupFn                      additionalFunc
	checkPreUpgradeAfterIngest   checkDataStreamWant
	postUpgradeFn                additionalFunc
	checkPostUpgradeBeforeIngest checkDataStreamWant
	checkPostUpgradeAfterIngest  checkDataStreamWant

	// apmErrorLogsIgnored are the error logs to be ignored when
	// checking for existence of errors in the upgrade test.
	apmErrorLogsIgnored []types.Query
}

func (tt singleUpgradeTestCase) Run(t *testing.T) {
	integrations := tt.apmDeployMode.enableIntegrations()
	if tt.dataStreamNamespace == "" {
		tt.dataStreamNamespace = "default"
	}

	start := time.Now()
	ctx := context.Background()
	copyTerraforms(t)
	tf, err := terraform.New(t, terraformDir(t))
	require.NoError(t, err)

	t.Log("------ cluster setup ------")
	deployInfo := createCluster(t, ctx, tf, *target, tt.fromVersion.String(), integrations)
	t.Logf("time elapsed: %s", time.Since(start))

	esc := createESClient(t, deployInfo)
	kbc := createKibanaClient(t, ctx, esc, deployInfo)
	g := createAPMGenerator(t, ctx, esc, deployInfo)

	atStartCount, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)

	if tt.setupFn != nil {
		t.Log("------ custom setup ------")
		err = tt.setupFn(t, ctx, esc, kbc)
		require.NoError(t, err, "custom setup failed")
	}

	t.Log("------ pre-upgrade ingestion ------")
	require.NoError(t, g.RunBlockingWait(ctx, kbc, tt.fromVersion.String()))
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ pre-upgrade ingestion assertions ------")
	t.Log("check number of documents after initial ingestion")
	firstIngestCount, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)
	assertDocCount(t, firstIngestCount, atStartCount,
		expectedIngestForASingleRun(tt.dataStreamNamespace),
		aggregationDataStreams(tt.dataStreamNamespace))

	t.Log("check data streams after initial ingestion")
	dss, err := esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, tt.checkPreUpgradeAfterIngest, dss)
	t.Logf("time elapsed: %s", time.Since(start))

	beforeUpgradeCount, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)

	t.Log("------ perform upgrade ------")
	upgradeCluster(t, ctx, tf, *target, tt.toVersion.String(), integrations)
	t.Logf("time elapsed: %s", time.Since(start))

	if tt.postUpgradeFn != nil {
		t.Log("------ custom post-upgrade ------")
		err = tt.postUpgradeFn(t, ctx, esc, kbc)
		require.NoError(t, err, "custom post-upgrade failed")
	}

	t.Log("------ post-upgrade assertions ------")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change before running the next ingestion round
	// and further assertions.
	// We don't expect any change here unless something broke during the upgrade.
	t.Log("check number of documents across upgrade")
	afterUpgradeCount, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)
	assertDocCount(t, afterUpgradeCount, beforeUpgradeCount,
		emptyIngestForASingleRun(tt.dataStreamNamespace),
		aggregationDataStreams(tt.dataStreamNamespace))

	t.Log("check data streams after upgrade")
	dss, err = esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, tt.checkPostUpgradeBeforeIngest, dss)

	t.Log("------ post-upgrade ingestion ------")
	require.NoError(t, g.RunBlockingWait(ctx, kbc, tt.toVersion.String()))
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade ingestion assertions ------")
	t.Log("check number of documents after final ingestion")
	secondIngestCount, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)
	assertDocCount(t, secondIngestCount, afterUpgradeCount,
		expectedIngestForASingleRun(tt.dataStreamNamespace),
		aggregationDataStreams(tt.dataStreamNamespace))

	t.Log("check data streams after final ingestion")
	dss2, err := esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, tt.checkPostUpgradeAfterIngest, dss2)
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := esc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = esc.GetAPMErrorLogs(ctx, tt.apmErrorLogsIgnored)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)
}
