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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func exportConfigCommand(t *testing.T, args ...string) (_ *apmservertest.ServerCmd, homedir string) {
	tempdir, err := ioutil.TempDir("", "systemtest")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempdir) })
	err = ioutil.WriteFile(filepath.Join(tempdir, "apm-server.yml"), nil, 0644)
	require.NoError(t, err)

	allArgs := []string{"config", "--path.home", tempdir}
	allArgs = append(allArgs, args...)
	return apmservertest.ServerCommand("export", allArgs...), tempdir
}

func TestExportConfigDefaults(t *testing.T) {
	cmd, tempdir := exportConfigCommand(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	expectedConfig := strings.ReplaceAll(`
logging:
  ecs: true
  json: true
  metrics:
    enabled: false
path:
  config: /home/apm-server
  data: /home/apm-server/data
  home: /home/apm-server
  logs: /home/apm-server/logs
`[1:], "/home/apm-server", tempdir)
	assert.Equal(t, expectedConfig, string(out))
}

func TestExportConfigOverrideDefaults(t *testing.T) {
	cmd, tempdir := exportConfigCommand(t,
		"-E", "logging.metrics.enabled=true",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	expectedConfig := strings.ReplaceAll(`
logging:
  ecs: true
  json: true
  metrics:
    enabled: true
path:
  config: /home/apm-server
  data: /home/apm-server/data
  home: /home/apm-server
  logs: /home/apm-server/logs
`[1:], "/home/apm-server", tempdir)
	assert.Equal(t, expectedConfig, string(out))
}
