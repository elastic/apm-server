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

package kbclient_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

// To regenerate the fixture, first delete the existing fixture. Then, you will
// need a deployed cluster that has been upgraded from 7.x to 8.x, and has
// ingested APM data.
//
// For reference, here is the minimal required code to run. Note that the t.Fail()
// is intended in order to stop the test cleanup.
//
//	func TestResolveMigrationDeprecationsSetup(t *testing.T) {
//		checkILM := asserts.CheckDataStreamsWant{
//			Quantity:         8,
//			PreferIlm:        true,
//			DSManagedBy:      managedByILM,
//			IndicesManagedBy: []string{managedByILM},
//		}
//
//		runner := testStepsRunner{
//			Steps: []testStep{
//				createStep{
//					DeployVersion:     getLatestSnapshot(t, "7.17"),
//					APMDeploymentMode: apmStandalone,
//				},
//				ingestV7Step{},
//				upgradeV7Step{NewVersion: getLatestSnapshot(t, "8")},
//				ingestStep{CheckDataStream: checkILM},
//			},
//		}
//		runner.Run(t)
//		t.Fail()
//	}
//
// Once you have the cluster setup, set the appropriate environment variables
// (KIBANA_URL, KIBANA_USERNAME and KIBANA_PASSWORD) and run this test.
func TestClient_ResolveMigrationDeprecations(t *testing.T) {
	kbc := newRecordedTestClient(t)

	ctx := context.Background()

	// Check that system indices are not migrated.
	status, err := kbc.QuerySystemIndicesMigrationStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, kbclient.MigrationNeeded, status)

	// Check that there are some critical deprecation warnings.
	deprecations, err := kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	require.Greater(t, len(deprecations), 0)
	for _, deprecation := range deprecations {
		require.True(t, deprecation.IsCritical)
	}

	// Resolve them.
	// NOTE: This test will not be effective against different deprecation
	// types than what was initially set.
	err = kbc.ResolveMigrationDeprecations(ctx)
	require.NoError(t, err)

	// Check that system indices are migrated.
	status, err = kbc.QuerySystemIndicesMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, kbclient.NoMigrationNeeded, status)

	// Check that there are no more warnings.
	deprecations, err = kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	assert.Len(t, deprecations, 0)
}
