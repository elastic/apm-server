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

package ecclient_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

func newStackVersionInfosFromStrs(strs []string) (ecclient.StackVersionInfos, error) {
	infos := make(ecclient.StackVersionInfos, 0, len(strs))
	for _, s := range strs {
		v, err := ecclient.NewStackVersionFromStr(s)
		if err != nil {
			return nil, err
		}
		infos = append(infos, ecclient.StackVersionInfo{Version: v})
	}
	return infos, nil
}

func TestStackVersions_Sort(t *testing.T) {
	got, err := newStackVersionInfosFromStrs([]string{"9.0.0-SNAPSHOT", "8.14.5", "7.17.28", "9.0.0"})
	require.NoError(t, err)

	expected, err := newStackVersionInfosFromStrs([]string{"7.17.28", "8.14.5", "9.0.0", "9.0.0-SNAPSHOT"})
	require.NoError(t, err)

	got.Sort()
	assert.EqualValues(t, expected, got)
}

func TestStackVersionInfos_LatestFor(t *testing.T) {
	type args struct {
		prefix string
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion ecclient.StackVersion
		wantExist   bool
	}{
		{
			name:      "no stack versions",
			vs:        []string{},
			args:      args{prefix: "8.17"},
			wantExist: false,
		},
		{
			name:      "no matching stack version",
			vs:        []string{"8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:      args{prefix: "9.0"},
			wantExist: false,
		},
		{
			name:        "latest stack version major",
			vs:          []string{"8.16.4", "8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{prefix: "8"},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 18, Patch: 0},
			wantExist:   true,
		},
		{
			name:        "latest stack version minor",
			vs:          []string{"8.16.4", "8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{prefix: "8.16"},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 16, Patch: 5},
			wantExist:   true,
		},
		{
			name:        "latest stack version patch",
			vs:          []string{"8.16.4", "8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{prefix: "8.17.1"},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 17, Patch: 1},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionInfos, err := newStackVersionInfosFromStrs(tt.vs)
			require.NoError(t, err)
			versionInfo, exist := versionInfos.LatestFor(tt.args.prefix)
			assert.Equal(t, tt.wantVersion, versionInfo.Version, "LatestFor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestFor() exist")
		})
	}

	t.Run("panic from error", func(t *testing.T) {
		versionInfos, err := newStackVersionInfosFromStrs([]string{"8.17.1", "8.17.2"})
		require.NoError(t, err)
		cases := []string{
			"abcdef",
			"8.abc",
			"9.0.123hello",
			"15832-gg-9123",
		}
		for _, c := range cases {
			assert.Panics(t, func() {
				_, _ = versionInfos.LatestFor(c)
			})
		}
	})
}

func TestStackVersionInfos_LatestForMajor(t *testing.T) {
	type args struct {
		major uint64
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion ecclient.StackVersion
		wantExist   bool
	}{
		{
			name:      "no stack versions",
			vs:        []string{},
			args:      args{major: 8},
			wantExist: false,
		},
		{
			name:      "no matching stack version",
			vs:        []string{"8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:      args{major: 9},
			wantExist: false,
		},
		{
			name:        "latest stack version",
			vs:          []string{"8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{major: 8},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 18, Patch: 0},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionInfos, err := newStackVersionInfosFromStrs(tt.vs)
			require.NoError(t, err)
			versionInfo, exist := versionInfos.LatestForMajor(tt.args.major)
			assert.Equal(t, tt.wantVersion, versionInfo.Version, "LatestForMajor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestForMajor() exist")
		})
	}
}

func TestStackVersionInfos_LatestForMinor(t *testing.T) {
	type args struct {
		major uint64
		minor uint64
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion ecclient.StackVersion
		wantExist   bool
	}{
		{
			name:      "no stack versions",
			vs:        []string{},
			args:      args{major: 8, minor: 17},
			wantExist: false,
		},
		{
			name:      "no matching stack version",
			vs:        []string{"8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:      args{major: 9, minor: 0},
			wantExist: false,
		},
		{
			name:        "latest stack version",
			vs:          []string{"8.16.5-SNAPSHOT", "8.17.1-SNAPSHOT", "8.17.2-SNAPSHOT", "8.17.3-SNAPSHOT", "8.18.0-SNAPSHOT"},
			args:        args{major: 8, minor: 17},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 17, Patch: 3, Suffix: "SNAPSHOT"},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionInfos, err := newStackVersionInfosFromStrs(tt.vs)
			require.NoError(t, err)
			versionInfo, exist := versionInfos.LatestForMinor(tt.args.major, tt.args.minor)
			assert.Equal(t, tt.wantVersion, versionInfo.Version, "LatestForMinor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestForMinor() exist")
		})
	}
}

func TestStackVersions_PreviousMinorLatest(t *testing.T) {
	type args struct {
		version string
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion ecclient.StackVersion
		wantExist   bool
	}{
		{
			name:        "minor is 0",
			vs:          []string{"4.11.0", "4.11.1", "4.11.2", "5.0.0"},
			args:        args{version: "5.0.0"},
			wantVersion: ecclient.StackVersion{Major: 4, Minor: 11, Patch: 2},
			wantExist:   true,
		},
		{
			name:        "minor is not 0",
			vs:          []string{"5.0.0", "5.0.1", "5.1.0"},
			args:        args{version: "5.1.0"},
			wantVersion: ecclient.StackVersion{Major: 5, Minor: 0, Patch: 1},
			wantExist:   true,
		},
		{
			name:      "minor does not exist",
			vs:        []string{"5.1.0"},
			args:      args{version: "5.1.0"},
			wantExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionInfos, err := newStackVersionInfosFromStrs(tt.vs)
			require.NoError(t, err)
			v, err := ecclient.NewStackVersionFromStr(tt.args.version)
			require.NoError(t, err)
			versionInfo, exist := versionInfos.PreviousMinorLatest(v)
			assert.Equal(t, tt.wantVersion, versionInfo.Version, "PreviousMinorLatest() version")
			assert.Equal(t, tt.wantExist, exist, "PreviousMinorLatest() exist")
		})
	}
}

func TestStackVersions_PreviousPatch(t *testing.T) {
	type args struct {
		version string
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion ecclient.StackVersion
		wantExist   bool
	}{
		{
			name:        "patch is 0",
			vs:          []string{"7.0.0", "7.0.1", "7.0.2", "7.1.0"},
			args:        args{version: "7.1.0"},
			wantVersion: ecclient.StackVersion{Major: 7, Minor: 0, Patch: 2},
			wantExist:   true,
		},
		{
			name:        "patch is not 0",
			vs:          []string{"8.1.0", "8.1.1", "8.1.2"},
			args:        args{version: "8.1.2"},
			wantVersion: ecclient.StackVersion{Major: 8, Minor: 1, Patch: 1},
			wantExist:   true,
		},
		{
			name:      "patch does not exist",
			vs:        []string{"8.1.2"},
			args:      args{version: "8.1.2"},
			wantExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionInfos, err := newStackVersionInfosFromStrs(tt.vs)
			require.NoError(t, err)
			v, err := ecclient.NewStackVersionFromStr(tt.args.version)
			require.NoError(t, err)
			versionInfo, exist := versionInfos.PreviousPatch(v)
			assert.Equal(t, tt.wantVersion, versionInfo.Version, "PreviousPatch() version")
			assert.Equal(t, tt.wantExist, exist, "PreviousPatch() exist")
		})
	}
}
