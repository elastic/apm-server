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

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/stretchr/testify/require"
)

func expectedStandaloneNoIngestion() esclient.APMIndexDocCount {
	return esclient.APMIndexDocCount{
		"apm-*-error-*":       0,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        0,
		"apm-*-transaction-*": 0,
		"apm-*-metric-*":      -1, // doc count is probably different than previous count
		"apm-*-onboarding-*":  0,
	}
}

func expectedStandaloneIngestion() esclient.APMIndexDocCount {
	return esclient.APMIndexDocCount{
		"apm-*-error-*":       364,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        10885,
		"apm-*-transaction-*": 4128,
		// we ignore metric doc count because it contains aggregations
		"apm-*-metric-*":     -1,
		"apm-*-onboarding-*": -1,
	}
}

func TestUpgrade_7_17_28_to_8_17_4(t *testing.T) {
	ecAPICheck(t)

	tt := singleUpgradeTestCase{
		fromVersion: "7.17.28",
		toVersion:   "8.17.3",

		checkPreUpgradeAfterIngest:   checkDatastreamWant{},
		checkPostUpgradeBeforeIngest: checkDatastreamWant{},
		checkPostUpgradeAfterIngest:  checkDatastreamWant{},
	}

	start := time.Now()
	defer func() { t.Logf("time elapsed: %s", time.Since(start)) }()
	ctx := context.Background()

	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)

	deploymentID, escfg := createCluster(t, ctx, tf, *target, tt.fromVersion, false)
	t.Logf("time elapsed: %s", time.Since(start))
	t.Log(deploymentID)

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	// kbc := createKibanaClient(t, ctx, ecc, escfg)

	t.Log("create APM API key")
	apiKey := createAPMAPIKey(t, ctx, ecc)

	g := gen.NewV7(escfg.APMServerURL, apiKey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := ecc.ApmDocCountV7(ctx)
	require.NoError(t, err)

	// t.Log("migrating to Integrations server")
	// require.NoError(t, kbc.MigrateAPMToIntegrationsServer(escfg.Password))
	// t.Logf("time elapsed: %s", time.Now().Sub(start))

	require.NoError(t, g.RunBlocking(ctx))
	time.Sleep(15 * time.Second)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	beforeUpgradeCount, err := ecc.ApmDocCountV7(ctx)
	require.NoError(t, err)
	asserts.StandaloneDocsCount(t, beforeUpgradeCount, expectedStandaloneIngestion(), previous)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("------ perform upgrade ------")
	upgradeCluster(t, ctx, tf, *target, tt.toVersion, false)
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade assertions ------")

	t.Log("check number of documents across upgrade")
	afterUpgradeCount, err := ecc.ApmDocCountV7(ctx)
	require.NoError(t, err)
	asserts.StandaloneDocsCount(t, afterUpgradeCount, expectedStandaloneNoIngestion(), beforeUpgradeCount)

	// t.Log("check indices after upgrade")
	// TODO: what checks do we want to run on standalone indices?

	t.Log("------ post-upgrade ingestion ------")
	require.NoError(t, g.RunBlocking(ctx))
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade ingestion assertions ------")
	t.Log("check number of documents after final ingestion")
	afterUpgradeIngestionCount, err := ecc.ApmDocCountV7(ctx)
	require.NoError(t, err)
	asserts.StandaloneDocsCount(t, afterUpgradeIngestionCount, expectedStandaloneIngestion(), beforeUpgradeCount)

	// t.Log("check indices after final ingestion")
	// TODO: what checks do we want to run on standalone indices?

	t.Log("------ check ES and APM error logs ------")
	resp, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)
}
