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
	"github.com/elastic/go-ucfg"
)

func TestFlattenAndFormat(t *testing.T) {
	tlsFieldsCount, loggingFieldCount := 0, 0
	cc, err := common.NewConfigWithYAML([]byte(serverYAML), "apm-server.yml")
	c := ucfg.Config(*cc)
	require.NoError(t, err)

	flat, err := flattenAndClean(&c)
	assert.NoError(t, err)

	flat = format(flat)
	assert.Contains(t, flat, "schema")

	flat = flat["schema"].(map[string]interface{})
	for k, v := range flat {
		assert.NotContains(t, k, "elasticsearch")
		assert.NotContains(t, k, "kibana")
		assert.NotContains(t, k, "instrumentation")
		assert.NotContains(t, k, "path.")
		assert.NotEqual(t, k, "name")
		assert.NotContains(t, k, "gc_percent")
		assert.NotContains(t, k, "xpack.monitoring.enabled")
		if strings.HasPrefix(k, "logging.") {
			switch k {
			case "logging.level", "logging.selectors", "logging.metrics.enabled", "logging.metrics.period":
				loggingFieldCount++
			default:
				assert.Fail(t, fmt.Sprintf("should not be present: %s", k))
			}
		}
		if k == "apm-server.host" {
			assert.Equal(t, "0.0.0.0:8200", v)
		}
		if strings.HasPrefix(k, "apm-server.ssl.") {
			switch k[15:] {
			case "enabled", "certificate", "key":
				tlsFieldsCount++
			default:
				assert.Fail(t, fmt.Sprintf("should not be present: %s", k))
			}
		}
	}
	assert.Equal(t, 3, tlsFieldsCount)
	assert.Equal(t, 4, loggingFieldCount)
	assert.Contains(t, flat, "apm-server.rum.event_rate.limit")
	assert.Contains(t, flat, "apm-server.rum.event_rate.lru_size")
}

var serverYAML = `apm-server:
  kibana:
    enabled: true
    api_key: abc123
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
  name: 'test-name'
  rum:
    enabled: false
    event_rate:
      lru_size: 1000
    rate_limit: 300
gc_percent: 70
logging:
  level: 'debug'
  selectors: ['intake']
  metrics:
    enabled: true
    period: 10s
  files.name: "apm.log"
  json: true
path.config: "/app/config"
path.data: "/app/data"
path:
  home: "/app/"
xpack.monitoring.enabled: true
output.elasticsearch:
  hosts: ["localhost:9200"]
  enabled: true
  compression_level: 0
  protocol: "https"
  username: "elastic"
  password: "changeme"
  worker: 1
instrumentation:
  enabled: false
  environment: ""
  hosts:
  - http://remote-apm-server:8200
`
