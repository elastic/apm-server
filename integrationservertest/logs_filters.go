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

package integrationservertest

import "github.com/elastic/go-elasticsearch/v8/typedapi/types"

// These vars are Elasticsearch query matchers to filter out some specific
// log lines from APM Server logs.
// To be used to filter the APM Server logs in test cases.
type (
	apmErrorLog  types.Query
	apmErrorLogs []apmErrorLog
)

func (e apmErrorLogs) ToQueries() []types.Query {
	queries := make([]types.Query, 0, len(e))
	for _, entry := range e {
		queries = append(queries, types.Query(entry))
	}
	return queries
}

var (
	tlsHandshakeError = apmErrorLog(types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "http: TLS handshake error from 127.0.0.1:"},
		},
	})

	esReturnedUnknown503 = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "ES returned unknown status code: 503 Service Unavailable"},
		},
	})

	grpcServerStopped = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "grpc: the server has been stopped"},
		},
	})

	preconditionFailed = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "precondition failed: context canceled"},
		},
	})
	preconditionClusterInfoCtxCanceled = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "precondition failed: failed to query cluster info: context canceled"},
		},
	})

	populateSourcemapServerShuttingDown = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "failed to populate sourcemap metadata: failed to run initial search query: fetcher unavailable: server shutting down"},
		},
	})
	populateSourcemapFetcher403 = apmErrorLog(types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "failed to populate sourcemap metadata: fetcher unavailable: 403 Forbidden:"},
		},
	})
	syncSourcemapFetcher403 = apmErrorLog(types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "failed to sync sourcemaps metadata: fetcher unavailable: 403 Forbidden:"},
		},
	})

	refreshCache403 = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache elasticsearch returned status 403"},
		},
	})
	refreshCacheESConfigInvalid = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "stopping refresh cache background job: elasticsearch config is invalid"},
		},
	})
	refreshCache503 = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache elasticsearch returned status 503"},
		},
	})
	refreshCacheCtxDeadline = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context deadline exceeded"},
		},
	})
	refreshCacheCtxCanceled = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context canceled"},
		},
	})

	initialSearchQueryContextCanceled = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "failed to sync sourcemaps metadata: failed to run initial search query: fetcher unavailable: context canceled"},
		},
	})

	scrollSearchQueryContextCanceled = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "failed to sync sourcemaps metadata: failed scroll search: failed to run scroll search query: context canceled"},
		},
	})

	waitServerReadyCtxCanceled = apmErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "error waiting for server to be ready: context canceled"},
		},
	})
)

// These vars are Elasticsearch query matchers to filter out some specific
// log lines from Elasticsearch logs.
// To be used to filter the Elasticsearch logs in test cases.
type (
	esErrorLog  types.Query
	esErrorLogs []esErrorLog
)

func (e esErrorLogs) ToQueries() []types.Query {
	queries := make([]types.Query, 0, len(e))
	for _, entry := range e {
		queries = append(queries, types.Query(entry))
	}
	return queries
}

var (
	// Safe to ignore: https://github.com/elastic/elasticsearch/pull/97301.
	eventLoopShutdown = esErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "Failed to submit a listener notification task. Event loop shut down?"},
		},
	})

	addIndexTemplateTracesError = esErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "error adding index template [traces-apm@mappings] for [apm]"},
		},
	})
)
