package ecclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackVersions_LatestFor(t *testing.T) {
	type args struct {
		prefix string
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion StackVersion
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
			wantVersion: StackVersion{Major: 8, Minor: 18, Patch: 0},
			wantExist:   true,
		},
		{
			name:        "latest stack version minor",
			vs:          []string{"8.16.4", "8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{prefix: "8.16"},
			wantVersion: StackVersion{Major: 8, Minor: 16, Patch: 5},
			wantExist:   true,
		},
		{
			name:        "latest stack version patch",
			vs:          []string{"8.16.4", "8.16.5", "8.17.1", "8.17.2", "8.17.3", "8.18.0"},
			args:        args{prefix: "8.17.1"},
			wantVersion: StackVersion{Major: 8, Minor: 17, Patch: 1},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := NewStackVersionsFromStrs(tt.vs)
			require.NoError(t, err)
			version, exist := versions.LatestFor(tt.args.prefix)
			assert.Equal(t, tt.wantVersion, version, "LatestFor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestFor() exist")
		})
	}

	t.Run("panic from error", func(t *testing.T) {
		versions, err := NewStackVersionsFromStrs([]string{"8.17.1", "8.17.2"})
		require.NoError(t, err)
		assert.Panics(t, func() {
			_, _ = versions.LatestFor("abcdef")
		})
	})
}

func TestStackVersions_LatestForMajor(t *testing.T) {
	type args struct {
		major uint
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion StackVersion
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
			wantVersion: StackVersion{Major: 8, Minor: 18, Patch: 0},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := NewStackVersionsFromStrs(tt.vs)
			require.NoError(t, err)
			version, exist := versions.LatestForMajor(tt.args.major)
			assert.Equal(t, tt.wantVersion, version, "LatestForMajor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestForMajor() exist")
		})
	}
}

func TestStackVersions_LatestForMinor(t *testing.T) {
	type args struct {
		major uint
		minor uint
	}
	tests := []struct {
		name        string
		vs          []string
		args        args
		wantVersion StackVersion
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
			wantVersion: StackVersion{Major: 8, Minor: 17, Patch: 3, Suffix: "SNAPSHOT"},
			wantExist:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := NewStackVersionsFromStrs(tt.vs)
			require.NoError(t, err)
			version, exist := versions.LatestForMinor(tt.args.major, tt.args.minor)
			assert.Equal(t, tt.wantVersion, version, "LatestForMinor() version")
			assert.Equal(t, tt.wantExist, exist, "LatestForMinor() exist")
		})
	}
}
