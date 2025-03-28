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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

func expectedStandaloneNoIngestion() esclient.IndicesDocCount {
	return esclient.IndicesDocCount{
		"apm-*-error-*":       0,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        0,
		"apm-*-transaction-*": 0,
		"apm-*-metric-*":      -1, // doc count is probably different than previous count
		"apm-*-onboarding-*":  0,
	}
}

func expectedStandaloneIngestion() esclient.IndicesDocCount {
	return esclient.IndicesDocCount{
		"apm-*-error-*":       364,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        10885,
		"apm-*-transaction-*": 4128,
		// we ignore metric doc count because it contains aggregations
		"apm-*-metric-*":     -1,
		"apm-*-onboarding-*": -1,
	}
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
type singleUpgrade7xTestCase struct {
	fromVersion ecclient.StackVersion
	toVersion   ecclient.StackVersion

	preIngestionSetup            func(*testing.T, *esclient.Client, *kbclient.Client) bool
	checkPreUpgradeAfterIngest   checkDataStreamWant
	checkPostUpgradeBeforeIngest checkDataStreamWant
	checkPostUpgradeAfterIngest  checkDataStreamWant
}

func (tt singleUpgrade7xTestCase) Run(t *testing.T) {
	// This test expects to start from a 7.x stack version.
	if !tt.fromVersion.IsMajor(7) {
		t.Fatalf("fromVersion should be a 7.x version, but %s found", tt.fromVersion)
	}

	start := time.Now()
	defer func() { t.Logf("time elapsed: %s", time.Since(start)) }()
	ctx := context.Background()

	copyTerraforms(t)
	tf, err := terraform.New(t, terraformDir(t))
	require.NoError(t, err)

	deploymentID, escfg := createCluster(t, ctx, tf, *target, tt.fromVersion.String(), false)
	t.Logf("time elapsed: %s", time.Since(start))
	t.Log(deploymentID)

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	kbc := createKibanaClient(t, ctx, ecc, escfg)

	t.Log("create APM API key")
	apiKey := createAPMAPIKey(t, ctx, ecc)

	g := gen.NewV7(escfg.APMServerURL, apiKey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := ecc.APMDocCountV7(ctx)
	require.NoError(t, err)

	if tt.preIngestionSetup != nil {
		t.Log("------ setup ------")
		if !tt.preIngestionSetup(t, ecc, kbc) {
			assert.Fail(t, "pre-ingestion setup failed")
			return
		}
	}

	// t.Log("migrating to Integrations server")
	// require.NoError(t, kbc.MigrateAPMToIntegrationsServer(escfg.Password))
	// t.Logf("time elapsed: %s", time.Now().Sub(start))

	require.NoError(t, g.RunBlocking(ctx))
	time.Sleep(15 * time.Second)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	beforeUpgradeCount, err := ecc.APMDocCountV7(ctx)
	require.NoError(t, err)
	// asserts.StandaloneDocsCount(t, beforeUpgradeCount, expectedStandaloneIngestion(), previous)
	fmt.Println(beforeUpgradeCount, expectedStandaloneIngestion(), previous)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("------ perform upgrade ------")
	upgradeCluster(t, ctx, tf, *target, tt.toVersion.String(), false)
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade assertions ------")

	t.Log("check number of documents across upgrade")
	if tt.toVersion.IsMajor(7) {
		afterUpgradeCount, err := ecc.APMDocCountV7(ctx)
		require.NoError(t, err)
		// asserts.StandaloneDocsCount(t, afterUpgradeCount, expectedStandaloneNoIngestion(), beforeUpgradeCount)
		fmt.Println(afterUpgradeCount, expectedStandaloneNoIngestion(), beforeUpgradeCount)
		// t.Log("check indices after upgrade")
		// TODO: what checks do we want to run on standalone indices?

		t.Log("------ post-upgrade ingestion ------")
		require.NoError(t, g.RunBlocking(ctx))
		t.Logf("time elapsed: %s", time.Since(start))

		t.Log("------ post-upgrade ingestion assertions ------")
		t.Log("check number of documents after final ingestion")
		afterUpgradeIngestionCount, err := ecc.APMDocCountV7(ctx)
		require.NoError(t, err)
		// asserts.StandaloneDocsCount(t, afterUpgradeIngestionCount, expectedStandaloneIngestion(), beforeUpgradeCount)
		fmt.Println(afterUpgradeIngestionCount, expectedStandaloneIngestion(), afterUpgradeCount)

		// t.Log("check indices after final ingestion")
		// TODO: what checks do we want to run on standalone indices?
	} else {
		afterUpgradeCount, err := ecc.APMDocCount(ctx)
		require.NoError(t, err)

		// Use default generator if not v7
		g = gen.New(escfg.APMServerURL, apiKey)
		g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

		t.Log("------ post-upgrade ingestion ------")
		require.NoError(t, g.RunBlocking(ctx))
		t.Logf("time elapsed: %s", time.Since(start))

		t.Log("------ post-upgrade ingestion assertions ------")
		t.Log("check number of documents after final ingestion")
		afterUpgradeIngestionCount, err := ecc.APMDocCount(ctx)
		require.NoError(t, err)
		assertDocCount(t, afterUpgradeIngestionCount, afterUpgradeCount,
			expectedIngestForASingleRun("default"),
			aggregationDataStreams("default"),
		)
	}

	t.Log("------ check ES and APM error logs ------")
	resp, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)
}
