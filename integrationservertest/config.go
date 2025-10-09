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
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

const (
	upgradeConfigFilename       = "upgrade-config.yaml"
	dockerImageOverrideFilename = "docker-image-override.yaml"
)

type upgradeTestConfig struct {
	DataStreamLifecycle    map[string]string
	LazyRolloverExceptions []lazyRolloverException
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

// LazyRollover checks if the upgrade path is expected to have lazy rollover.
func (cfg upgradeTestConfig) LazyRollover(from, to ech.Version) bool {
	for _, e := range cfg.LazyRolloverExceptions {
		if e.matchVersion(from, to) {
			return false
		}
	}
	return true
}

func parseConfigFile(filename string) (upgradeTestConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return upgradeTestConfig{}, fmt.Errorf("failed to open %s: %w", filename, err)
	}

	defer f.Close()
	return parseConfig(f)
}

func parseConfig(reader io.Reader) (upgradeTestConfig, error) {
	type lazyRolloverExceptionYAML struct {
		From string `yaml:"from"`
		To   string `yaml:"to"`
	}

	type upgradeTestConfigYAML struct {
		DataStreamLifecycle    map[string]string           `yaml:"data-stream-lifecycle"`
		LazyRolloverExceptions []lazyRolloverExceptionYAML `yaml:"lazy-rollover-exceptions"`
	}

	b, err := io.ReadAll(reader)
	if err != nil {
		return upgradeTestConfig{}, fmt.Errorf("failed to read config: %w", err)
	}

	configYAML := upgradeTestConfigYAML{}
	if err := yaml.Unmarshal(b, &configYAML); err != nil {
		return upgradeTestConfig{}, fmt.Errorf("failed to unmarshal upgrade test config: %w", err)
	}

	config := upgradeTestConfig{
		DataStreamLifecycle: configYAML.DataStreamLifecycle,
	}

	for _, e := range configYAML.LazyRolloverExceptions {
		from, err := parseLazyRolloverExceptionVersionRange(e.From)
		if err != nil {
			return upgradeTestConfig{}, fmt.Errorf(
				"failed to parse lazy-rollover-exception version '%s': %w", e.From, err)
		}
		to, err := parseLazyRolloverExceptionVersionRange(e.To)
		if err != nil {
			return upgradeTestConfig{}, fmt.Errorf(
				"failed to parse lazy-rollover-exception version '%s': %w", e.From, err)
		}
		config.LazyRolloverExceptions = append(config.LazyRolloverExceptions, lazyRolloverException{
			From: from,
			To:   to,
		})
	}

	return config, nil
}

type lazyRolloverException struct {
	From lreVersionRange
	To   lreVersionRange
}

func (e lazyRolloverException) matchVersion(from, to ech.Version) bool {
	// Either version not in range.
	if !e.From.inRange(from) || !e.To.inRange(to) {
		return false
	}
	// If both pattern have minor x, check if both version minors are the same.
	if e.From.isSingularWithMinorX() && e.To.isSingularWithMinorX() {
		return from.Minor == to.Minor
	}
	return true
}

type lreVersionRange struct {
	Start          lreVersion
	InclusiveStart bool
	End            *lreVersion // Nil if there is only one version, i.e. not a range.
	InclusiveEnd   bool
}

func (r lreVersionRange) inRange(version ech.Version) bool {
	// Range of versions.
	if r.End != nil {
		// Ranges will never have wildcards, so we don't need to check those!
		start := ech.NewVersion(r.Start.Major, *r.Start.Minor.Num, *r.Start.Patch.Num, "")
		end := ech.NewVersion(r.End.Major, *r.End.Minor.Num, *r.End.Patch.Num, "")
		cmpStart := start.Compare(version) < 0
		if r.InclusiveStart {
			cmpStart = start.Compare(version) <= 0
		}
		cmpEnd := end.Compare(version) > 0
		if r.InclusiveEnd {
			cmpEnd = end.Compare(version) >= 0
		}
		return cmpStart && cmpEnd
	}

	// Singular version only.
	if r.Start.Major != version.Major {
		return false
	}
	// Both wildcards, return true since major is equal.
	if r.Start.Minor.isWildcard() && r.Start.Patch.isWildcard() {
		return true
	}
	// Only patch is wildcard, check minor.
	if r.Start.Patch.isWildcard() {
		return *r.Start.Minor.Num == version.Minor
	}
	// Only minor is wildcard, check patch.
	return *r.Start.Patch.Num == version.Patch
}

