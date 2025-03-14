package ecclient

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// StackVersions is a list of StackVersion.
type StackVersions []StackVersion

func NewStackVersionsFromStrs(versionStrs []string) (StackVersions, error) {
	vs := make(StackVersions, 0, len(versionStrs))
	for _, s := range versionStrs {
		v, err := NewStackVersionFromStr(s)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

// Sort sorts the stack versions in ascending order based on
// major, minor, patch, suffix in order of importance.
func (vs StackVersions) Sort() {
	slices.SortFunc(vs, func(a, b StackVersion) int {
		return a.Compare(b)
	})
}

// LatestFor retrieves the latest stack version for that prefix.
// The prefix must follow semantic versioning in the form of:
//   - X.Y.Z
//   - X.Y
//   - X
//
// Invalid prefix will cause this function to panic.
//
// Note: This assumes that StackVersions is already sorted in ascending order.
func (vs StackVersions) LatestFor(prefix string) (StackVersion, bool) {
	for i := len(vs) - 1; i >= 0; i-- {
		if ok, err := vs[i].HasPrefix(prefix); err != nil {
			panic(err)
		} else if ok {
			return vs[i], true
		}
	}
	return StackVersion{}, false
}

// LatestForMajor retrieves the latest stack version for that major.
//
// Note: This assumes that StackVersions is already sorted in ascending order.
func (vs StackVersions) LatestForMajor(major uint) (StackVersion, bool) {
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].IsMajor(major) {
			return vs[i], true
		}
	}
	return StackVersion{}, false
}

// LatestForMinor retrieves the latest stack version for that minor.
//
// Note: This assumes that StackVersions is already sorted in ascending order.
func (vs StackVersions) LatestForMinor(major uint, minor uint) (StackVersion, bool) {
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].IsMinor(major, minor) {
			return vs[i], true
		}
	}
	return StackVersion{}, false
}

type StackVersion struct {
	Major  uint
	Minor  uint
	Patch  uint
	Suffix string // Optional
}

func NewStackVersion(major, minor, patch uint, suffix string) StackVersion {
	return StackVersion{
		Major:  major,
		Minor:  minor,
		Patch:  patch,
		Suffix: suffix,
	}
}

func NewStackVersionFromStr(versionStr string) (StackVersion, error) {
	splits := strings.SplitN(versionStr, ".", 3)
	if len(splits) != 3 {
		return StackVersion{}, errors.New("invalid format")
	}

	major, err := strconv.ParseUint(splits[0], 10, 32)
	if err != nil {
		return StackVersion{}, fmt.Errorf("invalid major version: %s", splits[0])
	}
	minor, err := strconv.ParseUint(splits[1], 10, 32)
	if err != nil {
		return StackVersion{}, fmt.Errorf("invalid minor version: %s", splits[1])
	}

	splits = strings.SplitN(splits[2], "-", 2)
	patch, err := strconv.ParseUint(splits[0], 10, 32)
	if err != nil {
		return StackVersion{}, fmt.Errorf("invalid patch version: %s", splits[0])
	}

	suffix := ""
	if len(splits) > 1 {
		suffix = splits[1]
	}
	return NewStackVersion(uint(major), uint(minor), uint(patch), suffix), nil
}

func (v StackVersion) String() string {
	var suffix string
	if v.Suffix != "" {
		suffix = "-" + v.Suffix
	}
	return fmt.Sprintf("%d.%d.%d%s", v.Major, v.Minor, v.Patch, suffix)
}

func (v StackVersion) IsMajor(major uint) bool {
	return v.Major == major
}

func (v StackVersion) IsMinor(major, minor uint) bool {
	return v.Major == major && v.Minor == minor
}

func (v StackVersion) IsPatch(major, minor, patch uint) bool {
	return v.Major == major && v.Minor == minor && v.Patch == patch
}

func (v StackVersion) Compare(other StackVersion) int {
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

// HasPrefix checks if the stack version contains the prefix.
// The prefix must follow semantic versioning in the form of:
//   - X.Y.Z
//   - X.Y
//   - X
func (v StackVersion) HasPrefix(prefix string) (bool, error) {
	splits := strings.SplitN(prefix, ".", 3)
	var finalErr error
	parseMajor := func() uint {
		major, err := strconv.ParseUint(splits[0], 10, 32)
		if err != nil && finalErr == nil {
			finalErr = fmt.Errorf("invalid major version: %s", splits[0])
		}
		return uint(major)
	}
	parseMinor := func() uint {
		minor, err := strconv.ParseUint(splits[1], 10, 32)
		if err != nil && finalErr == nil {
			finalErr = fmt.Errorf("invalid minor version: %s", splits[1])
		}
		return uint(minor)
	}
	parsePatch := func() uint {
		patch, err := strconv.ParseUint(splits[2], 10, 32)
		if err != nil && finalErr == nil {
			finalErr = fmt.Errorf("invalid patch version: %s", splits[2])
		}
		return uint(patch)
	}

	switch len(splits) {
	case 1:
		return v.IsMajor(parseMajor()), finalErr
	case 2:
		return v.IsMinor(parseMajor(), parseMinor()), finalErr
	case 3:
		return v.IsPatch(parseMajor(), parseMinor(), parsePatch()), finalErr
	default:
		return false, fmt.Errorf("invalid format: %s", prefix)
	}
}
