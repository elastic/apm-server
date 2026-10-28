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

package systemtest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// keystoreHome returns a path.home with the empty apm-server.yml that
// every keystore subcommand requires.
func keystoreHome(tb testing.TB) string {
	homedir := tb.TempDir()
	require.NoError(tb, os.WriteFile(filepath.Join(homedir, "apm-server.yml"), nil, 0644))
	return homedir
}

func keystoreCommand(homedir string, args ...string) *apmservertest.ServerCmd {
	allArgs := []string{"--path.home", homedir}
	allArgs = append(allArgs, args...)
	return apmservertest.ServerCommand(context.Background(), "keystore", allArgs...)
}

func keystoreList(tb testing.TB, homedir string) []string {
	out, err := keystoreCommand(homedir, "list").CombinedOutput()
	require.NoError(tb, err, string(out))
	return strings.Fields(string(out))
}

func TestKeystoreAddFromDir(t *testing.T) {
	homedir := keystoreHome(t)

	secrets := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(secrets, "some-secret"), []byte("tok-123"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(secrets, "some.elasticsearch.password"), []byte("password\n"), 0600))

	out, err := keystoreCommand(homedir, "add", "--from-dir", secrets).CombinedOutput()
	require.NoError(t, err, string(out))

	// keystore list iterates a map, so the order is not deterministic.
	assert.ElementsMatch(t, []string{"some-secret", "some.elasticsearch.password"}, keystoreList(t, homedir))
}

func TestKeystoreAddStdin(t *testing.T) {
	homedir := keystoreHome(t)

	cmd := keystoreCommand(homedir, "add", "secret_token", "--stdin")
	cmd.Stdin = strings.NewReader("tok-123")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	assert.Equal(t, []string{"secret_token"}, keystoreList(t, homedir))
}
