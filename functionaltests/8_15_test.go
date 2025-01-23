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

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
	ecAPICheck(t)

	start := time.Now()
	ctx := context.Background()

	t.Log("creating deployment with terraform")
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)
	ecTarget := terraform.Var("ec_target", *target)
	ecRegion := terraform.Var("ec_region", regionFrom(*target))
	version := terraform.Var("stack_version", "8.15.4")
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

	var escfg esclient.Config
	err = tf.Output("apm_url", &escfg.APMServerURL)
	require.NoError(t, err)
	err = tf.Output("es_url", &escfg.ElasticsearchURL)
	require.NoError(t, err)
	err = tf.Output("username", &escfg.Username)
	require.NoError(t, err)
	err = tf.Output("password", &escfg.Password)
	require.NoError(t, err)
	err = tf.Output("kb_url", &escfg.KibanaURL)
	require.NoError(t, err)

	t.Logf("created deployment %s", escfg.KibanaURL)

	ecc, err := esclient.New(escfg)
	require.NoError(t, err)

	t.Log("creating APM API key")
	apikey, err := ecc.CreateAPMAPIKey(ctx, t.Name())
	require.NoError(t, err)

	g := gen.New(escfg.APMServerURL, apikey)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	previous, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)

	g.RunBlockingWait(ctx, ecc, expectedIngestForASingleRun(), previous, 1*time.Minute)

	beforeUpgradeCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, beforeUpgradeCount, previous, expectedIngestForASingleRun())

	t.Log("check data streams")
	var dss []types.DataStream
	dss, err = ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     1,
		IndicesManagedBy: []string{"Data stream lifecycle"},
	}, dss)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Log("upgrade to 8.16.0")
	require.NoError(t, tf.Apply(ctx, ecTarget, ecRegion, name, terraform.Var("stack_version", "8.16.0")))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

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
	assertDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     1,
		IndicesManagedBy: []string{"Data stream lifecycle"},
	}, dss)

	g.RunBlockingWait(ctx, ecc, expectedIngestForASingleRun(), previous, 1*time.Minute)

	t.Log("check number of documents")
	afterUpgradeIngestionCount, err := getDocsCountPerDS(t, ctx, ecc)
	require.NoError(t, err)
	assertDocCount(t, afterUpgradeIngestionCount, afterUpgradeCount, expectedIngestForASingleRun())

	// Confirm datastreams are
	// v managed by DSL if created after 8.15.0
	// x managed by ILM if created before 8.15.0
	t.Log("check data streams and verify lazy rollover happened")
	dss2, err := ecc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     2,
		IndicesManagedBy: []string{"Data stream lifecycle", "Data stream lifecycle"},
	}, dss2)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	res, err := ecc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	assert.Zero(t, res.Hits.Total.Value)
}
