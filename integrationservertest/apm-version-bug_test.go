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
func TestAPMResourcesVersionBug(t *testing.T) {
	config, err := parseConfig("upgrade-config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// the event.ingested change was introduced in 8.16.3, as
	// it only shows if we upgrade a cluster from a version with
	// the bug we need to start from 8.16.2.
	start := ech.NewVersion(8, 16, 2, "SNAPSHOT")

	// this wraps the usual steps build to add additional checks in between.
	// Is a bit brittle, maybe we should consider allowing this as a first class citizen.
	buildteststepsForBug := func(t *testing.T, versions ech.Versions, config upgradeTestConfig) []testStep {
		steps := buildTestSteps(t, versions, config, false)

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
		eventIngestedHasMappings := checkMappingStep{
			datastreamname: "traces-apm-default",
			indexName:      regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000002"),
			checkFn: func(mappings types.TypeMapping) error {
				if hasNestedField(mappings, "event.ingested") {
					return nil
				}
				return fmt.Errorf("there should be an event.ingested here")
			},
		}
		if len(versions) == 3 {
			eventIngestedHasMappings.indexName = regexp.MustCompile(".ds-traces-apm-default-[0-9.]+-000003")
		}

		if len(versions) == 2 { // if 2 versions we expect the first to be bugged and the second to be not
			return []testStep{
				steps[0], // create
				steps[1], // ingest
				zeroEventIngestedDocs,
				noEventIngestedInMappings,
				steps[2], // upgrade
				steps[3], // ingest
				someEventIngestedDocs,
				eventIngestedHasMappings,
			}

		} else if len(versions) == 3 { // if 3 versions we expect the first to be bugged on create, the second on upgrade, the third to be not
			return []testStep{
				steps[0], // create
				steps[1], // ingest
				steps[2], // upgrade
				steps[3], // ingest
				zeroEventIngestedDocs,
				noEventIngestedInMappings,
				steps[4], // upgrade
				steps[5], // upgrade
				someEventIngestedDocs,
				eventIngestedHasMappings,
			}
		}

		return []testStep{}
	}

	t.Run("8.16.2 to 8.17.8", func(t *testing.T) {
		from := start
		to := ech.NewVersion(8, 17, 7, "SNAPSHOT")
		versions := []ech.Version{from, to}
		runner := testStepsRunner{
			Target: *target,
			Steps:  buildteststepsForBug(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.17.7 to 8.17.8", func(t *testing.T) {
		from := ech.NewVersion(8, 17, 7, "SNAPSHOT")
		to := vsCache.GetLatestSnapshot(t, "8.17")
		versions := []ech.Version{start, from, to}
		runner := testStepsRunner{
			Target: *target,
			Steps:  buildteststepsForBug(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.17.8 to 8.18.3", func(t *testing.T) {
		from := vsCache.GetLatestSnapshot(t, "8.17")
		to := vsCache.GetLatestSnapshot(t, "8.18")
		versions := []ech.Version{from, to}
		runner := testStepsRunner{
			Target: *target,
			Steps:  buildTestSteps(t, versions, config, false),
		}
		runner.Run(t)
	})

	t.Run("8.18.2 to 8.18.3", func(t *testing.T) {
		from := ech.NewVersion(8, 18, 2, "SNAPSHOT")
		to := vsCache.GetLatestSnapshot(t, "8.17")
		versions := []ech.Version{start, from, to}
		runner := testStepsRunner{
			Target: *target,
			Steps:  buildteststepsForBug(t, versions, config),
		}
		runner.Run(t)
	})

	t.Run("8.18.3 to 8.19.0", func(t *testing.T) {
		from := vsCache.GetLatestSnapshot(t, "8.18")
		to := vsCache.GetLatestSnapshot(t, "8.19")
		versions := []ech.Version{start, from, to}
		steps := buildteststepsForBug(t, versions, config)
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
