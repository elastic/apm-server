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
		"apm-*-metric-*":      -1,
		"apm-*-onboarding-*":  0,
	}
}

// ingestLegacyStep performs ingestion to the APM Server deployed on ECH.
// After ingestion, it checks if the document counts difference between
// current and previous is expected.
//
// The output of this step is the indices document counts after ingestion.
//
// NOTE: Only works for versions >= 8.0.
type ingestLegacyStep struct{}

var _ testStep = ingestLegacyStep{}

func (i ingestLegacyStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major >= 8 {
		t.Fatal("ingest legacy step should only be used for versions < 8.0")
	}

	t.Log("------ ingest ------")
	err := e.gen.RunBlockingWait(ctx, e.currentVersion(), e.integrations)
	require.NoError(t, err)

	t.Log("------ ingest check ------")
	t.Log("check number of documents after ingestion")
	idxDocCount := getDocCountPerIndex(t, ctx, e.esc)
	asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
		expectedIndicesIngest())

	return testStepResult{IndicesDocCount: idxDocCount}
}

// upgradeLegacyStep upgrades the ECH deployment from its current version to
// the new version. It also adds the new version into testStepEnv. After
// upgrade, it checks that the document counts did not change across upgrade.
//
// The output of this step is the indices document counts if upgrading to < 8.0,
// or data streams document counts if upgrading to >= 8.0.
//
// NOTE: Only works from versions < 8.0.
type upgradeLegacyStep struct {
	NewVersion ecclient.StackVersion
}

var _ testStep = upgradeLegacyStep{}

func (u upgradeLegacyStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.currentVersion().Major >= 8 {
		t.Fatal("upgrade legacy step should only be used from versions < 8.0")
	}

	t.Logf("------ upgrade %s to %s ------", e.currentVersion(), u.NewVersion)
	upgradeCluster(t, ctx, e.tf, *target, u.NewVersion, e.integrations)
	// Update the environment version to the new one.
	e.versions = append(e.versions, u.NewVersion)

	t.Log("------ upgrade check ------")
	t.Log("check number of documents across upgrade")
	// We assert that no changes happened in the number of documents after upgrade
	// to ensure the state didn't change.
	// We don't expect any change here unless something broke during the upgrade.
	idxDocCount := getDocCountPerIndex(t, ctx, e.esc)
	asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
		emptyIndicesIngest())

	// Upgrade from version < 8.0 to < 8.0, return indices as result.
	if e.currentVersion().Major < 8 {
		return testStepResult{IndicesDocCount: idxDocCount}
	}

	// Upgrade from version < 8.0 to >= 8.0, return data streams as result.
	// Don't need to assert data streams, since there's likely nothing.
	return testStepResult{DSDocCount: getDocCountPerDS(t, ctx, e.esc)}
}

type migrateManagedStep struct{}

var _ testStep = migrateManagedStep{}

func (m migrateManagedStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	if e.integrations {
		t.Fatal("migrate managed step should only be used on standalone")
	}

	t.Log("------ migrate to managed ------")
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
		idxDocCount := getDocCountPerIndex(t, ctx, e.esc)
		asserts.CheckDocCountV7(t, idxDocCount, previousRes.IndicesDocCount,
			emptyIndicesIngest())
		return testStepResult{IndicesDocCount: idxDocCount}
	}

	dsDocCount := getDocCountPerDS(t, ctx, e.esc)
	asserts.CheckDocCount(t, dsDocCount, previousRes.DSDocCount,
		emptyDataStreamsIngest(e.dsNamespace))
	return testStepResult{DSDocCount: dsDocCount}
}
