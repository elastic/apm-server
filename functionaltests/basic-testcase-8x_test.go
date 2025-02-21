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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// basicUpgradeTestCase8x is a basic functional test case that performs a
// cluster upgrade between 2 specified versions.
//
// The cluster is created, some data is ingested and the first
// check is run to ensure it's in a known state.
// Then an upgrade is triggered and once completed a second check
// is run, to confirm the state did not drift after upgrade.
// A new ingestion is performed and a final check is run, to
// verify that ingestion works after upgrade and brings the cluster
// to a know state.
type basicUpgradeTestCase8x struct {
	fromVersion string
	toVersion   string

	checkAfterIngestBeforeUpgrade checkDatastreamWant
	checkAfterUpgradeBeforeIngest checkDatastreamWant
	checkAfterUpgradeAfterIngest  checkDatastreamWant
}

func (tt basicUpgradeTestCase8x) Run(t *testing.T) {
	start := time.Now()
	ctx := context.Background()
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)

	deploymentID, escfg := createCluster(t, ctx, tf, *target, tt.fromVersion)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	kbc := createKibanaClient(t, ctx, ecc, escfg)

	t.Log("creating APM API key")
	apikey, err := ecc.CreateAPMAPIKey(ctx, t.Name())
	require.NoError(t, err)

	g := gen.New(escfg.APMServerURL, apikey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	require.NoError(t, g.RunBlockingWait(ctx, kbc, deploymentID))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("check number of documents after initial ingestion")
	atStartCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, atStartCount, previous, expectedIngestForASingleRun())

	t.Log("check data streams after initial ingestion")
	var dss []types.DataStream
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkAfterIngestBeforeUpgrade, dss)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	beforeUpgradeCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	upgradeCluster(t, ctx, tf, *target, tt.toVersion)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change before running the next ingestion round
	// and further assertions.
	// We don't expect any change here unless something broke during the upgrade.
	t.Log("check number of documents across upgrade")
	assertDocCount(t, beforeUpgradeCount, esclient.APMDataStreamsDocCount{}, atStartCount)

	t.Log("check data streams after upgrade")
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkAfterUpgradeBeforeIngest, dss)

	require.NoError(t, g.RunBlockingWait(ctx, kbc, deploymentID))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("check number of documents after final ingestion")
	afterUpgradeIngestionCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, afterUpgradeIngestionCount, beforeUpgradeCount, expectedIngestForASingleRun())

	t.Log("check data streams after final ingestion")
	dss2, err := ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkAfterUpgradeAfterIngest, dss2)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	resp, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

}
