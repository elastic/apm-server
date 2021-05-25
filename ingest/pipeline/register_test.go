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

package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"

	"github.com/elastic/apm-server/tests/loader"
)

func TestRegisterPipelines(t *testing.T) {
	esClients, err := eslegclient.NewClients(getFakeESConfig(9200))
	require.NoError(t, err)
	esClient := &esClients[0]
	path, err := filepath.Abs("definition.json")
	require.NoError(t, err)

	// pipeline loading goes wrong
	err = RegisterPipelines(esClient, true, "non-existing")
	assert.Error(t, err)
	assertContainsErrMsg(t, err.Error(), []string{"cannot find the file", "no such file or directory"})

	// pipeline definition empty
	emptyPath, err := filepath.Abs(filepath.FromSlash("../../testdata/ingest/pipeline/empty.json"))
	require.NoError(t, err)
	err = RegisterPipelines(esClient, true, emptyPath)
	assert.NoError(t, err)

	// invalid esClient
	invalidClients, err := eslegclient.NewClients(getFakeESConfig(1234))
	require.NoError(t, err)
	err = RegisterPipelines(&invalidClients[0], true, path)
	assert.Error(t, err)
	assertContainsErrMsg(t, err.Error(), []string{"connect: cannot assign requested address", "connection refused"})
}

func getFakeESConfig(port int) *common.Config {
	cfg := map[string]interface{}{
		"hosts": []string{fmt.Sprintf("http://localhost:%v", port)},
	}
	c, _ := common.NewConfigFrom(cfg)
	return c
}

func assertContainsErrMsg(t *testing.T, errMsg string, msgs []string) {
	var found bool
	for _, msg := range msgs {
		if strings.Contains(errMsg, msg) {
			found = true
			break
		}
	}
	assert.NotNil(t, found)
}
