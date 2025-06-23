package integrationservertest

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// See https://github.com/elastic/ingest-dev/issues/5701
// Available tests:
// TestAPMResourcesVersionBug/8.16.2_to_8.17.8
// TestAPMResourcesVersionBug/8.17.7_to_8.17.8
// TestAPMResourcesVersionBug/8.17.8_to_8.18.3
// TestAPMResourcesVersionBug/8.18.2_to_8.18.3
// TestAPMResourcesVersionBug/8.18.3_to_8.19.0
func TestAPMResourcesVersionBug(t *testing.T) {
	config, err := parseConfig("upgrade-config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	zeroEventIngestedDocs := checkFieldExistsInDocsStep{
		dataStreamName: "traces-apm-default",
		fieldName:      "event.ingested",
		checkFn:        func(i int64) bool { return i == 0 },
	}
	noEventIngestedInMappings := checkMappingStep{
		datastreamname: "traces-apm-default",
		indexName:      regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000001"),
		checkFn: func(mappings types.TypeMapping) error {
			if !hasNestedField(mappings, "event.ingested") {
				return nil
			}
			return errors.New("there should be no event.ingested here")
		},
	}
	someEventIngestedDocs := checkFieldExistsInDocsStep{
		dataStreamName: "traces-apm-default",
		fieldName:      "event.ingested",
	}

	// this wraps the usual steps build to add additional checks in between.
	// Expect one upgrade, no ingested docs before, some after.
	oneUpgradeZeroThenSome := func(t *testing.T, versions ech.Versions, config upgradeTestConfig) []testStep {
		steps := buildTestSteps(t, versions, config, false)
		return []testStep{
			steps[0], // create
			steps[1], // ingest
			zeroEventIngestedDocs,
			noEventIngestedInMappings,
			steps[2], // upgrade
			steps[3], // ingest
			someEventIngestedDocs,
			checkMappingStep{
				datastreamname: "traces-apm-default",
				indexName:      regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000002"),
				checkFn: func(mappings types.TypeMapping) error {
					if hasNestedField(mappings, "event.ingested") {
						return nil
					}
					return fmt.Errorf("there should be an event.ingested here")
				},
			},
		}
	}

	// this wraps the usual steps build to add additional checks in between.
	// Expect two upgrades, some ingested docs after the first upgrade, some after the second.
	twoUpgradesSomeThenSome := func(t *testing.T, versions ech.Versions, config upgradeTestConfig) []testStep {
		steps := buildTestSteps(t, versions, config, false)
		howmany := int64(0)
		return []testStep{
			steps[0], // create
			steps[1], // ingest
			steps[2], // upgrade
			steps[3], // ingest
			checkFieldExistsInDocsStep{
				dataStreamName: "traces-apm-default",
				fieldName:      "event.ingested",
				checkFn: func(i int64) bool {
					if i == 0 {
						return false
					}
					// Store howmany to cross check it's higher after next upgrade/ingest cycle.
					howmany = i
					return true
				},
			},
			checkMappingStep{
				datastreamname: "traces-apm-default",
				indexName:      regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000002"),
				checkFn: func(mappings types.TypeMapping) error {
					if hasNestedField(mappings, "event.ingested") {
						return nil
					}
					return fmt.Errorf("there should be an event.ingested here")
				},
			},
			steps[4], // upgrade
			steps[5], // upgrade
			checkFieldExistsInDocsStep{
				dataStreamName: "traces-apm-default",
				fieldName:      "event.ingested",
				checkFn: func(i int64) bool {
					if i == 0 {
						return false
					}
					if i <= howmany {
						return false
					}
					return true
				},
			},
			checkMappingStep{
				datastreamname: "traces-apm-default",
				indexName:      regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000003"),
				checkFn: func(mappings types.TypeMapping) error {
					if hasNestedField(mappings, "event.ingested") {
						return nil
					}
					return fmt.Errorf("there should be an event.ingested here")
				},
			},
		}
	}

	version8162 := vsCache.GetLatestVersionOrSkip(t, "8.16.2")
	version8177 := vsCache.GetLatestVersionOrSkip(t, "8.17.7")
	version8178 := vsCache.GetLatestBCOrSkip(t, "8.17.8") // latest 8.17 as of today
	version8182 := vsCache.GetLatestVersionOrSkip(t, "8.18.2")
	version8183 := vsCache.GetLatestBCOrSkip(t, "8.18.3") // latest 8.18 as of today
	// version8190 := vsCache.GetLatestBCOrSkip(t, "8.19.0") // latest 8.19 as of today
	// the event.ingested change was introduced in 8.16.3, as
	// it only shows if we upgrade a cluster from a version with
	// the bug we need to start from 8.16.2.
	start := version8162

	t.Run("8.16.2 to 8.17.8", func(t *testing.T) {
		versions := []ech.Version{start, version8177}
		runner := testStepsRunner{
			Target: *target,
			Steps:  oneUpgradeZeroThenSome(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.17.7 to 8.17.8", func(t *testing.T) {
		versions := []ech.Version{start, version8177, version8178}
		runner := testStepsRunner{
			Target: *target,
			Steps:  twoUpgradesSomeThenSome(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.17.8 to 8.18.3", func(t *testing.T) {
		versions := []ech.Version{version8178, version8183}
		runner := testStepsRunner{
			Target: *target,
			Steps:  buildTestSteps(t, versions, config, false),
		}
		runner.Run(t)
	})

	t.Run("8.18.2 to 8.18.3", func(t *testing.T) {
		versions := []ech.Version{start, version8182, version8183}
		runner := testStepsRunner{
			Target: *target,
			Steps:  twoUpgradesSomeThenSome(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.18.3 to 8.19.0", func(t *testing.T) {
		// FIXME: use 8.19.0 BC when it becomes available.
		start := vsCache.GetLatestSnapshot(t, "8.16.2")
		version8183 := vsCache.GetLatestSnapshot(t, "8.18.3")
		version8190 := vsCache.GetLatestSnapshot(t, "8.19.0")
		versions := []ech.Version{start, version8183, version8190}
		steps := twoUpgradesSomeThenSome(t, versions, config)
		steps = append(steps, checkFieldExistsInDocsStep{
			dataStreamName: "traces-apm-default",
			fieldName:      "event.success_count",
		})
		runner := testStepsRunner{
			Target: *target,
			Steps:  steps,
		}
		runner.Run(t)
	})
}
