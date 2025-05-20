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

package functionaltests

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/steps"
)

/* APM error logs */

var (
	TLSHandshakeError = steps.APMErrorLog(types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "http: TLS handshake error from 127.0.0.1:"},
		},
	})

	ESReturnedUnknown503 = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "ES returned unknown status code: 503 Service Unavailable"},
		},
	})

	GRPCServerStopped = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "grpc: the server has been stopped"},
		},
	})

	PreconditionFailed = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "precondition failed: context canceled"},
		},
	})
	PreconditionClusterInfoCtxCanceled = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "precondition failed: failed to query cluster info: context canceled"},
		},
	})

	PopulateSourcemapServerShuttingDown = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "failed to populate sourcemap metadata: failed to run initial search query: fetcher unavailable: server shutting down"},
		},
	})
	PopulateSourcemapFetcher403 = steps.APMErrorLog(types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "failed to populate sourcemap metadata: fetcher unavailable: 403 Forbidden:"},
		},
	})

	RefreshCache403 = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache elasticsearch returned status 403"},
		},
	})
	RefreshCacheESConfigInvalid = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "stopping refresh cache background job: elasticsearch config is invalid"},
		},
	})
	RefreshCache503 = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache elasticsearch returned status 503"},
		},
	})
	RefreshCacheCtxDeadline = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context deadline exceeded"},
		},
	})
	RefreshCacheCtxCanceled = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context canceled"},
		},
	})

	WaitServerReadyCtxCanceled = steps.APMErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "error waiting for server to be ready: context canceled"},
		},
	})
)

/* ES error logs */

var (
	// Safe to ignore: https://github.com/elastic/elasticsearch/pull/97301.
	EventLoopShutdown = steps.ESErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "Failed to submit a listener notification task. Event loop shut down?"},
		},
	})

	AddIndexTemplateTracesError = steps.ESErrorLog(types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "error adding index template [traces-apm@mappings] for [apm]"},
		},
	})
)
