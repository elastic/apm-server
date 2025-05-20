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

package steps

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

// These are Elasticsearch query matchers to filter out some specific
// log lines from APM Server logs.
// To be used to filter the APM Server logs in test cases.
type (
	APMErrorLog  types.Query
	APMErrorLogs []APMErrorLog
)

func (e APMErrorLogs) ToQueries() []types.Query {
	queries := make([]types.Query, 0, len(e))
	for _, entry := range e {
		queries = append(queries, types.Query(entry))
	}
	return queries
}

// These are Elasticsearch query matchers to filter out some specific
// log lines from Elasticsearch logs.
// To be used to filter the Elasticsearch logs in test cases.
type (
	ESErrorLog  types.Query
	ESErrorLogs []ESErrorLog
)

func (e ESErrorLogs) ToQueries() []types.Query {
	queries := make([]types.Query, 0, len(e))
	for _, entry := range e {
		queries = append(queries, types.Query(entry))
	}
	return queries
}

// getAPMDataStreams get all APM related data streams.
func getAPMDataStreams(t *testing.T, ctx context.Context, esc *esclient.Client, ignoreDS ...string) []types.DataStream {
	t.Helper()
	dataStreams, err := esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)

	ignore := sliceToSet(ignoreDS)
	return slices.DeleteFunc(dataStreams, func(ds types.DataStream) bool {
		return ignore[ds.Name]
	})
}

// getDocCountPerDS retrieves document count per data stream for Versions >= 8.0.
func getDocCountPerDS(t *testing.T, ctx context.Context, esc *esclient.Client, ignoreDS ...string) esclient.DataStreamsDocCount {
	t.Helper()
	count, err := esc.APMDSDocCount(ctx)
	require.NoError(t, err)

	ignore := sliceToSet(ignoreDS)
	maps.DeleteFunc(count, func(ds string, _ int) bool {
		return ignore[ds]
	})
	return count
}

// getDocCountPerDS retrieves document count per data stream for Versions < 8.0.
func getDocCountPerDSV7(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) esclient.DataStreamsDocCount {
	t.Helper()
	count, err := esc.APMDSDocCountV7(ctx, namespace)
	require.NoError(t, err)
	return count
}

// getDocCountPerIndexV7 retrieves document count per index for Versions < 8.0.
func getDocCountPerIndexV7(t *testing.T, ctx context.Context, esc *esclient.Client) esclient.IndicesDocCount {
	t.Helper()
	count, err := esc.APMIdxDocCountV7(ctx)
	require.NoError(t, err)
	return count
}

func allDataStreams(namespace string) []string {
	return slices.Collect(maps.Keys(expectedDataStreamsIngest(namespace)))
}
