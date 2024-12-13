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
	"flag"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

var cleanupOnFailure *bool = flag.Bool("cleanup-on-failure", true, "Whether to run cleanup even if the test failed.")
var target *string = flag.String("target", "production", "The target environment where to run tests againts.")

const testRegion = "aws-eu-west-1"

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
	require.NoError(t, ecAPICheck(t))

	start := time.Now()
	ctx := context.Background()

	// create Elastic Cloud Deployment at version 8.15.4
	t.Log("creating deploment with terraform")
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)
	ecTarget := terraform.Var("ec_target", *target)
	version := terraform.Var("stack_version", "8.15.4")
	name := terraform.Var("name", t.Name())
	require.NoError(t, tf.Apply(ctx, ecTarget, version, name))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	t.Cleanup(func() {
		// cleanup
		if !t.Failed() || (t.Failed() && *cleanupOnFailure) {
			t.Log("cleanup terraform resources")
			require.NoError(t, tf.Destroy(ctx, ecTarget, name, version))
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
	fmt.Printf("%+v\n", escfg)

	// Create APM Key
	t.Log("creating API key")
	ac, err := esclient.New(escfg)
	require.NoError(t, err)

	apikey, err := ac.CreateAPIKey(context.Background(),
		t.Name(), -1, map[string]types.RoleDescriptor{},
	)
	require.NoError(t, err)

	t.Log("ingest data")
	// Ingest data through elastic/apm-perf docker image using apmtelemetrygen
	// This is actually using an extracted telemetrygen as Go package.
	// See https://github.com/elastic/apm-perf/pull/197
	ingest(t, escfg.APMServerURL, apikey)

	// Wait few seconds before proceeding to ensure ES indexed all our documents.
	// In the tests I noticed that the datastream check could fail due to only
	// 4 APM data streams being reported. Manual inspection revealed the
	// correct number of data streams.
	time.Sleep(30 * time.Second)

	oldCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)

	var dss []types.DataStream
	// check data streams
	t.Log("check data streams")
	// GET _data_stream/*apm*
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	checkDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     1,
		IndicesManagedBy: []string{"Data stream lifecycle"},
	}, dss)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// Upgrade to 8.16.0
	// FIXME: the update failed because it took more than 10m
	t.Log("upgrade to 8.16.0")
	require.NoError(t, tf.Apply(ctx, ecTarget, name, terraform.Var("stack_version", "8.16.0")))
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// check data
	newCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)
	assertDocCount(t, oldCount, newCount)

	// check rollover happened
	t.Log("check data streams after upgrade")
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	// NO rollover here, expected? yes with lazy rollover
	checkDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     1,
		IndicesManagedBy: []string{"Data stream lifecycle"},
	}, dss)

	// ingest more
	t.Log("ingest more")
	ingest(t, escfg.APMServerURL, apikey)

	// check rollover
	// Confirm datastreams are
	// v managed by DSL if created after 8.15.0
	// x managed by ILM if created before 8.15.0
	t.Log("check data streams and verity lazy rollover happened")
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	checkDatastreams(t, checkDatastreamWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      "Data stream lifecycle",
		IndicesPerDs:     2,
		IndicesManagedBy: []string{"Data stream lifecycle", "Data stream lifecycle"},
	}, dss)
	t.Logf("time elapsed: %s", time.Now().Sub(start))

	// check ES logs, there should be no errors
	// TODO: how to get these from Elastic Cloud? Is it possible?
}

// assertDocCount asserts that count and datastream names in each ApmDocCount slice are equal.
func assertDocCount(t *testing.T, expected []esclient.ApmDocCount, want []esclient.ApmDocCount) {
	t.Helper()

	assert.Len(t, want, len(expected))
	for i, v := range expected {
		assert.Equal(t, v.Count, want[i].Count)
		assert.Equal(t, v.Count, want[i].Count)
	}
}

// ingest creates and run a telemetrygen that replays multiple APM agents events to the cluster
// a single time.
func ingest(t *testing.T, apmURL string, apikey string) {
	cfg := telemetrygen.DefaultConfig()
	cfg.APIKey = apikey

	u, err := url.Parse(apmURL)
	require.NoError(t, err)
	cfg.ServerURL = u

	cfg.EventRate.Set("1000/s")
	g, err := telemetrygen.New(cfg)
	g.Logger = zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	require.NoError(t, err)

	err = g.RunBlocking(context.Background())
}

type checkDatastreamWant struct {
	Quantity         int
	DSManagedBy      string
	IndicesPerDs     int
	PreferIlm        bool
	IndicesManagedBy []string
}

func checkDatastreams(t *testing.T, expected checkDatastreamWant, actual []types.DataStream) {
	t.Helper()

	require.Len(t, actual, expected.Quantity, "number of APM datastream differs from expectations")
	for _, v := range actual {
		if expected.PreferIlm {
			assert.True(t, v.PreferIlm)
		} else {
			assert.False(t, v.PreferIlm)
		}
		assert.Equal(t, expected.DSManagedBy, v.NextGenerationManagedBy.Name)
		assert.Len(t, v.Indices, expected.IndicesPerDs)

		for i, index := range v.Indices {
			assert.Equal(t, expected.IndicesManagedBy[i], index.ManagedBy.Name)
		}
	}

}
