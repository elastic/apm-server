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
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

var (
	// cleanupOnFailure determines whether the created resources should be cleaned up on test failure.
	cleanupOnFailure = flag.Bool(
		"cleanup-on-failure",
		true,
		"Whether to run cleanup even if the test failed.",
	)

	// target is the Elastic Cloud environment to target with these test.
	// We use 'pro' for production as that is the key used to retrieve EC_API_KEY from secret storage.
	target = flag.String(
		"target",
		"pro",
		"The target environment where to run tests againts. Valid values are: qa, pro.",
	)
)

const (
	// managedByDSL is the constant string used by Elasticsearch to specify that an Index is managed by Data Stream Lifecycle management.
	managedByDSL = "Data stream lifecycle"
	// managedByILM is the constant string used by Elasticsearch to specify that an Index is managed by Index Lifecycle Management.
	managedByILM = "Index Lifecycle Management"
)

var (
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates ecclient.StackVersions
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots ecclient.StackVersions
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions ecclient.StackVersions
)

func TestMain(m *testing.M) {
	flag.Parse()

	// This is a simple check to alert users if this necessary env var
	// is not available.
	//
	// Functional tests are expected to run Terraform code to operate
	// on infrastructure required for each test and to query Elastic
	// Cloud APIs. In both cases a valid API key is required.
	ecAPIKey := os.Getenv("EC_API_KEY")
	if ecAPIKey == "" {
		log.Fatal("EC_API_KEY env var not set")
		return
	}

	ctx := context.Background()
	ecRegion := regionFrom(*target)
	ecc, err := ecclient.New(endpointFrom(*target), ecAPIKey)
	if err != nil {
		log.Fatal(err)
		return
	}

	candidates, err := ecc.GetCandidateVersions(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedCandidates = candidates

	snapshots, err := ecc.GetSnapshotVersions(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedSnapshots = snapshots

	versions, err := ecc.GetVersions(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedVersions = versions

	code := m.Run()
	os.Exit(code)
}

// getBCVersionOrSkip retrieves the latest build-candidate version for the version prefix.
// If the version is not found, the test is skipped via t.Skip.
func getBCVersionOrSkip(t *testing.T, prefix string) ecclient.StackVersion {
	t.Helper()
	candidate, ok := fetchedCandidates.LatestFor(prefix)
	if !ok {
		t.Skip("skipping non-BC versions")
		return ecclient.StackVersion{}
	}
	return candidate
}

// getLatestSnapshot retrieves the latest snapshot version for the version prefix.
func getLatestSnapshot(t *testing.T, prefix string) ecclient.StackVersion {
	t.Helper()
	version, ok := fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "no snapshot with prefix '%s' found in EC region %s", prefix, regionFrom(*target))
	return version
}

// getLatestVersion retrieves the latest non-snapshot version for the version prefix.
func getLatestVersion(t *testing.T, prefix string) ecclient.StackVersion {
	t.Helper()
	version, ok := fetchedVersions.LatestFor(prefix)
	require.True(t, ok, "no version with prefix '%s' found in EC region %s", prefix, regionFrom(*target))
	return version
}

// expectedIngestForASingleRun represent the expected number of ingested document after a
// single run of ingest.
// Only non aggregation data streams are included, as aggregation ones differs on different
// runs.
func expectedIngestForASingleRun(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     15013,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 1437,
		fmt.Sprintf("metrics-apm.internal-%s", namespace):           1351,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 364,
	}
}

// emptyIngestForASingleRun represent an empty ingestion.
// It is useful for asserting that the document count did not change after an operation.
func emptyIngestForASingleRun(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     0,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 0,
		fmt.Sprintf("metrics-apm.internal-%s", namespace):           0,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 0,
	}
}

func aggregationDataStreams(namespace string) []string {
	return []string{
		fmt.Sprintf("metrics-apm.service_destination.1m-%s", namespace),
		fmt.Sprintf("metrics-apm.service_transaction.1m-%s", namespace),
		fmt.Sprintf("metrics-apm.service_summary.1m-%s", namespace),
		fmt.Sprintf("metrics-apm.transaction.1m-%s", namespace),
	}
}

func allDataStreams(namespace string) []string {
	res := aggregationDataStreams(namespace)
	for ds := range expectedIngestForASingleRun(namespace) {
		res = append(res, ds)
	}
	return res
}

// getDocsCountPerDS retrieves document count.
func getDocsCountPerDS(t *testing.T, ctx context.Context, esc *esclient.Client) (esclient.DataStreamsDocCount, error) {
	t.Helper()
	return esc.APMDocCount(ctx)
}

func sliceToMap(s []string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range s {
		m[v] = true
	}
	return m
}

// assertDocCount check if specified document count is equal to expected minus
// documents count from a previous state.
func assertDocCount(t *testing.T, current, previous, expectedDiff esclient.DataStreamsDocCount, skippedDataStreams []string) {
	t.Helper()
	skipped := sliceToMap(skippedDataStreams)
	for ds, v := range current {
		if skipped[ds] {
			continue
		}

		e, ok := expectedDiff[ds]
		if !ok {
			t.Errorf("unexpected documents (%d) for %s", v, ds)
			continue
		}

		assert.Equal(t, e, v-previous[ds],
			fmt.Sprintf("wrong document count difference for %s", ds))
	}
}

type checkDataStreamWant struct {
	Quantity         int
	DSManagedBy      string
	IndicesPerDs     int
	PreferIlm        bool
	IndicesManagedBy []string
}

// assertDataStreams assert expected values on specific data streams in a cluster.
func assertDataStreams(t *testing.T, expected checkDataStreamWant, actual []types.DataStream) {
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
			assert.Equal(t, expected.IndicesManagedBy[i], index.ManagedBy.Name,
				`index %s should be managed by "%s"`, index.IndexName,
				expected.IndicesManagedBy[i],
			)
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
		return "gcp-us-west2"
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

func deploymentTemplateFrom(region string) string {
	switch region {
	case "aws-eu-west-1":
		return "aws-storage-optimized"
	case "gcp-us-west2":
		return "gcp-storage-optimized"
	default:
		panic("region value is not accepted")
	}
}
