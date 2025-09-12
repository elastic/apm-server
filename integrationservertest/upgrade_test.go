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
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/elastic/apm-server/integrationservertest/internal/asserts"
	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

const (
	upgradeConfigFilename       = "upgrade-config.yaml"
	dockerImageOverrideFilename = "docker-image-override.yaml"
)

func formatUpgradePath(p string) string {
	splits := strings.Split(p, "->")
	for i := range splits {
		splits[i] = strings.TrimSpace(splits[i])
	}
	return strings.ReplaceAll(strings.Join(splits, "_to_"), ".", "_")
}

func TestUpgrade(t *testing.T) {
	upgradePathStr := strings.TrimSpace(*upgradePath)
	// The versions are separated by commas.
	if upgradePathStr == "" {
		t.Fatal("no upgrade versions specified")
	}
	splits := strings.Split(upgradePathStr, "->")

	var versions []ech.Version
	for i, split := range splits {
		s := strings.TrimSpace(split)
		version, err := ech.NewVersionFromString(strings.TrimSpace(split))
		if err != nil {
			t.Fatalf("failed to parse version %q: %v", s, err)
		}
		if i > 0 {
			prev := versions[len(versions)-1]
			if !vsCache.CanUpgrade(prev, version) {
				t.Fatalf("%q cannot upgrade to %q", prev, version)
			}
		}
		versions = append(versions, version)
	}

	config, err := parseConfig(upgradeConfigFilename)
	if err != nil {
		t.Fatal(err)
	}

	dockerImgOverride, err := parseDockerImageOverride(dockerImageOverrideFilename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// File does not exist, do nothing.
			dockerImgOverride = map[ech.Version]*dockerImageOverrideConfig{}
		} else {
			t.Fatal(err)
		}
	}

	t.Run(formatUpgradePath(*upgradePath), func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			t.Parallel()
			steps := buildTestSteps(t, versions, config, dockerImgOverride, false)
			runner := testStepsRunner{
				Target: *target,
				Steps:  steps,
			}
			runner.Run(t)
		})

		t.Run("Reroute", func(t *testing.T) {
			t.Parallel()
			steps := buildTestSteps(t, versions, config, dockerImgOverride, true)
			runner := testStepsRunner{
				Target: *target,
				Steps:  steps,
			}
			runner.Run(t)
		})
	})
}

func buildTestSteps(
	t *testing.T, versions ech.Versions,
	config upgradeTestConfig, dockerImgOverride map[ech.Version]*dockerImageOverrideConfig, reroute bool,
) []testStep {
	t.Helper()

	var steps []testStep
	var indicesManagedBy []string

	for i, ver := range versions {
		lifecycle := config.ExpectedLifecycle(ver)
		// Create deployment using first version, create reroute (if enabled) and ingest.
		if i == 0 {
			indicesManagedBy = append(indicesManagedBy, lifecycle)
			steps = append(steps, createStep{
				DeployVersion:       ver,
				CleanupOnFailure:    *cleanupOnFailure,
				DockerImageOverride: dockerImgOverride[ver],
			})
			if reroute {
				steps = append(steps, createReroutePipelineStep{DataStreamNamespace: "reroute"})
			}
			steps = append(steps, ingestStep{
				CheckDataStreams: dataStreamsExpectations(asserts.DataStreamExpectation{
					PreferIlm:        lifecycle == managedByILM,
					DSManagedBy:      lifecycle,
					IndicesManagedBy: indicesManagedBy,
				}),
			})
			continue
		}

		// Upgrade deployment to new version and ingest.
		prev := versions[i-1]
		oldIndicesManagedBy := slices.Clone(indicesManagedBy)
		if config.HasLazyRollover(prev, ver) {
			indicesManagedBy = append(indicesManagedBy, lifecycle)
		}
		steps = append(steps,
			upgradeStep{
				NewVersion: ver,
				CheckDataStreams: dataStreamsExpectations(asserts.DataStreamExpectation{
					PreferIlm:   lifecycle == managedByILM,
					DSManagedBy: lifecycle,
					// After upgrade, the indices should still be managed by
					// the same lifecycle management.
					IndicesManagedBy: oldIndicesManagedBy,
				}),
				DockerImageOverride: dockerImgOverride[ver],
			},
			ingestStep{
				CheckDataStreams: dataStreamsExpectations(asserts.DataStreamExpectation{
					PreferIlm:   lifecycle == managedByILM,
					DSManagedBy: lifecycle,
					// After ingestion, lazy rollover should kick in if applicable.
					IndicesManagedBy: indicesManagedBy,
				}),
			},
		)
	}

	// Check error logs, ignoring some that are due to intermittent issues
	// unrelated to our test.
	steps = append(steps, checkErrorLogsStep{
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
	})

	return steps
}

func dataStreamsExpectations(expect asserts.DataStreamExpectation) map[string]asserts.DataStreamExpectation {
	return map[string]asserts.DataStreamExpectation{
		"traces-apm-%s":                         expect,
		"metrics-apm.app.opbeans_python-%s":     expect,
		"metrics-apm.internal-%s":               expect,
		"logs-apm.error-%s":                     expect,
		"metrics-apm.service_destination.1m-%s": expect,
		"metrics-apm.service_transaction.1m-%s": expect,
		"metrics-apm.service_summary.1m-%s":     expect,
		"metrics-apm.transaction.1m-%s":         expect,
	}
}

type upgradeTest struct {
	Versions []string `yaml:"versions"`
}

type upgradeTestConfig struct {
	UpgradeTests               map[string]upgradeTest `yaml:"upgrade-tests"`
	DataStreamLifecycle        map[string]string      `yaml:"data-stream-lifecycle"`
	LazyRolloverWithExceptions map[string][]string    `yaml:"lazy-rollover-with-exceptions"`
}

// ExpectedLifecycle returns the lifecycle management that is expected of the provided version.
func (cfg upgradeTestConfig) ExpectedLifecycle(version ech.Version) string {
	lifecycle, ok := cfg.DataStreamLifecycle[version.MajorMinor()]
	if !ok {
		return managedByILM
	}
	if strings.EqualFold(lifecycle, "DSL") {
		return managedByDSL
	}
	return managedByILM
}

// HasLazyRollover checks if the upgrade path is expected to have lazy rollover.
func (cfg upgradeTestConfig) HasLazyRollover(from, to ech.Version) bool {
	exceptions, ok := cfg.LazyRolloverWithExceptions[to.MajorMinor()]
	if !ok {
		return false
	}
	for _, exception := range exceptions {
		if strings.EqualFold(from.MajorMinor(), exception) {
			return false
		}
	}
	return true
}

func parseConfig(filename string) (upgradeTestConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return upgradeTestConfig{}, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	config := upgradeTestConfig{}
	if err = yaml.Unmarshal(b, &config); err != nil {
		return upgradeTestConfig{}, fmt.Errorf("failed to unmarshal upgrade test config: %w", err)
	}

	return config, nil
}

func parseDockerImageOverride(filename string) (map[ech.Version]*dockerImageOverrideConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	config := map[string]*dockerImageOverrideConfig{}
	if err = yaml.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal docker image override config: %w", err)
	}

	result := map[ech.Version]*dockerImageOverrideConfig{}
	for k, v := range config {
		version, err := ech.NewVersionFromString(k)
		if err != nil {
			return nil, fmt.Errorf("invalid version in docker image override config: %w", err)
		}
		result[version] = v
	}

	return result, nil
}
