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

package asserts_test

import (
	"encoding/json"
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroESLogs(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name:     "with no logs",
			response: `{"hits":{"hits":[], "total": {"value": 0}}}`,
		},
		{
			name: "ignore org.elasticsearch.ingest.geoip.GeoIpDownloader",
			// this is not an entire response because it has been retrived before we added the logic to display it.
			response: `{"hits":{"hits":[{"_index":"","_source":{"agent":{"name":"df6e53b971fd","id":"7441fe9e-6f94-483b-881c-e218feeb8997","ephemeral_id":"8b18f265-3238-44c3-ac1a-2d51c31c532e","type":"filebeat","version":"8.16.0"},"process":{"thread":{"name":"elasticsearch[instance-0000000000][generic][T#14]"}},"log":{"file":{"path":"/app/logs/dcf3a6e1e25e4537a084c846786925c7_server.json"},"offset":713250,"level":"ERROR","logger":"org.elasticsearch.ingest.geoip.GeoIpDownloader"},"fileset":{"name":"server"},"message":"exception during geoip databases update","cloud":{"availability_zone":"eu-west-1b"},"input":{"type":"log"},"@timestamp":"2025-01-27T09:01:51.286Z","ecs":{"version":"1.2.0"},"elasticsearch":{"server":{"process":{"thread":{}},"log":{},"error.type":"org.elasticsearch.ElasticsearchException","ecs":{},"elasticsearch":{"cluster":{},"node":{}},"error.message":"not all primary shards of [.geoip_databases] index are active","service":{},"error.stack_trace":"org.elasticsearch.ElasticsearchException: not all primary shards of [.geoip_databases] index are active\n\tat org.elasticsearch.ingest.geoip@8.16.0/org.elasticsearch.ingest.geoip.GeoIpDownloader.updateDatabases(GeoIpDownloader.java:142)\n\tat org.elasticsearch.ingest.geoip@8.16.0/org.elasticsearch.ingest.geoip.GeoIpDownloader.runDownloader(GeoIpDownloader.java:294)\n\tat org.elasticsearch.ingest.geoip@8.16.0/org.elasticsearch.ingest.geoip.GeoIpDownloaderTaskExecutor.nodeOperation(GeoIpDownloaderTaskExecutor.java:165)\n\tat org.elasticsearch.ingest.geoip@8.16.0/org.elasticsearch.ingest.geoip.GeoIpDownloaderTaskExecutor.nodeOperation(GeoIpDownloaderTaskExecutor.java:64)\n\tat org.elasticsearch.server@8.16.0/org.elasticsearch.persistent.NodePersistentTasksExecutor$1.doRun(NodePersistentTasksExecutor.java:35)\n\tat org.elasticsearch.server@8.16.0/org.elasticsearch.common.util.concurrent.ThreadContext$ContextPreservingAbstractRunnable.doRun(ThreadContext.java:1023)\n\tat org.elasticsearch.server@8.16.0/org.elasticsearch.common.util.concurrent.AbstractRunnable.run(AbstractRunnable.java:27)\n\tat java.base/java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1144)\n\tat java.base/java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:642)\n\tat java.base/java.lang.Thread.run(Thread.java:1575)\n","event":{}},"cluster":{"name":"dcf3a6e1e25e4537a084c846786925c7","uuid":"M0sVrLQCRiWaVIkY6R4RpA"},"node":{"name":"instance-0000000000","id":"mb8RZp6TS6KhNboqoOd7xA"}},"service":{"node":{"name":"instance-0000000000"},"name":"ES_ECS","id":"edfcb0613ad26188c3a11c977a40970b","type":"elasticsearch","version":"8.16.0"},"host":{"name":"instance-0000000000","id":"mb8RZp6TS6KhNboqoOd7xA"},"event":{"ingested":"2025-01-27T09:03:51.502916073Z","created":"2025-01-27T09:01:57.494Z","kind":"event","module":"elasticsearch","category":"database","type":"error","dataset":"elasticsearch.server"},"deployment":{"name":"TestUpgrade_8_15_4_to_8_16_0-8.15.4"}}}],"total":{"relation":"","value":1}},"_shards":{"failed":0,"successful":0,"total":0},"timed_out":false,"took":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp search.Response
			require.NoError(t, json.Unmarshal([]byte(tt.response), &resp))

			asserts.ZeroESLogs(t, resp)
			// we only check that we running ZeroESLogs did not fail this test.
			assert.False(t, t.Failed())
		})
	}
}
