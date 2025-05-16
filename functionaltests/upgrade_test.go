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
	"strings"
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

var (
	upgradePath = flag.String(
		"upgrade-path",
		"",
		"Versions to be used in TestUpgrade_Provided_Versions_*, separated by commas",
	)
)

func TestUpgrade_Provided_Versions_Snapshot(t *testing.T) {
	// The versions are separated by commas.
	if strings.TrimSpace(*upgradePath) == "" {
		t.Fatal("no upgrade versions specified")
	}
	splits := strings.Split(*upgradePath, ",")
	if len(splits) < 2 {
		t.Fatal("need to specify at least 2 upgrade versions")
	}

	// Get all snapshot versions based on input.
	var versionInfos []ecclient.StackVersionInfo
	for i, s := range splits {
		versionInfo := getLatestSnapshot(t, strings.TrimSpace(s))
		if i != 0 {
			prevVersionInfo := versionInfos[len(versionInfos)-1]
			if !prevVersionInfo.CanUpgradeTo(versionInfo.Version) {
				t.Fatalf("%s is not upgradable to %s", prevVersionInfo.Version, versionInfo.Version)
			}
		}
		versionInfos = append(versionInfos, versionInfo)
	}

	checkILM := asserts.CheckDataStreamsWant{
		Quantity:    8,
		PreferIlm:   true,
		DSManagedBy: managedByILM,
	}

	// Build upgrade-and-ingest steps, omitting the first one since that will
	// be the starting version.
	upgradeIngestSteps := make([]testStep, 0, len(versionInfos)-1)
	for _, versionInfo := range versionInfos[1:] {
		// Upgrade to version.
		upgradeIngestSteps = append(upgradeIngestSteps, upgradeStep{
			NewVersion:      versionInfo.Version,
			CheckDataStream: checkILM,
			SkipIndices:     true,
		})
		// Perform ingestion on that version.
		upgradeIngestSteps = append(upgradeIngestSteps, ingestStep{
			CheckDataStream: checkILM,
			SkipIndices:     true,
		})
	}

	// Run default upgrade scenario.
	t.Run("Default", func(t *testing.T) {
		t.Parallel()

		// Create deployment using first version and perform ingestion.
		steps := []testStep{
			createStep{DeployVersion: versionInfos[0].Version},
			ingestStep{CheckDataStream: checkILM, SkipIndices: true},
		}
		// Upgrade and ingest in subsequent versions.
		steps = append(steps, upgradeIngestSteps...)
		// Check error logs.
		steps = append(steps, checkErrorLogsStep{
			APMErrorLogsIgnored: apmErrorLogs{
				tlsHandshakeError,
				esReturnedUnknown503,
				refreshCache503,
				populateSourcemapFetcher403,
			},
		})

		runner := testStepsRunner{Steps: steps}
		runner.Run(t)
	})

	// Run reroute upgrade scenario.
	t.Run("Reroute", func(t *testing.T) {
		t.Parallel()

		rerouteNamespace := "rerouted"
		setupFn := stepFunc(
			func(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
				t.Log("create reroute processors")
				createRerouteIngestPipeline(t, ctx, e.esc, rerouteNamespace)
				return previousRes
			},
		)
		// Create deployment using first version, add reroute, and perform ingestion.
		steps := []testStep{
			createStep{DeployVersion: versionInfos[0].Version},
			customStep{Func: setupFn},
			ingestStep{CheckDataStream: checkILM, SkipIndices: true},
		}
		// Upgrade and ingest in subsequent versions.
		steps = append(steps, upgradeIngestSteps...)
		// Check error logs.
		steps = append(steps, checkErrorLogsStep{
			APMErrorLogsIgnored: apmErrorLogs{
				tlsHandshakeError,
				esReturnedUnknown503,
				refreshCache503,
				populateSourcemapFetcher403,
			},
		})

		runner := testStepsRunner{
			DataStreamNamespace: rerouteNamespace,
			Steps:               steps,
		}
		runner.Run(t)
	})
}
