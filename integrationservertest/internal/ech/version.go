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

package ech

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Versions []Version

// Sort sorts the versions in ascending order based on
// major, minor, patch, suffix in order of importance.
func (vs Versions) Sort() {
	cmpFn := func(a, b Version) int {
		return a.Compare(b)
	}

	if slices.IsSortedFunc(vs, cmpFn) {
		return
	}

	slices.SortFunc(vs, cmpFn)
}

// Last returns the last version in the list.
func (vs Versions) Last() (Version, bool) {
	if len(vs) == 0 {
		return Version{}, false
	}
	return vs[len(vs)-1], true
}

// LatestFor retrieves the latest version for that prefix.
// The prefix must loosely follow semantic versioning in the form of:
//   - X.Y.Z
//   - X.Y
//   - X
//
// Invalid prefix will cause this function to panic.
//
// Note: This assumes that Versions is already sorted in ascending order.
func (vs Versions) LatestFor(prefix string) (Version, bool) {
	lv, err := parseVersionPrefix(prefix)
	if err != nil {
		panic(err)
	}

	for i := len(vs) - 1; i >= 0; i-- {
		if ok := vs[i].looseMatch(lv); ok {
			return vs[i], true
		}
	}
	return Version{}, false
}

// LatestForMajor retrieves the latest version for that major.
//
// Note: This assumes that Versions is already sorted in ascending order.
func (vs Versions) LatestForMajor(major uint64) (Version, bool) {
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].IsMajor(major) {
			return vs[i], true
		}
	}
	return Version{}, false
}

// LatestForMinor retrieves the latest version for that minor.
//
// Note: This assumes that Versions is already sorted in ascending order.
func (vs Versions) LatestForMinor(major, minor uint64) (Version, bool) {
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].IsMinor(major, minor) {
			return vs[i], true
		}
	}
	return Version{}, false
}

// PreviousMinorLatest retrieves the latest version from the previous
// minor of the provided `version`.
// If the minor of `version` is 0, the latest version for previous major is
// returned instead.
//
// Note: This assumes that Versions is already sorted in ascending order.
func (vs Versions) PreviousMinorLatest(version Version) (Version, bool) {
	if version.Minor == 0 {
		// When the minor is 0, we want the latest of the previous major
		return vs.LatestForMajor(version.Major - 1)
	}
	return vs.LatestForMinor(version.Major, version.Minor-1)
}

// PreviousPatch retrieves the previous patch version info from the provided `version`.
//
// Note: This assumes that Versions is already sorted in ascending order.
func (vs Versions) PreviousPatch(version Version) (Version, bool) {
	if version.Patch == 0 {
		// When the patch is 0, we want the latest of the previous minor
		return vs.LatestForMinor(version.Major, version.Minor-1)
	}
	prevPatch := version
	prevPatch.Patch = version.Patch - 1
	if vs.Has(prevPatch) {
		return prevPatch, true
	}
	return Version{}, false
}

// Has returns true if the provided `version` exists in the list.
func (vs Versions) Has(version Version) bool {
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i] == version {
			return true
		}
	}
	return false
}

// Filter filters the list of versions using the provided function.
// If the function returns true on a version, it is added to the return list.
// Otherwise, it is skipped.
func (vs Versions) Filter(fn func(Version) bool) Versions {
	var result Versions
	for _, v := range vs {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

type Version struct {
	Major  uint64
	Minor  uint64
	Patch  uint64
	Suffix string // Optional
}

func NewVersion(major, minor, patch uint64, suffix string) Version {
	return Version{
		Major:  major,
		Minor:  minor,
		Patch:  patch,
		Suffix: suffix,
	}
}

func NewVersionFromString(versionStr string) (Version, error) {
	splits := strings.SplitN(versionStr, ".", 3)
	if len(splits) != 3 {
		return Version{}, errors.New("invalid format")
	}

	major, err := strconv.ParseUint(splits[0], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %w", err)
	}
	minor, err := strconv.ParseUint(splits[1], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %w", err)
	}

	splits = strings.SplitN(splits[2], "-", 2)
	patch, err := strconv.ParseUint(splits[0], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %w", err)
	}

	suffix := ""
	if len(splits) > 1 {
		suffix = splits[1]
	}
	return NewVersion(major, minor, patch, suffix), nil
}

func (v Version) String() string {
	var suffix string
	if v.Suffix != "" {
		suffix = "-" + v.Suffix
	}
	return fmt.Sprintf("%d.%d.%d%s", v.Major, v.Minor, v.Patch, suffix)
}

func (v Version) MajorMinorPatch() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) MajorMinor() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func (v Version) IsMajor(major uint64) bool {
	return v.Major == major
}

func (v Version) IsMinor(major, minor uint64) bool {
	return v.Major == major && v.Minor == minor
}

func (v Version) IsPatch(major, minor, patch uint64) bool {
	return v.Major == major && v.Minor == minor && v.Patch == patch
}

func (v Version) IsSnapshot() bool {
	return v.Suffix == "SNAPSHOT"
}

func (v Version) Compare(other Version) int {
	res := cmp.Compare(v.Major, other.Major)
	if res != 0 {
		return res
	}
	res = cmp.Compare(v.Minor, other.Minor)
	if res != 0 {
		return res
	}
	res = cmp.Compare(v.Patch, other.Patch)
	if res != 0 {
		return res
	}
	return cmp.Compare(v.Suffix, other.Suffix)
}

// HasPrefix checks if the version contains the prefix.
// The prefix must loosely follow semantic versioning in the form of:
//   - X.Y.Z
//   - X.Y
//   - X
func (v Version) HasPrefix(prefix string) (bool, error) {
	lv, err := parseVersionPrefix(prefix)
	if err != nil {
		return false, err
	}
	return v.looseMatch(lv), nil
}

type looseVersion struct {
	major, minor, patch *uint64
}

func (v Version) looseMatch(lv looseVersion) bool {
	// Only major
	if lv.minor == nil {
		return v.IsMajor(*lv.major)
	}
	// Only major minor
	if lv.patch == nil {
		return v.IsMinor(*lv.major, *lv.minor)
	}
	// Major, minor, patch
	return v.IsPatch(*lv.major, *lv.minor, *lv.patch)
}

var looseVersionRegex = regexp.MustCompile(`^(\d*)(?:\.(\d*))?(?:\.(\d*))?(?:-(\w*))?$`)

func parseVersionPrefix(prefix string) (looseVersion, error) {
	lv := looseVersion{}
	// First match is the whole string, last match is the suffix, total 5
	matches := looseVersionRegex.FindStringSubmatch(prefix)
	if len(matches) == 0 || len(matches) > 5 {
		return looseVersion{}, fmt.Errorf("invalid prefix format: %s", prefix)
	}

	major, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return looseVersion{}, fmt.Errorf("invalid major version: %w", err)
	}

	// Only major
	lv.major = &major
	if matches[2] == "" {
		return lv, nil
	}

	minor, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return looseVersion{}, fmt.Errorf("invalid minor version: %w", err)
	}

	// Only major minor
	lv.minor = &minor
	if matches[3] == "" {
		return lv, nil
	}

	// Major, minor, patch
	patch, err := strconv.ParseUint(matches[3], 10, 64)
	if err != nil {
		return looseVersion{}, fmt.Errorf("invalid patch version: %w", err)
	}

	lv.patch = &patch
	return lv, nil
}
