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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

func expectedIndicesIngest() esclient.IndicesDocCount {
	return esclient.IndicesDocCount{
		"apm-*-error-*":       364,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        10885,
		"apm-*-transaction-*": 4128,
		// Ignore aggregation indices.
		"apm-*-metric-*":     -1,
		"apm-*-onboarding-*": -1,
	}
}

func emptyIndicesIngest() esclient.IndicesDocCount {
	return esclient.IndicesDocCount{
		"apm-*-error-*":       0,
		"apm-*-profile-*":     0,
		"apm-*-span-*":        0,
		"apm-*-transaction-*": 0,
		"apm-*-onboarding-*":  0,
		"apm-*-metric-*":      -1,
	}
}

func expectedDataStreamsIngestV7(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     15013,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 364,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 1492,
		fmt.Sprintf("metrics-apm.app.opbeans_node-%s", namespace):   27,
		fmt.Sprintf("metrics-apm.app.opbeans_go-%s", namespace):     11,
		fmt.Sprintf("metrics-apm.app.opbeans_ruby-%s", namespace):   24,
		// Document count fluctuates constantly.
		fmt.Sprintf("metrics-apm.internal-%s", namespace): -1,
	}
}

func emptyDataStreamsIngestV7(namespace string) esclient.DataStreamsDocCount {
	return map[string]int{
		fmt.Sprintf("traces-apm-%s", namespace):                     0,
		fmt.Sprintf("metrics-apm.app.opbeans_python-%s", namespace): 0,
		fmt.Sprintf("metrics-apm.app.opbeans_node-%s", namespace):   0,
		fmt.Sprintf("metrics-apm.app.opbeans_go-%s", namespace):     0,
		fmt.Sprintf("metrics-apm.app.opbeans_ruby-%s", namespace):   0,
		fmt.Sprintf("metrics-apm.internal-%s", namespace):           0,
		fmt.Sprintf("logs-apm.error-%s", namespace):                 0,
	}
}

// ingestV7Step performs ingestion to the APM Server deployed on ECH.
// After ingestion, it checks if the document counts difference between
// current and previous is expected.
//
// The output of this step is the indices document counts after ingestion.
//
// NOTE: Only works for versions 7.x.
type ingestV7Step struct{}

func (i ingestV7Step) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major >= 8 {
		t.Fatal("ingest v7 step should only be used for versions < 8.0")
	}

	t.Logf("------ ingest in %s ------", e.currentVersion())
	err := e.gen.RunBlockingWait(ctx, e.currentVersion(), e.integrations)
	require.NoError(t, err)

	t.Logf("------ ingest check in %s ------", e.currentVersion())
	t.Log("check number of documents after ingestion")
	// Standalone, check indices.
	if !e.integrations {
		idxDocCount := getDocCountPerIndexV7(t, ctx, e.esc)
		asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
			expectedIndicesIngest())
		return testStepResult{IndicesDocCount: idxDocCount}
	}

	// Managed, check data streams
	dsDocCount := getDocCountPerDSV7(t, ctx, e.esc, e.dsNamespace)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		expectedDataStreamsIngestV7(e.dsNamespace))
	return testStepResult{DSDocCount: dsDocCount}
}

// upgradeV7Step upgrades the ECH deployment from its current version to
// the new version. It also adds the new version into testStepEnv. After
// upgrade, it checks that the document counts did not change across upgrade.
//
// The output of this step is the indices document counts if upgrading to 7.x,
// or data streams document counts if upgrading to >= 8.0.
//
// NOTE: Only works from versions 7.x.
type upgradeV7Step struct {
	NewVersion ecclient.StackVersion
}

func (u upgradeV7Step) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major >= 8 {
		t.Fatal("upgrade v7 step should only be used from versions < 8.0")
	}

	t.Logf("------ upgrade %s to %s ------", e.currentVersion(), u.NewVersion)
	upgradeCluster(t, ctx, e.tf, *target, u.NewVersion, e.integrations)
	// Update the environment version to the new one.
	e.versions = append(e.versions, u.NewVersion)

	t.Logf("------ upgrade check in %s ------", e.currentVersion())
	t.Log("check number of documents across upgrade")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	if !e.integrations {
		// Standalone, return indices even if upgraded to >= 8.0, since indices
		// will simply be ignored by 8.x checks.
		idxDocCount := getDocCountPerIndexV7(t, ctx, e.esc)
		asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
			emptyIndicesIngest())
		return testStepResult{IndicesDocCount: idxDocCount}
	}

	// Managed, should be data streams regardless of upgrade.
	dsDocCount := getDocCountPerDSV7(t, ctx, e.esc, e.dsNamespace)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		emptyDataStreamsIngestV7(e.dsNamespace))
	return testStepResult{DSDocCount: getDocCountPerDS(t, ctx, e.esc)}
}

// migrateManagedStep migrates the ECH APM deployment from standalone mode to
// managed mode, which involves enabling the integrations server via Kibana.
// It also checks that the document counts did not change across the migration.
//
// The output of this step is the indices document counts if version < 8.0,
// or data streams document counts if version >= 8.0.
type migrateManagedStep struct{}

func (m migrateManagedStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.integrations {
		t.Fatal("migrate managed step should only be used on standalone")
	}

	t.Logf("------ migrate to managed for %s ------", e.currentVersion())
	t.Log("enable integrations server")
	err := e.kbc.EnableIntegrationsServer(ctx)
	require.NoError(t, err)
	e.integrations = true

	// APM Server needs some time to start serving requests again, and we don't have any
	// visibility on when this completes.
	// NOTE: This value comes from empirical observations.
	time.Sleep(80 * time.Second)

	t.Log("check number of documents across migration to managed")
	// We assert that no changes happened in the number of documents after migration
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the migration.
	if e.currentVersion().Major < 8 {
		idxDocCount := getDocCountPerIndexV7(t, ctx, e.esc)
		asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
			emptyIndicesIngest())
		return testStepResult{IndicesDocCount: idxDocCount}
	}

	dsDocCount := getDocCountPerDS(t, ctx, e.esc)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		emptyDataStreamsIngest(e.dsNamespace))
	return testStepResult{DSDocCount: dsDocCount}
}

// resolveDeprecationsStep resolves critical migration deprecation warnings from Elasticsearch regarding
// indices created in 7.x not being compatible with 9.x.
//
// The output of this step is the previous test step result.
type resolveDeprecationsStep struct{}

func (r resolveDeprecationsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Logf("------ resolve migration deprecations in %s ------", e.currentVersion())
	err := e.kbc.ResolveMigrationDeprecations(ctx)
	require.NoError(t, err)
	return previousRes
}
