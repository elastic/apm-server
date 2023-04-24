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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"

	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/internal/profiling"
)

var (
	// Used in certain gRPC calls to enable compression
	gzipOption = grpc.UseCompressor(gzip.Name)
)

func TestProfiling(t *testing.T) {
	cleanupProfiling := func() {
		_, err := systemtest.Elasticsearch.Do(
			context.Background(),
			&esapi.IndicesDeleteRequest{Index: []string{"profiling-*"}},
			nil,
		)
		require.NoError(t, err)
	}
	cleanupProfiling()
	defer cleanupProfiling()
	defer systemtest.InvalidateAPIKeys(t)

	// APM Server is running in standalone mode, which means
	// it is using the default elasticsearch credentials with narrow
	// privileges. We must inject an additional API Key for
	// the profiling code to write to profiling-* indices.
	var apiKey struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
	}
	_, err := systemtest.Elasticsearch.Do(
		context.Background(),
		&esapi.SecurityCreateAPIKeyRequest{Body: strings.NewReader(`
{
  "name": "systemtest_profiling",
  "role_descriptors": {
    "profiling_ingest": {
      "indices": [
        {
	  "names": ["profiling-*"],
	  "privileges": ["all"]
	}
      ]
    }
  }
}`)}, &apiKey)
	require.NoError(t, err)

	const secretToken = "test_token"

	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Profiling = &apmservertest.ProfilingConfig{
		Enabled: true,
		ESConfig: &apmservertest.ElasticsearchOutputConfig{
			APIKey: apiKey.ID + ":" + apiKey.APIKey,
		},
		MetricsESConfig: &apmservertest.ElasticsearchOutputConfig{
			Hosts:  []string{"localhost:9200"},
			APIKey: apiKey.ID + ":" + apiKey.APIKey,
		},
	}
	require.NoError(t, srv.Start())

	apmServerURL, _ := url.Parse(srv.URL)

	dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, apmServerURL.Host,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	client := profiling.NewCollectionAgentClient(conn)

	// proper authenticated context
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		"secretToken", secretToken,
		"projectID", "123",
		"hostID", "abc123",
		"rpcVersion", "2",
	)

	// We always insert 2 elements in KV indices for each test RPC below.
	// All RPCs use a columnar format, where arrays of fields are bundled
	// by their index into Elasticsearch documents
	const expectedKVDocs = 2

	_, err = client.AddExecutableMetadata(ctx, &profiling.AddExecutableMetadataRequest{
		HiFileIDs: []uint64{1, 2},
		LoFileIDs: []uint64{2, 1},
		Filenames: []string{"a.apl", "b.bas"},
		BuildIDs:  []string{"build1", "build2"},
	}, gzipOption)
	require.NoError(t, err)

	_, err = client.SetFramesForTraces(ctx, &profiling.SetFramesForTracesRequest{
		HiTraceHashes: []uint64{1, 2},
		LoTraceHashes: []uint64{3, 4},
		FrameCounts:   []uint32{1, 2},
		Types:         []uint32{7, 8, 9},
		HiContainers:  []uint64{9, 10, 11},
		LoContainers:  []uint64{11, 12, 13},
		Offsets:       []uint64{15, 16, 17},
	}, gzipOption)
	require.NoError(t, err)

	_, err = client.AddFrameMetadata(ctx, &profiling.AddFrameMetadataRequest{
		HiFileIDs:        []uint64{1, 2},
		LoFileIDs:        []uint64{3, 4},
		AddressOrLines:   []uint64{5, 6},
		HiSourceIDs:      []uint64{7, 8},  // NOTE(axw) unused?
		LoSourceIDs:      []uint64{9, 10}, // NOTE(axw) unused?
		LineNumbers:      []uint64{11, 12},
		FunctionNamesIdx: []uint32{0, 1},
		FunctionOffsets:  []uint32{15, 16},
		Types:            []uint32{17, 18},
		FilenamesIdx:     []uint32{2, 3},
		StringTable:      []string{"thirteen()", "fourteen()", "ninete.en", "twen.ty"},
	}, gzipOption)
	require.NoError(t, err)

	_, err = client.AddCountsForTraces(ctx, &profiling.AddCountsForTracesRequest{
		Timestamps:        []uint64{123},
		HiTraceHashes:     []uint64{1},
		LoTraceHashes:     []uint64{2},
		Counts:            []uint32{3},
		CommsIdx:          []uint32{0},
		PodNamesIdx:       map[uint32]uint32{0: 1},
		ContainerNamesIdx: map[uint32]uint32{0: 2},
		StringTable:       []string{"comm", "pod", "container"},
	}, gzipOption)
	require.NoError(t, err)

	// Perform queries and assertions on KV indices. We use wildcard searches below to prevent
	// the searches from failing immediately when the indices haven't yet been created.
	result := systemtest.Elasticsearch.ExpectMinDocs(t, expectedKVDocs, "profiling-executables-next*", nil)
	systemtest.ApproveEvents(t, t.Name()+"/executables", result.Hits.Hits, "@timestamp")

	result = systemtest.Elasticsearch.ExpectMinDocs(t, expectedKVDocs, "profiling-stackframes-next*", nil)
	systemtest.ApproveEvents(t, t.Name()+"/stackframes", result.Hits.Hits)

	result = systemtest.Elasticsearch.ExpectMinDocs(t, expectedKVDocs, "profiling-stacktraces-next*", nil)
	systemtest.ApproveEvents(t, t.Name()+"/stacktraces", result.Hits.Hits)

	result = systemtest.Elasticsearch.ExpectDocs(t, "profiling-events-all*", nil)
	systemtest.ApproveEvents(t, t.Name()+"/events", result.Hits.Hits)

	_, err = client.AddMetrics(ctx, &profiling.Metrics{
		TsMetrics: []*profiling.TsMetric{
			{Timestamp: 111, IDs: []uint32{0 /* should be omitted */, 1, 2}, Values: []int64{3, 4, 5}},
			{Timestamp: 222, IDs: []uint32{6, 7, 8}, Values: []int64{0 /* should be omitted */, 9, 10}},
		},
	}, gzipOption)
	require.NoError(t, err)
	result = systemtest.Elasticsearch.ExpectMinDocs(t, 2, "profiling-metrics*", nil)
	systemtest.ApproveEvents(t, t.Name()+"/metrics", result.Hits.Hits)
}
