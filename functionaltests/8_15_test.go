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

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/stretchr/testify/require"
)

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
	ecAPICheck(t)

	start := time.Now()
	ctx := context.Background()

	t.Log("creating deploment with terraform")
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

	var deploymentID string
	var escfg esclient.Config
	tf.Output("deployment_id", &deploymentID)
	tf.Output("apm_url", &escfg.APMServerURL)
	tf.Output("es_url", &escfg.ElasticsearchURL)
	tf.Output("username", &escfg.Username)
	tf.Output("password", &escfg.Password)
	tf.Output("kb_url", &escfg.KibanaURL)

	t.Logf("created deployment %s", deploymentID)

	ac, err := esclient.New(escfg)
	require.NoError(t, err)

	t.Log("creating APM API key")
	apikey, err := ac.CreateAPMAPIKey(ctx, t.Name())
	require.NoError(t, err)

	ingest(t, escfg.APMServerURL, apikey)

	// Wait few seconds before proceeding to ensure ES indexed all our documents.
	// Manual tests had failures due to only 4 data streams being reported
	// when no delay was used. Manual inspection always revealed the correct
	// number of data streams.
	time.Sleep(1 * time.Minute)

	oldCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)

	t.Log("check data streams")
	var dss []types.DataStream
	dss, err = ac.GetDataStream(ctx, "*apm*")
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

	t.Log("check number of documents")
	newCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)
	assertDocCountEqual(t, oldCount, newCount)

	t.Log("check data streams after upgrade, no rollover expected")
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     1,
		IndicesManagedBy: []string{"Data stream lifecycle"},
	}, dss)

	ingest(t, escfg.APMServerURL, apikey)
	time.Sleep(1 * time.Minute)

	t.Log("check number of documents")
	newCount2, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)
	assertDocCountGreaterThan(t, oldCount, newCount2)

	// Confirm datastreams are
	// v managed by DSL if created after 8.15.0
	// x managed by ILM if created before 8.15.0
	t.Log("check data streams and verify lazy rollover happened")
	dss2, err := ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     2,
		IndicesManagedBy: []string{"Data stream lifecycle", "Data stream lifecycle"},
	}, dss2)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// check ES logs, there should be no errors
	// TODO: how to get these from Elastic Cloud? Is it possible?
}
