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

import "github.com/elastic/go-elasticsearch/v8/typedapi/types"

// These vars are Elasticsearch query matchers to filter out some specific
// log lines from APM Server logs.
// To be used to filter the APM Server logs in test cases.
var (
	tlsHandshakeError = types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "http: TLS handshake error from 127.0.0.1:"},
		},
	}

	esReturnedUnknown503 = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "ES returned unknown status code: 503 Service Unavailable"},
		},
	}

	refreshCache503 = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache elasticsearch returned status 503"},
		},
	}
	refreshCacheCtxDeadline = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context deadline exceeded"},
		},
	}
	refreshCacheCtxCanceled = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "refresh cache error: context canceled"},
		},
	}

	preconditionFailed = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "precondition failed: context canceled"},
		},
	}

	populateSourcemapServerShuttingDown = types.Query{
		MatchPhrase: map[string]types.MatchPhraseQuery{
			"message": {Query: "failed to populate sourcemap metadata: failed to run initial search query: fetcher unavailable: server shutting down"},
		},
	}
	populateSourcemapFetcher403 = types.Query{
		MatchPhrasePrefix: map[string]types.MatchPhrasePrefixQuery{
			"message": {Query: "failed to populate sourcemap metadata: fetcher unavailable: 403 Forbidden:"},
		},
	}
)
