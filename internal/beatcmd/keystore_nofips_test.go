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

//go:build !requirefips

package beatcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/keystore"
)

func seedKeystore(t *testing.T, path string, secrets map[string]string) keystore.Keystore {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	ks, err := keystore.NewFileKeystore(path)
	require.NoError(t, err)
	wks, err := keystore.AsWritableKeystore(ks)
	require.NoError(t, err)
	require.NoError(t, wks.Create(true))
	for key, value := range secrets {
		require.NoError(t, wks.Store(key, []byte(value)))
	}
	require.NoError(t, wks.Save())
	return ks
}

func writeSecretFiles(t *testing.T, dir string, secrets map[string]string) {
	t.Helper()
	for name, value := range secrets {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600))
	}
}

// setStdin replaces os.Stdin with a file containing content for the test.
func setStdin(t *testing.T, content string) {
	t.Helper()
	stdin, err := os.CreateTemp(t.TempDir(), "stdin")
	require.NoError(t, err)
	_, err = stdin.WriteString(content)
	require.NoError(t, err)
	_, err = stdin.Seek(0, io.SeekStart)
	require.NoError(t, err)

	origStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = origStdin
		// Close the file before t.TempDir removes it: Windows refuses
		// to delete a file that still has an open handle.
		stdin.Close()
	})
}

func assertPersistedSecrets(t *testing.T, path string, expected map[string]string) {
	t.Helper()
	ks, err := keystore.NewFileKeystore(path)
	require.NoError(t, err)
	for key, value := range expected {
		secret, err := ks.Retrieve(key)
		require.NoError(t, err, "key %q", key)
		got, err := secret.Get()
		require.NoError(t, err)
		assert.Equal(t, value, string(got), "key %q", key)
	}
	listing, err := keystore.AsListingKeystore(ks)
	require.NoError(t, err)
	keys, err := listing.List()
	require.NoError(t, err)
	assert.Len(t, keys, len(expected))
}

func TestGetKeysFromDir(t *testing.T) {
	tests := []struct {
		name     string
		files    []string          // empty files to create
		dirs     []string          // subdirectories to create
		symlinks map[string]string // symlink name -> target
		path     string            // entry to list instead of the directory itself
		want     []string
		wantErr  string
	}{
		{
			name:  "regular files sorted by name",
			files: []string{"b", "a", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "subdirectories are skipped",
			files: []string{"a", "sub/nested"},
			dirs:  []string{"sub"},
			want:  []string{"a"},
		},
		{
			name:     "hidden entries are skipped",
			files:    []string{"a", ".b", "..c"},
			dirs:     []string{".d"},
			symlinks: map[string]string{"..e": ".d"},
			want:     []string{"a"},
		},
		{
			name:     "symlinks to files are included",
			files:    []string{"a"},
			symlinks: map[string]string{"b": "a"},
			want:     []string{"a", "b"},
		},
		{
			name: "empty directory",
			want: nil,
		},
		{
			name:     "symlinks to directories are skipped",
			files:    []string{"a"},
			dirs:     []string{"sub"},
			symlinks: map[string]string{"link": "sub"},
			want:     []string{"a"},
		},
		{
			name: "only subdirectories",
			dirs: []string{"sub"},
			want: nil,
		},
		{
			name:    "missing directory",
			path:    "missing",
			wantErr: "could not read directory",
		},
		{
			name:    "path is a file",
			files:   []string{"a"},
			path:    "a",
			wantErr: "could not read directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, d := range tt.dirs {
				require.NoError(t, os.Mkdir(filepath.Join(dir, d), 0o700))
			}
			for _, f := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, f), nil, 0o600))
			}
			for name, target := range tt.symlinks {
				require.NoError(t, os.Symlink(target, filepath.Join(dir, name)))
			}

			keys, err := getKeysFromDir(filepath.Join(dir, tt.path))

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, keys)
		})
	}
}

