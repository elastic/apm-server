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

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type additionalFn func(*esclient.Client, esclient.Config) error

type essUpgradeTestCase struct {
	from string
	to   string

	// setupFn allows to specify custom logic to happen after test cluster
	// boostrap and before any ingestion happens.
	setupFn additionalFn
	// afterUpgrade allows to specify custom logic to happen after test cluster
	// has been upgraded to `to` version and before any further check or ingestion
	// happens.
	afterUpgrade additionalFn

	beforeUpgradeAfterIngest checkDatastreamWant
	afterUpgradeBeforeIngest checkDatastreamWant
	afterUpgradeAfterIngest  checkDatastreamWant
}

// runESSUpgradeTest runs a ESS upgrade test.
func runESSUpgradeTest(t *testing.T, tc essUpgradeTestCase) {
	start := time.Now()
	ctx := context.Background()

	t.Log("creating deploment with terraform")
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)
	ecTarget := terraform.Var("ec_target", *target)
	ecRegion := terraform.Var("ec_region", regionFrom(*target))
	version := terraform.Var("stack_version", tc.from)
	name := terraform.Var("name", t.Name())
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, version, name))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Cleanup(func() {
		if !t.Failed() || (t.Failed() && *cleanupOnFailure) {
			t.Log("cleanup terraform resources")
			require.NoError(t, tf.Destroy(ctx, ecTarget, ecRegion, name, version))
		} else {
			t.Log("test failed and cleanup-on-failure is false, skipping cleanup")
		}
	})

	var deploymentID string
	require.NoError(t, tf.Output("deployment_id", &deploymentID))
	var escfg esclient.Config
	tf.Output("apm_url", &escfg.APMServerURL)
	tf.Output("es_url", &escfg.ElasticsearchURL)
	tf.Output("username", &escfg.Username)
	tf.Output("password", &escfg.Password)
	tf.Output("kb_url", &escfg.KibanaURL)

	t.Logf("created deployment %s", escfg.KibanaURL)

	c, err := ecclient.New(endpointFrom(*target))
	require.NoError(t, err)

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	t.Log("create API key for Kibana")
	kbApikey, err := ecc.CreateAPIKey(ctx, t.Name(), 0, map[string]types.RoleDescriptor{})
	require.NoError(t, err)
	kbc := kbclient.New(escfg.KibanaURL, kbApikey)

	// execute additional setup code
	if tc.setupFn != nil {
		err := tc.setupFn(ecc, escfg)
		require.NoError(t, err, "error executing custom setup logic")
	}

	t.Log("creating APM API key")
	apikey, err := ecc.CreateAPMAPIKey(ctx, t.Name())
	require.NoError(t, err)

	g := gen.New(escfg.APMServerURL, apikey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	require.NoError(t, g.RunBlockingWait(ctx, c, deploymentID))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	beforeUpgradeCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, beforeUpgradeCount, previous, expectedIngestForASingleRun())

	t.Log("check data streams")
	var dss []types.DataStream
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tc.beforeUpgradeAfterIngest, dss)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("upgrade to 8.16.0")
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, name, terraform.Var("stack_version", tc.to)))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// execute additional setup code
	if tc.afterUpgrade != nil {
		err := tc.afterUpgrade(ecc, escfg)
		require.NoError(t, err, "error executing after upgrade custom logic")
	}

	t.Log("check number of documents after upgrade")
	afterUpgradeCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change before running the next ingestion round
	// and further assertions.
	// We don't expect any change here unless something broke during the upgrade.
	assertDocCount(t, afterUpgradeCount, esclient.APMDataStreamsDocCount{}, beforeUpgradeCount)

	t.Log("check data streams after upgrade, no rollover expected")
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tc.afterUpgradeBeforeIngest, dss)

	require.NoError(t, g.RunBlockingWait(ctx, c, deploymentID))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("check number of documents")
	afterUpgradeIngestionCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, afterUpgradeIngestionCount, afterUpgradeCount, expectedIngestForASingleRun())

	t.Log("check data streams and verify lazy rollover happened")
	dss2, err := ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, tc.afterUpgradeAfterIngest, dss2)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	res, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	if !assert.Zero(t, res.Hits.Total.Value, "expected no error logs, but found some") {
		t.Log("found error logs:")
		for _, h := range res.Hits.Hits {
			t.Log(string(h.Source_))
		}
	}
}
