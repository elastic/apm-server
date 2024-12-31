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
	"net/url"
	"testing"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

var cleanupOnFailure *bool = flag.Bool("cleanup-on-failure", true, "Whether to run cleanup even if the test failed.")
var target *string = flag.String("target", "production", "The target environment where to run tests againts. Valid values are: qa, production")

const testRegionQA = "aws-eu-west-1"
const testRegionProduction = "europe-west1"

// assertDocCountEqual asserts that document counts in each data stream are equal.
func assertDocCountEqual(t *testing.T, want []esclient.ApmDocCount, actual []esclient.ApmDocCount) {
	t.Helper()

	assert.Len(t, want, len(actual))
	for i, v := range actual {
		assert.Equal(t, v.Count, want[i].Count, "expected doc count in %s to be equal to previous value", v.Datastream)
	}
}

// assertDocCountGreaterThan verifies if document count in datastreams is greated than expected.
func assertDocCountGreaterThan(t *testing.T, want []esclient.ApmDocCount, actual []esclient.ApmDocCount) {
	t.Helper()

	assert.Len(t, want, len(actual))
	for i, v := range actual {
		assert.Greater(t, v.Count, want[i].Count, "expected doc count in %s to be greater than previous value", v.Datastream)
	}
}

// ingest creates and run a telemetrygen that replays multiple APM agents events to the cluster
// a single time.
func ingest(t *testing.T, apmURL string, apikey string) {
	t.Helper()

	t.Log("ingest data")
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

// assertDatastreams assert expected values on specific data streams in a cluster.
func assertDatastreams(t *testing.T, expected checkDatastreamWant, actual []types.DataStream) {
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

// regionFrom returns the appropriate region to run test
// againts based on specified target.
func regionFrom(target string) string {
	switch target {
	case "qa":
		return testRegionQA
	case "pro":
		return testRegionProduction
	default:
		panic("target value is not accepted")
	}
}
