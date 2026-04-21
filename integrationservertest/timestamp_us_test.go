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

package integrationservertest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/integrationservertest/internal/asserts"
	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

// TestTimestampUSMappingUpgrade is a regression test for
// https://github.com/elastic/apm-server/issues/20496.
// It verifies that upgrading from a prefix to a postfix version
// produces backing indices with timestamp.us mapped as long.
// NOTE: This test is not run in CI.
func TestTimestampUSMappingUpgrade(t *testing.T) {
	config, err := parseConfigFile(upgradeConfigFilename)
	if err != nil {
		t.Fatal(err)
	}
	dockerImgOverride, err := parseDockerImageOverride(dockerImageOverrideFilename)
	if err != nil {
		t.Fatal(err)
	}

	type upgradePath struct {
		preFix  ech.Version
		postFix ech.Version
	}

	mustVersion := func(s string) ech.Version {
		v, err := ech.NewVersionFromString(s)
		require.NoError(t, err)
		return v
	}

	paths := []upgradePath{
		{preFix: mustVersion("8.19.12"), postFix: mustVersion("8.19.13")},
		{preFix: mustVersion("9.2.6"), postFix: mustVersion("9.2.7")},
		{preFix: mustVersion("9.3.1"), postFix: mustVersion("9.3.2")},
		{preFix: mustVersion("9.3.0-SNAPSHOT"), postFix: getLatestSnapshot(t, "9.4")},
	}

	for _, p := range paths {
		p := p
		name := fmt.Sprintf("%s_to_%s", p.preFix, p.postFix)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			steps := buildTimestampUSSteps(t, p.preFix, p.postFix, config, dockerImgOverride)
			runner := testStepsRunner{
				Target: *target,
				Steps:  steps,
			}
			runner.Run(t)
		})
	}
}

func buildTimestampUSSteps(
	t *testing.T,
	preFix, postFix ech.Version,
	config upgradeTestConfig,
	dockerImgOverride map[ech.Version]*dockerImageOverrideConfig,
) []testStep {
	t.Helper()

	preFixLifecycle := config.ExpectedLifecycle(preFix)
	postFixLifecycle := config.ExpectedLifecycle(postFix)

	indicesManagedBy := []string{preFixLifecycle}

	return []testStep{
		createStep{
			DeployVersion:       preFix,
			CleanupOnFailure:    *cleanupOnFailure,
			DockerImageOverride: dockerImgOverride[preFix],
		},
		indexRawDocStep{
			Index: "traces-apm-default",
			Body:  `{"@timestamp":"2024-01-01T00:00:00Z","data_stream":{"type":"traces","dataset":"apm","namespace":"default"},"processor":{"event":"transaction"},"timestamp":{"us":1772469840000000.0}}`,
		},
		checkFieldMappingStep{
			DataStream:   "traces-apm-default",
			Field:        "timestamp.us",
			ExpectedType: "float",
		},
		upgradeStep{
			NewVersion: postFix,
			CheckDataStreams: map[string]asserts.DataStreamExpectation{
				"traces-apm-%s": {
					PreferIlm:        postFixLifecycle == managedByILM,
					DSManagedBy:      postFixLifecycle,
					IndicesManagedBy: indicesManagedBy,
				},
			},
			DockerImageOverride: dockerImgOverride[postFix],
		},
		ingestStep{
			CheckDataStreams: func() map[string]asserts.DataStreamExpectation {
				twoIndices := asserts.DataStreamExpectation{
					PreferIlm:        postFixLifecycle == managedByILM,
					DSManagedBy:      postFixLifecycle,
					IndicesManagedBy: append(indicesManagedBy, postFixLifecycle),
				}
				oneIndex := asserts.DataStreamExpectation{
					PreferIlm:        postFixLifecycle == managedByILM,
					DSManagedBy:      postFixLifecycle,
					IndicesManagedBy: []string{postFixLifecycle},
				}
				return map[string]asserts.DataStreamExpectation{
					"traces-apm-%s":                         twoIndices,
					"metrics-apm.app.opbeans_python-%s":     oneIndex,
					"metrics-apm.internal-%s":               oneIndex,
					"logs-apm.error-%s":                     oneIndex,
					"metrics-apm.service_destination.1m-%s": oneIndex,
					"metrics-apm.service_transaction.1m-%s": oneIndex,
					"metrics-apm.service_summary.1m-%s":     oneIndex,
					"metrics-apm.transaction.1m-%s":         oneIndex,
				}
			}(),
		},
		checkFieldMappingStep{
			DataStream:   "traces-apm-default",
			Field:        "timestamp.us",
			ExpectedType: "long",
		},
		checkErrorLogsStep{
			APMErrorLogsIgnored: apmErrorLogs{
				bulkIndexingFailed,
				tlsHandshakeError,
				esReturnedUnknown503,
				refreshCache403,
				refreshCache503,
				refreshCacheCtxCanceled,
				refreshCacheCtxDeadline,
				refreshCacheESConfigInvalid,
				preconditionFailed,
				populateSourcemapServerShuttingDown,
				populateSourcemapFetcher403,
				syncSourcemapFetcher403,
				initialSearchQueryContextCanceled,
				scrollSearchQueryContextCanceled,
			},
		},
	}
}