func (r lreVersionRange) isSingularWithMinorX() bool {
	return r.End == nil && r.Start.Minor.isX()
}

type numberOrWildcard struct {
	Num *uint64 // Not nil if Str is a number.
	Str string
}

func (n numberOrWildcard) isWildcard() bool {
	return n.Num == nil
}

func (n numberOrWildcard) isX() bool {
	return n.Str == "x"
}

func (n numberOrWildcard) isStar() bool {
	return n.Str == "*"
}

func parseNumberOrWildcard(s string) (numberOrWildcard, error) {
	if s == "*" || s == "x" {
		return numberOrWildcard{Str: s}, nil
	}

	num, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return numberOrWildcard{}, err
	}
	return numberOrWildcard{Num: &num, Str: s}, nil
}

type lreVersion struct {
	Major uint64
	Minor numberOrWildcard
	Patch numberOrWildcard
}

func (v lreVersion) hasWildcards() bool {
	return v.Minor.isWildcard() || v.Patch.isWildcard()
}

var (
	lazyRolloverVersionRg      = regexp.MustCompile(`^(\d+).(\d+|\*|x).(\d+|\*)$`)
	lazyRolloverVersionRangeRg = regexp.MustCompile(`^([\[(])\s*(\d+.\d+.\d+)\s*-\s*(\d+.\d+.\d+)\s*([])])$`)
)

func parseLazyRolloverExceptionVersionRange(s string) (lreVersionRange, error) {
	// Range of versions.
	matches := lazyRolloverVersionRangeRg.FindStringSubmatch(s)
	if len(matches) > 0 {
		inclusiveStart := matches[1] == "["
		inclusiveEnd := matches[4] == "]"
		start, err := ech.NewVersionFromString(matches[2])
		if err != nil {
			return lreVersionRange{}, fmt.Errorf("failed to parse '%s' start: %w", s, err)
		}
		end, err := ech.NewVersionFromString(matches[3])
		if err != nil {
			return lreVersionRange{}, fmt.Errorf("failed to parse '%s' end: %w", s, err)
		}
		return lreVersionRange{
			Start: lreVersion{
				Major: start.Major,
				Minor: numberOrWildcard{
					Num: &start.Minor,
					Str: strconv.FormatUint(start.Minor, 10),
				},
				Patch: numberOrWildcard{
					Num: &start.Patch,
					Str: strconv.FormatUint(start.Patch, 10),
				},
			},
			InclusiveStart: inclusiveStart,
			End: &lreVersion{
				Major: end.Major,
				Minor: numberOrWildcard{
					Num: &end.Minor,
					Str: strconv.FormatUint(end.Minor, 10),
				},
				Patch: numberOrWildcard{
					Num: &end.Patch,
					Str: strconv.FormatUint(end.Patch, 10),
				},
			},
			InclusiveEnd: inclusiveEnd,
		}, nil
	}

	// Singular version only.
	matches = lazyRolloverVersionRg.FindStringSubmatch(s)
	if len(matches) > 0 {
		major, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return lreVersionRange{}, fmt.Errorf("failed to parse '%s' major: %w", s, err)
		}
		minor, err := parseNumberOrWildcard(matches[2])
		if err != nil {
			return lreVersionRange{}, fmt.Errorf("failed to parse '%s' minor: %w", s, err)
		}
		patch, err := parseNumberOrWildcard(matches[3])
		if err != nil {
			return lreVersionRange{}, fmt.Errorf("failed to parse '%s' patch: %w", s, err)
		}
		return lreVersionRange{Start: lreVersion{Major: major, Minor: minor, Patch: patch}}, nil
	}

	return lreVersionRange{}, fmt.Errorf("invalid version pattern '%s'", s)
}

func parseDockerImageOverride(filename string) (map[ech.Version]*dockerImageOverrideConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		// File does not exist, fallback to no overrides.
		if errors.Is(err, os.ErrNotExist) {
			return map[ech.Version]*dockerImageOverrideConfig{}, nil
		}
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
