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

package kibana

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestFlatten(t *testing.T) {
	tlsFieldsCount := 0
	cc, err := common.NewConfigWithYAML([]byte(serverYAML), "apm-server.yml")
	require.NoError(t, err)

	flat, err := flattenAndClean(cc)
	assert.NoError(t, err)

	for k := range flat {
		assert.NotContains(t, k, "elasticsearch")
		assert.NotContains(t, k, "kibana")
		assert.NotContains(t, k, "instrumentation")
		if strings.HasPrefix(k, "apm-server.ssl.") {
			switch k[15:] {
			case "enabled", "certificate", "key":
				tlsFieldsCount += 1
			default:
				assert.Fail(t, fmt.Sprintf("should not be present: %s", k))
			}
		}
	}
	assert.Equal(t, 3, tlsFieldsCount)
}

var serverYAML = `apm-server:
  host: "localhost:8200"
  auth:
    api_key:
      enabled: true
      limit: 100
  max_header_size: 1048576
  idle_timeout: 45s
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 5s
  ssl:
    enabled: true
    key: 'my-key'
    certificate: 'my-cert'
    key_passphrase: 'pass-phrase'
    verify_mode: 'strict'
  rum:
    enabled: false
    event_rate:
      limit: 300
      lru_size: 1000
output.elasticsearch:
  hosts: ["localhost:9200"]
  enabled: true
  compression_level: 0
  protocol: "https"
  username: "elastic"
  password: "changeme"
  worker: 1
`
