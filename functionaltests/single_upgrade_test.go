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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

type dependencies struct {
	ESClient     *esclient.Client
	KibanaClient *kbclient.Client
}

func newDependencies(esc *esclient.Client, kbc *kbclient.Client) dependencies {
	return dependencies{ESClient: esc, KibanaClient: kbc}
}

type config struct {
	// DSNamespace is the data stream namespace to be used for assertions.
	DSNamespace string
}

func defaultConfig() config {
	return config{
		DSNamespace: defaultNamespace,
	}
}

type setupFunc func(t *testing.T, ctx context.Context, cfg *config, deps dependencies) (continueTest bool)

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
	fromVersion string
	toVersion   string

	preUpgradeBeforeIngestSetup  setupFunc
	checkPreUpgradeAfterIngest   checkDatastreamWant
	checkPostUpgradeBeforeIngest checkDatastreamWant
	postUpgradeBeforeIngestSetup setupFunc
	checkPostUpgradeAfterIngest  checkDatastreamWant
}

func (tt singleUpgradeTestCase) Run(t *testing.T) {
	cfg := defaultConfig()
	start := time.Now()
	ctx := context.Background()
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)

	/* Cluster setup */
	deploymentID, escfg := createCluster(t, ctx, tf, *target, tt.fromVersion)
	t.Logf("time elapsed: %s", time.Since(start))

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	kbc := createKibanaClient(t, ctx, ecc, escfg)
	deps := newDependencies(ecc, kbc)

	t.Log("create APM API key")
	apiKey := createAPMAPIKey(t, ctx, ecc)

	g := gen.New(escfg.APMServerURL, apiKey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	if tt.preUpgradeBeforeIngestSetup != nil {
		t.Log("------ pre-upgrade before ingest setup ------")
		if !tt.preUpgradeBeforeIngestSetup(t, ctx, &cfg, deps) {
			assert.Fail(t, "pre-ingestion setup failed")
			return
		}
	}

	t.Log("------ pre-upgrade ingestion ------")
	require.NoError(t, g.RunBlockingWait(ctx, kbc, deploymentID))
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ pre-upgrade ingestion assertions ------")
	t.Log("check number of documents after initial ingestion")
	atStartCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, atStartCount, previous, expectedIngestForASingleRun(cfg.DSNamespace))

	t.Log("check data streams after initial ingestion")
	dss, err := ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkPreUpgradeAfterIngest, dss)
	t.Logf("time elapsed: %s", time.Since(start))

	beforeUpgradeCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	t.Log("------ perform upgrade ------")
	upgradeCluster(t, ctx, tf, *target, tt.toVersion)
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade assertions ------")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change before running the next ingestion round
	// and further assertions.
	// We don't expect any change here unless something broke during the upgrade.
	t.Log("check number of documents across upgrade")
	assertDocCount(t, beforeUpgradeCount, esclient.APMDataStreamsDocCount{}, atStartCount)

	t.Log("check data streams after upgrade")
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkPostUpgradeBeforeIngest, dss)

	if tt.postUpgradeBeforeIngestSetup != nil {
		t.Log("------ post-upgrade before ingest setup ------")
		if !tt.postUpgradeBeforeIngestSetup(t, ctx, &cfg, deps) {
			assert.Fail(t, "post-upgrade before ingest setup failed")
			return
		}
	}

	t.Log("------ post-upgrade ingestion ------")
	require.NoError(t, g.RunBlockingWait(ctx, kbc, deploymentID))
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ post-upgrade ingestion assertions ------")
	t.Log("check number of documents after final ingestion")
	afterUpgradeIngestionCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, afterUpgradeIngestionCount, beforeUpgradeCount, expectedIngestForASingleRun(cfg.DSNamespace))

	t.Log("check data streams after final ingestion")
	dss2, err := ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tt.checkPostUpgradeAfterIngest, dss2)
	t.Logf("time elapsed: %s", time.Since(start))

	t.Log("------ check ES and APM error logs ------")
	resp, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)
}
