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
		if e.matchVersions(from, to) {
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
		lre, err := parseLazyRolloverException(e.From, e.To)
		if err != nil {
			return upgradeTestConfig{}, fmt.Errorf("failed to parse lazy-rollover exception: %w", err)
		}
		config.LazyRolloverExceptions = append(config.LazyRolloverExceptions, lre)
	}

	return config, nil
}

type lazyRolloverException struct {
	From lreVersion
	To   lreVersion
}

func (e lazyRolloverException) matchVersions(from, to ech.Version) bool {
	// Either version not in range.
	if !e.From.matchVersion(from) || !e.To.matchVersion(to) {
		return false
	}
	// If both pattern have minor x, check if both version minors are the same.
	if e.From.isSingularWithMinorX() && e.To.isSingularWithMinorX() {
		return from.Minor == to.Minor
	}
	return true
}

type lreVersion struct {
	Singular *wildcardVersion // Nil if range.
	Range    *lreVersionRange // Nil if singular version.
}

func (r lreVersion) matchVersion(version ech.Version) bool {
	// Range of versions.
	if r.Range != nil {
		return r.Range.inRange(version)
	}

	// Singular version only.
	if r.Singular.Major != version.Major {
		return false
	}
	// Both wildcards, return true since major is equal.
	if r.Singular.Minor.isWildcard() && r.Singular.Patch.isWildcard() {
		return true
	}
	// Only patch is wildcard, check minor.
	if r.Singular.Patch.isWildcard() {
		return *r.Singular.Minor.Num == version.Minor
	}
	// Only minor is wildcard, check patch.
	return *r.Singular.Patch.Num == version.Patch
}

func (r lreVersion) isSingularWithMinorX() bool {
	return r.Singular != nil && r.Singular.Minor.isX()
}

type lreVersionRange struct {
	Start          ech.Version
	InclusiveStart bool
	End            ech.Version
	InclusiveEnd   bool
}

func (r lreVersionRange) inRange(version ech.Version) bool {
	cmpStart := r.Start.Compare(version) < 0
	if r.InclusiveStart {
		cmpStart = r.Start.Compare(version) <= 0
	}
	cmpEnd := r.End.Compare(version) > 0
	if r.InclusiveEnd {
		cmpEnd = r.End.Compare(version) >= 0
	}
	return cmpStart && cmpEnd
}

type wildcardVersion struct {
	Major uint64
	Minor numberOrWildcard
	Patch numberOrWildcard
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

func parseLazyRolloverException(fromStr, toStr string) (lazyRolloverException, error) {
	from, err := parseLazyRolloverExceptionVersionRange(fromStr)
	if err != nil {
		return lazyRolloverException{}, fmt.Errorf(
			"failed to parse from version '%s': %w", fromStr, err)
	}
	to, err := parseLazyRolloverExceptionVersionRange(toStr)
	if err != nil {
		return lazyRolloverException{}, fmt.Errorf(
			"failed to parse to version '%s': %w", toStr, err)
	}
	// Check that if one version have x wildcard, the other should also have it.
	if from.isSingularWithMinorX() != to.isSingularWithMinorX() {
		return lazyRolloverException{}, fmt.Errorf(
			"both versions ('%s', '%s') should have special wildcard or none at all", fromStr, toStr)
	}

	return lazyRolloverException{
		From: from,
		To:   to,
	}, nil
}

var (
	lazyRolloverVersionRg      = regexp.MustCompile(`^(\d+).(\d+|\*|x).(\d+|\*)$`)
	lazyRolloverVersionRangeRg = regexp.MustCompile(`^([\[(])\s*(\d+.\d+.\d+)\s*-\s*(\d+.\d+.\d+)\s*([])])$`)
)

func parseLazyRolloverExceptionVersionRange(s string) (lreVersion, error) {
	// Range of versions.
	matches := lazyRolloverVersionRangeRg.FindStringSubmatch(s)
	if len(matches) > 0 {
		inclusiveStart := matches[1] == "["
		inclusiveEnd := matches[4] == "]"
		start, err := ech.NewVersionFromString(matches[2])
		if err != nil {
			return lreVersion{}, fmt.Errorf("failed to parse '%s' start: %w", s, err)
		}
		end, err := ech.NewVersionFromString(matches[3])
		if err != nil {
			return lreVersion{}, fmt.Errorf("failed to parse '%s' end: %w", s, err)
		}
		return lreVersion{
			Range: &lreVersionRange{
				Start:          start,
				InclusiveStart: inclusiveStart,
				End:            end,
				InclusiveEnd:   inclusiveEnd,
			},
		}, nil
	}

	// Singular version only.
	matches = lazyRolloverVersionRg.FindStringSubmatch(s)
	if len(matches) > 0 {
		major, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return lreVersion{}, fmt.Errorf("failed to parse '%s' major: %w", s, err)
		}
		minor, err := parseNumberOrWildcard(matches[2])
		if err != nil {
			return lreVersion{}, fmt.Errorf("failed to parse '%s' minor: %w", s, err)
		}
		patch, err := parseNumberOrWildcard(matches[3])
		if err != nil {
			return lreVersion{}, fmt.Errorf("failed to parse '%s' patch: %w", s, err)
		}
		return lreVersion{
			Singular: &wildcardVersion{
				Major: major,
				Minor: minor,
				Patch: patch,
			},
		}, nil
	}

	return lreVersion{}, fmt.Errorf("invalid version pattern '%s'", s)
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
