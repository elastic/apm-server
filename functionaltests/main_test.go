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
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cleanupOnFailure *bool = flag.Bool("cleanup-on-failure", true, "Whether to run cleanup even if the test failed.")

// target is the Elastic Cloud environment to target with these test.
// We use 'pro' for production as that is the key used to retrieve EC_API_KEY from secret storage.
var target *string = flag.String("target", "pro", "The target environment where to run tests againts. Valid values are: qa, pro")

const (
	// lifecycleDSL is the string used in Elasticsearch responses to indicate a Data stream or Index managed by DSL.
	lifecycleDSL = "Data stream lifecycle"
	// lifecycleILM  is the string used in Elasticsearch responses to indicate a Data stream or Index managed by ILM.
	lifecycleILM = "Index Lifecycle Management"
)

// expectedIngestForASingleRun() represent the expected number of ingested document after a
// single run of ingest().
// Only non aggregation data streams are included, as aggregation ones differs on different
// runs.
func expectedIngestForASingleRun() esclient.APMDataStreamsDocCount {
	return map[string]int{
		"traces-apm-default":                     15013,
		"metrics-apm.app.opbeans_python-default": 1437,
		"metrics-apm.internal-default":           1351,
		"logs-apm.error-default":                 364,
	}
}

// getDocsCountPerDS retrieves document count.
func getDocsCountPerDS(t *testing.T, ctx context.Context, ecc *esclient.Client) (esclient.APMDataStreamsDocCount, error) {
	t.Helper()
	return ecc.ApmDocCount(ctx)
}

// assertDocCount check if specified document count is equal to expected minus
// documents count from a previous state.
func assertDocCount(t *testing.T, docsCount, previous, expected esclient.APMDataStreamsDocCount) {
	t.Helper()
	for ds, v := range docsCount {
		if e, ok := expected[ds]; ok {
			assert.Equal(t, e, v-previous[ds],
				fmt.Sprintf("wrong document count for %s", ds))
		}
	}
}

type checkDatastreamWant struct {
	Quantity         int
	DSManagedBy      string
	IndicesPerDs     int
	PreferIlm        bool
	IndicesManagedBy []checkDatastreamIndex
}

type checkDatastreamIndex struct {
	ManagedBy string
	PreferIlm bool
}

// assertDatastreams assert expected values on specific data streams in a cluster.
func assertDatastreams(t *testing.T, expected checkDatastreamWant, actual []types.DataStream) {
	t.Helper()

	require.Len(t, actual, expected.Quantity, "number of APM datastream differs from expectations")
	for _, v := range actual {
		if expected.PreferIlm {
			assert.True(t, v.PreferIlm, "datastream %s should prefer ILM", v.Name)
		} else {
			assert.False(t, v.PreferIlm, "datastream %s should not prefer ILM", v.Name)
		}

		assert.Equal(t, expected.DSManagedBy, v.NextGenerationManagedBy.Name,
			`datastream %s should be managed by "%s"`, v.Name, expected.DSManagedBy,
		)
		assert.Len(t, v.Indices, expected.IndicesPerDs,
			"datastream %s should have %d indices", v.Name, expected.IndicesPerDs,
		)
		for i, index := range v.Indices {
			assert.Equal(t, expected.IndicesManagedBy[i].ManagedBy, index.ManagedBy.Name,
				`index %s should be managed by "%s"`, index.IndexName, expected.IndicesManagedBy[i].ManagedBy,
			)
			if expected.IndicesManagedBy[i].PreferIlm {
				assert.True(t, *index.PreferIlm,
					fmt.Sprintf("index %s should prefer ILM", index.IndexName))
			} else {
				assert.False(t, *index.PreferIlm,
					fmt.Sprintf("index %s should not prefer ILM", index.IndexName))
			}
		}
	}

}

const (
	targetQA = "qa"
	// we use 'pro' because is the target passed by the Buildkite pipeline running
	// these tests.
	targetProd = "pro"
)

// regionFrom returns the appropriate region to run test
// againts based on specified target.
// https://www.elastic.co/guide/en/cloud/current/ec-regions-templates-instances.html
func regionFrom(target string) string {
	switch target {
	case targetQA:
		return "aws-eu-west-1"
	case targetProd:
		return "eu-west-1"
	default:
		panic("target value is not accepted")
	}
}

func endpointFrom(target string) string {
	switch target {
	case targetQA:
		return "https://public-api.qa.cld.elstc.co"
	case targetProd:
		return "https://api.elastic-cloud.com"
	default:
		panic("target value is not accepted")
	}
}
