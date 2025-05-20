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

package standalone

import (
	"testing"

	"github.com/elastic/apm-server/functionaltests"
	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/steps"
)

func TestUpgrade_7_17_to_8_x_to_9_x_Snapshot_Standalone_to_Managed(t *testing.T) {
	t.Parallel()

	from7 := versionsCache.GetLatestSnapshot(t, "7.17")
	to8 := versionsCache.GetLatestSnapshot(t, "8")
	to9 := versionsCache.GetLatestSnapshot(t, "9")
	if !from7.CanUpgradeTo(to8.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from7.Version, to8.Version)
		return
	}
	if !to8.CanUpgradeTo(to9.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", to8.Version, to9.Version)
		return
	}

	t.Run("Managed7", func(t *testing.T) {
		t.Parallel()
		runner := managed7Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed8", func(t *testing.T) {
		t.Parallel()
		runner := managed8Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed9", func(t *testing.T) {
		t.Parallel()
		runner := managed9Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})
}

func TestUpgrade_7_17_to_8_x_to_9_x_BC_Standalone_to_Managed(t *testing.T) {
	t.Parallel()

	from7 := versionsCache.GetLatestVersionOrSkip(t, "7.17")
	to8 := versionsCache.GetLatestVersionOrSkip(t, "8")
	to9 := versionsCache.GetLatestBCOrSkip(t, "9")
	if !from7.CanUpgradeTo(to8.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from7.Version, to8.Version)
		return
	}
	if !to8.CanUpgradeTo(to9.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", to8.Version, to9.Version)
		return
	}

	t.Run("Managed in 7", func(t *testing.T) {
		t.Parallel()
		runner := managed7Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed in 8", func(t *testing.T) {
		t.Parallel()
		runner := managed8Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed in 9", func(t *testing.T) {
		t.Parallel()
		runner := managed9Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})
}

func managed7Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) steps.Runner {
	checkILM := asserts.CheckDataStreamIndividualWant{
		PreferIlm:        true,
		DSManagedBy:      functionaltests.ManagedByILM,
		IndicesManagedBy: []string{functionaltests.ManagedByILM},
	}
	checkILMRollover := asserts.CheckDataStreamIndividualWant{
		PreferIlm:        true,
		DSManagedBy:      functionaltests.ManagedByILM,
		IndicesManagedBy: []string{functionaltests.ManagedByILM, functionaltests.ManagedByILM},
	}

	check := map[string]asserts.CheckDataStreamIndividualWant{
		// These data streams are created in 7.x as well, so when we ingest
		// again in 8.x, they will be rolled-over.
		"traces-apm-%s":                     checkILMRollover,
		"metrics-apm.app.opbeans_python-%s": checkILMRollover,
		"metrics-apm.internal-%s":           checkILMRollover,
		"logs-apm.error-%s":                 checkILMRollover,
		// These data streams are only created in 8.x, so they will only have
		// 1 index.
		"metrics-apm.service_destination.1m-%s": checkILM,
		"metrics-apm.service_transaction.1m-%s": checkILM,
		"metrics-apm.service_summary.1m-%s":     checkILM,
		"metrics-apm.transaction.1m-%s":         checkILM,
	}

	// These data streams are created in 7.x, but not used in 8.x and 9.x,
	// so we ignore them to avoid wrong assertions.
	ignoredDataStreams := []string{
		"metrics-apm.app.opbeans_node-%s",
		"metrics-apm.app.opbeans_ruby-%s",
		"metrics-apm.app.opbeans_go-%s",
	}

	return functionaltests.TestStepsRunner(*target, *cleanupOnFailure,
		[]steps.Step{
			// Start from 7.x.
			steps.CreateStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: steps.APMStandalone,
			},
			steps.IngestV7Step{},
			// Migrate to managed.
			steps.MigrateManagedStep{},
			steps.IngestV7Step{},
			// Upgrade to 8.x.
			steps.UpgradeV7Step{NewVersion: toVersion8},
			steps.IngestStep{
				IgnoreDataStreams:         ignoredDataStreams,
				CheckIndividualDataStream: check,
			},
			// Resolve deprecations and upgrade to 9.x.
			steps.ResolveDeprecationsStep{},
			steps.UpgradeStep{
				NewVersion:                toVersion9,
				IgnoreDataStreams:         ignoredDataStreams,
				CheckIndividualDataStream: check,
			},
			steps.IngestStep{
				IgnoreDataStreams:         ignoredDataStreams,
				CheckIndividualDataStream: check,
			},
			steps.CheckErrorLogsStep{
				ESErrorLogsIgnored: steps.ESErrorLogs{
					functionaltests.EventLoopShutdown,
					functionaltests.AddIndexTemplateTracesError,
				},
				APMErrorLogsIgnored: steps.APMErrorLogs{
					functionaltests.TLSHandshakeError,
					functionaltests.ESReturnedUnknown503,
					functionaltests.RefreshCache503,
					functionaltests.PreconditionClusterInfoCtxCanceled,
					functionaltests.WaitServerReadyCtxCanceled,
					functionaltests.GRPCServerStopped,
					functionaltests.PopulateSourcemapFetcher403,
					functionaltests.RefreshCache403,
					functionaltests.RefreshCacheESConfigInvalid,
				},
			},
		},
	)
}