// TestAddKeystoreCmd drives run against the keystore LoadConfig resolves from
// a temporary path.home, and asserts on the keystore file afterwards. run is
// invoked directly rather than through cli.RunWith, which exits the process
// on error.
func TestAddKeystoreCmd(t *testing.T) {
	dirWithOnlySubdir := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
		return dir
	}
	missingDir := func(t *testing.T) string {
		return filepath.Join(t.TempDir(), "missing")
	}

	tests := []struct {
		name     string
		seed     map[string]string         // existing keystore entries; nil means no keystore
		files    map[string]string         // written to a new directory passed as --from-dir
		fromDir  func(t *testing.T) string // alternative to files: builds the --from-dir path
		stdin    string                    // fed to --stdin
		args     []string
		force    bool
		wantErr  string
		expected map[string]string // keystore contents afterwards; nil means the keystore must not exist
	}{
		{
			name:     "from-dir creates keystore",
			files:    map[string]string{"a": "1", "b": "2\n"},
			expected: map[string]string{"a": "1", "b": "2\n"},
		},
		{
			name:     "from-dir adds to existing keystore",
			seed:     map[string]string{"a": "1"},
			files:    map[string]string{"b": "2"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "from-dir conflict without force does not overwrite existing values",
			seed:     map[string]string{"b": "old"},
			files:    map[string]string{"a": "1", "b": "new"},
			wantErr:  "the settings b already exist",
			expected: map[string]string{"b": "old"},
		},
		{
			name:     "from-dir conflict with force overwrites",
			seed:     map[string]string{"a": "old"},
			files:    map[string]string{"a": "new", "b": "2"},
			force:    true,
			expected: map[string]string{"a": "new", "b": "2"},
		},
		{
			name:    "from-dir with key argument",
			files:   map[string]string{"a": "1"},
			args:    []string{"a"},
			wantErr: "cannot be combined with a key argument",
		},
		{
			name:    "from-dir with stdin",
			files:   map[string]string{"a": "1"},
			stdin:   "secret",
			wantErr: "cannot be combined with --stdin",
		},
		{
			name:    "from-dir with no files",
			fromDir: dirWithOnlySubdir,
			wantErr: "is empty",
		},
		{
			name:    "from-dir missing",
			fromDir: missingDir,
			wantErr: "could not read directory",
		},
		{
			name:     "stdin creates keystore and trims the key",
			stdin:    "secret\n",
			args:     []string{" token "},
			expected: map[string]string{"token": "secret\n"},
		},
		{
			name:     "stdin conflict without force does not overwrite values",
			seed:     map[string]string{"token": "old"},
			stdin:    "new",
			args:     []string{"token"},
			wantErr:  "the settings token already exist",
			expected: map[string]string{"token": "old"},
		},
		{
			name:     "stdin conflict with force overwrites",
			seed:     map[string]string{"token": "old"},
			stdin:    "new",
			args:     []string{"token"},
			force:    true,
			expected: map[string]string{"token": "new"},
		},
		{
			name:    "stdin without key",
			stdin:   "secret",
			wantErr: "no key provided",
		},
		{
			name:    "no key",
			wantErr: "no key provided",
		},
		{
			name:    "more than one key",
			args:    []string{"a", "b"},
			wantErr: "only provide one key per invocation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := initCfgfile(t, "")
			ksPath := filepath.Join(home, "data", "apm-server.keystore")
			if tt.seed != nil {
				seedKeystore(t, ksPath, tt.seed)
			}

			c := &addKeystoreCmd{force: tt.force}
			switch {
			case tt.fromDir != nil:
				c.fromDir = tt.fromDir(t)
			case tt.files != nil:
				c.fromDir = t.TempDir()
				writeSecretFiles(t, c.fromDir, tt.files)
			}
			if tt.stdin != "" {
				c.stdin = true
				setStdin(t, tt.stdin)
			}

			err := c.run(nil, tt.args)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.expected == nil {
				_, err := os.Stat(ksPath)
				require.ErrorIs(t, err, os.ErrNotExist, "keystore must not be created")
				return
			}
			assertPersistedSecrets(t, ksPath, tt.expected)
		})
	}
}
