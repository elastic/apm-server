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
	"net/url"
	"testing"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const testRegion = "aws-eu-west-1"

const (
	ecAPIEndpoint   = "api.elastic-cloud.com"
	ecAPIEndpointQA = "https://public-api.qa.cld.elstc.co"
)

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
	require.NoError(t, ecAPICheck(t))

	ctx := context.Background()

	// create Elastic Cloud Deployment at version 8.15.4
	t.Log("creating deploment with terraform")
	tf, err := terraform.New(t, t.Name())
	require.NoError(t, err)
	version := tfexec.Var(fmt.Sprintf("stack_version=%s", "8.15.4"))
	require.NoError(t, tf.Apply(ctx, version))

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

	oldCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)

	var dss []types.DataStream
	// check data streams
	t.Log("check data streams")
	// GET _data_stream/*apm*
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)

	require.Len(t, dss, 8)
	for _, v := range dss {
		assert.False(t, v.PreferIlm)
		assert.Equal(t, "Data stream lifecycle", v.NextGenerationManagedBy.Name)

		assert.Len(t, v.Indices, 1)
		assert.Equal(t, "Data stream lifecycle", v.Indices[0].ManagedBy.Name)
	}

	// Upgrade to 8.16.0
	// FIXME: the update failed because it took more than 10m
	t.Log("upgrade to 8.16.0")
	require.NoError(t, tf.Apply(context.Background(),
		tfexec.Var(fmt.Sprintf("stack_version=%s", "8.16.0"))))

	// check data
	newCount, err := ac.ApmDocCount(ctx)
	require.NoError(t, err)

	fmt.Printf("%+v\n", newCount)
	assertDocCount(t, oldCount, newCount)

	// check rollover happened
	t.Log("check data streams")
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)

	require.Len(t, dss, 8)
	for _, v := range dss {
		assert.False(t, v.PreferIlm)
		assert.Equal(t, "Data stream lifecycle", v.NextGenerationManagedBy.Name)

		// NO rollover here, expected?
		// assert.Len(t, v.Indices, 2)
		assert.Equal(t, "Data stream lifecycle", v.Indices[0].ManagedBy.Name)
	}

	// ingest more
	t.Log("ingest more")
	ingest(t, escfg.APMServerURL, apikey)

	// check rollover
	// Confirm datastreams are
	// v managed by DSL if created after 8.15.0
	// x managed by ILM if created before 8.15.0
	t.Log("check data streams")
	dss, err = ac.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)

	require.Len(t, dss, 8)
	for _, v := range dss {
		assert.False(t, v.PreferIlm)
		assert.Equal(t, "Data stream lifecycle", v.NextGenerationManagedBy.Name)

		assert.Len(t, v.Indices, 2)
		assert.Equal(t, "Data stream lifecycle", v.Indices[0].ManagedBy.Name)
	}

	// check ES logs, there should be no errors
	// TODO: how to get these from Elastic Cloud? Is it possible?

	// cleanup
	t.Log("cleanup")
	require.NoError(t, tf.Apply(context.Background(),
		version, tfexec.Destroy(true)))
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
	g.Logger = zaptest.NewLogger(t)
	require.NoError(t, err)

	err = g.RunBlocking(context.Background())
}