func managed8Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) steps.Runner {
	checkILMAll := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      functionaltests.ManagedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{functionaltests.ManagedByILM},
	}
	checkILM := asserts.CheckDataStreamIndividualWant{
		PreferIlm:        true,
		DSManagedBy:      functionaltests.ManagedByILM,
		IndicesManagedBy: []string{functionaltests.ManagedByILM},
	}

	check := map[string]asserts.CheckDataStreamIndividualWant{
		// These data streams are created in 7.x.
		"traces-apm-%s":                     checkILM,
		"metrics-apm.app.opbeans_python-%s": checkILM,
		"metrics-apm.internal-%s":           checkILM,
		"logs-apm.error-%s":                 checkILM,
		// These data streams are created in 8.x.
		"metrics-apm.service_destination.1m-%s": checkILM,
		"metrics-apm.service_transaction.1m-%s": checkILM,
		"metrics-apm.service_summary.1m-%s":     checkILM,
		"metrics-apm.transaction.1m-%s":         checkILM,
	}

	// These data streams are created in 7.x, but not used in 8.x and 9.x,
	// so we ignore them to avoid wrong assertions.
	ignoredDataStreams := []string{
		"metrics-apm.app.opbeans_node-%s",
		"metrics-apm.app.opbeans_ruby-%s",
		"metrics-apm.app.opbeans_go-%s",
	}

	return functionaltests.TestStepsRunner(*target, *cleanupOnFailure,
		[]steps.Step{
			// Start from 7.x.
			steps.CreateStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: steps.APMStandalone,
			},
			steps.IngestV7Step{},
			// Upgrade to 8.x.
			steps.UpgradeV7Step{NewVersion: toVersion8},
			steps.IngestStep{CheckDataStream: checkILMAll},
			// Migrate to managed
			steps.MigrateManagedStep{},
			steps.IngestStep{CheckDataStream: checkILMAll},
			// Resolve deprecations and upgrade to 9.x.
			steps.ResolveDeprecationsStep{},
			steps.UpgradeStep{
				NewVersion:                toVersion9,
				IgnoreDataStreams:         ignoredDataStreams,
				CheckIndividualDataStream: check,
			},
			steps.IngestStep{
				IgnoreDataStreams:         ignoredDataStreams,
				CheckIndividualDataStream: check,
			},
			steps.CheckErrorLogsStep{
				ESErrorLogsIgnored: steps.ESErrorLogs{
					functionaltests.EventLoopShutdown,
				},
				APMErrorLogsIgnored: steps.APMErrorLogs{
					functionaltests.TLSHandshakeError,
					functionaltests.ESReturnedUnknown503,
					functionaltests.RefreshCache503,
					functionaltests.PopulateSourcemapFetcher403,
					functionaltests.RefreshCache403,
					functionaltests.RefreshCacheESConfigInvalid,
				},
			},
		},
	)
}

func managed9Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) steps.Runner {
	// Data streams in 8.x should be all ILM if upgraded to a stack < 8.15 and > 8.16.
	checkILMAll := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      functionaltests.ManagedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{functionaltests.ManagedByILM},
	}

	return functionaltests.TestStepsRunner(*target, *cleanupOnFailure,
		[]steps.Step{
			// Start from 7.x.
			steps.CreateStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: steps.APMStandalone,
			},
			steps.IngestV7Step{},
			// Upgrade to 8.x.
			steps.UpgradeV7Step{NewVersion: toVersion8},
			steps.IngestStep{CheckDataStream: checkILMAll},
			// Resolve deprecations and upgrade to 9.x.
			steps.ResolveDeprecationsStep{},
			steps.UpgradeStep{
				NewVersion:      toVersion9,
				CheckDataStream: checkILMAll,
			},
			steps.IngestStep{CheckDataStream: checkILMAll},
			// Migrate to managed.
			steps.MigrateManagedStep{},
			steps.IngestStep{CheckDataStream: checkILMAll},
			steps.CheckErrorLogsStep{
				ESErrorLogsIgnored: steps.ESErrorLogs{
					functionaltests.EventLoopShutdown,
				},
				APMErrorLogsIgnored: steps.APMErrorLogs{
					functionaltests.TLSHandshakeError,
					functionaltests.ESReturnedUnknown503,
					functionaltests.RefreshCache503,
					functionaltests.PopulateSourcemapFetcher403,
					functionaltests.RefreshCache403,
					functionaltests.RefreshCacheESConfigInvalid,
				},
			},
		},
	)
}
