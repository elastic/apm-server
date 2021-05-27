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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/systemtest/estest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestDataStreamsEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			systemtest.CleanupElasticsearch(t)
			srv := apmservertest.NewUnstartedServer(t)
			if enabled {
				// Enable data streams.
				srv.Config.DataStreams = &apmservertest.DataStreamsConfig{Enabled: true}
				srv.Config.Setup = nil

				// Create a data stream index template.
				resp, err := systemtest.Elasticsearch.Indices.PutIndexTemplate("apm-data-streams", strings.NewReader(fmt.Sprintf(`{
				  "index_patterns": ["traces-apm*", "logs-apm*", "metrics-apm*"],
				  "data_stream": {},
				  "priority": 200,
				  "template": {"settings": {"number_of_shards": 1, "refresh_interval": "250ms"}}
				}`)))
				require.NoError(t, err)
				body, _ := ioutil.ReadAll(resp.Body)
				require.False(t, resp.IsError(), string(body))

				// Create an API Key which can write to traces-* etc.
				// The default APM Server user can only write to apm-*.
				//
				// NOTE(axw) importantly, this API key lacks privileges
				// to manage templates, pipelines, ILM, etc. Enabling
				// data streams should disable all automatic setup.
				resp, err = systemtest.Elasticsearch.Security.CreateAPIKey(strings.NewReader(fmt.Sprintf(`{
				  "name": "%s",
				  "expiration": "1h",
				  "role_descriptors": {
				    "write-apm-data": {
				      "cluster": ["monitor"],
				      "index": [
				        {
				          "names": ["traces-*", "metrics-*", "logs-*"],
					  "privileges": ["write", "create_index"]
				        }
				      ]
				    }
				  }
				}`, t.Name())))
				require.NoError(t, err)

				var apiKeyResponse struct {
					ID     string
					Name   string
					APIKey string `json:"api_key"`
				}
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiKeyResponse))

				// Use an API Key to mimic running under Fleet, with limited permissions.
				srv.Config.Output.Elasticsearch.Username = ""
				srv.Config.Output.Elasticsearch.Password = ""
				srv.Config.Output.Elasticsearch.APIKey = fmt.Sprintf("%s:%s", apiKeyResponse.ID, apiKeyResponse.APIKey)
			}
			require.NoError(t, srv.Start())

			tracer := srv.Tracer()
			tx := tracer.StartTransaction("name", "type")
			tx.Duration = time.Second
			tx.End()
			tracer.Flush(nil)

			result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*,traces-apm*", estest.TermQuery{
				Field: "processor.event", Value: "transaction",
			})
			systemtest.ApproveEvents(
				t, t.Name(), result.Hits.Hits,
				"@timestamp", "timestamp.us",
				"trace.id", "transaction.id",
			)

			// There should be no warnings or errors logged.
			for _, record := range srv.Logs.All() {
				assert.Condition(t, func() bool {
					if record.Level == zapcore.ErrorLevel {
						return assert.Equal(t, "Started apm-server with data streams enabled but no active fleet management mode was specified", record.Message)
					}
					return record.Level < zapcore.WarnLevel
				}, "%s: %s", record.Level, record.Message)
			}
		})
	}
}

func TestDataStreamsSetupErrors(t *testing.T) {
	cfg := apmservertest.DefaultConfig()
	cfg.DataStreams = &apmservertest.DataStreamsConfig{Enabled: true}
	cfgargs, err := cfg.Args()
	require.NoError(t, err)

	test := func(args []string, expected string) {
		args = append(args, cfgargs...)
		cmd := apmservertest.ServerCommand("setup", args...)
		out, err := cmd.CombinedOutput()
		require.Error(t, err)
		assert.Equal(t, "Exiting: "+expected+"\n", string(out))
	}

	test([]string{}, "index setup must be performed externally when using data streams, by installing the 'apm' integration package")
	test([]string{"--index-management"}, "index setup must be performed externally when using data streams, by installing the 'apm' integration package")
	test([]string{"--pipelines"}, "index pipeline setup must be performed externally when using data streams, by installing the 'apm' integration package")
}
